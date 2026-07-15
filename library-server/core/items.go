package main

import (
	"encoding/base64"
	"encoding/json"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type Collection struct {
	ID           string           `json:"id"`
	Source       ItemSourceInfo   `json:"source,omitempty"`
	Kind         string           `json:"kind"`
	Title        string           `json:"title"`
	Description  string           `json:"description,omitempty"`
	Language     string           `json:"language,omitempty"`
	ItemCount    int              `json:"itemCount,omitempty"`
	SizeBytes    int64            `json:"sizeBytes,omitempty"`
	Updated      string           `json:"updated,omitempty"`
	Preview      Preview          `json:"preview,omitempty"`
	Open         *OpenTarget      `json:"open,omitempty"`
	Capabilities ItemCapabilities `json:"capabilities"`
}

type Item struct {
	ID           string           `json:"id"`
	CollectionID string           `json:"collectionId,omitempty"`
	Source       ItemSourceInfo   `json:"source,omitempty"`
	Kind         string           `json:"kind"`
	Title        string           `json:"title"`
	Description  string           `json:"description,omitempty"`
	Language     string           `json:"language,omitempty"`
	Contributor  string           `json:"contributor,omitempty"`
	License      string           `json:"license,omitempty"`
	Authors      []string         `json:"authors,omitempty"`
	Date         string           `json:"date,omitempty"`
	Tags         []string         `json:"tags,omitempty"`
	Preview      Preview          `json:"preview,omitempty"`
	Open         *OpenTarget      `json:"open,omitempty"`
	Capabilities ItemCapabilities `json:"capabilities"`
	Files        []ItemFile       `json:"files,omitempty"`
	TextURL      string           `json:"textUrl,omitempty"` // /media/… texto OCR completo local
	Tracks       []ItemTrack      `json:"tracks,omitempty"`  // pistas de un audiolibro
	// Campos de vídeo: duración, subtítulos, capítulos, avatar del canal.
	Duration      int           `json:"duration,omitempty"`
	Subtitles     []ItemSub     `json:"subtitles,omitempty"`
	Chapters      []ItemChapter `json:"chapters,omitempty"`
	ChannelAvatar string        `json:"channelAvatar,omitempty"` // /media/… imagen del canal
}

// ItemTrack = una pista de audiolibro con URLs locales (audio + onda opcional).
type ItemTrack struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Waveform string `json:"waveform,omitempty"`
}

// ItemSub = una pista de subtítulos con URL local (/media/…vtt).
type ItemSub struct {
	Lang string `json:"lang"`
	URL  string `json:"url"`
}

// ItemChapter = un marcador de capítulo (segundo de inicio + título).
type ItemChapter struct {
	Start float64 `json:"start"`
	Title string  `json:"title"`
}

type ItemFile struct {
	Name    string `json:"name"`
	Format  string `json:"format,omitempty"`
	Size    int64  `json:"size,omitempty"`
	URL     string `json:"url"`
	Local   bool   `json:"local"`
	Primary bool   `json:"primary,omitempty"`
}

type ItemSourceInfo struct {
	Provider       string `json:"provider,omitempty"`
	ProviderItemID string `json:"providerItemId,omitempty"`
	OriginalURL    string `json:"originalUrl,omitempty"`
}

type ItemCapabilities struct {
	Open      bool `json:"open"`
	Search    bool `json:"search"`
	Preview   bool `json:"preview"`
	Translate bool `json:"translate"`
	Favorite  bool `json:"favorite"`
	Note      bool `json:"note"`
	Tag       bool `json:"tag"`
	Download  bool `json:"download"` // el fichero del Item se puede descargar al equipo del cliente
}

type Preview struct {
	Kind  string `json:"kind"`
	URL   string `json:"url,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Text  string `json:"text,omitempty"`
	Color string `json:"color,omitempty"`
}

type OpenTarget struct {
	Mode     string `json:"mode"`
	URL      string `json:"url,omitempty"`
	ItemID   string `json:"itemId"`
	Title    string `json:"title,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Provider string `json:"provider,omitempty"` // carril de origen: manual/local (el reader elige plantilla)
}

type FederatedSearchResult struct {
	ItemID       string      `json:"itemId"`
	CollectionID string      `json:"collectionId,omitempty"`
	Title        string      `json:"title"`
	Subtitle     string      `json:"subtitle,omitempty"`
	Snippet      string      `json:"snippet,omitempty"`
	Kind         string      `json:"kind"`
	Score        int         `json:"score,omitempty"`
	Preview      Preview     `json:"preview,omitempty"`
	Highlights   []Highlight `json:"highlights,omitempty"`
}

type Highlight struct {
	Field string `json:"field"`
	Text  string `json:"text"`
}

func (s *Server) registerItemRoutes(mux *http.ServeMux, media *mediaDeps) {
	mux.HandleFunc("/api/collections", s.handleCollections(media))
	mux.HandleFunc("/api/collections/", s.handleCollectionSub(media))
	mux.HandleFunc("/api/items/search", s.handleItemSearch(media))
	mux.HandleFunc("/api/items/resolve", s.handleItemResolve(media))
	mux.HandleFunc("/api/items/", s.handleItemSub(media))
}

// Resuelve una dirección estable library://<provider>/<sourceId> al Item local.
func (s *Server) handleItemResolve(media *mediaDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo no permitido"})
			return
		}
		provider := strings.TrimSpace(r.URL.Query().Get("provider"))
		sourceID := strings.TrimSpace(r.URL.Query().Get("sourceId"))
		if provider == "" || sourceID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falta provider o sourceId"})
			return
		}
		items, err := media.scan("")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		for _, it := range items {
			if it.Source == provider && it.SourceID == sourceID {
				if !s.canSeeCollectionID(s.currentUser(r), collectionIDForMedia(it.Collection)) {
					writeJSON(w, http.StatusForbidden, map[string]string{"error": "sin acceso a esta coleccion"})
					return
				}
				writeJSON(w, http.StatusOK, mediaToItem(it))
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "contenido no importado"})
	}
}

func (s *Server) handleCollections(media *mediaDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo no permitido"})
			return
		}
		collections, err := s.allCollections(media)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		collections = s.filterCollections(s.currentUser(r), collections) // acceso/edad
		writeJSON(w, http.StatusOK, map[string]any{"collections": collections})
	}
}

func (s *Server) handleCollectionSub(media *mediaDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo no permitido"})
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/api/collections/")
		id, action, hasAction := strings.Cut(rest, "/")
		if id == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "coleccion no encontrada"})
			return
		}
		// Gate: la LISTA ya iba filtrada, pero pedir una colección por su ID no
		// comprobaba nada. Los IDs son adivinables (base64 del nombre), así que
		// esto era la puerta de atrás del filtro.
		if !s.canSeeCollectionID(s.currentUser(r), id) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "sin acceso a esta coleccion"})
			return
		}
		if hasAction && action == "items" {
			s.handleCollectionItems(w, r, media, id)
			return
		}
		if hasAction {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "accion no soportada"})
			return
		}
		col, ok, err := s.findCollection(media, id)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "coleccion no encontrada"})
			return
		}
		writeJSON(w, http.StatusOK, col)
	}
}

func (s *Server) handleCollectionItems(w http.ResponseWriter, r *http.Request, media *mediaDeps, collectionID string) {
	switch {
	case strings.HasPrefix(collectionID, "col:media:"):
		collection, ok := decodeOpaque(strings.TrimPrefix(collectionID, "col:media:"))
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "collection id invalido"})
			return
		}
		items, err := media.scan(collection)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		out := make([]Item, 0, len(items))
		for _, it := range items {
			out = append(out, mediaToItem(it))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
	case strings.HasPrefix(collectionID, "col:zim:"):
		writeJSON(w, http.StatusOK, map[string]any{
			"items": []Item{},
			"note":  "esta coleccion ZIM se enumera mediante busqueda; listado completo pendiente del motor",
		})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "coleccion no encontrada"})
	}
}

func (s *Server) handleItemSub(media *mediaDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo no permitido"})
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/api/items/")
		itemID, action, hasAction := strings.Cut(rest, "/")
		if itemID == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item no encontrado"})
			return
		}
		item, ok, err := s.findItem(media, itemID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item no encontrado"})
			return
		}
		// Gate: el item lleva su CollectionID, así que la pregunta es la misma
		// para un artículo de la Wikipedia que para un PDF del pool. Cubre /open
		// (que da la URL del fichero) y /preview.
		if !s.canSeeCollectionID(s.currentUser(r), item.CollectionID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "sin acceso a esta coleccion"})
			return
		}
		if hasAction {
			if action == "open" {
				writeJSON(w, http.StatusOK, item.Open)
				return
			}
			if action == "preview" {
				writeJSON(w, http.StatusOK, item.Preview)
				return
			}
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "accion no soportada"})
			return
		}
		writeJSON(w, http.StatusOK, item)
	}
}

func (s *Server) handleItemSearch(media *mediaDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo no permitido"})
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			writeJSON(w, http.StatusOK, map[string]any{"results": []FederatedSearchResult{}})
			return
		}
		if !s.acquireSearch(w, r) {
			return
		}
		defer s.releaseSearch()

		results := []FederatedSearchResult{}

		user := s.currentUser(r)
		libs, err := s.visibleLibs(user) // solo colecciones con acceso
		if err == nil {
			for _, lib := range libs {
				group := s.searchOne(lib, q, 8)
				for _, hit := range group.Results {
					results = append(results, zimSearchResult(lib, hit))
				}
			}
		}
		if mediaResults, merr := media.search(q); merr == nil {
			// La mitad ZIM iba filtrada; la de media no. El buscador enseñaba
			// título y snippet de lo que el usuario no puede abrir.
			results = append(results, s.filterSearchResults(user, mediaResults)...)
		}
		sort.SliceStable(results, func(i, j int) bool {
			if results[i].Score != results[j].Score {
				return results[i].Score > results[j].Score
			}
			return strings.ToLower(results[i].Title) < strings.ToLower(results[j].Title)
		})
		writeJSON(w, http.StatusOK, map[string]any{"results": results})
	}
}

func (s *Server) allCollections(media *mediaDeps) ([]Collection, error) {
	libs, err := s.fetchLibraries()
	out := make([]Collection, 0, len(libs))
	if err == nil {
		for _, lib := range libs {
			out = append(out, zimToCollection(lib))
		}
	}
	mediaCollections, mediaErr := media.collections()
	if mediaErr == nil {
		out = append(out, mediaCollections...)
	}
	if len(out) == 0 {
		if err != nil {
			return nil, err
		}
		if mediaErr != nil {
			return nil, mediaErr
		}
	}
	return out, nil
}

func (s *Server) findCollection(media *mediaDeps, id string) (Collection, bool, error) {
	cols, err := s.allCollections(media)
	if err != nil {
		return Collection{}, false, err
	}
	for _, col := range cols {
		if col.ID == id {
			return col, true, nil
		}
	}
	return Collection{}, false, nil
}

func (s *Server) findItem(media *mediaDeps, id string) (Item, bool, error) {
	switch {
	case strings.HasPrefix(id, "zim:"):
		payload, ok := decodeOpaque(strings.TrimPrefix(id, "zim:"))
		if !ok {
			return Item{}, false, nil
		}
		lib, itemPath, ok := strings.Cut(payload, "/")
		if !ok || lib == "" {
			return Item{}, false, nil
		}
		return zimToItem(Library{ID: lib}, itemPath, titleFromPath(itemPath), "", "", 0), true, nil
	case strings.HasPrefix(id, "media:"):
		items, err := media.scan("")
		if err != nil {
			return Item{}, false, err
		}
		for _, it := range items {
			item := mediaToItem(it)
			if item.ID == id {
				return item, true, nil
			}
		}
	}
	return Item{}, false, nil
}

func (m *mediaDeps) collections() ([]Collection, error) {
	items, err := m.scan("")
	if err != nil {
		return nil, err
	}
	byCollection := map[string][]mediaItem{}
	for _, item := range items {
		byCollection[item.Collection] = append(byCollection[item.Collection], item)
	}
	keys := make([]string, 0, len(byCollection))
	for key := range byCollection {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]Collection, 0, len(keys))
	for _, key := range keys {
		list := byCollection[key]
		meta := m.collectionMetadata(key)
		out = append(out, Collection{
			ID:          collectionIDForMedia(key),
			Source:      ItemSourceInfo{Provider: firstNonEmpty(meta.Source, "local"), ProviderItemID: meta.SourceID, OriginalURL: meta.SourceURL},
			Kind:        "media",
			Title:       firstNonEmpty(meta.Title, collectionTitle(key)),
			Description: firstNonEmpty(meta.Description, "Contenido local publicado"),
			ItemCount:   len(list),
			Preview:     collectionPreview(list),
			Capabilities: ItemCapabilities{
				Open: true, Search: true, Preview: true,
				Favorite: false, Note: false, Tag: false,
			},
		})
	}
	return out, nil
}

func (m *mediaDeps) collectionMetadata(collection string) collectionMeta {
	path, err := m.safeResolve(filepath.ToSlash(filepath.Join(collection, "collection.json")))
	if err != nil {
		return collectionMeta{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return collectionMeta{}
	}
	var meta collectionMeta
	if json.Unmarshal(raw, &meta) != nil {
		return collectionMeta{}
	}
	return meta
}

// mediaScore puntúa un item por título/desc + CANAL (autor) + TAGS y coge el
// mejor: así buscar el nombre del canal ("mat armstrong") o un tag ("bugatti
// rebuild") surfacea el vídeo, no solo si casa el título.
// lenientFuzzy: fuzzy más tolerante que fuzzyToken (permite tokens de 3 letras,
// p. ej. "mad"↔"mat"). Se usa solo para casar el NOMBRE DEL CANAL.
func lenientFuzzy(a, b string) bool {
	if len(a) < 3 || len(b) < 3 {
		return false
	}
	d := levenshtein(a, b)
	if len(a) <= 4 || len(b) <= 4 {
		return d <= 1
	}
	return d <= 2
}

// channelFuzzyScore da puntos si TODA la búsqueda se parece al nombre del canal,
// tolerando erratas ("mad amstrong" → "Mat Armstrong"). Exige que casen todos los
// tokens de la query (evita falsos por un solo token suelto).
func channelFuzzyScore(qTokens []string, channel string) int {
	cw := strings.Fields(normalizeText(channel))
	if len(cw) == 0 || len(qTokens) == 0 {
		return 0
	}
	matched := 0
	for _, tok := range qTokens {
		for _, w := range cw {
			if w == tok || strings.Contains(w, tok) || lenientFuzzy(tok, w) {
				matched++
				break
			}
		}
	}
	if matched < len(qTokens) {
		return 0
	}
	return 300 + matched*150
}

func mediaScore(q, nq string, qTokens []string, it mediaItem) int {
	score := scoreHit(q, it.Title, it.Media, it.Description)
	if a := scoreHit(q, it.Author, "", ""); a > score {
		score = a
	}
	if c := channelFuzzyScore(qTokens, it.Author); c > score {
		score = c // tolerante a erratas en el nombre del canal
	}
	if tagsText := normalizeText(strings.Join(it.Tags, " ")); tagsText != "" {
		ts := 0
		if strings.Contains(tagsText, nq) {
			ts = 620
		}
		if th, tf := tokenCoverage(qTokens, tagsText); th+tf > 0 {
			cov := th*110 + tf*55
			if len(qTokens) > 0 && th+tf == len(qTokens) {
				cov += 180
			}
			if cov > ts {
				ts = cov
			}
		}
		if ts > score {
			score = ts
		}
	}
	return score
}

func mediaMatchText(it mediaItem) string {
	return strings.Join([]string{it.Title, it.Author, it.Description, strings.Join(it.Tags, " "), it.Collection}, " ")
}

// mediaMatch decide si un item casa la búsqueda (+ su score). MISMO criterio para
// la búsqueda de texto y la de imágenes → si un vídeo sale en una, sale en la otra.
// Casa si contiene la frase exacta o si el score (título/canal/tags) es alto.
func mediaMatch(q, nq string, qTokens []string, it mediaItem) (score int, ok bool) {
	score = mediaScore(q, nq, qTokens, it)
	ok = strings.Contains(normalizeText(mediaMatchText(it)), nq) || score >= 200
	return
}

func (m *mediaDeps) search(q string) ([]FederatedSearchResult, error) {
	items, err := m.scan("")
	if err != nil {
		return nil, err
	}
	nq := normalizeText(q)
	qTokens := queryTokens(q)
	out := []FederatedSearchResult{}
	for _, it := range items {
		score, ok := mediaMatch(q, nq, qTokens, it)
		if !ok {
			continue
		}
		// Empuje moderado: los vídeos son pocos y de alto valor; que no queden
		// sepultados bajo cientos de artículos ZIM cuando el término casa de verdad.
		score += 120
		item := mediaToItem(it)
		out = append(out, FederatedSearchResult{
			ItemID:       item.ID,
			CollectionID: item.CollectionID,
			Title:        item.Title,
			Subtitle:     compactJoin(" - ", it.Author, it.Collection),
			Snippet:      stripSnippetNoise(it.Description),
			Kind:         item.Kind,
			Score:        score,
			Preview:      item.Preview,
		})
	}
	return out, nil
}

// searchImages alimenta el buscador de IMÁGENES del home con media: el LOGO del
// canal (avatar) de cada canal que casa + la PORTADA de cada vídeo que casa. Así
// buscar un autor saca su logo y las portadas de todos sus vídeos.
func (m *mediaDeps) searchImages(q string) ([]ImageHit, error) {
	items, err := m.scan("")
	if err != nil {
		return nil, err
	}
	nq := normalizeText(q)
	qTokens := queryTokens(q)
	type scored struct {
		it    mediaItem
		score int
	}
	var matched []scored
	for _, it := range items {
		score, ok := mediaMatch(q, nq, qTokens, it) // mismo criterio que la búsqueda de texto
		if !ok {
			continue
		}
		matched = append(matched, scored{it, score})
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].score > matched[j].score })

	out := []ImageHit{}
	seenChan := map[string]bool{}
	for _, s := range matched { // 1) logo del canal (uno por canal), primero
		ch := s.it.Author
		if ch == "" || seenChan[ch] || s.it.ChannelAvatarURL == "" {
			continue
		}
		seenChan[ch] = true
		// El logo abre el vídeo top de ese canal (así no cae en /content/undefined;
		// desde su ficha ves el canal y el resto de sus vídeos en Relacionados).
		out = append(out, ImageHit{Thumb: s.it.ChannelAvatarURL, Title: ch, Book: ch, ItemID: mediaToItem(s.it).ID})
	}
	for _, s := range matched { // 2) portada de cada vídeo que casa
		if s.it.CoverURL == "" {
			continue
		}
		out = append(out, ImageHit{Thumb: s.it.CoverURL, Title: s.it.Title, Book: s.it.Author, ItemID: mediaToItem(s.it).ID})
	}
	return out, nil
}

func zimToCollection(lib Library) Collection {
	return Collection{
		ID:          collectionIDForZIM(lib.ID),
		Source:      ItemSourceInfo{Provider: "openzim", ProviderItemID: lib.ID},
		Kind:        "zim",
		Title:       lib.Name,
		Description: lib.Description,
		Language:    lib.Lang,
		ItemCount:   lib.Articles,
		SizeBytes:   lib.Size,
		Updated:     lib.Date,
		Preview:     Preview{Kind: "icon", URL: lib.Icon, Icon: lib.Icon},
		Open: &OpenTarget{
			Mode:   "iframe",
			ItemID: itemIDForZIM(lib.ID, ""),
			URL:    "/content/" + escapePath(lib.ID) + "/",
			Title:  lib.Name,
		},
		Capabilities: ItemCapabilities{
			Open: true, Search: true, Preview: true, Translate: true,
		},
	}
}

func zimToItem(lib Library, itemPath, title, snippet, thumb string, score int) Item {
	if title == "" {
		title = titleFromPath(itemPath)
	}
	itemID := itemIDForZIM(lib.ID, itemPath)
	return Item{
		ID:           itemID,
		CollectionID: collectionIDForZIM(lib.ID),
		Source:       ItemSourceInfo{Provider: "openzim", ProviderItemID: lib.ID},
		Kind:         "article",
		Title:        title,
		Description:  snippet,
		Preview:      Preview{Kind: previewKind(thumb, snippet), URL: thumb, Text: snippet},
		Open: &OpenTarget{
			Mode:   "iframe",
			ItemID: itemID,
			URL:    "/content/" + escapePath(lib.ID) + "/" + escapePath(itemPath),
			Title:  title,
		},
		Capabilities: ItemCapabilities{
			Open: true, Search: true, Preview: thumb != "" || snippet != "",
			Translate: true, Favorite: true, Note: true, Tag: true,
		},
	}
}

func mediaToItem(it mediaItem) Item {
	relMedia := strings.Trim(strings.TrimPrefix(it.MediaURL, "/media/"), "/")
	itemID := itemIDForMedia(relMedia)
	kind := kindFromTemplate(it.Template)
	mimeType := mime.TypeByExtension(filepath.Ext(it.Media))
	files := make([]ItemFile, 0, len(it.Tracks)+1)
	// Audiolibro: las pistas van primero, locales y descargables (una por
	// capítulo). Así "Archivos y formatos" no queda pelado y el audio se puede
	// bajar, además de sonar en el reproductor.
	for _, tr := range it.Tracks {
		files = append(files, ItemFile{Name: tr.Title, Format: "MP3", URL: tr.URL, Local: true})
	}
	if len(files) == 0 {
		files = append(files, ItemFile{Name: it.Media, Size: mediaFileSize(it), URL: it.MediaURL, Local: true, Primary: true})
	}
	// Texto OCR completo descargado junto al media: descargable desde la ficha y
	// combustible de la búsqueda dentro del libro / traducción de pasajes (offline).
	if it.TextURL != "" {
		base := strings.TrimSuffix(it.Media, filepath.Ext(it.Media))
		files = append(files, ItemFile{Name: base + ".txt", Format: "Texto (OCR)", URL: it.TextURL, Local: true})
	}
	provider := firstNonEmpty(it.Source, "local")
	subs := make([]ItemSub, 0, len(it.Subtitles))
	for _, s := range it.Subtitles {
		subs = append(subs, ItemSub{Lang: s.Lang, URL: s.URL})
	}
	chaps := make([]ItemChapter, 0, len(it.Chapters))
	for _, c := range it.Chapters {
		chaps = append(chaps, ItemChapter{Start: c.Start, Title: c.Title})
	}
	return Item{
		ID:           itemID,
		CollectionID: collectionIDForMedia(it.Collection),
		Source: ItemSourceInfo{
			Provider:       provider,
			ProviderItemID: firstNonEmpty(it.SourceID, it.ID),
			OriginalURL:    it.SourceURL,
		},
		Kind:        kind,
		Title:       firstNonEmpty(it.Title, it.Media),
		Description: stripSnippetNoise(stripHTMLTags(it.Description)),
		Language:    it.Language,
		Contributor: it.Contributor,
		License:     it.License,
		Authors:     splitAuthor(it.Author),
		Date:        it.Date,
		Tags:        it.Tags,
		Preview:     mediaPreview(it),
		Open: &OpenTarget{
			Mode:     openModeForKind(kind),
			ItemID:   itemID,
			URL:      it.MediaURL,
			Title:    firstNonEmpty(it.Title, it.Media),
			MimeType: mimeType,
			Provider: provider,
		},
		Capabilities: ItemCapabilities{
			Open: true, Search: true, Preview: true,
			Favorite: true, Note: true, Tag: true,
			Download: true, // fichero publicado → descargable al equipo del cliente
		},
		Files:         files,
		TextURL:       it.TextURL,
		Tracks:        it.Tracks,
		Duration:      it.Duration,
		Subtitles:     subs,
		Chapters:      chaps,
		ChannelAvatar: it.ChannelAvatarURL,
	}
}

func mediaFileSize(it mediaItem) int64 {
	// El tamaño es opcional; los sidecars antiguos no lo conservaban.
	return 0
}

func zimSearchResult(lib Library, hit SearchHit) FederatedSearchResult {
	item := zimToItem(lib, hit.Path, hit.Title, hit.Snippet, hit.Thumb, hit.Score)
	return FederatedSearchResult{
		ItemID:       item.ID,
		CollectionID: item.CollectionID,
		Title:        item.Title,
		Subtitle:     lib.Name,
		Snippet:      hit.Snippet,
		Kind:         item.Kind,
		Score:        hit.Score,
		Preview:      item.Preview,
	}
}

func collectionIDForZIM(lib string) string { return "col:zim:" + encodeOpaque(lib) }
func collectionIDForMedia(collection string) string {
	return "col:media:" + encodeOpaque(collection)
}
func itemIDForZIM(lib, itemPath string) string {
	return "zim:" + encodeOpaque(strings.Trim(lib, "/")+"/"+strings.TrimLeft(itemPath, "/"))
}
func itemIDForMedia(relMedia string) string {
	return "media:" + encodeOpaque(strings.Trim(relMedia, "/"))
}

func encodeOpaque(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func decodeOpaque(s string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", false
	}
	return string(raw), true
}

func escapePath(s string) string {
	parts := strings.Split(strings.Trim(s, "/"), "/")
	for i, part := range parts {
		parts[i] = pathEscape(part)
	}
	return strings.Join(parts, "/")
}

func pathEscape(s string) string {
	// net/url.PathEscape would be ideal, but media.go already keeps this helper
	// local. Use the same escaping behavior via mediaURLPath for one segment.
	return mediaURLPath(s)
}

func titleFromPath(p string) string {
	p = strings.Trim(strings.ReplaceAll(p, "_", " "), "/")
	if p == "" {
		return "Inicio"
	}
	base := path.Base(p)
	if ext := path.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return strings.TrimSpace(base)
}

func collectionTitle(collection string) string {
	name := strings.TrimSpace(path.Base(filepath.ToSlash(collection)))
	if name == "." || name == "/" || name == "" {
		return "Archivo local"
	}
	return strings.ReplaceAll(name, "_", " ")
}

func collectionPreview(items []mediaItem) Preview {
	// Preferir una portada (cualquier tipo la tiene); si no, una imagen real.
	for _, it := range items {
		if it.CoverURL != "" {
			return Preview{Kind: "image", URL: it.CoverURL}
		}
	}
	for _, it := range items {
		if it.MediaURL != "" && (it.Template == "image" || it.Template == "gallery") {
			return Preview{Kind: "image", URL: it.MediaURL}
		}
	}
	if len(items) > 0 {
		return Preview{Kind: "text", Text: items[0].Title}
	}
	return Preview{Kind: "none"}
}

func mediaPreview(it mediaItem) Preview {
	// La portada (guardada en local junto al media) es la miniatura
	// de TODOS los tipos (vídeo/pdf/audio/imagen), no solo de las imágenes.
	if it.CoverURL != "" {
		return Preview{Kind: "image", URL: it.CoverURL}
	}
	if it.MediaURL != "" && (it.Template == "image" || it.Template == "gallery") {
		return Preview{Kind: "image", URL: it.MediaURL}
	}
	if it.Title != "" {
		return Preview{Kind: "text", Text: it.Title}
	}
	return Preview{Kind: "none"}
}

func previewKind(url, text string) string {
	if url != "" {
		return "image"
	}
	if text != "" {
		return "text"
	}
	return "none"
}

func kindFromTemplate(template string) string {
	switch template {
	case "pdf":
		return "pdf"
	case "video":
		return "video"
	case "audio":
		return "audio"
	case "image", "gallery":
		return "image"
	case "reader":
		return "document"
	default:
		return "file"
	}
}

func openModeForKind(kind string) string {
	switch kind {
	case "pdf", "video", "audio", "image":
		return kind
	case "document":
		return "iframe"
	default:
		return "media"
	}
}

func splitAuthor(author string) []string {
	author = strings.TrimSpace(author)
	if author == "" {
		return nil
	}
	parts := strings.FieldsFunc(author, func(r rune) bool { return r == ';' })
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func compactJoin(sep string, values ...string) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, sep)
}
