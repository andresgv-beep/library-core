package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNormalizeRemoteTarget(t *testing.T) {
	t.Parallel()
	valid, err := normalizeRemoteTarget(" https://library.example/ ")
	if err != nil || valid.String() != "https://library.example" {
		t.Fatalf("target valido: %v, %v", valid, err)
	}
	for _, raw := range []string{"", "library.example", "ftp://library.example", "https://user@library.example", "https://library.example/panel", "https://library.example?q=1"} {
		if _, err := normalizeRemoteTarget(raw); err == nil {
			t.Errorf("se acepto target invalido %q", raw)
		}
	}
}

func TestGatewayRewritesHostAndOnlyInjectsClient(t *testing.T) {
	t.Parallel()
	var receivedHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHost = r.Host
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><html><head></head><body>ok</body></html>")
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)

	s := &shell{remote: true}
	s.installProxy(target)
	s.configured.Store(true)
	s.ready.Store(true)
	server := httptest.NewServer(s)
	defer server.Close()

	root := mustBody(t, server.URL+"/")
	if !strings.Contains(root, "__NIMOS_LIBRARY_SERVER__") || !strings.Contains(root, "__NIMOS_LIBRARY_GATEWAY__=true") {
		t.Fatalf("el documento cliente no recibio la configuracion del shell: %s", root)
	}
	if receivedHost != target.Host {
		t.Fatalf("Host recibido = %q; esperado %q", receivedHost, target.Host)
	}

	content := mustBody(t, server.URL+"/content/wiki/A/Portada")
	if strings.Contains(content, "__NIMOS_LIBRARY_SERVER__") {
		t.Fatal("el gateway inyecto codigo dentro del contenido ZIM")
	}
}

func TestPanelShellStartsAtPanel(t *testing.T) {
	t.Parallel()
	paths := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<html><head></head><body>panel</body></html>")
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)

	s := &shell{startPath: "/panel/"}
	s.installProxy(target)
	s.configured.Store(true)
	s.ready.Store(true)
	server := httptest.NewServer(s)
	defer server.Close()
	_ = mustBody(t, server.URL+"/")
	if path := <-paths; path != "/panel/" {
		t.Fatalf("ruta inicial = %q; esperada /panel/", path)
	}
}

func mustBody(t *testing.T, address string) string {
	t.Helper()
	response, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
