// admin_catalog.go — Catálogo remoto de Kiwix para el Panel de Control.
//
// Deja explorar el catálogo público de Kiwix (library.kiwix.org) por categoría
// y descargar un .zim AL POOL (zim/) con el motor de descargas. La descarga
// aparece en la Cola; al terminar, el .zim sale en Colecciones como "sin
// registrar" y se registra con el flujo de admin_zim (descarga y registro
// separados = más limpio, y control para ZIMs de varios GB).

package main

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/andresgv-beep/nimos-library/download"
)

type adminCatalog struct {
	base   string // base del catálogo, p.ej. https://library.kiwix.org
	client *http.Client
	mgr    *download.Manager
	zimDir string
}

func newAdminCatalog(mgr *download.Manager, zimDir string) *adminCatalog {
	return &adminCatalog{
		base:   strings.TrimRight(env("KIWIX_CATALOG_URL", "https://library.kiwix.org"), "/"),
		client: &http.Client{Timeout: 30 * time.Second},
		mgr:    mgr,
		zimDir: zimDir,
	}
}

func (c *adminCatalog) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/catalog/categories", c.handleCategories)
	mux.HandleFunc("/api/admin/catalog/entries", c.handleEntries)
	mux.HandleFunc("/api/admin/catalog/download", c.handleDownload)
}

// OPDS (opdsFeed/opdsEntry/opdsLink) se reutilizan de catalog.go — mismo formato
// que el catálogo local de kiwix.

func (c *adminCatalog) fetchFeed(route string) (*opdsFeed, error) {
	resp, err := c.client.Get(c.base + route)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	var feed opdsFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

// ── Categorías ─────────────────────────────────────────────────────────────

func (c *adminCatalog) handleCategories(w http.ResponseWriter, r *http.Request) {
	feed, err := c.fetchFeed("/catalog/v2/categories")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "catálogo no accesible: " + err.Error()})
		return
	}
	cats := make([]string, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		if t := strings.TrimSpace(e.Title); t != "" {
			cats = append(cats, t)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": cats})
}

// ── Entradas ───────────────────────────────────────────────────────────────

type catalogEntry struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Summary  string `json:"summary,omitempty"`
	Language string `json:"language,omitempty"`
	Articles int    `json:"articles,omitempty"`
	Media    int    `json:"media,omitempty"`
	Bytes    int64  `json:"bytes,omitempty"`
	ZimURL   string `json:"zimUrl"`
	Filename string `json:"filename"`
	InPool   bool   `json:"inPool"` // el .zim ya está en el pool (descargado)
}

func (c *adminCatalog) handleEntries(w http.ResponseWriter, r *http.Request) {
	q := url.Values{}
	if v := r.URL.Query().Get("category"); v != "" {
		q.Set("category", v)
	}
	if v := r.URL.Query().Get("lang"); v != "" {
		q.Set("lang", v)
	}
	if v := r.URL.Query().Get("q"); v != "" {
		q.Set("q", v)
	}
	count := r.URL.Query().Get("count")
	if count == "" {
		count = "60"
	}
	q.Set("count", count)

	feed, err := c.fetchFeed("/catalog/v2/entries?" + q.Encode())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "catálogo no accesible: " + err.Error()})
		return
	}

	// Ficheros .zim ya en el pool (para señalar lo descargado). Case-insensitive
	// por Windows.
	have := map[string]bool{}
	if c.zimDir != "" {
		if ents, derr := os.ReadDir(c.zimDir); derr == nil {
			for _, e := range ents {
				have[strings.ToLower(e.Name())] = true
			}
		}
	}

	entries := make([]catalogEntry, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		zimURL, size := zimLink(e.Links)
		if zimURL == "" {
			continue // sin fichero descargable: no interesa al Panel
		}
		file := path.Base(zimURL)
		entries = append(entries, catalogEntry{
			ID: e.Name, Title: strings.TrimSpace(e.Title), Summary: strings.TrimSpace(e.Summary),
			Language: e.Language, Articles: e.Articles, Media: e.Media,
			Bytes: size, ZimURL: zimURL, Filename: file, InPool: have[strings.ToLower(file)],
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// zimLink busca el enlace de adquisición del .zim y devuelve la URL directa
// (quita el sufijo .meta4, que es un metalink de mirrors) + el tamaño.
func zimLink(links []opdsLink) (string, int64) {
	for _, l := range links {
		if strings.Contains(l.Type, "zim") && strings.Contains(l.Rel, "acquisition") {
			href := strings.TrimSuffix(l.Href, ".meta4")
			return href, l.Length
		}
	}
	return "", 0
}

// ── Descarga al pool ───────────────────────────────────────────────────────

func (c *adminCatalog) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	if c.zimDir == "" {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "carpeta de ZIMs no configurada (POOL_ROOT/ZIM_DIR)"})
		return
	}
	var req struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body inválido"})
		return
	}
	u, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.Contains(u.Host, "kiwix.org") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "URL inválida (solo descargas de kiwix.org)"})
		return
	}
	name := sanitizeFilename(req.Filename)
	if name == "" {
		name = sanitizeFilename(path.Base(u.Path))
	}
	if name == "" || !strings.HasSuffix(strings.ToLower(name), ".zim") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "el destino debe ser un .zim"})
		return
	}
	dest := filepath.Join(c.zimDir, name)
	job, err := c.mgr.Enqueue(u.String(), dest, "kiwix", name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, job)
}
