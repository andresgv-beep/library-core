package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleUpload_MomentsVideo(t *testing.T) {
	root := t.TempDir()
	h := (&Server{}).handleUpload(&uploadDeps{root: root})

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("source", "moments")
	mw.WriteField("collection", "Tutoriales")
	mw.WriteField("title", "Cómo funciona un ZIM")
	mw.WriteField("author", "Mi canal")
	mw.WriteField("tags", "tecnología, offline, tecnología") // duplicado a propósito
	mw.WriteField("duration", "615")
	fw, _ := mw.CreateFormFile("file", "demo.mp4")
	fw.Write([]byte("fake mp4 bytes"))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/admin/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// El fichero se guarda bajo Moments/<colección>/.
	media := filepath.Join(root, "Moments", "Tutoriales", "demo.mp4")
	if _, err := os.Stat(media); err != nil {
		t.Fatalf("no se guardó el vídeo: %v", err)
	}
	// Y su sidecar con la ficha.
	scRaw, err := os.ReadFile(filepath.Join(root, "Moments", "Tutoriales", "demo.json"))
	if err != nil {
		t.Fatalf("no se escribió el sidecar: %v", err)
	}
	var sc sidecar
	if err := json.Unmarshal(scRaw, &sc); err != nil {
		t.Fatalf("sidecar ilegible: %v", err)
	}
	if sc.Template != "video" {
		t.Errorf("template = %q, quería video", sc.Template)
	}
	if sc.Source != "moments" {
		t.Errorf("source = %q, quería moments", sc.Source)
	}
	if sc.Title != "Cómo funciona un ZIM" || sc.Author != "Mi canal" {
		t.Errorf("título/autor mal: %q / %q", sc.Title, sc.Author)
	}
	if sc.Duration != 615 {
		t.Errorf("duration = %d, quería 615", sc.Duration)
	}
	if len(sc.Tags) != 2 { // "tecnología, offline, tecnología" → 2 (dedup)
		t.Errorf("tags = %v, quería 2 sin duplicados", sc.Tags)
	}
	// collection.json de la carpeta.
	if _, err := os.Stat(filepath.Join(root, "Moments", "Tutoriales", "collection.json")); err != nil {
		t.Errorf("falta collection.json: %v", err)
	}
}

func TestHandleMediaUpdate(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Moments", "General")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "v.mp4"), []byte("bytes"), 0o644)
	os.WriteFile(filepath.Join(dir, "v.json"), []byte(`{"template":"video","media":"v.mp4","title":"viejo","author":"antiguo","source":"moments"}`), 0o644)

	h := (&Server{}).handleMediaUpdate(&uploadDeps{root: root})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("id", "Moments/General/v.json")
	mw.WriteField("title", "nuevo título")
	mw.WriteField("author", "nuevo autor")
	mw.WriteField("tags", "a, b, a")
	mw.Close()
	req := httptest.NewRequest("POST", "/api/admin/media/update", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "v.json"))
	var sc sidecar
	if err := json.Unmarshal(raw, &sc); err != nil {
		t.Fatal(err)
	}
	if sc.Title != "nuevo título" || sc.Author != "nuevo autor" {
		t.Errorf("metadatos no actualizados: %q / %q", sc.Title, sc.Author)
	}
	if len(sc.Tags) != 2 {
		t.Errorf("tags = %v, quería 2 (dedup)", sc.Tags)
	}
	if sc.Media != "v.mp4" || sc.Template != "video" || sc.Source != "moments" {
		t.Errorf("no se conservaron media/template/source: %+v", sc)
	}
}

func TestHandleUpload_Rejects(t *testing.T) {
	root := t.TempDir()
	h := (&Server{}).handleUpload(&uploadDeps{root: root})

	post := func(source, filename string) int {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		mw.WriteField("source", source)
		mw.WriteField("collection", "X")
		fw, _ := mw.CreateFormFile("file", filename)
		fw.Write([]byte("data"))
		mw.Close()
		req := httptest.NewRequest("POST", "/api/admin/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec.Code
	}

	if got := post("badapp", "a.mp4"); got != 400 {
		t.Errorf("app inválida → %d, quería 400", got)
	}
	if got := post("cabinet", "malware.exe"); got != 400 {
		t.Errorf("extensión no soportada → %d, quería 400", got)
	}
	if got := post("cabinet", "libro.pdf"); got != 200 {
		t.Errorf("pdf válido a cabinet → %d, quería 200", got)
	}
}

func TestHandleUploadDefaultsToBlocked(t *testing.T) {
	root := t.TempDir()
	s := testAuthServer(t, "")
	h := s.handleUpload(&uploadDeps{root: root})

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("source", "cabinet")
	mw.WriteField("collection", "Privada")
	mw.WriteField("title", "Documento interno")
	fw, _ := mw.CreateFormFile("file", "interno.pdf")
	fw.Write([]byte("%PDF fake"))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/admin/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if cfg := s.collectionAccess(collectionIDForMedia("Cabinet/Privada")); cfg.Access != "blocked" {
		t.Fatalf("una subida sin visibilidad nació %q, quería blocked", cfg.Access)
	}
}

func TestHandleUploadRejectsActiveCoverAndUsesDetectedExtension(t *testing.T) {
	post := func(t *testing.T, root, coverName string, coverBody []byte) *httptest.ResponseRecorder {
		t.Helper()
		h := (&Server{}).handleUpload(&uploadDeps{root: root})
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		mw.WriteField("source", "cabinet")
		mw.WriteField("collection", "Segura")
		fw, _ := mw.CreateFormFile("file", "documento.pdf")
		fw.Write([]byte("%PDF fake"))
		cw, _ := mw.CreateFormFile("cover", coverName)
		cw.Write(coverBody)
		mw.Close()
		req := httptest.NewRequest("POST", "/api/admin/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec
	}

	badRoot := t.TempDir()
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	if rec := post(t, badRoot, "ataque.svg", svg); rec.Code != 400 {
		t.Fatalf("SVG activo: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(badRoot, "Cabinet", "Segura", "documento.pdf")); !os.IsNotExist(err) {
		t.Fatal("una portada rechazada dejó el fichero principal huérfano")
	}

	goodRoot := t.TempDir()
	png := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, 32)...)
	if rec := post(t, goodRoot, "nombre-falso.svg", png); rec.Code != 200 {
		t.Fatalf("PNG válido: status=%d body=%s", rec.Code, rec.Body.String())
	}
	raw, err := os.ReadFile(filepath.Join(goodRoot, "Cabinet", "Segura", "documento.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sc sidecar
	if err := json.Unmarshal(raw, &sc); err != nil {
		t.Fatal(err)
	}
	if sc.Cover != "documento.cover.png" {
		t.Fatalf("cover=%q; debe usar la extensión detectada", sc.Cover)
	}
}

func TestMultipartRequestHasRealSizeLimit(t *testing.T) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("payload", string(bytes.Repeat([]byte("x"), 256)))
	mw.Close()
	req := httptest.NewRequest("POST", "/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	err := parseMultipartLimited(httptest.NewRecorder(), req, 64)
	if err == nil || !maxBytesError(err) {
		t.Fatalf("una petición sobre el límite no fue reconocida como demasiado grande: %v", err)
	}
}
