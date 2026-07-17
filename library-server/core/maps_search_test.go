package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeMapSearchFixture(t *testing.T, withGeo bool) (*Server, *mapManager, *sql.DB) {
	t.Helper()
	root := t.TempDir()
	active := &mapInstall{
		File: "iberia-z14.pmtiles", RegionID: "iberia", Name: "España y Portugal",
		BBox: [4]float64{-10.2, 35.5, 4.8, 44.3}, Center: [2]float64{-3.5, 40.2}, MaxZoom: 14,
	}
	if err := os.WriteFile(filepath.Join(root, active.File), []byte("PMTiles fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &mapManager{root: root}
	if err := m.saveState(mapState{Active: active}); err != nil {
		t.Fatal(err)
	}
	server := &Server{mapsDir: root}
	if !withGeo {
		return server, m, nil
	}
	db := makeGeoTestDB(t, filepath.Join(root, "geo.db"), [][]any{
		{"Madrid", "place:ppla", 40.4168, -3.7038, "Comunidad de Madrid ES", "ES", 3_300_000},
	})
	server.geo = db
	return server, m, db
}

func requestMapSearch(t *testing.T, s *Server, m *mapManager, target string) (int, mapSearchResponse) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleMapSearch(m)(rec, httptest.NewRequest(http.MethodGet, target, nil))
	var response mapSearchResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("respuesta no valida: %v body=%s", err, rec.Body.String())
		}
	}
	return rec.Code, response
}

func TestMapSearchRadiusZeroReturnsExactLocation(t *testing.T) {
	s, m, db := makeMapSearchFixture(t, true)
	defer db.Close()
	status, response := requestMapSearch(t, s, m, "/api/maps/search?q=Madrid&radius=0")
	if status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	if !response.Available || response.Reason != "" || response.Location == nil {
		t.Fatalf("respuesta inesperada: %+v", response)
	}
	if response.Location.Name != "Madrid" || response.Location.MatchQuality != "exact" {
		t.Fatalf("ubicacion inesperada: %+v", response.Location)
	}
	if response.Radius != 0 || len(response.POIs) != 0 {
		t.Fatalf("radius=0 no debe devolver POI: %+v", response)
	}
	if response.Map == nil || response.Map.File != "iberia-z14.pmtiles" || response.Map.MaxZoom != 14 {
		t.Fatalf("mapa inesperado: %+v", response.Map)
	}
}

func TestMapSearchRadiusZeroReturnsStrongStreet(t *testing.T) {
	s, m, db := makeMapSearchFixture(t, true)
	defer db.Close()
	streets := makeGeoTestDB(t, streetIndexPath(m.root, "iberia-z14.pmtiles"), [][]any{
		{"Calle de Alcalá", "street:major_road", 40.4196, -3.6920, "Madrid", "", 0},
	})
	if err := streets.Close(); err != nil {
		t.Fatal(err)
	}
	status, response := requestMapSearch(t, s, m, "/api/maps/search?q=Calle%20de%20Alcal%C3%A1%2042%20Madrid&radius=0")
	if status != http.StatusOK || !response.Available || response.Location == nil {
		t.Fatalf("respuesta inesperada: status=%d response=%+v", status, response)
	}
	if response.Location.Name != "Calle de Alcalá" || response.Location.MatchQuality != "strong" || response.Location.HouseNumber != "42" || !response.Location.Approximate {
		t.Fatalf("calle inesperada: %+v", response.Location)
	}
}

func TestMapSearchThematicQueryDoesNotCreateLocation(t *testing.T) {
	s, m, db := makeMapSearchFixture(t, true)
	defer db.Close()
	status, response := requestMapSearch(t, s, m, "/api/maps/search?q=historia%20de%20Madrid&radius=0")
	if status != http.StatusOK || response.Available || response.Reason != "no_match" {
		t.Fatalf("respuesta inesperada: status=%d response=%+v", status, response)
	}
}

func TestGeocodeContractDoesNotExposeMapSearchMetadata(t *testing.T) {
	s, _, db := makeMapSearchFixture(t, true)
	defer db.Close()
	rec := httptest.NewRecorder()
	s.handleGeocode(rec, httptest.NewRequest(http.MethodGet, "/api/maps/geocode?q=Madrid", nil))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "matchQuality") || strings.Contains(rec.Body.String(), "available") {
		t.Fatalf("contrato geocode alterado: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var hits []GeoHit
	if err := json.Unmarshal(rec.Body.Bytes(), &hits); err != nil || len(hits) != 1 || hits[0].Name != "Madrid" {
		t.Fatalf("geocode inesperado: hits=%+v err=%v", hits, err)
	}
}

func TestMapSearchWithoutMapDegradesNormally(t *testing.T) {
	s := &Server{}
	m := &mapManager{root: t.TempDir()}
	status, response := requestMapSearch(t, s, m, "/api/maps/search?q=Madrid&radius=0")
	if status != http.StatusOK || response.Available || response.Reason != "no_map" {
		t.Fatalf("respuesta inesperada: status=%d response=%+v", status, response)
	}
}

func TestMapSearchWithoutGeocoderDegradesNormally(t *testing.T) {
	s, m, _ := makeMapSearchFixture(t, false)
	status, response := requestMapSearch(t, s, m, "/api/maps/search?q=Madrid&radius=0")
	if status != http.StatusOK || response.Available || response.Reason != "no_geocoder" {
		t.Fatalf("respuesta inesperada: status=%d response=%+v", status, response)
	}
}

func TestMapSearchRejectsInvalidRadius(t *testing.T) {
	s, m, db := makeMapSearchFixture(t, true)
	defer db.Close()
	rec := httptest.NewRecorder()
	s.handleMapSearch(m)(rec, httptest.NewRequest(http.MethodGet, "/api/maps/search?q=Madrid&radius=5500", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGeoMatchQualityAvoidsThematicFalsePositive(t *testing.T) {
	hit := GeoHit{Name: "Madrid", Kind: "place:ppla", Context: "Comunidad de Madrid ES"}
	if quality := geoMatchQuality("Madrid", hit); quality != "exact" {
		t.Fatalf("exacta=%s", quality)
	}
	if quality := geoMatchQuality("historia de Madrid", hit); quality != "weak" {
		t.Fatalf("tematica=%s", quality)
	}
	street := GeoHit{Name: "Calle de Alcalá", Kind: "street:major_road", Context: "Madrid"}
	if quality := geoMatchQuality("Calle de Alcalá 42 Madrid", street); quality != "strong" {
		t.Fatalf("calle=%s", quality)
	}
}

func TestRankMapSearchLocationsPrefersExactPlace(t *testing.T) {
	locations := []mapSearchLocation{
		{GeoHit: GeoHit{Name: "Madrid", Kind: "street:minor_road", Context: "Ceclavin"}, MatchQuality: "exact"},
		{GeoHit: GeoHit{Name: "Madrid - Cadiz", Kind: "street:rail"}, MatchQuality: "strong"},
		{GeoHit: GeoHit{Name: "Madrid", Kind: "place:pplc", Context: "Madrid ES"}, MatchQuality: "exact"},
	}
	rankMapSearchLocations(locations)
	if locations[0].Kind != "place:pplc" {
		t.Fatalf("expected exact place first, got %#v", locations[0])
	}
	if locations[2].MatchQuality != "strong" {
		t.Fatalf("expected strong match last, got %#v", locations[2])
	}
}
