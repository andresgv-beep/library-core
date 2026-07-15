// media.go — la mitad LECTORA del contenido descargado (consumidor del sidecar).
//
// El generador (sidecar.go) escribe, al bajar algo, una ficha `<fichero>.json`
// junto al fichero. Este módulo hace lo contrario: escanea DOWNLOAD_ROOT, lee esas
// fichas y las expone al lector, y sirve los ficheros por streaming.
//
//	GET /api/media                    → todos los items del pool (rejilla del lector)
//	GET /api/media?collection=<rel>   → acotado a una carpeta-colección
//	GET /media/<ruta>                 → sirve el fichero con Range (vídeo/pdf/imagen)
//
// Así "cada descarga se muestra sola": bajas → sidecar → el escáner la ve →
// aparece en la web. Sin pasos manuales. Render propio (la UI decide plantilla
// por `template`); el shim solo da datos + bytes (DESIGN §2).
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// mediaDeps: raíz de descargas (misma que el download handler). Se inyecta desde main.
type mediaDeps struct {
	root string // DOWNLOAD_ROOT, absoluta y resuelta
}

// mediaItem = lo que ve el lector por cada item del pool. Espejo del
// sidecar + las rutas que la UI necesita para pintarlo y abrirlo.
type mediaItem struct {
	ID               string           `json:"id"`         // ruta relativa del sidecar (estable, única)
	Collection       string           `json:"collection"` // carpeta relativa ("Libros/Clásicos")
	Template         string           `json:"template"`   // video | pdf | gallery | audio | reader (por TIPO, no por superficie)
	Title            string           `json:"title"`
	Media            string           `json:"media"`     // nombre del fichero
	MediaURL         string           `json:"media_url"` // /media/<rel> — para <video>/<embed>/<img>
	Author           string           `json:"author,omitempty"`
	Date             string           `json:"date,omitempty"`
	Description      string           `json:"description,omitempty"`
	Tags             []string         `json:"tags,omitempty"`
	Source           string           `json:"source,omitempty"`
	SourceID         string           `json:"source_id,omitempty"`
	SourceURL        string           `json:"source_url,omitempty"`
	Language         string           `json:"language,omitempty"`
	Contributor      string           `json:"contributor,omitempty"`
	License          string           `json:"license,omitempty"`
	CoverURL         string           `json:"cover_url,omitempty"`          // /media/<rel> de la portada local
	TextURL          string           `json:"text_url,omitempty"`           // /media/<rel> del texto OCR local
	Tracks           []ItemTrack      `json:"tracks,omitempty"`             // pistas de audiolibro con URLs locales
	Duration         int              `json:"duration,omitempty"`           // segundos (vídeo)
	Subtitles        []mediaSub       `json:"subtitles,omitempty"`          // pistas .vtt con URL local
	Chapters         []sidecarChapter `json:"chapters,omitempty"`           // marcadores de tiempo
	ChannelAvatarURL string           `json:"channel_avatar_url,omitempty"` // /media/… imagen del canal
}

// mediaSub = una pista de subtítulos con su URL local servible.
type mediaSub struct {
	Lang string `json:"lang"`
	URL  string `json:"url"`
}

// registerMediaRoutes: rutas del contenido descargado, SIEMPRE detrás del gate de
// acceso (access.go). Es método de *Server (no de mediaDeps) precisamente porque
// necesita saber quién pregunta: mediaDeps solo conoce el disco.
//
// ⚠ handleMediaList/handleMediaFile son los handlers CRUDOS: no comprueban
// permisos. No los registres directamente en un mux.
func (s *Server) registerMediaRoutes(mux *http.ServeMux, m *mediaDeps) {
	mux.HandleFunc("/api/media", s.gateMediaList(m)) // lista (rejilla)
	mux.HandleFunc("/media/", s.gateMediaFile(m))    // streaming del fichero
}

// gateMediaList: la rejilla solo enseña las colecciones que el usuario puede ver.
func (s *Server) gateMediaList(m *mediaDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
			return
		}
		collection := strings.TrimSpace(r.URL.Query().Get("collection"))
		items, err := m.scan(collection)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		items = s.filterMediaItems(s.currentUser(r), items)
		if items == nil {
			items = []mediaItem{} // nunca null en el JSON
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

// gateMediaFile: los BYTES. Aquí es donde de verdad se cierra la puerta — de nada
// sirve esconder el item de la lista si luego /media/<ruta> lo sirve igual.
func (s *Server) gateMediaFile(m *mediaDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/media/")
		u := s.currentUser(r)
		// Gate de VER: sin esto no se sirve nada (streaming ni descarga).
		if rel != "" && !s.canSeeMediaPath(u, rel) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "sin acceso a esta colección"})
			return
		}
		// Intención de DESCARGA: ?dl=1. Es un permiso adicional al de ver — el
		// contenido puede ser público para reproducir/leer en la página pero exigir
		// cuenta para bajarse el fichero (salvo que el admin marque descarga anónima).
		// La cerradura vive AQUÍ, en el servidor: el cliente puede pedir dl=1, pero
		// quien decide es esto, no el botón de la UI.
		if r.URL.Query().Get("dl") == "1" {
			if rel != "" && !s.canDownloadMediaPath(u, rel) {
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error":           "regístrate para descargar este contenido",
					"loginToDownload": "true",
				})
				return
			}
			// Forzar descarga: el navegador guarda en vez de reproducir inline.
			w.Header().Set("Content-Disposition", "attachment; filename=\""+downloadFilename(rel)+"\"")
		}
		// H-2: el token de sesión puede viajar en ?st= (elementos nativos que no
		// mandan cabecera). no-referrer evita que se filtre por el header Referer
		// al navegar fuera. /content ya lo pone en setContentIsolation; aquí es
		// para /media.
		w.Header().Set("Referrer-Policy", "no-referrer")
		m.handleMediaFile(w, r)
	}
}

// downloadFilename saca un nombre de fichero seguro para el Content-Disposition
// (último segmento de la ruta, sin comillas que rompan la cabecera).
func downloadFilename(rel string) string {
	base := rel
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		base = rel[i+1:]
	}
	return strings.ReplaceAll(base, "\"", "")
}

// handleMediaList: handler CRUDO (sin permisos). Ver gateMediaList.
func (m *mediaDeps) handleMediaList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
		return
	}
	collection := strings.TrimSpace(r.URL.Query().Get("collection"))
	items, err := m.scan(collection)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if items == nil {
		items = []mediaItem{} // nunca null en el JSON
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// scan recorre DOWNLOAD_ROOT, lee cada sidecar `.json` (menos collection.json) y
// devuelve los items. Si `collection` no está vacío, acota a esa carpeta (o sus
// hijas). Un sidecar ilegible se salta sin romper el listado (best-effort).
func (m *mediaDeps) scan(collection string) ([]mediaItem, error) {
	if m.root == "" {
		return nil, fmt.Errorf("DOWNLOAD_ROOT no configurado")
	}
	collection = filepath.ToSlash(strings.Trim(collection, "/"))

	var out []mediaItem
	err := filepath.WalkDir(m.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // carpeta ilegible: sáltala, no abortes todo el escaneo
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == "collection.json" || !strings.HasSuffix(strings.ToLower(name), ".json") {
			return nil
		}

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var sc sidecar
		if json.Unmarshal(raw, &sc) != nil || sc.Media == "" {
			return nil // no es un sidecar válido (o le falta el fichero de media)
		}

		item := m.toItem(path, sc)
		if collection != "" && item.Collection != collection &&
			!strings.HasPrefix(item.Collection, collection+"/") {
			return nil
		}
		out = append(out, item)
		return nil
	})
	return out, err
}

// toItem construye el mediaItem a partir de la ruta del sidecar y su contenido.
func (m *mediaDeps) toItem(sidecarPath string, sc sidecar) mediaItem {
	dir := filepath.Dir(sidecarPath)
	relSidecar, _ := filepath.Rel(m.root, sidecarPath)
	relColl, _ := filepath.Rel(m.root, dir)
	relMedia, _ := filepath.Rel(m.root, filepath.Join(dir, sc.Media))

	coverURL := ""
	if sc.Cover != "" {
		relCover, _ := filepath.Rel(m.root, filepath.Join(dir, sc.Cover))
		coverURL = "/media/" + mediaURLPath(filepath.ToSlash(relCover))
	}

	mediaURLFor := func(name string) string {
		rel, _ := filepath.Rel(m.root, filepath.Join(dir, name))
		return "/media/" + mediaURLPath(filepath.ToSlash(rel))
	}
	var tracks []ItemTrack
	for _, tr := range sc.Tracks {
		it := ItemTrack{Title: tr.Title, URL: mediaURLFor(tr.Media)}
		if tr.Waveform != "" {
			it.Waveform = mediaURLFor(tr.Waveform)
		}
		tracks = append(tracks, it)
	}
	var subs []mediaSub
	for _, s := range sc.Subtitles {
		subs = append(subs, mediaSub{Lang: s.Lang, URL: mediaURLFor(s.File)})
	}
	// Imagen del canal: del sidecar, o (fallback) si hay un channel.jpg en la carpeta
	// aunque el sidecar sea viejo — así los vídeos ya importados también la muestran.
	channelAvatarURL := ""
	if sc.ChannelAvatar != "" {
		channelAvatarURL = mediaURLFor(sc.ChannelAvatar)
	} else if _, err := os.Stat(filepath.Join(dir, "channel.jpg")); err == nil {
		channelAvatarURL = mediaURLFor("channel.jpg")
	}

	textURL := ""
	if sc.Text != "" {
		textURL = mediaURLFor(sc.Text)
	}

	return mediaItem{
		ID:               filepath.ToSlash(relSidecar),
		Collection:       filepath.ToSlash(relColl),
		Template:         sc.Template,
		Title:            sc.Title,
		Media:            sc.Media,
		MediaURL:         "/media/" + mediaURLPath(filepath.ToSlash(relMedia)),
		Author:           sc.Author,
		Date:             sc.Date,
		Description:      sc.Description,
		Tags:             sc.Tags,
		Source:           sc.Source,
		SourceID:         sc.SourceID,
		SourceURL:        sc.SourceURL,
		Language:         sc.Language,
		Contributor:      sc.Contributor,
		License:          sc.License,
		CoverURL:         coverURL,
		TextURL:          textURL,
		Tracks:           tracks,
		Duration:         sc.Duration,
		Subtitles:        subs,
		Chapters:         sc.Chapters,
		ChannelAvatarURL: channelAvatarURL,
	}
}

// mediaURLPath escapa cada segmento de una ruta relativa (los nombres de IA
// llevan espacios/acentos) manteniendo las "/" como separadores.
func mediaURLPath(slashRel string) string {
	parts := strings.Split(slashRel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// handleMediaFile sirve el fichero bajado con soporte de Range (seek de vídeo,
// visor de PDF por páginas). Ancla la ruta a DOWNLOAD_ROOT (anti path-traversal,
// mismo criterio que el download handler §5).
func (m *mediaDeps) handleMediaFile(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/media/")
	if rel == "" {
		http.NotFound(w, r)
		return
	}
	abs, err := m.safeResolve(rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	// WebVTT necesita text/vtt explícito para que <track> lo acepte (en Windows la
	// extensión .vtt no está registrada → ServeContent lo serviría como text/plain).
	if strings.EqualFold(filepath.Ext(abs), ".vtt") {
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	}
	// ServeContent pone Content-Type por extensión y gestiona Range/If-Modified.
	http.ServeContent(w, r, filepath.Base(abs), info.ModTime(), f)
}

// safeResolve ancla `rel` a DOWNLOAD_ROOT y garantiza que no escapa (../..).
func (m *mediaDeps) safeResolve(rel string) (string, error) {
	if m.root == "" {
		return "", fmt.Errorf("DOWNLOAD_ROOT no configurado")
	}
	full := filepath.Clean(filepath.Join(m.root, filepath.FromSlash(rel)))
	root := strings.TrimRight(m.root, string(filepath.Separator))
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fmt.Errorf("ruta fuera de la carpeta permitida")
	}
	return full, nil
}

// handleMediaDelete borra un item descargado del pool: su ficha (.json) + el/los
// fichero(s) de media + la portada + (audiolibro) todas las pistas y ondas. Solo
// admin. `id` es el ID del item (la ruta relativa del sidecar). Best-effort:
// borra lo que encuentra listado en la ficha, dentro de la carpeta del item.
func (s *Server) handleMediaDelete(md *mediaDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
			return
		}
		me := s.currentUser(r)
		if me == nil || !me.IsAdmin {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "solo admin"})
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.ID) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falta id"})
			return
		}
		sidecarAbs, err := md.safeResolve(req.ID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		raw, err := os.ReadFile(sidecarAbs)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item no encontrado"})
			return
		}
		var sc sidecar
		if json.Unmarshal(raw, &sc) != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ficha ilegible"})
			return
		}
		dir := filepath.Dir(sidecarAbs)
		rm := func(name string) {
			if name == "" {
				return
			}
			p := filepath.Clean(filepath.Join(dir, name))
			if filepath.Dir(p) == dir { // solo dentro de la carpeta del item (anti-traversal)
				_ = os.Remove(p)
			}
		}
		rm(sc.Media)
		rm(sc.Cover)
		rm(sc.Text)
		for _, t := range sc.Tracks {
			rm(t.Media)
			rm(t.Waveform)
		}
		// Ficheros auxiliares del item: subtítulos .vtt y restos de descarga.
		for _, sub := range sc.Subtitles {
			rm(sub.File)
		}
		if sc.Media != "" {
			rm(sc.Media + ".part") // descarga a medias, por si quedó
		}
		_ = os.Remove(sidecarAbs)
		cleanEmptyCollectionDir(dir)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// cleanEmptyCollectionDir quita collection.json y la carpeta si ya no queda
// ningún otro item (sidecar) dentro. No borra si hay más contenido.
func cleanEmptyCollectionDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		n := strings.ToLower(e.Name())
		if strings.HasSuffix(n, ".json") && n != "collection.json" {
			return // aún hay otro item en esta carpeta
		}
	}
	_ = os.Remove(filepath.Join(dir, "collection.json"))
	_ = os.Remove(filepath.Join(dir, "channel.jpg")) // imagen del canal/autor, compartida
	_ = os.Remove(dir)                               // solo tiene efecto si la carpeta queda vacía
}
