package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andresgv-beep/nimos-library/download"
)

// TestSidecarEndToEnd ejerce el cableado real: un download.Manager con el
// callback del sidecarWriter enganchado baja un fichero de un servidor local y,
// al terminar, debe aparecer la ficha .json y el collection.json al lado.
// owner_kind "manual" → sin red externa (determinista).
func TestSidecarEndToEnd(t *testing.T) {
	// Servidor que sirve un mp4 de pega (unos bytes cualquiera).
	body := []byte("fake mp4 bytes for the test, long enough to matter 0123456789")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Write(body)
	}))
	defer srv.Close()

	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "downloads.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("abrir db: %v", err)
	}
	defer db.Close()

	sw := &sidecarWriter{root: root}
	mgr, err := download.NewManager(db, 2, sw.onJobEvent)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	dest := filepath.Join(root, "Cine", "caligari.mp4")
	if _, err := mgr.Enqueue(srv.URL+"/caligari.mp4", dest, "manual", ""); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// El sidecar se escribe en una goroutine tras el `done`; esperamos a que aparezca.
	scPath := filepath.Join(root, "Cine", "caligari.json")
	if !waitForFile(scPath, 5*time.Second) {
		t.Fatalf("no apareció el sidecar %s", scPath)
	}

	raw, _ := os.ReadFile(scPath)
	var sc sidecar
	if err := json.Unmarshal(raw, &sc); err != nil {
		t.Fatalf("sidecar ilegible: %v", err)
	}
	if sc.Template != "video" {
		t.Errorf("template = %q, quería video", sc.Template)
	}
	if sc.Media != "caligari.mp4" {
		t.Errorf("media = %q, quería caligari.mp4", sc.Media)
	}
	if sc.Source != "manual" {
		t.Errorf("source = %q, quería manual", sc.Source)
	}
	if sc.Title != "caligari" {
		t.Errorf("title = %q, quería caligari (derivado del nombre)", sc.Title)
	}

	// collection.json de la carpeta, con el tipo derivado del item.
	collPath := filepath.Join(root, "Cine", "collection.json")
	if !waitForFile(collPath, 2*time.Second) {
		t.Fatalf("no apareció collection.json")
	}
	rawColl, _ := os.ReadFile(collPath)
	var coll collectionMeta
	if err := json.Unmarshal(rawColl, &coll); err != nil {
		t.Fatalf("collection.json ilegible: %v", err)
	}
	if coll.Type != "video" || coll.Template != "video" || coll.Title != "Cine" {
		t.Errorf("collection.json mal: %+v", coll)
	}
}

func waitForFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
