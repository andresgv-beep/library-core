package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andresgv-beep/nimos-library/download"
)

// TestSidecarEndToEnd ejerce la reacción real al evento done. No usa un servidor
// HTTP local porque el gestor de producción debe bloquear loopback por diseño
// (anti-SSRF); las pruebas del paquete download cubren esa cerradura de red.
func TestSidecarEndToEnd(t *testing.T) {
	body := []byte("fake mp4 bytes for the test, long enough to matter 0123456789")
	root := t.TempDir()
	sw := &sidecarWriter{root: root}
	dest := filepath.Join(root, "Cine", "caligari.mp4")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		t.Fatal(err)
	}
	sw.onJobEvent(download.Job{Status: download.StatusDone, DestPath: dest, OwnerKind: "manual"})

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
