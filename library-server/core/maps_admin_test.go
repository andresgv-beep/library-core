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

func TestMapCatalogCoversWorldCategories(t *testing.T) {
	want := []string{"Europa", "Asia", "Africa", "America del Norte", "America del Sur", "Oceania"}
	for _, category := range want {
		found := false
		for _, region := range mapCatalog {
			found = found || region.Category == category
		}
		if !found {
			t.Fatalf("falta categoria %s", category)
		}
	}
}

func TestMapActivationFeedsPublicViewerConfig(t *testing.T) {
	root := t.TempDir()
	file := "new-zealand-z13.pmtiles"
	if err := os.WriteFile(filepath.Join(root, file), []byte("PMTiles test"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &mapManager{root: root}
	body := strings.NewReader(`{"file":"new-zealand-z13.pmtiles"}`)
	activate := httptest.NewRecorder()
	m.handleActivate(activate, httptest.NewRequest(http.MethodPost, "/api/admin/maps/activate", body))
	if activate.Code != http.StatusOK {
		t.Fatalf("activate status=%d body=%s", activate.Code, activate.Body.String())
	}
	viewer := httptest.NewRecorder()
	m.handlePublicConfig(viewer, httptest.NewRequest(http.MethodGet, "/api/maps/config", nil))
	if viewer.Code != http.StatusOK {
		t.Fatalf("viewer status=%d body=%s", viewer.Code, viewer.Body.String())
	}
	var result struct {
		Name    string     `json:"name"`
		URL     string     `json:"url"`
		MaxZoom int        `json:"maxZoom"`
		Center  [2]float64 `json:"center"`
	}
	if err := json.Unmarshal(viewer.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Name != "Nueva Zelanda" || result.URL != "/mapdata/"+file || result.MaxZoom != 13 || result.Center == [2]float64{} {
		t.Fatalf("config inesperada: %+v", result)
	}
}

func TestMapDeleteRejectsTraversal(t *testing.T) {
	m := &mapManager{root: t.TempDir()}
	recorder := httptest.NewRecorder()
	m.handleDelete(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/maps/delete", strings.NewReader(`{"file":"../secret.pmtiles"}`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
