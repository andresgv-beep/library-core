package main

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

// ─── Búsqueda de imágenes (aproximación tipo Google Images) ──────────────────
// kiwix no indexa medios, así que: buscamos artículos que casan con la palabra
// y extraemos la imagen principal de cada uno → rejilla de imágenes.

type ImageHit struct {
	Thumb  string `json:"thumb"` // URL /content/... (ZIM) o /media/... (vídeo/logo)
	Title  string `json:"title"`
	Lib    string `json:"lib,omitempty"`
	Path   string `json:"path,omitempty"`
	Book   string `json:"book"`
	ItemID string `json:"itemId,omitempty"` // media (vídeo) → el front abre con onOpenItem
}

type imgCand struct {
	lib, book, title, path string
	score                  int
}

func (s *Server) handleImageSearch(media *mediaDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.imageSearch(media, w, r)
	}
}

func (s *Server) imageSearch(media *mediaDeps, w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []ImageHit{})
		return
	}
	// Misma puerta que /api/search: doble fan-out (búsqueda + fetch de artículos).
	if !s.acquireSearch(w, r) {
		return
	}
	defer s.releaseSearch()
	user := s.currentUser(r)
	libs, err := s.visibleLibs(user) // solo colecciones con acceso
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	// 1) Búsqueda de texto por colección (en paralelo).
	perLib := 10
	groups := make([]SearchGroup, len(libs))
	var wg sync.WaitGroup
	for i, lib := range libs {
		wg.Add(1)
		go func(i int, lib Library) {
			defer wg.Done()
			groups[i] = s.searchOne(lib, q, perLib)
		}(i, lib)
	}
	wg.Wait()

	// 2) Usar el mismo ranking que paginas. Intercalar por coleccion metia
	// resultados debiles delante de la coincidencia principal.
	var cands []imgCand
	seen := map[string]bool{}
	for _, g := range groups {
		for _, h := range g.Results {
			key := g.Lib + "\n" + h.Path
			if h.Path == "" || seen[key] {
				continue
			}
			seen[key] = true
			cands = append(cands, imgCand{lib: g.Lib, book: g.Book, title: h.Title, path: h.Path, score: h.Score})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	const maxImages = 36
	if len(cands) > maxImages {
		cands = cands[:maxImages]
	}

	// 3) Extraer la imagen principal de cada artículo (concurrencia limitada).
	hits := make([]ImageHit, len(cands))
	sem := make(chan struct{}, 6) // concurrencia moderada para no saturar el motor
	var wg2 sync.WaitGroup
	for i, c := range cands {
		wg2.Add(1)
		go func(i int, c imgCand) {
			defer wg2.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if img := s.firstImage(c.lib, c.path); img != "" {
				hits[i] = ImageHit{Thumb: img, Title: c.title, Lib: c.lib, Path: c.path, Book: c.book}
			}
		}(i, c)
	}
	wg2.Wait()

	out := make([]ImageHit, 0, len(hits)+8)
	// Media primero cuando casa: logo del canal + portadas de vídeos (buscar un
	// autor saca su logo y las portadas de sus vídeos).
	if media != nil {
		access := s.accessMap()
		visible := func(it mediaItem) bool {
			return canSeeCached(user, access, collectionIDForMedia(it.Collection))
		}
		if mi, mErr := media.searchImages(q, visible); mErr == nil {
			out = append(out, mi...)
		}
	}
	for _, h := range hits {
		if h.Thumb != "" {
			out = append(out, h)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// firstImage devuelve la URL de la primera imagen "de contenido" de un artículo
// (salta banderas, iconos y miniaturas diminutas). Lee el artículo por la costura
// articleDoc (zim_fts.go): motor nativo o kiwix según toggle — era el último
// consumidor de /api que pegaba a kiwix por HTTP en modo nativo.
func (s *Server) firstImage(lib, path string) string {
	doc, finalPath, err := s.articleDoc(context.Background(), lib, path)
	if err != nil {
		return ""
	}
	base, _ := url.Parse("/content/" + lib + "/" + finalPath)
	return firstImageFromDoc(doc, base)
}

// isContentImage descarta banderas, iconos y miniaturas diminutas.
func isContentImage(n *html.Node) bool {
	cls := getAttr(n, "class")
	if strings.Contains(cls, "flagicon") || strings.Contains(cls, "noviewer") {
		return false
	}
	if w := getAttr(n, "width"); w != "" {
		if v, err := strconv.Atoi(w); err == nil && v < 80 {
			return false
		}
	}
	return getAttr(n, "src") != ""
}
