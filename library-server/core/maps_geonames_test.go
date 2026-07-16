package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildGeoNamesCitiesIndexSearchesOffline(t *testing.T) {
	var raw bytes.Buffer
	zw := zip.NewWriter(&raw)
	w, err := zw.Create("cities500.txt")
	if err != nil {
		t.Fatal(err)
	}
	// geonameid, name, asciiname, alternates, lat, lon, class, code,
	// country, cc2, admin1..admin4, population (19 columnas en el dump real).
	_, _ = w.Write([]byte("3128760\tBarcelona\tBarcelona\t\t41.38879\t2.15899\tP\tPPLA\tES\t\t56\tB\t\t\t1620343\t\t\tEurope/Madrid\t2025-01-01\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	zipPath, dbPath := filepath.Join(root, "cities500.zip"), filepath.Join(root, "geo.db")
	if err := os.WriteFile(zipPath, raw.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	count, err := buildGeoNamesCitiesIndex(zipPath, dbPath)
	if err != nil || count != 1 {
		t.Fatalf("build count=%d err=%v", count, err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hits := geoSearch(db, matchExpr(geoTokens("barcelona")), "barcelona")
	if len(hits) != 1 || hits[0].Name != "Barcelona" || hits[0].Lat == 0 || hits[0].Lon == 0 {
		t.Fatalf("hits inesperados: %+v", hits)
	}
}
