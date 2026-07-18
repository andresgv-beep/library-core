package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testServer: Server mínimo con store real en disco temporal. No necesita kiwix:
// requireAdmin decide antes de tocar ningún motor.
func testAuthServer(t *testing.T, token string) *Server {
	t.Helper()
	st, err := openStore(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	t.Cleanup(func() { st.db.Close() })
	return &Server{store: st, token: token}
}

// guarded monta una ruta administrativa de mentira detrás de requireAdmin y
// devuelve el handler. Si el guard deja pasar, el handler responde 200 "ok".
func guarded(s *Server) http.Handler {
	admin := http.NewServeMux()
	admin.HandleFunc("/api/admin/zim/unregister", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux := http.NewServeMux()
	mux.Handle("/api/admin/", s.requireAdmin(admin))
	return mux
}

// sessionFor crea usuario + sesión y devuelve la cookie lista para inyectar.
func sessionFor(t *testing.T, s *Server, name string, age int, admin bool) *http.Cookie {
	t.Helper()
	if _, err := s.createUser(name, "password!9", age, admin); err != nil {
		t.Fatalf("createUser(%s): %v", name, err)
	}
	tok, err := s.newSession(name)
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	return &http.Cookie{Name: sessionCookie, Value: tok}
}

func do(h http.Handler, cookie *http.Cookie, hdr map[string]string) int {
	r := httptest.NewRequest(http.MethodPost, "/api/admin/zim/unregister", strings.NewReader(`{"id":"wikipedia_es"}`))
	if cookie != nil {
		r.AddCookie(cookie)
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

// El agujero original: anónimo podía desregistrar un ZIM. Ahora → 401.
func TestAdminRouteRejectsAnonymous(t *testing.T) {
	s := testAuthServer(t, "")
	if code := do(guarded(s), nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("anónimo en ruta admin: quiero 401, tengo %d", code)
	}
}

// Un usuario normal con sesión (el crío de la LAN) NO es admin → 403.
func TestAdminRouteRejectsNonAdminSession(t *testing.T) {
	s := testAuthServer(t, "")
	c := sessionFor(t, s, "critico", 12, false)
	if code := do(guarded(s), c, nil); code != http.StatusForbidden {
		t.Fatalf("usuario no-admin en ruta admin: quiero 403, tengo %d", code)
	}
}

// El admin sí pasa (si no, habríamos roto el Panel).
func TestAdminRouteAllowsAdminSession(t *testing.T) {
	s := testAuthServer(t, "")
	c := sessionFor(t, s, "andres", 40, true)
	if code := do(guarded(s), c, nil); code != http.StatusOK {
		t.Fatalf("admin en ruta admin: quiero 200, tengo %d", code)
	}
}

// Carril máquina: el daemon de NimOS con el token válido pasa sin cookie.
func TestAdminRouteAllowsMachineToken(t *testing.T) {
	s := testAuthServer(t, "secreto")
	if code := do(guarded(s), nil, map[string]string{"X-Nimos-Token": "secreto"}); code != http.StatusOK {
		t.Fatalf("token máquina válido: quiero 200, tengo %d", code)
	}
	if code := do(guarded(s), nil, map[string]string{"X-Nimos-Token": "otro"}); code != http.StatusUnauthorized {
		t.Fatalf("token máquina inválido: quiero 401, tengo %d", code)
	}
}

// Una sesión más vieja que sessionTTL no vale aunque el navegador mande la cookie.
func TestSessionExpires(t *testing.T) {
	s := testAuthServer(t, "")
	c := sessionFor(t, s, "andres", 40, true)

	old := time.Now().Add(-sessionTTL - time.Hour).Unix()
	if _, err := s.store.db.Exec(`UPDATE sessions SET created = ? WHERE token = ?`, old, c.Value); err != nil {
		t.Fatalf("envejecer sesión: %v", err)
	}
	if code := do(guarded(s), c, nil); code != http.StatusUnauthorized {
		t.Fatalf("sesión caducada: quiero 401, tengo %d", code)
	}

	s.purgeSessions()
	var n int
	s.store.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n)
	if n != 0 {
		t.Fatalf("purgeSessions dejó %d sesiones caducadas", n)
	}
}

// El bootstrap solo puede crear UN admin, aunque lleguen dos peticiones a la vez.
func TestFirstAdminBootstrapIsAtomic(t *testing.T) {
	s := testAuthServer(t, "")

	u, err := s.createFirstAdmin("andres", "password!9", 40)
	if err != nil {
		t.Fatalf("primer admin: %v", err)
	}
	if !u.IsAdmin {
		t.Fatal("el primer usuario tiene que ser admin")
	}
	if _, err := s.createFirstAdmin("intruso", "password!9", 30); err == nil {
		t.Fatal("el segundo register debería fallar: ya hay usuarios")
	}
	if n := s.userCount(); n != 1 {
		t.Fatalf("quiero 1 usuario, tengo %d", n)
	}
}

func TestRequestSessionTokenSupportsSeparatedClient(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "cookie-token"})
	req.Header.Set("Authorization", "Bearer client-token")
	if got := requestSessionToken(req); got != "client-token" {
		t.Fatalf("Bearer debe tener prioridad: %q", got)
	}

	req.Header.Del("Authorization")
	if got := requestSessionToken(req); got != "cookie-token" {
		t.Fatalf("la cookie del Panel debe seguir funcionando: %q", got)
	}
}

func TestQuerySessionTokenIsLimitedToNativeMediaReads(t *testing.T) {
	for _, tc := range []struct {
		method, path, want string
	}{
		{http.MethodGet, "/media/Publico/video.mp4?st=query-token", "query-token"},
		{http.MethodHead, "/content/wiki/A/Portada?st=query-token", "query-token"},
		{http.MethodGet, "/api/admin/users?st=query-token", ""},
		{http.MethodPost, "/media/Publico/video.mp4?st=query-token", ""},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if got := requestSessionToken(req); got != tc.want {
			t.Errorf("%s %s: token=%q, quiero %q", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestAnonymousBrowsersReceiveIsolatedIdentities(t *testing.T) {
	s := testAuthServer(t, "")
	resolve := func(cookie *http.Cookie) (string, *http.Cookie) {
		req := httptest.NewRequest(http.MethodGet, "/api/favorites", nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		req = s.withGuestIdentity(rec, req)
		name := s.currentUsername(req)
		cookies := rec.Result().Cookies()
		if cookie != nil {
			return name, cookie
		}
		if len(cookies) != 1 {
			t.Fatalf("quiero una cookie de invitado, tengo %d", len(cookies))
		}
		return name, cookies[0]
	}

	first, firstCookie := resolve(nil)
	second, _ := resolve(nil)
	again, _ := resolve(firstCookie)
	if first == "" || second == "" || first == second {
		t.Fatalf("invitados no aislados: first=%q second=%q", first, second)
	}
	if again != first {
		t.Fatalf("la identidad invitada no es estable: %q != %q", again, first)
	}
}

func TestRemoteBootstrapRequiresSetupCode(t *testing.T) {
	s := testAuthServer(t, "")
	t.Setenv("NIMOS_SETUP_TOKEN", "codigo-seguro")
	remote := httptest.NewRequest(http.MethodPost, "/api/auth/register", nil)
	remote.RemoteAddr = "192.168.1.25:45000"
	if s.setupAllowed(remote, "") {
		t.Fatal("el bootstrap remoto no debe pasar sin código")
	}
	if !s.setupAllowed(remote, "codigo-seguro") {
		t.Fatal("el bootstrap remoto debe aceptar el código configurado")
	}
	local := httptest.NewRequest(http.MethodPost, "/api/auth/register", nil)
	local.RemoteAddr = "127.0.0.1:45000"
	if !s.setupAllowed(local, "") {
		t.Fatal("el bootstrap local debe seguir funcionando sin código")
	}
}

func TestClientOriginsAreExplicit(t *testing.T) {
	t.Setenv("CLIENT_ORIGINS", "https://client.example, http://192.168.1.20:4173")
	if !clientOriginAllowed("https://client.example") {
		t.Fatal("debe aceptar el origen configurado")
	}
	if clientOriginAllowed("https://evil.example") {
		t.Fatal("no debe aceptar origenes no configurados")
	}
	// H-5: el atajo loopback NO está activo por defecto (evita apps locales
	// hostiles con Allow-Credentials); solo con DEV_CORS=1.
	if clientOriginAllowed("http://localhost:5173") {
		t.Fatal("loopback no debe aceptarse sin DEV_CORS")
	}
	t.Setenv("DEV_CORS", "1")
	if !clientOriginAllowed("http://localhost:5173") {
		t.Fatal("loopback debe funcionar con DEV_CORS=1")
	}
}

// ── Política de contraseñas (10 + especial) ────────────────────────────────

func TestPasswordPolicy(t *testing.T) {
	cases := []struct {
		pw   string
		ok   bool
		name string
	}{
		{"password!9", true, "10 con especial"},
		{"holamundo!", true, "10 justos con especial"},
		{"añoñísimo!", true, "unicode + especial"},
		{"short!1", false, "menos de 10"},
		{"passwordlonga", false, "10+ pero sin especial"},
		{"1234567890", false, "solo dígitos"},
		{"", false, "vacía"},
	}
	for _, c := range cases {
		err := validatePassword(c.pw)
		if (err == nil) != c.ok {
			t.Errorf("%s: validatePassword(%q) err=%v, esperaba ok=%v", c.name, c.pw, err, c.ok)
		}
	}
}

func TestCreateUserRejectsWeakPassword(t *testing.T) {
	s := testAuthServer(t, "")
	if _, err := s.createUser("ana", "1234", 20, false); err == nil {
		t.Fatal("createUser aceptó una contraseña débil de 4 caracteres")
	}
	if _, err := s.createUser("ana", "buenaclave!", 20, false); err != nil {
		t.Fatalf("createUser rechazó una contraseña válida: %v", err)
	}
}

// ── Reset por admin ────────────────────────────────────────────────────────

func TestAdminResetPassword(t *testing.T) {
	s := testAuthServer(t, "")
	u, err := s.createUser("bob", "viejaclave!", 30, false)
	if err != nil {
		t.Fatalf("createUser: %v", err)
	}
	// Reset a una temporal válida.
	if err := s.setPasswordByID(u.ID, "nuevaclave!"); err != nil {
		t.Fatalf("setPasswordByID: %v", err)
	}
	if _, ok := s.userForCredentials("bob", "nuevaclave!"); !ok {
		t.Fatal("la contraseña nueva no autentica")
	}
	if _, ok := s.userForCredentials("bob", "viejaclave!"); ok {
		t.Fatal("la contraseña vieja sigue autenticando tras el reset")
	}
	// El reset también exige la política.
	if err := s.setPasswordByID(u.ID, "corta"); err == nil {
		t.Fatal("el reset por admin aceptó una temporal débil")
	}
}

// ── Cambio por el propio usuario ───────────────────────────────────────────

func TestSelfChangePassword(t *testing.T) {
	s := testAuthServer(t, "")
	if _, err := s.createUser("caro", "claveinicial!", 25, false); err != nil {
		t.Fatalf("createUser: %v", err)
	}
	tok, _ := s.newSession("caro")
	cookie := &http.Cookie{Name: sessionCookie, Value: tok}

	body := `{"current":"claveinicial!","new":"claverenovada!"}`
	r := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(body))
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	s.handleChangePassword(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("cambio propio: código %d, cuerpo %s", w.Code, w.Body.String())
	}
	if _, ok := s.userForCredentials("caro", "claverenovada!"); !ok {
		t.Fatal("la contraseña nueva no autentica tras el cambio propio")
	}

	// Contraseña actual incorrecta → 401.
	r2 := httptest.NewRequest(http.MethodPost, "/api/auth/password",
		strings.NewReader(`{"current":"noeslamia","new":"otraclave!!"}`))
	r2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	s.handleChangePassword(w2, r2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("con contraseña actual errónea esperaba 401, tuve %d", w2.Code)
	}
}

// ── H-1: el borrado de usuario purga su estado personal ────────────────────

func TestDeleteUserPurgesPersonalState(t *testing.T) {
	s := testAuthServer(t, "")
	u, err := s.createUser("dora", "claveinicial!", 30, false)
	if err != nil {
		t.Fatalf("createUser: %v", err)
	}
	// Sembrar estado personal de "dora".
	if err := s.store.PutFavorite("dora", Fav{Lib: "wiki", Path: "A/Foo", Title: "Foo"}, now()); err != nil {
		t.Fatalf("PutFavorite: %v", err)
	}
	if err := s.store.AddHistory("dora", Visit{Lib: "wiki", Path: "A/Foo", Title: "Foo"}, now()); err != nil {
		t.Fatalf("AddHistory: %v", err)
	}

	// Borrar la cuenta vía handler.
	admin := sessionFor(t, s, "jefa", 0, true)
	r := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(u.ID, 10), nil)
	r.AddCookie(admin)
	w := httptest.NewRecorder()
	s.handleAdminUserOp(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("borrado: código %d, cuerpo %s", w.Code, w.Body.String())
	}

	// El estado de "dora" no debe sobrevivir (si no, una cuenta nueva con el
	// mismo nombre lo heredaría).
	favs, _ := s.store.ListFavorites("dora")
	if len(favs) != 0 {
		t.Fatalf("favoritos huérfanos tras borrar: %d", len(favs))
	}
	hist, _ := s.store.ListHistory("dora", 10)
	if len(hist) != 0 {
		t.Fatalf("historial huérfano tras borrar: %d", len(hist))
	}
}

// ── H-3: el enqueue rechaza destinos internos ──────────────────────────────

func TestHostIsPublic(t *testing.T) {
	internal := []string{"127.0.0.1", "localhost", "10.0.0.5", "192.168.1.10", "169.254.169.254", "0.0.0.0", ""}
	for _, h := range internal {
		if hostIsPublic(h) {
			t.Errorf("hostIsPublic(%q) = true, debería rechazarse", h)
		}
	}
	// Público real (IP literal, sin depender de DNS).
	if !hostIsPublic("93.184.216.34") { // example.com
		t.Error("una IP pública debería aceptarse")
	}
}

// ── H-6: el limitador de login frena tras N fallos ─────────────────────────

func TestLoginLimiterBacksOff(t *testing.T) {
	l := &ipLimiter{m: make(map[string]*loginAttempt)}
	ip := "203.0.113.7"
	// Los primeros intentos libres no bloquean.
	for i := 0; i < loginFreeTries; i++ {
		l.fail(ip)
	}
	if _, blocked := l.blocked(ip); blocked {
		t.Fatal("no debería bloquear dentro de los intentos libres")
	}
	// Un fallo más ya impone espera.
	l.fail(ip)
	if _, blocked := l.blocked(ip); !blocked {
		t.Fatal("debería bloquear tras superar los intentos libres")
	}
	// reset lo limpia.
	l.reset(ip)
	if _, blocked := l.blocked(ip); blocked {
		t.Fatal("reset debería limpiar el bloqueo")
	}
}

func TestClientIPIgnoresForwardedHeaderByDefault(t *testing.T) {
	t.Setenv("TRUST_PROXY", "")
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.RemoteAddr = "192.0.2.10:4321"
	r.Header.Set("X-Forwarded-For", "203.0.113.99")
	if got := clientIP(r); got != "192.0.2.10" {
		t.Fatalf("clientIP = %q; debe usar la conexion directa sin TRUST_PROXY", got)
	}
}

func TestClientIPHonorsForwardedHeaderForTrustedProxy(t *testing.T) {
	t.Setenv("TRUST_PROXY", "1")
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.RemoteAddr = "192.0.2.10:4321"
	r.Header.Set("X-Forwarded-For", "203.0.113.99, 192.0.2.20")
	if got := clientIP(r); got != "203.0.113.99" {
		t.Fatalf("clientIP = %q; debe tomar el primer salto con TRUST_PROXY=1", got)
	}
}
