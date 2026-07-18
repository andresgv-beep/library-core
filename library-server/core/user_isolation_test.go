package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchCacheKeyIncludesVisibleCollections(t *testing.T) {
	admin := []Library{{ID: "publica"}, {ID: "privada"}}
	guest := []Library{{ID: "publica"}}
	if searchVisibilityCacheKey("atlas", admin) == searchVisibilityCacheKey("atlas", guest) {
		t.Fatal("la caché de admin e invitado no puede compartir clave")
	}
	// El orden del catálogo no debe fragmentar la caché si la visibilidad es igual.
	reversed := []Library{{ID: "privada"}, {ID: "publica"}}
	if searchVisibilityCacheKey("atlas", admin) != searchVisibilityCacheKey("atlas", reversed) {
		t.Fatal("la misma visibilidad debería producir la misma clave")
	}
}

func TestMediaImageSearchAppliesVisibilityFilter(t *testing.T) {
	root := t.TempDir()
	seedCollection(t, root, "Publico", "visible.mp4", "video",
		sidecar{Template: "video", Title: "Viaje visible", Cover: "visible.jpg"})
	seedCollection(t, root, "Privado", "secreto.mp4", "video",
		sidecar{Template: "video", Title: "Viaje secreto", Cover: "secreto.jpg"})
	for _, path := range []string{
		filepath.Join(root, "Publico", "visible.jpg"),
		filepath.Join(root, "Privado", "secreto.jpg"),
	} {
		if err := os.WriteFile(path, []byte("jpg"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	md := &mediaDeps{root: root}
	hits, err := md.searchImages("viaje", func(it mediaItem) bool { return it.Collection == "Publico" })
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Title != "Viaje visible" {
		t.Fatalf("la búsqueda de imágenes filtró mal: %+v", hits)
	}
}

func TestGuestPersonalStateIsNotShared(t *testing.T) {
	s := testAuthServer(t, "")
	first := &http.Cookie{Name: guestCookie, Value: strings.Repeat("a", 32)}
	second := &http.Cookie{Name: guestCookie, Value: strings.Repeat("b", 32)}

	put := httptest.NewRequest(http.MethodPut, "/api/favorites", strings.NewReader(`{"lib":"wiki","path":"A/Uno","title":"Uno"}`))
	put.AddCookie(first)
	putRec := httptest.NewRecorder()
	s.handleFavorites(putRec, put)
	if putRec.Code != http.StatusOK {
		t.Fatalf("guardar favorito: %d %s", putRec.Code, putRec.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/api/favorites", nil)
	get.AddCookie(second)
	getRec := httptest.NewRecorder()
	s.handleFavorites(getRec, get)
	var favorites []Fav
	if err := json.Unmarshal(getRec.Body.Bytes(), &favorites); err != nil {
		t.Fatal(err)
	}
	if len(favorites) != 0 {
		t.Fatalf("el segundo invitado ve datos del primero: %+v", favorites)
	}
}
