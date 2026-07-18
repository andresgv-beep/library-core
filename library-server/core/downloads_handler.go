// downloads_handler.go — capa REST del shim sobre download.Manager.
//
// El Manager es genérico y no sabe nada de HTTP; este fichero traduce las
// peticiones REST de la UI a llamadas del Manager, valida el destino contra
// DOWNLOAD_ROOT (anti path-traversal, contrato §5) y devuelve JSON limpio.
//
// Rutas (contrato §3):
//
//	POST /api/downloads                 → encolar
//	GET  /api/downloads?owner=kiwix     → listar
//	POST /api/downloads/{id}/pause|resume|cancel
//
// El progreso NO va por aquí: la UI hace polling del GET (DESIGN: sin WebSocket).
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/andresgv-beep/nimos-library/download"
)

// downloadDeps es lo que el handler necesita del resto del server. Se inyecta
// desde main al construir el Server, para no acoplar este fichero a la struct
// completa. downloadRoot es la carpeta raíz permitida para escribir (pool
// concedido por NimOS); mgr es el motor ya inicializado.
type downloadDeps struct {
	mgr  *download.Manager
	root string // DOWNLOAD_ROOT, absoluta y ya resuelta (filepath.Abs)
}

// enqueueReq — cuerpo del POST /api/downloads (contrato §4).
type enqueueReq struct {
	URL       string `json:"url"`
	OwnerKind string `json:"owner_kind"`
	OwnerID   string `json:"owner_id"`
	DestDir   string `json:"dest_dir"`
	Filename  string `json:"filename"`
}

// registerDownloadRoutes engancha las rutas al mux. Llamar desde main junto al
// resto de mux.HandleFunc(...).
func (d *downloadDeps) registerDownloadRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/downloads", d.handleDownloads)   // POST (encolar) · GET (listar)
	mux.HandleFunc("/api/downloads/clear", d.handleClear) // POST — limpia historial terminado
	mux.HandleFunc("/api/downloads/", d.handleDownloadOp) // /{id}/pause|resume|cancel
}

func (d *downloadDeps) handleDownloads(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		d.handleList(w, r)
	case http.MethodPost:
		d.handleEnqueue(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
	}
}

func (d *downloadDeps) handleList(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	jobs, err := d.mgr.ListByOwner(owner) // owner "" → el Manager puede tratar como "todos" si se amplía; hoy filtra exacto
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if jobs == nil {
		jobs = []download.Job{} // nunca null en el JSON: la UI espera array
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (d *downloadDeps) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	var req enqueueReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido"})
		return
	}

	// Validar URL: solo http/https (contrato §5).
	u, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "URL inválida (solo http/https)"})
		return
	}

	// Anti-SSRF (auditoría H-3): la descarga manual acepta host arbitrario, así que
	// un admin comprometido podría alcanzar la red interna del NAS (loopback, metadata
	// del cloud, otros servicios). Rechazamos destinos privados/loopback ANTES de
	// encolar. Nota: el download.Manager sigue redirects; esto tapa el host inicial,
	// no cada salto — el cierre completo (CheckRedirect que revalide cada hop) queda
	// como mejora futura anotada.
	if !hostIsPublic(u.Hostname()) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "destino no permitido (apunta a una red interna)"})
		return
	}

	if req.OwnerKind == "" {
		req.OwnerKind = "manual"
	}

	// Nombre de fichero: el dado, o derivado del último segmento de la URL.
	name := sanitizeFilename(req.Filename)
	if name == "" {
		name = sanitizeFilename(filepath.Base(u.Path))
	}
	if name == "" || name == "." || name == "/" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no se pudo determinar el nombre de fichero"})
		return
	}

	// Anclar destino a DOWNLOAD_ROOT y verificar que no escapa (§5).
	dest, err := d.safeDest(req.DestDir, name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	job, err := d.mgr.Enqueue(u.String(), dest, req.OwnerKind, req.OwnerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// 201 si es nuevo; el Manager devuelve el existente si ya estaba (idempotente).
	writeJSON(w, http.StatusCreated, job)
}

// handleClear limpia el historial de descargas terminadas (done/error/cancelled).
func (d *downloadDeps) handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
		return
	}
	n, err := d.mgr.ClearFinished()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"cleared": n})
}

// handleDownloadOp maneja /api/downloads/{id}/{op}.
func (d *downloadDeps) handleDownloadOp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
		return
	}
	// path: /api/downloads/{id}/{op}
	rest := strings.TrimPrefix(r.URL.Path, "/api/downloads/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ruta inválida: se espera /{id}/{op}"})
		return
	}
	id, op := parts[0], parts[1]

	var err error
	switch op {
	case "pause":
		err = d.mgr.Pause(id)
	case "resume":
		err = d.mgr.Resume(id)
	case "cancel":
		err = d.mgr.Cancel(id)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "op desconocida: " + op})
		return
	}
	if err != nil {
		// El Manager devuelve error de "no activo"/"estado incompatible"/"no encontrado".
		// Sin tipos de error ricos aún, mapeamos a 409 (conflicto de estado) salvo pausa
		// de algo no activo, que es lo más común y también es 409.
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// safeDest ancla dir+name a la raíz y garantiza que la ruta final no escapa
// de DOWNLOAD_ROOT. Devuelve la ruta absoluta lista para el Manager.
func (d *downloadDeps) safeDest(dir, name string) (string, error) {
	if d.root == "" {
		return "", fmt.Errorf("DOWNLOAD_ROOT no configurado en el shim")
	}
	// Unir root+dir+name y limpiar. filepath.Join ya colapsa los ".." al vuelo,
	// así que si el resultado escapa de root, el prefijo NO cuadrará → rechazo.
	full := filepath.Clean(filepath.Join(d.root, dir, name))

	root := strings.TrimRight(d.root, string(filepath.Separator))
	rootWithSep := root + string(filepath.Separator)
	if full != root && !strings.HasPrefix(full, rootWithSep) {
		return "", fmt.Errorf("destino fuera de la carpeta permitida")
	}
	// El destino no puede ser la propia raíz (haría falta un nombre de fichero).
	if full == root {
		return "", fmt.Errorf("falta nombre de fichero en el destino")
	}
	return full, nil
}

// hostIsPublic resuelve el host y rechaza si CUALQUIER IP resuelta es privada,
// loopback, link-local o no especificada (anti-SSRF, H-3). Un host que no resuelve
// también se rechaza: si no podemos comprobarlo, no lo dejamos pasar. Se evalúa el
// host inicial; los redirects del Manager son una capa aparte (mejora anotada).
func hostIsPublic(host string) bool {
	if host == "" {
		return false
	}
	// Si el host ya es una IP literal, la comprobamos directa (sin DNS).
	if ip := net.ParseIP(host); ip != nil {
		return ipIsPublic(ip)
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !ipIsPublic(ip) {
			return false
		}
	}
	return true
}

func ipIsPublic(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast())
}

// sanitizeFilename quita separadores de ruta y caracteres peligrosos del nombre,
// dejando solo el nombre base. Complementa a safeDest (defensa en profundidad).
func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	// Nos quedamos solo con el último segmento, por si viene "a/b/c.pdf".
	name = filepath.Base(name)
	// Fuera separadores, caracteres reservados de Windows y controles. En
	// particular ':' impediría crear Alternate Data Streams (archivo:stream).
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return -1
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	return name
}
