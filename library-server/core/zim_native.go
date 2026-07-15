// zim_native.go — Camino NATIVO de /content/* (motor zim-engine, Go puro).
//
// FASE A del plan de retirada de kiwix (ZIM-ENGINE.md §5): detrás del toggle
// ZIM_ENGINE=native, /content/{zim}/{ruta} se sirve leyendo el .zim directamente
// del pool con el paquete zim/ — sin proxy, sin kiwixSem, sin salto HTTP. Ambos
// caminos conviven: rollback = ZIM_ENGINE=kiwix (default), sin redeploy.
//
// Separación de responsabilidades (§2): zim/ entrega bytes exactos y errores
// tipados; ESTE fichero pone la semántica HTTP (status, headers, redirects) y el
// gate de acceso queda donde estaba (handleContent → canSeeZim).

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andresgv-beep/zim-engine/zim"
)

// nativeZims: registro id público → zim.Archive abierto. El id es el que usa la
// UI en /content/{id}/… (el book name de kiwix); se resuelve contra library.xml,
// que sigue siendo la fuente de verdad del registro (§8: no cambia en Fase A).
type nativeZims struct {
	az   *adminZim
	mu   sync.Mutex
	open map[string]nativeArchive // id público → archive abierto (apertura perezosa)

	// Índices full-text (Fase C2, zim_fts.go): registro aparte con su propio
	// mutex — la búsqueda no debe serializarse con la apertura de archives.
	ftsMu  sync.Mutex
	fts    map[string]ftsState // id público → índice abierto (solo positivos)
	ftsErr map[string]string   // último error logueado por colección (dedup de log)
}

type nativeArchive struct {
	arc   zim.Archive
	path  string    // ruta del .zim en el pool (el índice .bleve vive al lado)
	mtime time.Time // mtime del .zim: Last-Modified (el archivo entero es un snapshot)
}

func newNativeZims(az *adminZim) *nativeZims {
	return &nativeZims{
		az:     az,
		open:   make(map[string]nativeArchive),
		fts:    make(map[string]ftsState),
		ftsErr: make(map[string]string),
	}
}

// get resuelve el id público a un archive abierto. Apertura perezosa con caché;
// el id casa contra el name del book o contra el nombre del fichero sin .zim
// (kiwix construye su book id desde name; el fallback cubre books sin name).
func (n *nativeZims) get(id string) (nativeArchive, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if a, ok := n.open[id]; ok {
		return a, nil
	}
	books, err := n.az.readLibrary()
	if err != nil {
		return nativeArchive{}, err
	}
	for _, b := range books {
		base := strings.TrimSuffix(filepath.Base(b.Path), filepath.Ext(b.Path))
		if b.Name != id && base != id && b.ID != id {
			continue
		}
		path := b.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(n.az.zimDir, filepath.Base(b.Path))
		}
		a, err := zim.Open(context.Background(), path, nil)
		if err != nil {
			return nativeArchive{}, err
		}
		na := nativeArchive{arc: a, path: path}
		if st, serr := os.Stat(path); serr == nil {
			na.mtime = st.ModTime()
		}
		n.open[id] = na
		log.Printf("zim nativo: %s abierto (%s, %d entradas)", id, filepath.Base(path), a.EntryCount())
		return na, nil
	}
	return nativeArchive{}, zim.ErrNotFound
}

// invalidate vacía el registro y cierra los archives que había abiertos: se llama
// cuando el Panel registra/desregistra un ZIM (admin_zim.go), para no seguir
// sirviendo una colección que ya no está en library.xml y para que una nueva se
// abra a la primera petición. El Close() de cada archive es GRACEFUL (§23: espera
// a los lectores en vuelo) y se hace FUERA del mutex del registro, para no
// bloquear el hot path mientras un lector drena.
func (n *nativeZims) invalidate() {
	n.closeFTS() // los índices FTS primero (apuntan a los archives)
	n.mu.Lock()
	old := n.open
	n.open = make(map[string]nativeArchive)
	n.mu.Unlock()
	for _, a := range old {
		a.arc.Close()
	}
}

// closeAll es invalidate sin repoblar: cierre del shim.
func (n *nativeZims) closeAll() { n.invalidate() }

// registeredCount cuenta las colecciones de library.xml (para /api/health).
func (n *nativeZims) registeredCount() (int, error) {
	books, err := n.az.readLibrary()
	if err != nil {
		return 0, err
	}
	return len(books), nil
}

// ── Handler ────────────────────────────────────────────────────────────────

// serveZimNative sirve GET/HEAD /content/{id}/{ruta} desde el motor nativo.
func (s *Server) serveZimNative(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo GET/HEAD"})
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/content/")
	id, sub, _ := strings.Cut(rest, "/")
	if id == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "colección no indicada"})
		return
	}
	na, err := s.zimNative.get(id)
	if err != nil {
		s.zimHTTPError(w, err, "colección no registrada: "+id)
		return
	}
	arc := na.arc

	// Raíz de la colección → 302 a la portada (mismo contrato que kiwix: la UI
	// enlaza /content/{id}/ y espera aterrizar en el main page).
	if sub == "" {
		mp, err := arc.MainPage()
		if err != nil {
			s.zimHTTPError(w, err, "la colección no declara portada")
			return
		}
		http.Redirect(w, r, zimContentURL(id, mp.Key()), http.StatusFound)
		return
	}

	e, err := zimResolveEntry(arc, sub)
	if err != nil {
		s.zimHTTPError(w, err, "")
		return
	}

	// Redirect del ZIM → 302 HTTP, para que las rutas relativas del artículo
	// destino resuelvan contra SU URL (igual que hace kiwix).
	if e.IsRedirect() {
		if tgt, ok := e.RedirectTarget(); ok {
			http.Redirect(w, r, zimContentURL(id, tgt), http.StatusFound)
			return
		}
		s.zimHTTPError(w, zim.ErrCorrupt, "redirect irresoluble")
		return
	}

	rc, info, err := e.Open(r.Context())
	if err != nil {
		s.zimHTTPError(w, err, "")
		return
	}
	defer rc.Close()

	h := w.Header()
	if info.MIME != "" {
		h.Set("Content-Type", info.MIME)
	}
	// ETag estable por blob sin hashear contenido (§5.3): el contenido de un ZIM
	// es inmutable mientras el fichero no cambie, y el UUID cambia con él.
	etag := zimETag(arc.UUID(), info)
	h.Set("ETag", etag)
	h.Set("Cache-Control", "public, max-age=86400")

	// Nivel 1 (§18): el motor expone Seek en blobs sin comprimir o materializados
	// en RAM → http.ServeContent regala Range/If-Range/206/416/304/Last-Modified
	// correctos sin copiar nada.
	if rs, ok := rc.(io.ReadSeeker); ok {
		http.ServeContent(w, r, "", na.mtime, rs)
		return
	}

	// Nivel 2: blob comprimido servido en streaming. Si piden Range y cabe en el
	// tope, se materializa UNA vez y ServeContent hace el resto (el vídeo real
	// vive en clusters sin comprimir, así que este camino es raro).
	if r.Header.Get("Range") != "" && info.Size <= zimRangeMaxBytes {
		data, err := io.ReadAll(rc)
		if err != nil {
			s.zimHTTPError(w, err, "")
			return
		}
		http.ServeContent(w, r, "", na.mtime, bytesReadSeeker(data))
		return
	}

	// Sin Range (o blob comprimido gigante: el Range se ignora → 200 completo,
	// permitido por RFC 7233): respuesta entera en streaming.
	h.Set("Content-Length", strconv.FormatInt(info.Size, 10))
	if !na.mtime.IsZero() {
		h.Set("Last-Modified", na.mtime.UTC().Format(http.TimeFormat))
	}
	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(w, rc); err != nil && !errors.Is(err, context.Canceled) {
		// Cabeceras ya emitidas: no se puede cambiar el status. Solo log.
		log.Printf("zim nativo: corte sirviendo %s/%s: %v", id, sub, err)
	}
}

// zimRangeMaxBytes: tope de materialización para servir Range sobre un blob
// comprimido (nivel 2 §18). Por encima, el Range se ignora y se sirve entero.
const zimRangeMaxBytes = 32 << 20

func bytesReadSeeker(b []byte) io.ReadSeeker { return bytes.NewReader(b) }

// zimResolveEntry mapea la ruta de la URL a una entrada: primero el esquema
// moderno (todo vive en 'C', la URL no lleva namespace), luego el legacy
// ("A/Saturno" ⇒ ns 'A'), igual que resuelve kiwix. La URL de la UI no cambia.
func zimResolveEntry(arc zim.Archive, sub string) (zim.Entry, error) {
	e, err := arc.EntryAt(zim.EntryKey{Namespace: 'C', Path: sub})
	if err == nil || !errors.Is(err, zim.ErrNotFound) {
		return e, err
	}
	if len(sub) > 2 && sub[1] == '/' {
		return arc.EntryAt(zim.EntryKey{Namespace: sub[0], Path: sub[2:]})
	}
	return nil, err
}

// zimEntryPath: forma de la ruta en la URL — las entradas modernas ('C') van sin
// namespace; el resto ("A/…", "I/…" legacy) lo llevan delante. Sin escapar (es la
// misma forma que devuelve el /suggest de kiwix en su campo "path").
func zimEntryPath(k zim.EntryKey) string {
	if k.Namespace == 'C' {
		return k.Path
	}
	return string(k.Namespace) + "/" + k.Path
}

// zimContentURL construye la URL pública de una entrada con el mismo esquema que
// kiwix.
func zimContentURL(id string, k zim.EntryKey) string {
	return "/content/" + escapePath(id) + "/" + escapePath(zimEntryPath(k))
}

// ── Suggest / catálogo: costura común de backend (Fase B, §6) ────────────────

// suggestResult: forma neutra que consumen handleSuggest (autocompletado) y
// suggestHits (sembrado de la búsqueda global), sea cual sea el backend. Calcada
// al /suggest de kiwix (label, kind, path) para que el resto del shim no cambie.
type suggestResult struct {
	Label string
	Kind  string
	Path  string
}

// suggestBackend devuelve candidatos por título de una colección. Con el motor
// nativo tira del TitleIndex propio (§6, normalización §21); sin él, del /suggest
// de kiwix. La lógica de arriba (variantes de typo, scoring, dedup, gate, caché)
// es idéntica en ambos caminos.
func (s *Server) suggestBackend(libID, term string, limit int) ([]suggestResult, error) {
	if s.zimNative != nil {
		return s.nativeSuggest(libID, term, limit)
	}
	return s.kiwixSuggest(libID, term)
}

func (s *Server) nativeSuggest(libID, term string, limit int) ([]suggestResult, error) {
	na, err := s.zimNative.get(libID)
	if err != nil {
		return nil, err
	}
	ti, err := na.arc.TitleIndex()
	if err != nil {
		return nil, err
	}
	// Se piden 2× candidatos porque los redirects colapsan: un artículo popular
	// tiene N variantes de título ("Micheal jackson", "Michael jackson"…) que
	// tras resolver quedan en UNA.
	keys, err := ti.Search(term, limit*2)
	if err != nil {
		return nil, err
	}
	out := make([]suggestResult, 0, min(len(keys), limit))
	seen := make(map[string]bool, len(keys))
	for _, k := range keys {
		e, err := na.arc.EntryAt(k)
		if err != nil {
			continue
		}
		// Resolver redirects al DESTINO: el redirect sirve para ENCONTRAR (su
		// título casa con lo que teclea el humano), pero lo que se muestra y
		// dedupe es el artículo canónico — sin esto, el mismo artículo sale N
		// veces con títulos casi iguales (visto en vivo con "michale jack").
		for i := 0; i < 4 && e.IsRedirect(); i++ {
			tgt, ok := e.RedirectTarget()
			if !ok {
				break
			}
			te, terr := na.arc.EntryAt(tgt)
			if terr != nil {
				break
			}
			e = te
		}
		if e.IsRedirect() {
			continue // cadena sin resolver: mejor fuera que duplicado
		}
		p := zimEntryPath(e.Key())
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, suggestResult{Label: e.Title(), Kind: "path", Path: p})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *Server) kiwixSuggest(libID, term string) ([]suggestResult, error) {
	resp, err := s.kget(s.kiwix.String() + "/suggest?content=" + urlq(libID) + "&term=" + urlq(term))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var raw []struct {
		Label string `json:"label"`
		Kind  string `json:"kind"`
		Path  string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]suggestResult, 0, len(raw))
	for _, r := range raw {
		out = append(out, suggestResult{Label: r.Label, Kind: r.Kind, Path: r.Path})
	}
	return out, nil
}

func zimETag(uuid [16]byte, info zim.BlobInfo) string {
	return fmt.Sprintf("\"%x-%d-%d\"", uuid, info.ClusterNumber, info.BlobNumber)
}

// zimHTTPError traduce los errores tipados del motor a HTTP (§2.1). El motor no
// conoce HTTP; esta tabla es del handler.
func (s *Server) zimHTTPError(w http.ResponseWriter, err error, msg string) {
	if errors.Is(err, context.Canceled) {
		return // el cliente abortó: no hay nada que responder
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, zim.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, zim.ErrCorrupt),
		errors.Is(err, zim.ErrUnsupportedCompression),
		errors.Is(err, zim.ErrUnsupportedVersion),
		errors.Is(err, zim.ErrRedirectCycle):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, zim.ErrResourceLimit), errors.Is(err, zim.ErrClosed):
		status = http.StatusServiceUnavailable
	case errors.Is(err, os.ErrNotExist):
		status = http.StatusNotFound
	}
	if msg == "" {
		msg = err.Error()
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

// ── Catálogo nativo (Fase B, §5.2 punto 3) ──────────────────────────────────

// nativeLibraries construye el catálogo leyendo la metadata M/* de cada ZIM
// registrado, sin OPDS de kiwix. El id público es el nombre de fichero sin .zim
// (idéntico al que emite kiwix → las URLs y los favoritos/historial no cambian).
func (s *Server) nativeLibraries() ([]Library, error) {
	books, err := s.zimNative.az.readLibrary()
	if err != nil {
		return nil, err
	}
	out := make([]Library, 0, len(books))
	for _, b := range books {
		id := strings.TrimSuffix(filepath.Base(b.Path), filepath.Ext(b.Path))
		na, err := s.zimNative.get(id)
		if err != nil {
			continue // fichero ausente o ilegible: se omite del catálogo, no rompe
		}
		arc := na.arc
		lib := Library{
			ID:          id,
			Name:        zimMeta(arc, "Title", id),
			Description: zimMeta(arc, "Description", ""),
			Lang:        zimMeta(arc, "Language", ""),
			Date:        dateOnly(zimMeta(arc, "Date", "")),
			Articles:    arc.ArticleCount(),
		}
		if st, serr := os.Stat(zimPath(s.zimNative.az, b.Path)); serr == nil {
			lib.Size = st.Size()
		}
		lib.Media = zimMediaCount(zimMeta(arc, "Counter", ""))
		if _, e := arc.EntryAt(zim.EntryKey{Namespace: 'M', Path: "Illustration_48x48@1"}); e == nil {
			lib.Icon = "/content/" + escapePath(id) + "/M/Illustration_48x48@1"
		}
		out = append(out, lib)
	}
	return out, nil
}

func zimPath(az *adminZim, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(az.zimDir, filepath.Base(p))
}

// zimMeta lee M/<name>, con un valor por defecto si falta o falla.
func zimMeta(arc zim.Archive, name, def string) string {
	if v, err := arc.Metadata(name); err == nil && v != "" {
		return v
	}
	return def
}

// zimMediaCount suma las entradas de imagen/vídeo/audio del M/Counter
// ("mime=count;mime=count;…"), el mismo dato que kiwix expone como mediaCount.
func zimMediaCount(counter string) int {
	total := 0
	for _, part := range strings.Split(counter, ";") {
		mime, cnt, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		mime = strings.TrimSpace(mime)
		if strings.HasPrefix(mime, "image/") || strings.HasPrefix(mime, "video/") || strings.HasPrefix(mime, "audio/") {
			if n, err := strconv.Atoi(strings.TrimSpace(cnt)); err == nil {
				total += n
			}
		}
	}
	return total
}

// setContentIsolation aplica los headers §19 a TODO /content/* (nativo Y proxy
// kiwix: el riesgo existe hoy, no espera al motor). script-src 'none' mata el
// vector real: un ZIM con JS hostil corriendo en el navegador del admin y
// haciendo fetch a /api/admin/* con su cookie. connect-src 'none' además corta
// cualquier conexión a Internet desde contenido offline.
//
// ENMIENDA práctica sobre el §19 del doc: style-src/img-src/font-src/media-src
// van explícitos con 'unsafe-inline' y data: — los artículos ZIM llevan <style>
// inline y data-URIs por todas partes, y con el default-src 'self' pelado se
// romperían en el render. El vector que importa (script/connect) sigue en none.
func setContentIsolation(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Security-Policy",
		"default-src 'self'; script-src 'none'; connect-src 'none'; frame-src 'none'; object-src 'none'; "+
			"style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; media-src 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
}
