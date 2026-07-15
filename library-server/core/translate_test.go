package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// mockEngine simula el sidecar-motor: cuenta cuántos textos ha traducido para
// verificar que la caché evita segundas llamadas.
func mockEngine(t *testing.T, translated *atomic.Int32) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/languages", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"pairs": []map[string]string{{"from": "en", "to": "es"}},
		})
	})
	mux.HandleFunc("/translate", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			From, To string
			Texts    []string
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		out := make([]string, len(body.Texts))
		for i, tx := range body.Texts {
			translated.Add(1)
			out[i] = "ES:" + tx // "traducción" determinista de prueba
		}
		writeJSON(w, http.StatusOK, map[string]any{"texts": out})
	})
	return httptest.NewServer(mux)
}

func newTestServer(t *testing.T, engineURL string) *Server {
	st, err := openStore(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	t.Cleanup(func() { st.db.Close() })
	return &Server{
		store:         st,
		http:          &http.Client{},
		translate:     strings.TrimRight(engineURL, "/"),
		translateGate: make(chan struct{}, 2),
	}
}

func postTranslate(t *testing.T, s *Server, jsonBody string) transResp {
	req := httptest.NewRequest(http.MethodPost, "/api/translate", strings.NewReader(jsonBody))
	rec := httptest.NewRecorder()
	s.handleTranslate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("handleTranslate status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp transResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	return resp
}

func TestTranslateCacheRoundTrip(t *testing.T) {
	var engineCalls atomic.Int32
	engine := mockEngine(t, &engineCalls)
	defer engine.Close()
	s := newTestServer(t, engine.URL)

	body := `{"lib":"","path":"A/Foo","to":"es","sourceHint":"en",
		"segments":[{"id":"p0","text":"Hello"},{"id":"p1","text":"World"}]}`

	// 1ª llamada: caché vacía → el motor traduce los 2 segmentos.
	r1 := postTranslate(t, s, body)
	if r1.Source != "en" || r1.To != "es" {
		t.Fatalf("source/to = %q/%q", r1.Source, r1.To)
	}
	if len(r1.Segments) != 2 || r1.Segments[0].Text != "ES:Hello" || r1.Segments[1].Text != "ES:World" {
		t.Fatalf("segments = %+v", r1.Segments)
	}
	if n := engineCalls.Load(); n != 2 {
		t.Fatalf("esperaba 2 traducciones del motor, got %d", n)
	}

	// 2ª llamada idéntica: todo de caché → el motor NO se vuelve a llamar.
	r2 := postTranslate(t, s, body)
	if r2.Segments[0].Text != "ES:Hello" || r2.Segments[1].Text != "ES:World" {
		t.Fatalf("segments (cache) = %+v", r2.Segments)
	}
	if n := engineCalls.Load(); n != 2 {
		t.Fatalf("la caché no funcionó: motor llamado %d veces (esperaba 2)", n)
	}

	// 3ª llamada con un segmento nuevo + uno cacheado: solo se traduce el nuevo.
	body2 := `{"lib":"","path":"A/Foo","to":"es","sourceHint":"en",
		"segments":[{"id":"p0","text":"Hello"},{"id":"p2","text":"New"}]}`
	r3 := postTranslate(t, s, body2)
	if r3.Segments[0].Text != "ES:Hello" || r3.Segments[1].Text != "ES:New" {
		t.Fatalf("segments (mixto) = %+v", r3.Segments)
	}
	if n := engineCalls.Load(); n != 3 {
		t.Fatalf("esperaba 3 traducciones totales (solo 1 nueva), got %d", n)
	}

	// Editar el texto origen del mismo seg_id invalida la caché (src_hash).
	body3 := `{"lib":"","path":"A/Foo","to":"es","sourceHint":"en",
		"segments":[{"id":"p0","text":"Hello EDITED"}]}`
	r4 := postTranslate(t, s, body3)
	if r4.Segments[0].Text != "ES:Hello EDITED" {
		t.Fatalf("segment (editado) = %+v", r4.Segments)
	}
	if n := engineCalls.Load(); n != 4 {
		t.Fatalf("el origen editado debía retraducir: got %d (esperaba 4)", n)
	}
}

func TestTranslateLanguagesDetection(t *testing.T) {
	// Sin motor: available=false.
	off := newTestServer(t, "")
	rec := httptest.NewRecorder()
	off.handleTranslateLanguages(rec, httptest.NewRequest(http.MethodGet, "/api/translate/languages", nil))
	var d1 struct {
		Available bool `json:"available"`
	}
	json.Unmarshal(rec.Body.Bytes(), &d1)
	if d1.Available {
		t.Fatalf("sin TRANSLATE_URL debía ser available=false")
	}

	// Con motor: available=true + pares.
	var calls atomic.Int32
	engine := mockEngine(t, &calls)
	defer engine.Close()
	on := newTestServer(t, engine.URL)
	rec2 := httptest.NewRecorder()
	on.handleTranslateLanguages(rec2, httptest.NewRequest(http.MethodGet, "/api/translate/languages", nil))
	var d2 struct {
		Available bool                `json:"available"`
		Pairs     []map[string]string `json:"pairs"`
	}
	json.Unmarshal(rec2.Body.Bytes(), &d2)
	if !d2.Available || len(d2.Pairs) != 1 || d2.Pairs[0]["from"] != "en" || d2.Pairs[0]["to"] != "es" {
		t.Fatalf("detección con motor = %+v", d2)
	}
}
