package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTemplateForExt(t *testing.T) {
	cases := []struct {
		path         string
		wantTemplate string
		wantColl     string
	}{
		{"Cine/caligari.mp4", "video", "video"},
		{"a/b/clip.MP4", "video", "video"}, // extensión en mayúsculas
		{"pod.mp3", "audio", "audio"},
		{"foto.JPG", "gallery", "images"},
		{"libro.pdf", "pdf", "pdf"},
		{"novela.epub", "reader", "documents"},
		{"apunte.md", "reader", "documents"},
		{"raro.xyz", "", ""}, // desconocido → sin plantilla
		{"sinext", "", ""},   // sin extensión
	}
	for _, c := range cases {
		gotT, gotC := templateForExt(c.path)
		if gotT != c.wantTemplate || gotC != c.wantColl {
			t.Errorf("templateForExt(%q) = (%q,%q), quería (%q,%q)",
				c.path, gotT, gotC, c.wantTemplate, c.wantColl)
		}
	}
}

func TestSidecarPathFor(t *testing.T) {
	cases := []struct{ dest, want string }{
		{"/pool/Cine/Clasicos/caligari.mp4", "/pool/Cine/Clasicos/caligari.json"},
		{"tesla.pdf", "tesla.json"},
		{"sinext", "sinext.json"},
		{"con.varios.puntos.mp4", "con.varios.puntos.json"},
	}
	for _, c := range cases {
		if got := sidecarPathFor(c.dest); got != c.want {
			t.Errorf("sidecarPathFor(%q) = %q, quería %q", c.dest, got, c.want)
		}
	}
}

func TestTitleFromFilename(t *testing.T) {
	cases := []struct{ path, want string }{
		{"das_cabinet-des-doktor.mp4", "das cabinet des doktor"},
		{"Cosmos Ep 1.mp4", "Cosmos Ep 1"},
		{"a/b/mi_video.webm", "mi video"},
		{"  espacios  .pdf", "espacios"},
	}
	for _, c := range cases {
		if got := titleFromFilename(c.path); got != c.want {
			t.Errorf("titleFromFilename(%q) = %q, quería %q", c.path, got, c.want)
		}
	}
}

func TestKeywordsFromSubjects(t *testing.T) {
	got := keywordsFromSubjects([]string{"Horror", "horror", " Silent Film ", "", "Expressionism"})
	want := []string{"horror", "silent film", "expressionism"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keywordsFromSubjects = %v, quería %v", got, want)
	}
	if keywordsFromSubjects(nil) != nil {
		t.Errorf("keywordsFromSubjects(nil) debería ser nil")
	}
}

func TestWriteJSONFileIfAbsent_NoClobber(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "item.json")

	// Primera escritura: crea el fichero.
	written, err := writeJSONFileIfAbsent(path, sidecar{Title: "original", Template: "video", Source: "manual"})
	if err != nil || !written {
		t.Fatalf("primera escritura: written=%v err=%v", written, err)
	}

	// Segunda escritura con contenido distinto: NO debe pisar (el usuario manda).
	written2, err := writeJSONFileIfAbsent(path, sidecar{Title: "PISADO", Template: "otro", Source: "manual"})
	if err != nil {
		t.Fatalf("segunda escritura devolvió error: %v", err)
	}
	if written2 {
		t.Errorf("segunda escritura pisó un fichero existente (no debería)")
	}

	raw, _ := os.ReadFile(path)
	var back sidecar
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("no se pudo releer el sidecar: %v", err)
	}
	if back.Title != "original" {
		t.Errorf("el contenido cambió: título = %q, quería \"original\"", back.Title)
	}

	// No debe quedar ningún .tmp suelto.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("quedó un fichero .tmp residual")
	}
}
