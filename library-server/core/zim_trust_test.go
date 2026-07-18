package main

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTrustTestAdmin(t *testing.T) *adminZim {
	t.Helper()
	root := t.TempDir()
	st, err := openStore(filepath.Join(root, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.db.Close() })
	file := filepath.Join(root, "manual.zim")
	if err := os.WriteFile(file, []byte("zim de prueba"), 0o644); err != nil {
		t.Fatal(err)
	}
	xml := `<library version="20110515"><book id="manual-id" path="manual.zim" title="Manual"/></library>`
	libraryXML := filepath.Join(root, "library.xml")
	if err := os.WriteFile(libraryXML, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	return &adminZim{libraryXML: libraryXML, zimDir: root, store: st}
}

func TestZimInteractiveTrustRequiresAcknowledgementAndExpiresOnChange(t *testing.T) {
	a := newTrustTestAdmin(t)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/zim/interactive", strings.NewReader(`{"id":"manual-id","enabled":true}`))
	w := httptest.NewRecorder()
	a.handleInteractive(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("sin aceptar el aviso: quiero 400, tengo %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/zim/interactive", strings.NewReader(`{"id":"manual-id","enabled":true,"acknowledge":true}`))
	w = httptest.NewRecorder()
	a.handleInteractive(w, req)
	// El Panel desbloquea por id del <book> ("manual-id"); el contenido se consulta
	// por id PÚBLICO ("manual", nombre sin ext). Este cruce es justo el bug que
	// TED destapó: si ambas claves no coinciden, interactiveAllowed da false.
	if w.Code != http.StatusOK || !a.interactiveAllowed("manual") {
		t.Fatalf("desbloqueo aceptado: code=%d body=%s", w.Code, w.Body.String())
	}

	if err := os.WriteFile(filepath.Join(a.zimDir, "manual.zim"), []byte("archivo sustituido y distinto"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := a.trustState("manual")
	if state.Interactive || !state.Stale {
		t.Fatalf("el cambio de archivo debe revocar la confianza: %+v", state)
	}
}

func TestOfficialZimIsInteractiveUntilAdminBlocksIt(t *testing.T) {
	a := newTrustTestAdmin(t)
	// Se siembra bajo el id público, que es como lo escribe ahora el auto-registro
	// oficial (writeTrust(zimTrustKey(file), …)).
	if err := a.writeTrust("manual", "manual.zim", "official", true, true); err != nil {
		t.Fatal(err)
	}
	state := a.trustState("manual")
	if !state.Interactive || !state.Official {
		t.Fatalf("el origen oficial debe entrar habilitado: %+v", state)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/zim/interactive", strings.NewReader(`{"id":"manual-id","enabled":false}`))
	w := httptest.NewRecorder()
	a.handleInteractive(w, req)
	state = a.trustState("manual")
	if w.Code != http.StatusOK || state.Interactive || !state.Official {
		t.Fatalf("el bloqueo manual debe persistir sin perder procedencia: code=%d state=%+v", w.Code, state)
	}
	if err := a.writeTrust("manual", "manual.zim", "official", true, false); err != nil {
		t.Fatal(err)
	}
	if a.interactiveAllowed("manual") {
		t.Fatal("la reconciliacion historica no debe deshacer el bloqueo del admin")
	}
}

func TestContentIsolationPolicies(t *testing.T) {
	blocked := httptest.NewRecorder()
	setContentIsolation(blocked, false)
	if csp := blocked.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'none'") || !strings.Contains(csp, "connect-src 'none'") {
		t.Fatalf("CSP bloqueada inesperada: %s", csp)
	}

	interactive := httptest.NewRecorder()
	setContentIsolation(interactive, true)
	csp := interactive.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline'") || !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("CSP interactiva inesperada: %s", csp)
	}
	if !strings.Contains(csp, "worker-src 'none'") || !strings.Contains(csp, "frame-src 'none'") {
		t.Fatalf("faltan limites de defensa en profundidad: %s", csp)
	}
}

func TestTrustedZimGetsInteractivePolicyInsideReaderIframe(t *testing.T) {
	a := newTrustTestAdmin(t)
	if err := a.writeTrust("manual", "manual.zim", "official", true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.store.db.Exec(`INSERT INTO collection_access (collection_id,access,min_age,updated) VALUES (?,?,0,?)`,
		collectionIDForZIM("manual"), "open", time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	s := &Server{store: a.store, zimAdmin: a, proxy: httputil.NewSingleHostReverseProxy(u)}

	req := httptest.NewRequest(http.MethodGet, "/content/manual/index", nil)
	req.Header.Set("Sec-Fetch-Dest", "iframe")
	w := httptest.NewRecorder()
	s.handleContent(w, req)
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("el iframe del lector debe recibir la politica interactiva: %s", csp)
	}

	req = httptest.NewRequest(http.MethodGet, "/content/manual/app.js", nil)
	req.Header.Set("Sec-Fetch-Dest", "script")
	w = httptest.NewRecorder()
	s.handleContent(w, req)
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'none'") {
		t.Fatalf("los recursos secundarios no deben consultar ni ampliar la confianza: %s", csp)
	}
}
