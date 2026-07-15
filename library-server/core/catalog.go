package main

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
)

// ─── Colecciones: OPDS (XML) → JSON limpio ────────────────────────────────────
// La UI solo conoce "colecciones", nunca ficheros .zim (DESIGN §3).

type opdsFeed struct {
	Entries []opdsEntry `xml:"entry"`
}

type opdsEntry struct {
	Title    string     `xml:"title"`
	Name     string     `xml:"name"` // book name (el {ZIM} de kiwix)
	Summary  string     `xml:"summary"`
	Language string     `xml:"language"`
	Articles int        `xml:"articleCount"`
	Media    int        `xml:"mediaCount"`
	Updated  string     `xml:"updated"`
	Links    []opdsLink `xml:"link"`
}

type opdsLink struct {
	Rel    string `xml:"rel,attr"`
	Href   string `xml:"href,attr"`
	Type   string `xml:"type,attr"`
	Length int64  `xml:"length,attr"`
}

// Library es el contrato JSON que consume la UI (DESIGN §7).
type Library struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Lang        string `json:"lang,omitempty"`
	Articles    int    `json:"articles,omitempty"`
	Media       int    `json:"media,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Date        string `json:"date,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

func (s *Server) handleLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := s.visibleLibs(s.currentUser(r)) // filtra por acceso/edad
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, libs)
}

// fetchLibraries obtiene y normaliza el catálogo (reutilizado por búsqueda global).
// Con el motor nativo (ZIM_ENGINE=native) sale de la metadata M/* de cada ZIM
// (Fase B); si no, del OPDS de kiwix.
func (s *Server) fetchLibraries() ([]Library, error) {
	if s.zimNative != nil {
		return s.nativeLibraries()
	}
	// count=-1 = sin límite; kiwix pagina a 10 por defecto (si no, solo veríamos 10 colecciones).
	resp, err := s.kget(s.kiwix.String() + "/catalog/v2/entries?count=-1")
	if err != nil {
		return nil, fmt.Errorf("motor no disponible")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var feed opdsFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("OPDS ilegible")
	}

	libs := make([]Library, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		lib := Library{
			Name:        e.Title,
			Description: strings.TrimSpace(e.Summary),
			Lang:        e.Language,
			Articles:    e.Articles,
			Media:       e.Media,
			Date:        dateOnly(e.Updated),
		}
		for _, l := range e.Links {
			switch {
			// El identificador de URL real (para /content y /suggest) viene del
			// link text/html, NO del <name> (que es el metadato interno del ZIM).
			case l.Type == "text/html" && strings.HasPrefix(l.Href, "/content/"):
				lib.ID = strings.Trim(strings.TrimPrefix(l.Href, "/content/"), "/")
			case strings.Contains(l.Rel, "acquisition") && l.Length > 0:
				lib.Size = l.Length
			case strings.Contains(l.Rel, "image/thumbnail") || strings.Contains(l.Rel, "image"):
				lib.Icon = l.Href // ruta a la ilustración
			}
		}
		if lib.ID == "" {
			lib.ID = e.Name // fallback al metadato si no hubiera link de contenido
		}
		if lib.ID == "" {
			continue // sin identificador servible
		}
		libs = append(libs, lib)
	}
	return libs, nil
}

// ─── Sub-rutas de una colección: /api/libraries/{id}/search|fulltext ──────────

func (s *Server) handleLibrarySub(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/libraries/")
	id, action, ok := strings.Cut(rest, "/")
	if !ok || id == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ruta desconocida"})
		return
	}
	if !s.canSeeZim(s.currentUser(r), id) { // no sugerir dentro de una colección sin acceso
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "sin acceso a esta colección"})
		return
	}
	switch action {
	case "search":
		s.handleSuggest(w, r, id)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "acción no soportada: " + action})
	}
}

// Suggestion: autocompletado por título (índice de títulos, barato incluso en Pi).
type Suggestion struct {
	Label string `json:"label"`
	Path  string `json:"path,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

// handleSuggest resuelve el autocompletado por título de una colección. El
// backend es el TitleIndex nativo (ZIM_ENGINE=native) o el /suggest de kiwix
// (suggestBackend, §6). Limpia el resaltado <b>…</b> que kiwix mete en los labels
// (inofensivo sobre los labels ya limpios del motor nativo).
func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request, id string) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, []Suggestion{})
		return
	}
	raw, err := s.suggestBackend(id, q, suggestLimit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "motor no disponible"})
		return
	}
	out := make([]Suggestion, 0, len(raw))
	for _, sr := range raw {
		if sr.Kind == "pattern" {
			continue // pseudo-sugerencia "buscar X": la añade la UI, no el motor
		}
		out = append(out, Suggestion{
			Label: stripTags(sr.Label),
			Path:  sr.Path,
			Kind:  sr.Kind,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// suggestLimit: candidatos por término que pide el shim al backend de títulos.
const suggestLimit = 20

// stripTags limpia el resaltado de kiwix. Los labels vienen con el <b>…</b>
// escapado como entidades (&lt;b&gt;), así que primero desescapamos y luego
// quitamos las etiquetas de negrita, dejando texto plano.
func stripTags(s string) string {
	s = html.UnescapeString(s) // &lt;b&gt; → <b>
	r := strings.NewReplacer("<b>", "", "</b>", "", "<B>", "", "</B>", "")
	return strings.TrimSpace(r.Replace(s))
}

func dateOnly(iso string) string {
	if len(iso) >= 10 {
		return iso[:10]
	}
	return iso
}
