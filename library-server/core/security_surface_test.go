package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTopLevelPagesDenyFraming(t *testing.T) {
	w := httptest.NewRecorder()
	setTopLevelIsolation(w)
	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options=%q", got)
	}
	if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") {
		t.Fatalf("CSP no bloquea frames: %q", got)
	}
}

func TestMapDataOnlyServesPMTilesFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "world.pmtiles"), []byte("0123456789"), 0o644)
	os.WriteFile(filepath.Join(dir, "geo.db"), []byte("SQLite secreto"), 0o644)
	h := mapDataHandler(dir)

	for _, target := range []string{"/mapdata/", "/mapdata/geo.db", "/mapdata/sub/world.pmtiles"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s devolvió %d; quería 404", target, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/mapdata/world.pmtiles", nil)
	r.Header.Set("Range", "bytes=2-5")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusPartialContent || w.Body.String() != "2345" {
		t.Fatalf("PMTiles con Range: status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestMediaResponsesDisableActiveContent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Cabinet", "Publica")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "cover.svg"), []byte(`<svg><script>alert(1)</script></svg>`), 0o644)
	s := &Server{}
	h := s.gateMediaFile(&mediaDeps{root: root})
	r := httptest.NewRequest(http.MethodGet, "/media/Cabinet/Publica/cover.svg", nil)
	r = withUser(r, machineUser())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("cabeceras insuficientes: %#v", w.Header())
	}
}

func TestKiwixHostBoundary(t *testing.T) {
	valid := []string{"kiwix.org", "library.kiwix.org", "DOWNLOAD.KIWIX.ORG", "library.kiwix.org."}
	invalid := []string{"kiwix.org.evil.com", "evilkiwix.org", "kiwix.org@evil.com", ""}
	for _, host := range valid {
		if !isKiwixHost(host) {
			t.Errorf("rechazó host Kiwix válido %q", host)
		}
	}
	for _, host := range invalid {
		if isKiwixHost(host) {
			t.Errorf("aceptó host ajeno %q", host)
		}
	}
}

func TestMachineTokenComparisonStillAuthenticatesExactToken(t *testing.T) {
	s := &Server{token: "secreto-largo"}
	r := httptest.NewRequest(http.MethodPost, "/api/admin/test", nil)
	r.Header.Set("X-Nimos-Token", "secreto-largo")
	if !s.hasMachineToken(r) {
		t.Fatal("el token exacto debe autenticar")
	}
	r.Header.Set("X-Nimos-Token", "secreto-largX")
	if s.hasMachineToken(r) {
		t.Fatal("un token distinto no debe autenticar")
	}
}
