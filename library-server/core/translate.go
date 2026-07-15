// Nimos Translate — traducción de artículos offline (TRANSLATE.md).
//
// El shim NO traduce: compone un motor maduro (Bergamot/translateLocally) que
// vive detrás como sidecar, igual que kiwix. El motor se descubre por una URL
// (TRANSLATE_URL); si está vacía o no responde, la traducción queda desactivada
// y la UI oculta el toggle (detección tipo Maps).
//
// Contrato del sidecar-motor (HTTP fino sobre translateLocally):
//
//	GET  {TRANSLATE_URL}/languages          -> {"pairs":[{"from":"en","to":"es"}]}
//	POST {TRANSLATE_URL}/translate          -> {"texts":[...]}
//	     body: {"from":"en","to":"es","texts":[...]}
package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// ─── Contrato UI ↔ shim (TRANSLATE.md §3) ─────────────────────────────────────

type transSegIn struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type transReq struct {
	Lib        string       `json:"lib"`
	Path       string       `json:"path"`
	To         string       `json:"to"`
	SourceHint string       `json:"sourceHint"`
	HTML       bool         `json:"html"` // segmentos son HTML → preservar enlaces
	Segments   []transSegIn `json:"segments"`
}

type transSegOut struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type transResp struct {
	To       string        `json:"to"`
	Source   string        `json:"source"`
	Segments []transSegOut `json:"segments"`
}

// ─── Detección de motor: /api/translate/languages ─────────────────────────────

func (s *Server) handleTranslateLanguages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo no permitido"})
		return
	}
	if s.translate == "" {
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	resp, err := s.http.Get(s.translate + "/languages")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	var payload struct {
		Pairs []map[string]string `json:"pairs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"available": true, "pairs": payload.Pairs})
}

// ─── Traducción: /api/translate ───────────────────────────────────────────────

func (s *Server) handleTranslate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo no permitido"})
		return
	}
	if s.translate == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "traductor no disponible"})
		return
	}
	var req transReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "peticion invalida"})
		return
	}
	req.To = strings.TrimSpace(req.To)
	if req.To == "" || len(req.Segments) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "faltan idioma destino o segmentos"})
		return
	}
	source := firstNonEmpty(normalizeLang(req.SourceHint), s.libLang(req.Lib), "en")

	// Traducir es CPU-caro → puerta de concurrencia (igual que la búsqueda global).
	if !s.acquireTranslate(w, r) {
		return
	}
	defer s.releaseTranslate()

	// Caché primero: solo se traduce lo que falta (o cambió el origen).
	cached, err := s.store.GetTranslations(req.Lib, req.Path, req.To)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errmap(err))
		return
	}
	out := make([]transSegOut, len(req.Segments))
	var missIdx []int
	var missTexts []string
	for i, seg := range req.Segments {
		out[i] = transSegOut{ID: seg.ID}
		h := srcHash(seg.Text)
		if c, ok := cached[seg.ID]; ok && c.SrcHash == h {
			out[i].Text = c.Text
			continue
		}
		missIdx = append(missIdx, i)
		missTexts = append(missTexts, seg.Text)
	}

	if len(missTexts) > 0 {
		translated, terr := s.engineTranslate(source, req.To, missTexts, req.HTML)
		if terr != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": terr.Error()})
			return
		}
		if len(translated) != len(missTexts) {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "respuesta del motor incompleta"})
			return
		}
		ts := now()
		for j, idx := range missIdx {
			seg := req.Segments[idx]
			out[idx].Text = translated[j]
			if err := s.store.PutTranslation(req.Lib, req.Path, req.To, seg.ID, srcHash(seg.Text), translated[j], ts); err != nil {
				writeJSON(w, http.StatusInternalServerError, errmap(err))
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, transResp{To: req.To, Source: source, Segments: out})
}

// engineTranslate llama al sidecar-motor para traducir un lote de textos.
func (s *Server) engineTranslate(from, to string, texts []string, html bool) ([]string, error) {
	body, err := json.Marshal(map[string]any{"from": from, "to": to, "html": html, "texts": texts})
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Post(s.translate+"/translate", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("motor de traduccion respondio " + resp.Status)
	}
	var payload struct {
		Texts []string `json:"texts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Texts, nil
}

// ─── Detección de idioma origen ───────────────────────────────────────────────

// libLang deduce el idioma de un ZIM desde el catálogo (Library.Lang, ISO 639-2)
// y lo normaliza a 639-1. Best-effort: si falla, devuelve "".
func (s *Server) libLang(id string) string {
	if id == "" {
		return ""
	}
	libs, err := s.fetchLibraries()
	if err != nil {
		return ""
	}
	for _, l := range libs {
		if l.ID == id {
			return normalizeLang(l.Lang)
		}
	}
	return ""
}

// normalizeLang lleva códigos ISO 639-2 (eng, spa…) a 639-1 (en, es…) que usa el
// motor. Si ya es 639-1 lo respeta; "mul"/desconocido → "".
func normalizeLang(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return ""
	}
	switch code {
	case "eng":
		return "en"
	case "spa":
		return "es"
	case "fra", "fre":
		return "fr"
	case "deu", "ger":
		return "de"
	case "ita":
		return "it"
	case "por":
		return "pt"
	case "rus":
		return "ru"
	case "cat":
		return "ca"
	case "nld", "dut":
		return "nl"
	case "mul", "und":
		return ""
	}
	if len(code) == 2 {
		return code
	}
	return ""
}

// srcHash: huella del texto origen para invalidar caché si el ZIM cambia.
func srcHash(text string) string {
	sum := sha1.Sum([]byte(text))
	return hex.EncodeToString(sum[:])
}

// ─── Puerta de concurrencia (TRANSLATE.md §4, patrón de acquireSearch) ─────────

const (
	translateQueueMax = 8
	translateWaitMax  = 20 * time.Second
)

func (s *Server) acquireTranslate(w http.ResponseWriter, r *http.Request) bool {
	select {
	case s.translateGate <- struct{}{}:
		return true
	default:
	}
	if s.translateWaiters.Add(1) > translateQueueMax {
		s.translateWaiters.Add(-1)
		writeJSON(w, http.StatusTooManyRequests,
			map[string]string{"error": "traductor ocupado, reintenta en un momento"})
		return false
	}
	defer s.translateWaiters.Add(-1)

	t := time.NewTimer(translateWaitMax)
	defer t.Stop()
	select {
	case s.translateGate <- struct{}{}:
		return true
	case <-t.C:
		writeJSON(w, http.StatusTooManyRequests,
			map[string]string{"error": "el traductor lleva demasiado ocupado, reintenta"})
		return false
	case <-r.Context().Done():
		return false
	}
}

func (s *Server) releaseTranslate() { <-s.translateGate }
