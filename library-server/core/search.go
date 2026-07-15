package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	zimhtml "golang.org/x/net/html"
)

// Busqueda global cross-ZIM. Kiwix da los candidatos; el shim reordena con
// intencion de usuario: titulo exacto primero, luego titulo parecido, luego texto.

type SearchHit struct {
	Title     string `json:"title"`
	Path      string `json:"path"`
	Snippet   string `json:"snippet,omitempty"`
	Thumb     string `json:"thumb,omitempty"`
	WordCount string `json:"wordCount,omitempty"`
	Score     int    `json:"score,omitempty"`
}

type SearchGroup struct {
	Lib     string      `json:"lib"`
	Book    string      `json:"book"`
	Total   int         `json:"total"`
	Results []SearchHit `json:"results"`
}

// XML de kiwix (OpenSearch RSS). totalResults viene con coma de miles
// ("35,306"), asi que se parsea como texto.
type searchRSS struct {
	Total string       `xml:"channel>totalResults"`
	Items []searchItem `xml:"channel>item"`
}

type searchItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	WordCount   string `xml:"wordCount"`
}

func (s *Server) handleGlobalSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []SearchGroup{})
		return
	}
	// Cache: repetir una búsqueda no golpea el motor ni consume slot del gate.
	cacheKey := normalizeText(q)
	if data, ok := s.searchCache.get(cacheKey); ok {
		writeCachedJSON(w, data)
		return
	}
	// Puerta anti-DoS (§6): la búsqueda global es fan-out caro sobre Xapian.
	if !s.acquireSearch(w, r) {
		return
	}
	defer s.releaseSearch()
	perLib := 8

	libs, err := s.visibleLibs(s.currentUser(r)) // no buscar en colecciones sin acceso
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

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

	out := make([]SearchGroup, 0, len(groups))
	for _, g := range groups {
		if len(g.Results) > 0 {
			out = append(out, g)
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Results[0].Score != out[b].Results[0].Score {
			return out[a].Results[0].Score > out[b].Results[0].Score
		}
		return out[a].Total > out[b].Total
	})
	data, _ := json.Marshal(out)
	s.searchCache.set(cacheKey, data)
	writeCachedJSON(w, data)
}

func (s *Server) searchOne(lib Library, q string, limit int) SearchGroup {
	g := SearchGroup{Lib: lib.ID, Book: lib.Name}
	seen := map[string]int{}

	add := func(h SearchHit) {
		if h.Path == "" {
			return
		}
		h.Title = strings.TrimSpace(h.Title)
		if h.Title == "" {
			h.Title = h.Path
		}
		if h.Score == 0 {
			h.Score = scoreHit(q, h.Title, h.Path, h.Snippet)
		}
		if i, ok := seen[h.Path]; ok {
			if h.Score > g.Results[i].Score {
				if h.Snippet == "" {
					h.Snippet = g.Results[i].Snippet
				}
				if h.Thumb == "" {
					h.Thumb = g.Results[i].Thumb
				}
				if h.WordCount == "" {
					h.WordCount = g.Results[i].WordCount
				}
				g.Results[i] = h
			} else if g.Results[i].Snippet == "" && h.Snippet != "" {
				g.Results[i].Snippet = h.Snippet
			} else if g.Results[i].Thumb == "" && h.Thumb != "" {
				g.Results[i].Thumb = h.Thumb
			}
			return
		}
		seen[h.Path] = len(g.Results)
		g.Results = append(g.Results, h)
	}

	for _, h := range s.suggestHits(lib, q) {
		add(h)
	}

	// Full-text: índice bleve propio (Fase C2, zim_fts.go) con el motor nativo;
	// el /search de kiwix (Xapian) en el camino clásico. Mismo merge/scoring.
	if s.zimNative != nil {
		hits, total := s.nativeSearchHits(lib.ID, q, limit)
		g.Total = total
		for _, h := range hits {
			add(h)
		}
	} else if err := s.kiwixSearchHits(lib, q, limit, &g, add); err != nil {
		return g
	}

	sort.SliceStable(g.Results, func(i, j int) bool {
		if g.Results[i].Score != g.Results[j].Score {
			return g.Results[i].Score > g.Results[j].Score
		}
		return strings.ToLower(g.Results[i].Title) < strings.ToLower(g.Results[j].Title)
	})
	if len(g.Results) > limit {
		g.Results = g.Results[:limit]
	}
	s.fillMissingPreviews(lib, q, g.Results)
	if g.Total < len(g.Results) {
		g.Total = len(g.Results)
	}
	return g
}

// kiwixSearchHits: el camino clásico del full-text — /search de kiwix (Xapian,
// OpenSearch RSS). Muere con la retirada de kiwix (§8).
func (s *Server) kiwixSearchHits(lib Library, q string, limit int, g *SearchGroup, add func(SearchHit)) error {
	u := s.kiwix.String() + "/search?format=xml" +
		"&books.name=" + urlq(lib.ID) +
		"&pattern=" + urlq(q) +
		"&pageLength=" + strconv.Itoa(limit)

	resp, err := s.kget(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("kiwix /search: status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)

	var rss searchRSS
	if err := xml.Unmarshal(body, &rss); err != nil {
		return err
	}
	g.Total = parseIntComma(rss.Total)

	prefix := "/content/" + lib.ID + "/"
	for _, it := range rss.Items {
		path := strings.TrimPrefix(it.Link, prefix)
		if dec, err := url.PathUnescape(path); err == nil {
			path = dec
		}
		add(SearchHit{
			Title:     strings.TrimSpace(stdhtml.UnescapeString(it.Title)),
			Path:      path,
			Snippet:   strings.TrimSpace(stdhtml.UnescapeString(it.Description)),
			WordCount: it.WordCount,
		})
	}
	return nil
}

func (s *Server) suggestHits(lib Library, q string) []SearchHit {
	// Orden: original + tokens primero (baratos, alto valor); las variantes de
	// typo van al final y son las que caen si se pasa el tope. Así el recorte
	// nunca sacrifica la búsqueda por palabra.
	queries := []string{q}
	for _, tok := range queryTokens(q) {
		if len(tok) >= 4 {
			queries = append(queries, tok)
		}
	}
	boostQuery := map[string]bool{}
	for _, tq := range typoPhraseQueries(q) {
		queries = append(queries, tq)
		boostQuery[normalizeText(tq)] = true
	}
	// Tope de llamadas a /suggest por colección: con 12 variantes de typo el
	// fan-out se dispara (colecciones × variantes).
	const maxSuggestQueries = 8
	if len(queries) > maxSuggestQueries {
		queries = queries[:maxSuggestQueries]
	}

	seenQ := map[string]bool{}
	seenPath := map[string]int{}
	out := []SearchHit{}
	for _, term := range queries {
		term = strings.TrimSpace(term)
		if term == "" || seenQ[term] {
			continue
		}
		seenQ[term] = true
		raw, err := s.suggestBackend(lib.ID, term, suggestLimit)
		if err != nil {
			continue
		}
		for _, r := range raw {
			if r.Path == "" || r.Kind == "pattern" {
				continue
			}
			title := stripTags(r.Label)
			score := scoreHit(q, title, r.Path, "")
			if boostQuery[normalizeText(term)] {
				score += scoreHit(term, title, r.Path, "") / 2
			}
			hit := SearchHit{
				Title: title,
				Path:  r.Path,
				Score: score,
			}
			if i, ok := seenPath[r.Path]; ok {
				if hit.Score > out[i].Score {
					out[i] = hit
				}
				continue
			}
			seenPath[r.Path] = len(out)
			out = append(out, hit)
		}
	}
	return out
}

type searchPreview struct {
	Snippet string
	Thumb   string
}

func (s *Server) fillMissingPreviews(lib Library, q string, hits []SearchHit) {
	const maxFetches = 8
	fetched := 0
	for i := range hits {
		if fetched >= maxFetches {
			return
		}
		if usefulSnippet(hits[i].Snippet) && hits[i].Thumb != "" {
			continue
		}
		preview := s.articlePreview(lib.ID, hits[i].Path, q, hits[i].Title)
		fetched++
		if !usefulSnippet(hits[i].Snippet) && preview.Snippet != "" {
			hits[i].Snippet = preview.Snippet
		}
		if hits[i].Thumb == "" && preview.Thumb != "" {
			hits[i].Thumb = preview.Thumb
		}
	}
}

func usefulSnippet(s string) bool {
	return len(strings.Fields(stripSnippetNoise(s))) >= 8
}

func (s *Server) articlePreview(lib, path, q, title string) searchPreview {
	// articleDoc (zim_fts.go) trae el artículo del motor nativo o de kiwix según
	// el toggle: los previews de la búsqueda ya no dependen de kiwix en nativo.
	doc, finalPath, err := s.articleDoc(context.Background(), lib, path)
	if err != nil {
		return searchPreview{}
	}
	base, _ := url.Parse("/content/" + lib + "/" + finalPath)
	return searchPreview{
		Snippet: bestSnippetFromDoc(doc, q, title),
		Thumb:   firstImageFromDoc(doc, base),
	}
}

func firstImageFromDoc(doc *zimhtml.Node, base *url.URL) string {
	var found string
	var walk func(n *zimhtml.Node)
	walk = func(n *zimhtml.Node) {
		if found != "" {
			return
		}
		if n.Type == zimhtml.ElementNode && n.Data == "img" {
			if isContentImage(n) {
				if abs := absResource(getAttr(n, "src"), base); abs != "" && !strings.Contains(abs, "flagicon") {
					found = abs
					return
				}
			}
		}
		for c := n.FirstChild; c != nil && found == ""; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

func bestSnippetFromDoc(doc *zimhtml.Node, q, title string) string {
	tokens := queryTokens(q)
	titleNorm := normalizeText(title)
	var fallback string
	best := ""
	bestScore := -1

	var walk func(*zimhtml.Node)
	walk = func(n *zimhtml.Node) {
		if n.Type == zimhtml.ElementNode && skipSnippetNode(n) {
			return
		}
		if n.Type == zimhtml.ElementNode && snippetCandidateNode(n) {
			txt := cleanSnippetText(textOf(n))
			if txt != "" && normalizeText(txt) != titleNorm {
				if fallback == "" {
					fallback = txt
				}
				score := snippetScore(tokens, txt)
				if score > bestScore {
					bestScore = score
					best = txt
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if best != "" && (bestScore > 0 || fallback == "") {
		return best
	}
	return fallback
}

func snippetCandidateNode(n *zimhtml.Node) bool {
	switch n.Data {
	case "p", "figcaption":
		return true
	case "li":
		return len(cleanSnippetText(textOf(n))) >= 80
	}
	return false
}

func skipSnippetNode(n *zimhtml.Node) bool {
	switch n.Data {
	case "script", "style", "table", "nav", "footer", "header", "sup":
		return true
	}
	cls := strings.ToLower(getAttr(n, "class"))
	return strings.Contains(cls, "navbox") ||
		strings.Contains(cls, "metadata") ||
		strings.Contains(cls, "mw-editsection") ||
		strings.Contains(cls, "reference")
}

func snippetScore(tokens []string, text string) int {
	nt := normalizeText(text)
	exact, fuzzy := tokenCoverage(tokens, nt)
	score := exact*3 + fuzzy*2
	words := strings.Fields(nt)
	if len(words) >= 14 && len(words) <= 60 {
		score += 2
	}
	return score
}

func cleanSnippetText(s string) string {
	s = stripSnippetNoise(s)
	if len([]rune(s)) > 260 {
		r := []rune(s)
		cut := 260
		for cut > 180 && r[cut] != ' ' {
			cut--
		}
		if cut <= 180 {
			cut = 260
		}
		s = strings.TrimSpace(string(r[:cut])) + "..."
	}
	if len(strings.Fields(s)) < 8 {
		return ""
	}
	return s
}

func stripSnippetNoise(s string) string {
	s = strings.NewReplacer("\n", " ", "\t", " ", "\r", " ").Replace(stdhtml.UnescapeString(s))
	return strings.Join(strings.Fields(s), " ")
}

// stripHTMLTags quita las etiquetas HTML de un texto (p. ej. las descripciones de
// algunos proveedores traen <a href>…</a> y <br/>). Convierte los saltos de bloque en
// nueva línea y elimina el resto de tags, dejando texto plano legible.
func stripHTMLTags(s string) string {
	s = strings.NewReplacer(
		"<br />", "\n", "<br/>", "\n", "<br>", "\n",
		"</p>", "\n", "</div>", "\n", "</li>", "\n",
	).Replace(s)
	var b strings.Builder
	depth := 0
	for _, c := range s {
		switch c {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(c)
			}
		}
	}
	return b.String()
}

func typoPhraseQueries(q string) []string {
	parts := strings.Fields(strings.TrimSpace(q))
	if len(parts) == 0 || len(parts) > 4 {
		return nil
	}
	out := []string{}
	for i, part := range parts {
		clean := normalizeText(part)
		if len(clean) < 4 || len(clean) > 12 {
			continue
		}
		for _, v := range adjacentSwapVariants(clean) {
			next := append([]string(nil), parts...)
			next[i] = v
			out = append(out, strings.Join(next, " "))
			if len(out) >= 12 {
				return out
			}
		}
	}
	return out
}

func adjacentSwapVariants(s string) []string {
	r := []rune(s)
	out := make([]string, 0, len(r)-1)
	seen := map[string]bool{}
	for i := 0; i < len(r)-1; i++ {
		if r[i] == r[i+1] {
			continue
		}
		cp := append([]rune(nil), r...)
		cp[i], cp[i+1] = cp[i+1], cp[i]
		v := string(cp)
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func scoreHit(q, title, path, snippet string) int {
	nq := normalizeText(q)
	nt := normalizeText(title)
	np := normalizeText(strings.ReplaceAll(path, "_", " "))
	ns := normalizeText(snippet)
	if nq == "" {
		return 0
	}

	score := 0
	switch {
	case nt == nq:
		score += 1000
	case strings.HasPrefix(nt, nq):
		score += 850
	case strings.Contains(nt, nq):
		score += 760
	case np == nq:
		score += 720
	case strings.Contains(np, nq):
		score += 650
	}
	if nt != nq {
		if d := levenshtein(nq, nt); d > 0 && d <= 2 {
			score += 900
		}
	}

	qTokens := queryTokens(q)
	titleHits, titleFuzzy := tokenCoverage(qTokens, nt)
	pathHits, pathFuzzy := tokenCoverage(qTokens, np)
	snippetHits, _ := tokenCoverage(qTokens, ns)
	score += titleHits*120 + titleFuzzy*70
	score += pathHits*60 + pathFuzzy*35
	score += snippetHits * 15

	if len(qTokens) > 0 && titleHits+titleFuzzy == len(qTokens) {
		score += 260
	}
	if len(qTokens) > 0 && pathHits+pathFuzzy == len(qTokens) {
		score += 120
	}
	lt := strings.ToLower(title)
	if strings.HasPrefix(lt, "anexo:") || strings.HasPrefix(strings.ToLower(path), "anexo:") {
		score -= 80
	}
	if strings.Contains(nt, "desambiguacion") {
		score -= 60
	}
	if score < 1 {
		score = 1
	}
	return score
}

func tokenCoverage(tokens []string, text string) (exact, fuzzy int) {
	if text == "" {
		return 0, 0
	}
	words := strings.Fields(text)
	for _, tok := range tokens {
		found := false
		for _, w := range words {
			if w == tok || strings.Contains(w, tok) || (len(w) >= 4 && strings.Contains(tok, w)) {
				exact++
				found = true
				break
			}
		}
		if found {
			continue
		}
		for _, w := range words {
			if fuzzyToken(tok, w) {
				fuzzy++
				break
			}
		}
	}
	return exact, fuzzy
}

func fuzzyToken(a, b string) bool {
	if len(a) < 4 || len(b) < 4 {
		return false
	}
	d := levenshtein(a, b)
	if len(a) <= 5 || len(b) <= 5 {
		return d <= 1
	}
	return d <= 2
}

func queryTokens(s string) []string {
	raw := strings.Fields(normalizeText(s))
	out := make([]string, 0, len(raw))
	stop := map[string]bool{"de": true, "del": true, "la": true, "el": true, "los": true, "las": true, "the": true, "and": true, "of": true}
	for _, tok := range raw {
		if len(tok) < 2 || stop[tok] {
			continue
		}
		out = append(out, tok)
	}
	return out
}

func normalizeText(s string) string {
	s = strings.ToLower(stdhtml.UnescapeString(s))
	var b strings.Builder
	space := false
	for _, r := range s {
		r = foldRune(r)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func foldRune(r rune) rune {
	switch r {
	case 'á', 'à', 'ä', 'â', 'ã':
		return 'a'
	case 'é', 'è', 'ë', 'ê':
		return 'e'
	case 'í', 'ì', 'ï', 'î':
		return 'i'
	case 'ó', 'ò', 'ö', 'ô', 'õ':
		return 'o'
	case 'ú', 'ù', 'ü', 'û':
		return 'u'
	case 'ñ':
		return 'n'
	case 'ç':
		return 'c'
	}
	return r
}

func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ca := range ar {
		cur := make([]int, len(br)+1)
		cur[0] = i + 1
		for j, cb := range br {
			cost := 0
			if ca != cb {
				cost = 1
			}
			cur[j+1] = min3(cur[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev = cur
	}
	return prev[len(br)]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// parseIntComma convierte "35,306" (o "35 306") en 35306.
func parseIntComma(s string) int {
	s = strings.NewReplacer(",", "", " ", "", ".", "", "\u00a0", "").Replace(strings.TrimSpace(s))
	n, _ := strconv.Atoi(s)
	return n
}

// ─── Puerta de búsquedas globales (rate-limit v1.1, §6) ───────────────────────
// Máximo N búsquedas globales simultáneas (search/images), N escalado a la
// máquina (ver main.go, SEARCH_CONCURRENCY). Las que no caben NO reciben 429
// directo: esperan slot en una cola acotada. La espera es event-driven — el
// send al canal despierta en el instante exacto en que una búsqueda activa
// libera slot; no hay polling ni observación externa de carga (el semáforo YA
// sabe cuántas búsquedas hay en vuelo, con precisión exacta y latencia cero).
//
// El timeout NO es el mecanismo de admisión: es un límite de cordura para el
// caso patológico (búsquedas activas colgadas por ZIM corrupto / motor tostado)
// en que no tiene sentido retener esperantes cuyo usuario probablemente ya se
// fue. Debe quedar por debajo del timeout del cliente HTTP del shim (30s) y
// del timeout del fetch en la UI.
//
// 429 solo cuando hay saturación real: gate lleno Y cola llena, o espera
// agotada. Nota (§6): esto no es fairness por usuario — un mismo cliente con
// 3 pestañas ocupa gate y cola igual que 3 personas. Fairness por IP y gate
// adaptativo por latencia quedan como evolución futura (ver CAMBIOS §6).

const (
	searchQueueMax = 4                // esperantes máximos por encima del gate
	searchWaitMax  = 15 * time.Second // cordura: tope de espera en cola
)

func (s *Server) acquireSearch(w http.ResponseWriter, r *http.Request) bool {
	// Camino rápido: slot libre → entra ya (usuario solo = rendimiento pleno).
	select {
	case s.searchGate <- struct{}{}:
		return true
	default:
	}

	// Gate lleno: intentar entrar en la cola de espera acotada.
	if s.searchWaiters.Add(1) > searchQueueMax {
		s.searchWaiters.Add(-1)
		writeJSON(w, http.StatusTooManyRequests,
			map[string]string{"error": "demasiadas búsquedas simultáneas, reintenta en un momento"})
		return false
	}
	defer s.searchWaiters.Add(-1)

	// Esperar slot (despierta al liberarse), rendirse por cordura, o soltar
	// el hueco al instante si el cliente cerró la conexión.
	t := time.NewTimer(searchWaitMax)
	defer t.Stop()
	select {
	case s.searchGate <- struct{}{}:
		return true
	case <-t.C:
		writeJSON(w, http.StatusTooManyRequests,
			map[string]string{"error": "el motor lleva demasiado ocupado, reintenta en un momento"})
		return false
	case <-r.Context().Done():
		return false
	}
}

func (s *Server) releaseSearch() { <-s.searchGate }
