// admin_translate.go — Gestión de modelos de traducción para el Panel.
//
// translate-wrap posee el binario translateLocally y expone /models/available,
// /models/download y /models/remove. El shim solo hace de puerta admin: proxya
// esas rutas bajo /api/admin/translate/*. La descarga puede tardar (red), por
// eso usa un cliente con timeout amplio, no el de 30s del resto.

package main

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

var adminTranslateClient = &http.Client{Timeout: 15 * time.Minute}

func (s *Server) registerAdminTranslateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/translate/available", s.handleAdminTranslateAvailable)
	mux.HandleFunc("/api/admin/translate/download", s.handleAdminTranslateDownload)
	mux.HandleFunc("/api/admin/translate/remove", s.handleAdminTranslateRemove)
}

func (s *Server) handleAdminTranslateAvailable(w http.ResponseWriter, r *http.Request) {
	if s.translate == "" {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "traductor no disponible"})
		return
	}
	resp, err := adminTranslateClient.Get(s.translate + "/models/available")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	relayUpstream(w, resp)
}

func (s *Server) handleAdminTranslateDownload(w http.ResponseWriter, r *http.Request) {
	s.proxyModelOp(w, r, "/models/download")
}

func (s *Server) handleAdminTranslateRemove(w http.ResponseWriter, r *http.Request) {
	s.proxyModelOp(w, r, "/models/remove")
}

func (s *Server) proxyModelOp(w http.ResponseWriter, r *http.Request, path string) {
	if s.translate == "" {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "traductor no disponible"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	resp, err := adminTranslateClient.Post(s.translate+path, "application/json", bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	relayUpstream(w, resp)
}

// relayUpstream copia estado + cuerpo JSON de la respuesta de translate-wrap.
func relayUpstream(w http.ResponseWriter, resp *http.Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
