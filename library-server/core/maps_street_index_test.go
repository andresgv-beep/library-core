package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func makeGeoTestDB(t *testing.T, path string, rows [][]any) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE VIRTUAL TABLE geo USING fts5(
		name, kind UNINDEXED, lat UNINDEXED, lon UNINDEXED, context, display UNINDEXED, popularity UNINDEXED,
		tokenize='unicode61 remove_diacritics 2')`)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if _, err := db.Exec(`INSERT INTO geo(name,kind,lat,lon,context,display,popularity) VALUES(?,?,?,?,?,?,?)`, row...); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestGeocodeUsesStreetIndexOfActiveMap(t *testing.T) {
	root := t.TempDir()
	mapFile := "iberia-z15.pmtiles"
	cityDB := makeGeoTestDB(t, filepath.Join(root, "geo.db"), [][]any{{"Barcelona", "place:ppla", 41.38, 2.15, "Barcelona ES", "ES", 1_600_000}})
	defer cityDB.Close()
	streets := makeGeoTestDB(t, streetIndexPath(root, mapFile), [][]any{{"Carrer de Mallorca", "street:minor_road", 41.397, 2.167, "Barcelona", "", 0}})
	_ = streets.Close()
	s := &Server{geo: cityDB, mapsDir: root}
	req := httptest.NewRequest(http.MethodGet, "/api/maps/geocode?q=Carrer%20de%20Mallorca%20Barcelona&map="+mapFile+"&bbox=-10.2,35.5,4.8,44.3", nil)
	rec := httptest.NewRecorder()
	s.handleGeocode(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var hits []GeoHit
	if err := json.Unmarshal(rec.Body.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].Name != "Carrer de Mallorca" || hits[0].Context != "Barcelona" {
		t.Fatalf("resultado de calle inesperado: %+v", hits)
	}
}
