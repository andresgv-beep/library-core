package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// seedCollection crea, bajo root, una colección con un fichero + su sidecar.
func seedCollection(t *testing.T, root, coll, media, content string, sc sidecar) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(coll))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, media), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sc.Media = media
	if sc.Source == "" { // por defecto, contenido propio para que el escáner lo tome
		sc.Source = "cabinet"
	}
	raw, _ := json.MarshalIndent(sc, "", "  ")
	base := media[:len(media)-len(filepath.Ext(media))]
	if err := os.WriteFile(filepath.Join(dir, base+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	// collection.json de la carpeta (el escáner debe IGNORARLO como item).
	coljson, _ := json.Marshal(collectionMeta{Type: "pdf", Template: "pdf", Title: filepath.Base(dir)})
	os.WriteFile(filepath.Join(dir, "collection.json"), coljson, 0o644)
}

func TestMediaScan(t *testing.T) {
	root := t.TempDir()
	seedCollection(t, root, "Biblioteca/Libros", "westminster.pdf", "%PDF-1.4 fake",
		sidecar{Template: "pdf", Title: "Westminster Abbey", Author: "Stanley", Source: "cabinet",
			SourceURL: "https://example.org/item/x"})
	seedCollection(t, root, "Biblioteca/Video", "caligari.mp4", "fake mp4",
		sidecar{Template: "video", Title: "Caligari", Source: "moments"})

	md := &mediaDeps{root: root}

	// Sin filtro: los dos items, y NUNCA el collection.json.
	all, err := md.scan("")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("scan sin filtro = %d items, quería 2: %+v", len(all), all)
	}

	// Filtro por colección.
	libros, _ := md.scan("Biblioteca/Libros")
	if len(libros) != 1 || libros[0].Title != "Westminster Abbey" {
		t.Fatalf("scan filtrado mal: %+v", libros)
	}
	it := libros[0]
	if it.Collection != "Biblioteca/Libros" {
		t.Errorf("collection = %q", it.Collection)
	}
	if it.Template != "pdf" || it.Media != "westminster.pdf" {
		t.Errorf("item mal: template=%q media=%q", it.Template, it.Media)
	}
	if it.MediaURL != "/media/Biblioteca/Libros/westminster.pdf" {
		t.Errorf("media_url = %q", it.MediaURL)
	}
	if it.ID != "Biblioteca/Libros/westminster.json" {
		t.Errorf("id = %q", it.ID)
	}
}

func TestMediaScanEscapesSpaces(t *testing.T) {
	root := t.TempDir()
	seedCollection(t, root, "Biblioteca/Libros", "un libro con espacios.pdf", "x",
		sidecar{Template: "pdf", Title: "Con espacios", Source: "cabinet"})
	items, _ := (&mediaDeps{root: root}).scan("")
	if len(items) != 1 {
		t.Fatalf("quería 1 item, hay %d", len(items))
	}
	if items[0].MediaURL != "/media/Biblioteca/Libros/un%20libro%20con%20espacios.pdf" {
		t.Errorf("media_url sin escapar: %q", items[0].MediaURL)
	}
}

func TestMediaServeAndRange(t *testing.T) {
	root := t.TempDir()
	body := "0123456789ABCDEF"
	seedCollection(t, root, "Biblioteca/Libros", "f.pdf", body, sidecar{Template: "pdf", Title: "T", Source: "cabinet"})

	md := &mediaDeps{root: root}
	srv := httptest.NewServer(http.HandlerFunc(md.handleMediaFile))
	defer srv.Close()

	// Descarga completa.
	resp, err := http.Get(srv.URL + "/media/Biblioteca/Libros/f.pdf")
	if err != nil {
		t.Fatal(err)
	}
	got := readAll(t, resp)
	resp.Body.Close()
	if got != body {
		t.Errorf("cuerpo completo = %q, quería %q", got, body)
	}

	// Petición con Range (seek): bytes 5-9 → "56789".
	req, _ := http.NewRequest("GET", srv.URL+"/media/Biblioteca/Libros/f.pdf", nil)
	req.Header.Set("Range", "bytes=5-9")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusPartialContent {
		t.Fatalf("status Range = %d, quería 206", resp2.StatusCode)
	}
	if part := readAll(t, resp2); part != "56789" {
		t.Errorf("Range = %q, quería 56789", part)
	}
}

func TestMediaTraversalBlocked(t *testing.T) {
	root := t.TempDir()
	// Fichero secreto FUERA de la raíz.
	secret := filepath.Join(filepath.Dir(root), "secret.txt")
	os.WriteFile(secret, []byte("no me deberías ver"), 0o644)
	defer os.Remove(secret)

	md := &mediaDeps{root: root}
	// safeResolve debe rechazar el escape.
	if _, err := md.safeResolve("../secret.txt"); err == nil {
		t.Errorf("safeResolve permitió escapar de la raíz")
	}
	if _, err := md.safeResolve("Biblioteca/../../secret.txt"); err == nil {
		t.Errorf("safeResolve permitió escapar con subcarpeta")
	}
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := make([]byte, 0, 64)
	tmp := make([]byte, 32)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return string(buf)
}
