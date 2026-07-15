package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func itemTestServer(t *testing.T, engineURL string) *Server {
	t.Helper()
	ku, _ := url.Parse(engineURL)
	// Store real: el contrato de Items pasa ahora por el gate de acceso
	// (access.go), y sin store el gate se cierra (fail-closed).
	st, err := openStore(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	t.Cleanup(func() { st.db.Close() })
	return &Server{
		kiwix:       ku,
		http:        &http.Client{Timeout: time.Second},
		kiwixSem:    make(chan struct{}, 1),
		searchGate:  make(chan struct{}, 1),
		searchCache: newLRUCache(8, time.Minute),
		store:       st,
	}
}

// grantOpen abre una colección de media para los tests de contrato (que miran la
// FORMA del Item, no los permisos). Los permisos tienen sus propios tests.
func grantOpen(t *testing.T, s *Server, collection string) {
	t.Helper()
	_, err := s.store.db.Exec(
		`INSERT INTO collection_access (collection_id, access, min_age, updated) VALUES (?,?,?,?)`,
		collectionIDForMedia(collection), "open", 0, time.Now().Unix())
	if err != nil {
		t.Fatalf("grantOpen(%s): %v", collection, err)
	}
}

func TestMediaItemContract(t *testing.T) {
	root := t.TempDir()
	seedCollection(t, root, "Published/Books", "linux bible.pdf", "%PDF-1.4 fake",
		sidecar{
			Template:    "pdf",
			Title:       "Linux Bible",
			Author:      "Christopher Negus",
			Description: "A practical Linux reference.",
			Source:      "archive",
			SourceURL:   "https://example.org/item/linux-bible",
		})

	items, err := (&mediaDeps{root: root}).scan("")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}

	item := mediaToItem(items[0])
	if item.Kind != "pdf" {
		t.Fatalf("kind = %q, want pdf", item.Kind)
	}
	if item.Title != "Linux Bible" {
		t.Fatalf("title = %q", item.Title)
	}
	if item.CollectionID != collectionIDForMedia("Published/Books") {
		t.Fatalf("collectionId = %q", item.CollectionID)
	}
	if item.Open == nil || item.Open.Mode != "pdf" || item.Open.URL != "/media/Published/Books/linux%20bible.pdf" {
		t.Fatalf("open target = %+v", item.Open)
	}
	if len(item.Authors) != 1 || item.Authors[0] != "Christopher Negus" {
		t.Fatalf("authors = %#v", item.Authors)
	}
	if !item.Capabilities.Open || !item.Capabilities.Favorite || item.Capabilities.Translate {
		t.Fatalf("capabilities = %+v", item.Capabilities)
	}
}

func TestCollectionsIncludeMediaWhenKiwixUnavailable(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer engine.Close()

	root := t.TempDir()
	seedCollection(t, root, "Published/Books", "book.pdf", "x",
		sidecar{Template: "pdf", Title: "Offline Book"})

	cols, err := itemTestServer(t, engine.URL).allCollections(&mediaDeps{root: root})
	if err != nil {
		t.Fatalf("allCollections: %v", err)
	}
	if len(cols) != 1 {
		t.Fatalf("collections = %d, want media-only collection: %+v", len(cols), cols)
	}
	if cols[0].ID != collectionIDForMedia("Published/Books") || cols[0].Title != "Books" {
		t.Fatalf("collection = %+v", cols[0])
	}
}

func TestItemOpenEndpointForMedia(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer engine.Close()

	root := t.TempDir()
	seedCollection(t, root, "Published/Video", "clip.mp4", "fake mp4",
		sidecar{Template: "video", Title: "Clip"})
	md := &mediaDeps{root: root}
	items, _ := md.scan("")
	item := mediaToItem(items[0])

	mux := http.NewServeMux()
	srv := itemTestServer(t, engine.URL)
	grantOpen(t, srv, "Published/Video")
	srv.registerItemRoutes(mux, md)
	req := httptest.NewRequest(http.MethodGet, "/api/items/"+item.ID+"/open", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var target OpenTarget
	if err := json.Unmarshal(rec.Body.Bytes(), &target); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if target.Mode != "video" || target.URL != "/media/Published/Video/clip.mp4" || target.ItemID != item.ID {
		t.Fatalf("open target = %+v", target)
	}
}

func TestItemSearchEndpointIncludesMedia(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer engine.Close()

	root := t.TempDir()
	seedCollection(t, root, "Published/Books", "linux.pdf", "fake pdf",
		sidecar{Template: "pdf", Title: "Linux Bible", Author: "Christopher Negus"})

	mux := http.NewServeMux()
	srv := itemTestServer(t, engine.URL)
	grantOpen(t, srv, "Published/Books")
	srv.registerItemRoutes(mux, &mediaDeps{root: root})
	req := httptest.NewRequest(http.MethodGet, "/api/items/search?q=linux", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Results []FederatedSearchResult `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("results = %d, want 1: %+v", len(payload.Results), payload.Results)
	}
	if payload.Results[0].ItemID == "" || payload.Results[0].Kind != "pdf" {
		t.Fatalf("result = %+v", payload.Results[0])
	}
}

func TestZIMItemIDRoundTrip(t *testing.T) {
	id := itemIDForZIM("wikipedia_es", "A/Saturno")
	payload, ok := decodeOpaque(id[len("zim:"):])
	if !ok {
		t.Fatal("could not decode zim item id")
	}
	if payload != "wikipedia_es/A/Saturno" {
		t.Fatalf("payload = %q", payload)
	}
	item := zimToItem(Library{ID: "wikipedia_es"}, "A/Saturno", "Saturno", "", "", 0)
	if item.Open == nil || item.Open.Mode != "iframe" || item.Open.URL != "/content/wikipedia_es/A/Saturno" {
		t.Fatalf("zim open = %+v", item.Open)
	}
}
