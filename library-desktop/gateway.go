package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const gatewayConfigPath = "/__nimos/gateway"

type gatewayConfig struct {
	Target string `json:"target"`
}

func resolveShellTarget() (*url.URL, bool, bool, error) {
	remote := distributionMode == "remote"
	if raw := strings.TrimSpace(os.Getenv("NIMOS_LIBRARY_SERVER")); raw != "" {
		target, err := normalizeRemoteTarget(raw)
		return target, true, err == nil, err
	}
	if !remote {
		target, err := url.Parse(localCoreURL)
		return target, false, err == nil, err
	}

	raw, err := os.ReadFile(shellConfigPath())
	if os.IsNotExist(err) {
		return nil, true, false, nil
	}
	if err != nil {
		return nil, true, false, fmt.Errorf("leer configuracion del gateway: %w", err)
	}
	var cfg gatewayConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, true, false, fmt.Errorf("configuracion del gateway invalida: %w", err)
	}
	target, err := normalizeRemoteTarget(cfg.Target)
	return target, true, err == nil, err
}

func normalizeRemoteTarget(raw string) (*url.URL, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return nil, fmt.Errorf("escribe la direccion de Library Server")
	}
	target, err := url.Parse(raw)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return nil, fmt.Errorf("la direccion debe empezar por http:// o https://")
	}
	if target.User != nil || (target.Path != "" && target.Path != "/") || target.RawQuery != "" || target.Fragment != "" {
		return nil, fmt.Errorf("usa solo el origen de Library Server, sin ruta, usuario, query ni fragmento")
	}
	target.Path = ""
	return target, nil
}

func shellConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "Nimos Library", "gateway.json")
}

func saveGatewayTarget(target *url.URL) error {
	path := shellConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(gatewayConfig{Target: target.String()}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func (s *shell) installProxy(target *url.URL) {
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalDirector(r)
		r.Host = target.Host
		// Garantiza que ModifyResponse pueda inspeccionar el HTML de la SPA.
		r.Header.Del("Accept-Encoding")
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "Library Server no disponible", http.StatusServiceUnavailable)
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		if !isClientDocument(response) {
			return nil
		}
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		response.Body.Close()
		injection := `<script>window.__NIMOS_LIBRARY_SERVER__="";window.__NIMOS_LIBRARY_SHELL__=true;window.__NIMOS_LIBRARY_GATEWAY__=` + strconv.FormatBool(s.remote) + `;</script>`
		body = bytes.Replace(body, []byte("<head>"), []byte("<head>"+injection), 1)
		response.Body = io.NopCloser(bytes.NewReader(body))
		response.ContentLength = int64(len(body))
		response.Header.Set("Content-Length", strconv.Itoa(len(body)))
		return nil
	}

	s.mu.Lock()
	s.target = target
	s.proxy = proxy
	s.mu.Unlock()
}

func isClientDocument(response *http.Response) bool {
	if response.Request == nil || !strings.Contains(response.Header.Get("Content-Type"), "text/html") {
		return false
	}
	path := response.Request.URL.Path
	reserved := []string{"/api", "/content", "/media", "/panel", "/maps", "/mapdata", "/catalog", "/assets", "/pdfjs"}
	for _, prefix := range reserved {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return false
		}
	}
	return true
}

func (s *shell) targetString() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.target == nil {
		return ""
	}
	return s.target.String()
}

func (s *shell) serveGatewayConfig(w http.ResponseWriter, r *http.Request) {
	if !s.remote {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodGet {
		_ = json.NewEncoder(w).Encode(gatewayConfig{Target: s.targetString()})
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, `{"error":"metodo no permitido"}`, http.StatusMethodNotAllowed)
		return
	}

	var cfg gatewayConfig
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeGatewayError(w, http.StatusBadRequest, "peticion invalida")
		return
	}
	target, err := normalizeRemoteTarget(cfg.Target)
	if err != nil {
		writeGatewayError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := saveGatewayTarget(target); err != nil {
		writeGatewayError(w, http.StatusInternalServerError, "no se pudo guardar la configuracion")
		return
	}
	s.installProxy(target)
	s.configured.Store(true)
	s.ready.Store(false)
	s.startBoot()
	_ = json.NewEncoder(w).Encode(gatewayConfig{Target: target.String()})
}

func writeGatewayError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
