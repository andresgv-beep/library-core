// Package download implementa un motor de descargas resumable y persistente
// para NimOS. Diseñado para ser genérico: el catálogo ZIM es el primer
// consumidor, pero sirve igual para YouPlayer o actualizaciones de AppStore.
//
// Principios de diseño:
//   - Persistencia en SQLite: un job sobrevive a un reinicio del daemon.
//   - Resume real vía HTTP Range: si hay X bytes en disco, pide el resto.
//   - Escritura atómica: se descarga a "<dest>.part" y solo se renombra al
//     terminar. Nunca queda un fichero "completo" a medias.
//   - Cola con concurrencia limitada: evita saturar un Pi con 5 descargas
//     simultáneas de 300MB.
package download

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ---------------------------------------------------------------------
// Tipos y estado
// ---------------------------------------------------------------------

type JobStatus string

const (
	StatusQueued      JobStatus = "queued"
	StatusDownloading JobStatus = "downloading"
	StatusPaused      JobStatus = "paused"
	StatusDone        JobStatus = "done"
	StatusError       JobStatus = "error"
	StatusCancelled   JobStatus = "cancelled"
)

// stallTimeout: si no llegan bytes nuevos en este tiempo, se aborta la
// descarga (socket muerto). El job cae a 'error' y es resumable. Evita que
// una conexión colgada retenga un slot del semáforo para siempre.
const stallTimeout = 60 * time.Second

// Job representa una descarga individual. OwnerKind/OwnerID permiten que
// cualquier módulo (kiwix, manual…) identifique de quién es
// el job sin que este paquete sepa nada de esos módulos.
type Job struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	DestPath     string    `json:"dest_path"`
	OwnerKind    string    `json:"owner_kind"` // "kiwix", "manual"...
	OwnerID      string    `json:"owner_id"`   // identificador del item en su proveedor
	Status       JobStatus `json:"status"`
	TotalBytes   int64     `json:"total_bytes"`
	WrittenBytes int64     `json:"written_bytes"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
	CreatedAt    int64     `json:"created_at"`
	UpdatedAt    int64     `json:"updated_at"`
}

func (j *Job) Progress() float64 {
	if j.TotalBytes <= 0 {
		return 0
	}
	return float64(j.WrittenBytes) / float64(j.TotalBytes)
}

// ProgressFunc se llama periódicamente durante la descarga. El frontend
// puede engancharse a esto vía el mismo patrón de polling por topic que
// ya usan los widgets (refcount), sin necesidad de WebSockets.
type ProgressFunc func(job Job)

// ---------------------------------------------------------------------
// Manager
// ---------------------------------------------------------------------

type Manager struct {
	db          *sql.DB
	client      *http.Client
	maxParallel int

	mu      sync.Mutex
	active  map[string]context.CancelFunc // jobID -> cancel de esa descarga
	onEvent ProgressFunc

	sem chan struct{} // limita concurrencia
}

func NewManager(db *sql.DB, maxParallel int, onEvent ProgressFunc) (*Manager, error) {
	if maxParallel < 1 {
		maxParallel = 2
	}
	// SQLite: un solo escritor. Sin esto, varios upsert() concurrentes desde
	// descargas paralelas dan "database is locked". WAL + límite de conexiones.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		return nil, fmt.Errorf("download: pragma: %w", err)
	}
	db.SetMaxOpenConns(1)

	m := &Manager{
		db:          db,
		client:      &http.Client{Timeout: 0}, // sin timeout global; controlamos con deadline por stall
		maxParallel: maxParallel,
		active:      make(map[string]context.CancelFunc),
		onEvent:     onEvent,
		sem:         make(chan struct{}, maxParallel),
	}
	if err := m.migrate(); err != nil {
		return nil, fmt.Errorf("download: migrate schema: %w", err)
	}
	return m, nil
}

func (m *Manager) migrate() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS download_jobs (
			id            TEXT PRIMARY KEY,
			url           TEXT NOT NULL,
			dest_path     TEXT NOT NULL,
			owner_kind    TEXT NOT NULL,
			owner_id      TEXT NOT NULL,
			status        TEXT NOT NULL,
			total_bytes   INTEGER NOT NULL DEFAULT 0,
			written_bytes INTEGER NOT NULL DEFAULT 0,
			error_msg     TEXT NOT NULL DEFAULT '',
			created_at    INTEGER NOT NULL,
			updated_at    INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_download_jobs_owner
			ON download_jobs(owner_kind, owner_id);
	`)
	return err
}

// jobID genera un ID estable y único por (owner, url). Usar la URL (no el
// timestamp) evita colisiones cuando se encolan varios ficheros del mismo
// mismo item dentro del mismo segundo, y hace el encolado idempotente:
// re-encolar la misma descarga cae en el mismo ID en vez de duplicar.
func jobID(ownerKind, ownerID, url string) string {
	sum := sha1.Sum([]byte(url))
	return fmt.Sprintf("%s-%s-%x", ownerKind, ownerID, sum[:6])
}

// Enqueue crea un job nuevo (o devuelve el existente si ya hay uno para
// ese owner+URL en estado no terminal, para evitar duplicados).
func (m *Manager) Enqueue(url, destPath, ownerKind, ownerID string) (*Job, error) {
	existing, err := m.findActiveByOwner(ownerKind, ownerID, url)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	now := time.Now().Unix()
	job := &Job{
		ID:        jobID(ownerKind, ownerID, url),
		URL:       url,
		DestPath:  destPath,
		OwnerKind: ownerKind,
		OwnerID:   ownerID,
		Status:    StatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := m.upsert(job); err != nil {
		return nil, err
	}
	go m.runWhenSlotFree(job.ID)
	return job, nil
}

func (m *Manager) findActiveByOwner(ownerKind, ownerID, url string) (*Job, error) {
	row := m.db.QueryRow(`
		SELECT id, url, dest_path, owner_kind, owner_id, status,
		       total_bytes, written_bytes, error_msg, created_at, updated_at
		FROM download_jobs
		WHERE owner_kind = ? AND owner_id = ? AND url = ?
		  AND status IN ('queued','downloading','paused')
		LIMIT 1`, ownerKind, ownerID, url)
	j, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return j, err
}

func scanJob(row *sql.Row) (*Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.URL, &j.DestPath, &j.OwnerKind, &j.OwnerID,
		&j.Status, &j.TotalBytes, &j.WrittenBytes, &j.ErrorMsg,
		&j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (m *Manager) upsert(j *Job) error {
	j.UpdatedAt = time.Now().Unix()
	_, err := m.db.Exec(`
		INSERT INTO download_jobs
			(id, url, dest_path, owner_kind, owner_id, status,
			 total_bytes, written_bytes, error_msg, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status,
			total_bytes=excluded.total_bytes,
			written_bytes=excluded.written_bytes,
			error_msg=excluded.error_msg,
			updated_at=excluded.updated_at`,
		j.ID, j.URL, j.DestPath, j.OwnerKind, j.OwnerID, j.Status,
		j.TotalBytes, j.WrittenBytes, j.ErrorMsg, j.CreatedAt, j.UpdatedAt)
	return err
}

// ---------------------------------------------------------------------
// Ejecución
// ---------------------------------------------------------------------

// runWhenSlotFree bloquea hasta que hay hueco en el semáforo de concurrencia
// y entonces arranca la descarga. Se lanza en su propia goroutine desde Enqueue.
func (m *Manager) runWhenSlotFree(jobID string) {
	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	job, err := m.getByID(jobID)
	if err != nil || job == nil {
		return
	}
	if job.Status == StatusCancelled || job.Status == StatusDone {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.active[jobID] = cancel
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.active, jobID)
		m.mu.Unlock()
	}()

	m.download(ctx, job)
}

func (m *Manager) getByID(id string) (*Job, error) {
	row := m.db.QueryRow(`
		SELECT id, url, dest_path, owner_kind, owner_id, status,
		       total_bytes, written_bytes, error_msg, created_at, updated_at
		FROM download_jobs WHERE id = ?`, id)
	j, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return j, err
}

// Pause detiene una descarga en curso sin borrar el .part. Se puede
// reanudar después con Resume.
func (m *Manager) Pause(jobID string) error {
	m.mu.Lock()
	cancel, ok := m.active[jobID]
	m.mu.Unlock()
	if !ok {
		return errors.New("download: job no está activo")
	}
	cancel() // el loop de download() detecta ctx.Err() y marca 'paused'
	return nil
}

// Resume vuelve a encolar un job pausado o con error.
func (m *Manager) Resume(jobID string) error {
	job, err := m.getByID(jobID)
	if err != nil || job == nil {
		return fmt.Errorf("download: job %s no encontrado", jobID)
	}
	if job.Status != StatusPaused && job.Status != StatusError {
		return fmt.Errorf("download: job %s no está pausado ni en error (status=%s)", jobID, job.Status)
	}
	job.Status = StatusQueued
	if err := m.upsert(job); err != nil {
		return err
	}
	go m.runWhenSlotFree(job.ID)
	return nil
}

// Cancel detiene la descarga y borra el fichero parcial.
func (m *Manager) Cancel(jobID string) error {
	m.mu.Lock()
	cancel, ok := m.active[jobID]
	m.mu.Unlock()
	if ok {
		cancel()
	}
	job, err := m.getByID(jobID)
	if err != nil || job == nil {
		return err
	}
	job.Status = StatusCancelled
	_ = os.Remove(partPath(job.DestPath))
	return m.upsert(job)
}

// download hace el trabajo real: GET con Range si ya hay bytes en disco,
// escribiendo a "<dest>.part" y renombrando atómicamente al terminar.
func (m *Manager) download(ctx context.Context, job *Job) {
	partFile := partPath(job.DestPath)

	// ¿Cuánto llevamos ya descargado? (soporte de resume real)
	var startAt int64
	if info, err := os.Stat(partFile); err == nil {
		startAt = info.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, job.URL, nil)
	if err != nil {
		m.fail(job, err)
		return
	}
	if startAt > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startAt))
	}

	resp, err := m.client.Do(req)
	if err != nil {
		m.pauseOnCancel(ctx, job, err)
		return
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// El servidor no soporta Range o decidió mandar todo: empezamos de cero.
		startAt = 0
		if err := os.Remove(partFile); err != nil && !os.IsNotExist(err) {
			m.fail(job, err)
			return
		}
	case http.StatusPartialContent:
		// Continúa donde estaba. resp.ContentLength es SOLO el resto.
	case http.StatusRequestedRangeNotSatisfiable:
		// 416: el .part ya tiene (al menos) todo el fichero. Pasó si crasheamos
		// justo antes del Rename. Cerramos atómicamente y marcamos 'done' en vez
		// de fallar para siempre.
		if m.finalizeIfComplete(job, partFile) {
			return
		}
		// Si no cuadra el tamaño, el .part está corrupto: reintentamos de cero.
		_ = os.Remove(partFile)
		m.fail(job, fmt.Errorf("rango no satisfacible y .part inconsistente; reintenta"))
		return
	default:
		m.fail(job, fmt.Errorf("servidor devolvió status %d", resp.StatusCode))
		return
	}

	total := startAt + resp.ContentLength
	if resp.ContentLength <= 0 {
		total = job.TotalBytes // servidor no dio Content-Length; conserva lo que ya sabíamos
	}
	job.TotalBytes = total
	job.Status = StatusDownloading
	job.WrittenBytes = startAt
	job.ErrorMsg = "" // este intento arrancó bien: limpia el error de un intento previo
	_ = m.upsert(job)
	m.emit(*job)

	if err := os.MkdirAll(filepath.Dir(partFile), 0o755); err != nil {
		m.fail(job, err)
		return
	}

	flag := os.O_CREATE | os.O_WRONLY
	if startAt > 0 {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	out, err := os.OpenFile(partFile, flag, 0o644)
	if err != nil {
		m.fail(job, err)
		return
	}
	defer out.Close()

	written, copyErr := copyWithProgress(ctx, out, resp.Body, startAt, job, m)
	job.WrittenBytes = written

	if copyErr != nil {
		m.pauseOnCancel(ctx, job, copyErr)
		return
	}

	// Durabilidad: forzar los bytes a disco ANTES del rename. Sin esto, un
	// corte de luce tras el Rename puede dejar el fichero final con huecos.
	if err := out.Sync(); err != nil {
		m.fail(job, fmt.Errorf("sync a disco: %w", err))
		return
	}
	out.Close()

	// Descarga completa: cierre atómico moviendo .part -> destino final.
	if err := os.Rename(partFile, job.DestPath); err != nil {
		m.fail(job, fmt.Errorf("no se pudo mover fichero final: %w", err))
		return
	}
	job.Status = StatusDone
	_ = m.upsert(job)
	m.emit(*job)
}

// finalizeIfComplete comprueba si el .part ya está entero (usando el tamaño
// que conocíamos) y, si es así, lo renombra al destino y marca 'done'. Se usa
// al recibir un 416 tras un crash previo al Rename.
func (m *Manager) finalizeIfComplete(job *Job, partFile string) bool {
	info, err := os.Stat(partFile)
	if err != nil {
		return false
	}
	if job.TotalBytes <= 0 || info.Size() < job.TotalBytes {
		return false
	}
	if err := os.Rename(partFile, job.DestPath); err != nil {
		m.fail(job, fmt.Errorf("finalize tras 416: %w", err))
		return true
	}
	job.WrittenBytes = info.Size()
	job.Status = StatusDone
	_ = m.upsert(job)
	m.emit(*job)
	return true
}

// copyWithProgress copia el body al fichero, actualizando WrittenBytes y
// emitiendo progreso cada ~1s (no en cada chunk, para no saturar SQLite).
// Si pasan stallTimeout sin recibir bytes, aborta (socket muerto).
func copyWithProgress(ctx context.Context, dst *os.File, src io.Reader, startAt int64, job *Job, m *Manager) (int64, error) {
	buf := make([]byte, 256*1024)
	written := startAt
	lastEmit := time.Now()
	lastData := time.Now()

	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		if time.Since(lastData) > stallTimeout {
			return written, fmt.Errorf("descarga estancada: sin datos en %s", stallTimeout)
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			lastData = time.Now()
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return written, writeErr
			}
			written += int64(n)
			if time.Since(lastEmit) > time.Second {
				job.WrittenBytes = written
				_ = m.upsert(job)
				m.emit(*job)
				lastEmit = time.Now()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}

func (m *Manager) pauseOnCancel(ctx context.Context, job *Job, err error) {
	if errors.Is(ctx.Err(), context.Canceled) {
		job.Status = StatusPaused
		job.ErrorMsg = ""
	} else {
		job.Status = StatusError
		job.ErrorMsg = err.Error()
	}
	_ = m.upsert(job)
	m.emit(*job)
}

func (m *Manager) fail(job *Job, err error) {
	job.Status = StatusError
	job.ErrorMsg = err.Error()
	_ = m.upsert(job)
	m.emit(*job)
}

func (m *Manager) emit(job Job) {
	if m.onEvent != nil {
		m.onEvent(job)
	}
}

func partPath(dest string) string {
	return dest + ".part"
}

// ListByOwner permite a un módulo pedir el estado de todos
// sus jobs para pintar el panel de cola sin tener que trackear IDs sueltos.
func (m *Manager) ListByOwner(ownerKind string) ([]Job, error) {
	rows, err := m.db.Query(`
		SELECT id, url, dest_path, owner_kind, owner_id, status,
		       total_bytes, written_bytes, error_msg, created_at, updated_at
		FROM download_jobs WHERE (? = '' OR owner_kind = ?)
		ORDER BY created_at DESC`, ownerKind, ownerKind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.URL, &j.DestPath, &j.OwnerKind, &j.OwnerID,
			&j.Status, &j.TotalBytes, &j.WrittenBytes, &j.ErrorMsg,
			&j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ClearFinished borra del historial las descargas ya terminadas (done, error o
// cancelled), dejando solo las activas (queued/downloading/paused). Devuelve
// cuántas quitó. Los ficheros en disco NO se tocan.
func (m *Manager) ClearFinished() (int, error) {
	res, err := m.db.Exec(`DELETE FROM download_jobs WHERE status IN ('done','error','cancelled')`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ResumeIncomplete se llama una vez al arrancar el daemon: recoge todo lo que
// quedó a medias de una sesión anterior (crash o reboot) y lo vuelve a encolar.
// Incluye 'downloading' (interrumpido en pleno vuelo) y 'queued' (nunca llegó
// a arrancar). Los marca 'paused' primero para que Resume los acepte.
func (m *Manager) ResumeIncomplete() error {
	rows, err := m.db.Query(
		`SELECT id FROM download_jobs WHERE status IN (?, ?)`,
		StatusDownloading, StatusQueued)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		// Normaliza a 'paused' para que Resume no lo rechace.
		if job, err := m.getByID(id); err == nil && job != nil {
			job.Status = StatusPaused
			_ = m.upsert(job)
		}
		if err := m.Resume(id); err != nil {
			// No abortamos todo el arranque por un job suelto; solo lo logueamos.
			fmt.Printf("download: no se pudo reanudar job %s: %v\n", id, err)
		}
	}
	return nil
}
