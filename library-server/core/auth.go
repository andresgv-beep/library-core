// auth.go — Usuarios, sesiones y control de acceso por edad (Panel · fase 2).
//
// Modelo (decidido con Andrés): el contenido que añade el admin queda BLOQUEADO
// por defecto; por colección se elige nivel (open/login/blocked) + edad mínima.
// Los usuarios tienen nombre, contraseña (bcrypt) y EDAD. El primer usuario que
// se registra es admin (bootstrap estilo Immich); a partir de ahí, el admin crea
// las cuentas. Si una colección tiene edad mínima, exige cuenta (sin sesión no se
// puede comprobar la edad → bloqueado).
//
// El enforcement en el lector vive en access.go; aquí está la identidad.
//
// ── Modelo de autoridad (dos carriles, una cerradura) ──────────────────────
//
// El Panel de Control es la puerta que se ENSEÑA al admin, pero esconder un
// botón no es un permiso: los endpoints son HTTP y cualquiera puede llamarlos
// con curl o desde las devtools del propio lector. El permiso se comprueba
// SIEMPRE en el servidor.
//
//	X-Nimos-Token   carril máquina (el daemon de NimOS). Si coincide → admin.
//	Cookie sesión   carril humano. Si el usuario tiene is_admin → admin.
//
// `requireAdmin` exige UNO DE LOS DOS y es el único punto por el que pasan
// TODAS las rutas administrativas (ver main.go: adminMux). Un endpoint admin
// nuevo entra por ahí, o no entra.

package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookie = "nimos_session"
	guestCookie   = "nimos_guest"
	// sessionTTL: caducidad REAL, comprobada en servidor. El MaxAge de la cookie
	// solo es una sugerencia al navegador; el token de la DB es el que manda.
	sessionTTL       = 30 * 24 * time.Hour
	sessionIdleTTL   = 7 * 24 * time.Hour
	sessionTouchStep = 5 * time.Minute
	mediaTokenTTL    = 15 * time.Minute
)

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Age      int    `json:"age"`
	IsAdmin  bool   `json:"isAdmin"`
}

// machineUser: identidad sintética del carril máquina (X-Nimos-Token válido).
// No existe en la tabla `users`; solo sirve para que los handlers que ya
// comprueban IsAdmin (access.go, media.go, este fichero) dejen pasar a NimOS.
func machineUser() *User { return &User{ID: -1, Username: "nimos", Age: 999, IsAdmin: true} }

// ── Usuario en contexto ────────────────────────────────────────────────────

type ctxKey int

const (
	ctxUserKey  ctxKey = 0
	ctxGuestKey ctxKey = 1
)

// withUser mete el usuario ya resuelto en el contexto: los handlers que están
// detrás de requireAdmin no vuelven a pegarle a SQLite en cada comprobación.
func withUser(r *http.Request, u *User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxUserKey, u))
}

// withGuestIdentity garantiza que cada navegador anónimo tenga un espacio
// personal distinto. La cookie solo contiene un identificador aleatorio; no
// concede acceso a colecciones ni sustituye una sesión de usuario.
func (s *Server) withGuestIdentity(w http.ResponseWriter, r *http.Request) *http.Request {
	id := ""
	if c, err := r.Cookie(guestCookie); err == nil && validGuestID(c.Value) {
		id = c.Value
	}
	if id == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return r
		}
		id = hex.EncodeToString(b)
		secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
		http.SetCookie(w, &http.Cookie{
			Name: guestCookie, Value: id, Path: "/", HttpOnly: true,
			Secure: secure, SameSite: http.SameSiteLaxMode,
			MaxAge: 365 * 24 * 60 * 60,
		})
	}
	return r.WithContext(context.WithValue(r.Context(), ctxGuestKey, id))
}

func validGuestID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

// ── Consultas de usuarios/sesiones ─────────────────────────────────────────

func (s *Server) userCount() int {
	var n int
	s.store.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n
}

func (s *Server) createUser(username, password string, age int, isAdmin bool) (*User, error) {
	username = strings.TrimSpace(username)
	if err := validCreds(username, password, age); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	adm := 0
	if isAdmin {
		adm = 1
	}
	res, err := s.store.db.Exec(`INSERT INTO users (username, pass_hash, age, is_admin, created) VALUES (?,?,?,?,?)`,
		username, string(hash), age, adm, time.Now().Unix())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, errBadInput("ese nombre de usuario ya existe")
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, Age: age, IsAdmin: isAdmin}, nil
}

// setPasswordByID cambia el hash de un usuario por su id (reset por admin). No
// toca las sesiones abiertas; el admin decide si además fuerza el cierre.
func (s *Server) setPasswordByID(id int64, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res, err := s.store.db.Exec(`UPDATE users SET pass_hash = ? WHERE id = ?`, string(hash), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errBadInput("usuario no encontrado")
	}
	return nil
}

// setPasswordByUsername cambia el hash de un usuario por su nombre (cambio propio).
func (s *Server) setPasswordByUsername(username, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.store.db.Exec(`UPDATE users SET pass_hash = ? WHERE username = ?`, string(hash), username)
	return err
}

// createFirstAdmin: bootstrap ATÓMICO. El `INSERT ... WHERE NOT EXISTS` deja que
// SQLite decida quién gana; dos POST simultáneos ya no pueden crear dos admins
// (el segundo sale con 0 filas afectadas).
func (s *Server) createFirstAdmin(username, password string, age int) (*User, error) {
	username = strings.TrimSpace(username)
	if err := validCreds(username, password, age); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	res, err := s.store.db.Exec(`
		INSERT INTO users (username, pass_hash, age, is_admin, created)
		SELECT ?, ?, ?, 1, ?
		WHERE NOT EXISTS (SELECT 1 FROM users)`,
		username, string(hash), age, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, errBadInput("ya hay usuarios; el admin crea las cuentas")
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, Age: age, IsAdmin: true}, nil
}

func validCreds(username, password string, age int) error {
	if username == "" {
		return errBadInput("falta el nombre de usuario")
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	if age < 0 || age > 120 {
		return errBadInput("edad fuera de rango")
	}
	return nil
}

// validatePassword es la ÚNICA regla de contraseñas, compartida por alta,
// bootstrap, reset por admin y cambio por el propio usuario: mínimo 10 caracteres
// y al menos un carácter especial (no alfanumérico). Un solo sitio → no hay puerta
// que cree contraseñas débiles (decisión firme con Andrés: cuentas sólidas sin
// excepciones; el reset por admin usa una temporal que TAMBIÉN cumple la regla).
func validatePassword(password string) error {
	if len([]rune(password)) < 10 {
		return errBadInput("la contraseña debe tener al menos 10 caracteres")
	}
	for _, r := range password {
		// Especial = cualquier no-alfanumérico. unicode.IsLetter/IsDigit cubren
		// también letras acentuadas y dígitos no-ASCII; lo que no sea ninguno de
		// los dos cuenta como especial (símbolos, puntuación, espacio…).
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return nil
		}
	}
	return errBadInput("la contraseña debe incluir al menos un carácter especial (por ejemplo !@#$%)")
}

// hash falso, con formato válido, para gastar el mismo tiempo cuando el usuario
// no existe: si no, el tiempo de respuesta delata qué nombres están dados de alta.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func (s *Server) userForCredentials(username, password string) (*User, bool) {
	var u User
	var hash string
	var adm int
	err := s.store.db.QueryRow(`SELECT id, username, pass_hash, age, is_admin FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &hash, &u.Age, &adm)
	if err != nil {
		bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password)) // coste constante
		return nil, false
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return nil, false
	}
	u.IsAdmin = adm == 1
	return &u, true
}

func (s *Server) newSession(username string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	now := time.Now().Unix()
	_, err := s.store.db.Exec(`INSERT INTO sessions (token, username, created, last_seen) VALUES (?,?,?,?)`,
		token, username, now, now)
	return token, err
}

// purgeSessions borra las sesiones caducadas. Se llama en login: barato y
// suficiente (no necesita scheduler propio).
func (s *Server) purgeSessions() {
	now := time.Now()
	s.store.db.Exec(`DELETE FROM sessions WHERE created < ? OR (CASE WHEN last_seen = 0 THEN created ELSE last_seen END) < ?`,
		now.Add(-sessionTTL).Unix(), now.Add(-sessionIdleTTL).Unix())
	s.store.db.Exec(`DELETE FROM media_tokens WHERE expires < ? OR session_token NOT IN (SELECT token FROM sessions)`, now.Unix())
}

// currentUser devuelve el usuario de la sesión (cookie) o nil si es anónimo.
// Comprueba la CADUCIDAD en el servidor: una cookie de hace tres meses no vale
// aunque el navegador siga mandándola.
func (s *Server) currentUser(r *http.Request) *User {
	if u, ok := r.Context().Value(ctxUserKey).(*User); ok {
		return u // ya resuelto por requireAdmin: no repetimos la consulta
	}
	token := primarySessionToken(r)
	mediaToken := ""
	if token == "" {
		mediaToken = mediaTokenFromRequest(r)
	}
	if token == "" && mediaToken == "" {
		return nil
	}
	var u User
	var adm int
	var sessionToken string
	var lastSeen int64
	now := time.Now()
	var err error
	if token != "" {
		sessionToken = token
		err = s.store.db.QueryRow(`
			SELECT u.id, u.username, u.age, u.is_admin, s.last_seen
			FROM sessions s JOIN users u ON u.username = s.username
			WHERE s.token = ? AND s.created > ? AND (CASE WHEN s.last_seen = 0 THEN s.created ELSE s.last_seen END) > ?`,
			token, now.Add(-sessionTTL).Unix(), now.Add(-sessionIdleTTL).Unix()).
			Scan(&u.ID, &u.Username, &u.Age, &adm, &lastSeen)
	} else {
		err = s.store.db.QueryRow(`
			SELECT u.id, u.username, u.age, u.is_admin, s.token, s.last_seen
			FROM media_tokens mt
			JOIN sessions s ON s.token = mt.session_token
			JOIN users u ON u.username = mt.username AND u.username = s.username
			WHERE mt.token = ? AND mt.expires > ? AND s.created > ? AND (CASE WHEN s.last_seen = 0 THEN s.created ELSE s.last_seen END) > ?`,
			mediaToken, now.Unix(), now.Add(-sessionTTL).Unix(), now.Add(-sessionIdleTTL).Unix()).
			Scan(&u.ID, &u.Username, &u.Age, &adm, &sessionToken, &lastSeen)
	}
	if err != nil {
		return nil
	}
	if lastSeen < now.Add(-sessionTouchStep).Unix() {
		_, _ = s.store.db.Exec(`UPDATE sessions SET last_seen = ? WHERE token = ?`, now.Unix(), sessionToken)
	}
	u.IsAdmin = adm == 1
	return &u
}

func requestSessionToken(r *http.Request) string {
	if token := primarySessionToken(r); token != "" {
		return token
	}
	return mediaTokenFromRequest(r)
}

func primarySessionToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	return ""
}

func mediaTokenFromRequest(r *http.Request) string {
	// Fallback para elementos NATIVOS del navegador (video, embed/pdf, img) que
	// no pueden mandar la cabecera Authorization y, cross-origin (cliente en otro
	// puerto/host), tampoco siempre la cookie: el token viaja en la query (?st=).
	// Lo usa el cliente solo para /media y /content. Contrato de auth §6.4.
	mediaRead := (r.Method == http.MethodGet || r.Method == http.MethodHead) &&
		(strings.HasPrefix(r.URL.Path, "/media/") || strings.HasPrefix(r.URL.Path, "/content/"))
	if mediaRead {
		if st := r.URL.Query().Get("st"); st != "" {
			return st
		}
	}
	return ""
}

// currentUsername devuelve una clave aislada para la cuenta o navegador actual.
// Los invitados usan una identidad opaca creada por withGuestIdentity, por lo que
// dos navegadores anónimos no comparten favoritos, notas, historial ni etiquetas.
func (s *Server) currentUsername(r *http.Request) string {
	if u := s.currentUser(r); u != nil {
		return u.Username
	}
	if id, ok := r.Context().Value(ctxGuestKey).(string); ok && validGuestID(id) {
		return "guest:" + id
	}
	if c, err := r.Cookie(guestCookie); err == nil && validGuestID(c.Value) {
		return "guest:" + c.Value
	}
	return ""
}

// hasMachineToken: ¿viene del daemon de NimOS por el carril máquina?
func (s *Server) hasMachineToken(r *http.Request) bool {
	provided := r.Header.Get("X-Nimos-Token")
	return s.token != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) == 1
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	// Secure solo si la petición llegó por HTTPS (directa o detrás de Caddy).
	// Ponerlo siempre rompería el acceso por HTTP plano en la LAN; no ponerlo
	// nunca dejaría viajar la cookie en claro por DuckDNS.
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(sessionTTL / time.Second),
	})
}

// ── La cerradura: requireAdmin ─────────────────────────────────────────────

// requireAdmin envuelve TODAS las rutas administrativas. Pasa si:
//   - la petición trae un X-Nimos-Token válido (carril máquina), o
//   - hay sesión de un usuario con is_admin (carril humano).
//
// 401 si no hay identidad; 403 si la hay pero no es admin. Distinguirlos permite
// al frontend saber si tiene que pedir login o decir "no tienes permiso".
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.hasMachineToken(r) {
			next.ServeHTTP(w, withUser(r, machineUser()))
			return
		}
		u := s.currentUser(r)
		if u == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "necesitas iniciar sesión"})
			return
		}
		if !u.IsAdmin {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "solo admin"})
			return
		}
		next.ServeHTTP(w, withUser(r, u))
	})
}

// ── Handlers de auth ───────────────────────────────────────────────────────

// registerAuthRoutes: rutas PÚBLICAS de identidad. Login y register tienen que
// ser alcanzables sin estar autenticado (si no, no hay forma de entrar).
func (s *Server) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/register", s.handleRegister)
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/auth/logout", s.handleLogout)
	mux.HandleFunc("/api/auth/logout-all", s.handleLogoutAll)
	mux.HandleFunc("/api/auth/refresh", s.handleRefreshSession)
	mux.HandleFunc("/api/auth/me", s.handleMe)
	mux.HandleFunc("/api/auth/media-token", s.handleMediaToken)
	mux.HandleFunc("/api/auth/password", s.handleChangePassword) // el usuario cambia la suya
}

// POST /api/auth/password — el usuario autenticado cambia SU propia contraseña.
// Exige la contraseña actual (no es un reset: eso es potestad del admin). Aplica
// la misma regla que el resto (validatePassword). No es admin-only: cualquier
// usuario con sesión puede cambiar la suya, y solo la suya.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	me := s.currentUser(r)
	if me == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "necesitas iniciar sesión"})
		return
	}
	// El carril máquina (nimos) no tiene contraseña que cambiar.
	if me.ID < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "esta cuenta no gestiona contraseña"})
		return
	}
	ip := "password:" + clientIP(r)
	if _, blocked := passwordLimiter.blocked(ip); blocked {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "demasiados intentos de contraseña; espera unos segundos"})
		return
	}
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body inválido"})
		return
	}
	// Verificar la contraseña actual (mismo camino que el login, bcrypt).
	if _, ok := s.userForCredentials(me.Username, req.Current); !ok {
		passwordLimiter.fail(ip)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "la contraseña actual no es correcta"})
		return
	}
	passwordLimiter.reset(ip)
	if err := s.setPasswordByUsername(me.Username, req.New); err != nil {
		writeInputError(w, err)
		return
	}
	// Cerrar las OTRAS sesiones (por si la contraseña se cambió por sospecha de
	// filtración) y renovar la de esta petición para no quedar fuera.
	s.store.db.Exec(`DELETE FROM sessions WHERE username = ?`, me.Username)
	s.store.db.Exec(`DELETE FROM media_tokens WHERE username = ?`, me.Username)
	token, err := s.newSession(me.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sessionToken": token})
}

// registerAdminUserRoutes: gestión de cuentas. Se registra en el adminMux, o sea
// detrás de requireAdmin. Las comprobaciones internas de IsAdmin se quedan como
// defensa en profundidad (si alguien mueve la ruta de sitio, sigue protegida).
func (s *Server) registerAdminUserRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/users", s.handleAdminUsers)   // GET listar · POST crear
	mux.HandleFunc("/api/admin/users/", s.handleAdminUserOp) // DELETE /{id}
}

type credsReq struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Age        int    `json:"age"`
	IsAdmin    bool   `json:"isAdmin"`
	SetupToken string `json:"setupToken,omitempty"`
}

// POST /api/auth/register — bootstrap: solo funciona si NO hay usuarios; crea el
// primer admin. Después, las cuentas las crea el admin (/api/admin/users).
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	var req credsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body inválido"})
		return
	}
	// Evita calcular bcrypt en cada petición anónima una vez terminado el setup.
	// La inserción atómica de createFirstAdmin sigue siendo la autoridad final.
	if s.userCount() != 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "ya hay usuarios; el admin crea las cuentas"})
		return
	}
	if !s.setupAllowed(r, req.SetupToken) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "el alta inicial remota requiere el código de configuración"})
		return
	}
	u, err := s.createFirstAdmin(req.Username, req.Password, req.Age) // atómico
	if err != nil {
		writeInputError(w, err)
		return
	}
	token, err := s.newSession(u.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusCreated, map[string]any{"user": u, "sessionToken": token})
}

// setupAllowed permite el bootstrap directamente desde el propio equipo. Si la
// instalación se expone por LAN/proxy, exige NIMOS_SETUP_TOKEN (o el carril de
// máquina) para que un visitante no pueda apropiarse del primer administrador.
func (s *Server) setupAllowed(r *http.Request, provided string) bool {
	if s.hasMachineToken(r) || requestIsLocal(r) {
		return true
	}
	expected := strings.TrimSpace(os.Getenv("NIMOS_SETUP_TOKEN"))
	provided = strings.TrimSpace(provided)
	return expected != "" && subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

func requestIsLocal(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil || !ip.IsLoopback() {
		return false
	}
	// Un proxy local conserva RemoteAddr=loopback, pero X-Forwarded-For revela
	// que el navegador real está fuera. En ese caso sigue haciendo falta código.
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		forwarded := net.ParseIP(strings.TrimSpace(first))
		return forwarded != nil && forwarded.IsLoopback()
	}
	return true
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	// Rate-limit por IP (auditoría H-6): frena el fuerza-bruta contra el login.
	// El dummy-hash ya evita el oráculo de tiempo; esto acota el ritmo.
	ip := clientIP(r)
	if wait, blocked := loginLimiter.blocked(ip); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "demasiados intentos; espera unos segundos e inténtalo de nuevo",
		})
		return
	}
	var req credsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body inválido"})
		return
	}
	u, ok := s.userForCredentials(strings.TrimSpace(req.Username), req.Password)
	if !ok {
		loginLimiter.fail(ip) // cuenta el fallo (backoff incremental)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "usuario o contraseña incorrectos"})
		return
	}
	loginLimiter.reset(ip) // login correcto: limpia el contador de esta IP
	s.purgeSessions()
	token, err := s.newSession(u.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.setSessionCookie(w, r, token)
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "sessionToken": token})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	if token := primarySessionToken(r); token != "" {
		s.store.db.Exec(`DELETE FROM media_tokens WHERE session_token = ?`, token)
		s.store.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	me := s.currentUser(r)
	if me == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "necesitas iniciar sesión"})
		return
	}
	s.store.db.Exec(`DELETE FROM media_tokens WHERE username = ?`, me.Username)
	s.store.db.Exec(`DELETE FROM sessions WHERE username = ?`, me.Username)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleRefreshSession rota el secreto sin ampliar su vida absoluta: conserva
// created y solo sustituye el token. El cliente lo llama al arrancar, reduciendo
// la ventana útil de un token copiado sin convertir la sesión en infinita.
func (s *Server) handleRefreshSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	oldToken := primarySessionToken(r)
	if oldToken == "" || s.currentUser(r) == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "necesitas iniciar sesión"})
		return
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo renovar la sesión"})
		return
	}
	newToken := hex.EncodeToString(b)
	tx, err := s.store.db.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo renovar la sesión"})
		return
	}
	if _, err = tx.Exec(`DELETE FROM media_tokens WHERE session_token = ?`, oldToken); err == nil {
		var result sql.Result
		result, err = tx.Exec(`UPDATE sessions SET token = ?, last_seen = ? WHERE token = ?`, newToken, time.Now().Unix(), oldToken)
		if err == nil {
			if changed, _ := result.RowsAffected(); changed != 1 {
				err = fmt.Errorf("sesión no encontrada")
			}
		}
	}
	if err != nil {
		tx.Rollback()
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "la sesión ya no es válida"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo renovar la sesión"})
		return
	}
	s.setSessionCookie(w, r, newToken)
	writeJSON(w, http.StatusOK, map[string]string{"sessionToken": newToken})
}

func (s *Server) handleMediaToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	sessionToken := primarySessionToken(r)
	me := s.currentUser(r)
	if sessionToken == "" || me == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "necesitas iniciar sesión"})
		return
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo crear el token multimedia"})
		return
	}
	token := hex.EncodeToString(b)
	expires := time.Now().Add(mediaTokenTTL).Unix()
	s.purgeSessions()
	if _, err := s.store.db.Exec(`INSERT INTO media_tokens (token, session_token, username, expires) VALUES (?,?,?,?)`,
		token, sessionToken, me.Username, expires); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo crear el token multimedia"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expires": expires})
}

// GET /api/auth/me — quién soy + si falta el setup inicial (0 usuarios).
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{"setupNeeded": s.userCount() == 0}
	if u := s.currentUser(r); u != nil {
		resp["user"] = u
	} else {
		resp["user"] = nil
	}
	writeJSON(w, http.StatusOK, resp)
}

// ── Admin: gestión de usuarios ─────────────────────────────────────────────

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	me := s.currentUser(r)
	if me == nil || !me.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "solo admin"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := s.store.db.Query(`SELECT id, username, age, is_admin FROM users ORDER BY is_admin DESC, username`)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		users := []User{}
		for rows.Next() {
			var u User
			var adm int
			rows.Scan(&u.ID, &u.Username, &u.Age, &adm)
			u.IsAdmin = adm == 1
			users = append(users, u)
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	case http.MethodPost:
		var req credsReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body inválido"})
			return
		}
		u, err := s.createUser(req.Username, req.Password, req.Age, req.IsAdmin)
		if err != nil {
			writeInputError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, u)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
	}
}

// handleAdminUserOp maneja las operaciones sobre una cuenta concreta:
//
//	DELETE /api/admin/users/{id}           → borrar
//	PUT    /api/admin/users/{id}/password  → restablecer contraseña (reset por admin)
func (s *Server) handleAdminUserOp(w http.ResponseWriter, r *http.Request) {
	me := s.currentUser(r)
	if me == nil || !me.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "solo admin"})
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	idPart, sub, _ := strings.Cut(rest, "/")
	id, err := strconv.ParseInt(idPart, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id inválido"})
		return
	}

	// Reset de contraseña por admin: PUT /api/admin/users/{id}/password {password}.
	// La regla de contraseñas (validatePassword) se aplica también aquí: la temporal
	// que pone el admin debe cumplir 10+especial; el usuario la cambia luego desde su
	// interfaz (POST /api/auth/password).
	if sub == "password" {
		if r.Method != http.MethodPut {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo PUT"})
			return
		}
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body inválido"})
			return
		}
		if err := s.setPasswordByID(id, req.Password); err != nil {
			writeInputError(w, err)
			return
		}
		// Cerrar las sesiones abiertas del usuario: si pidió reset por olvido, no
		// tiene sesión; y si alguien la tenía, un reset debe invalidarla.
		var uname string
		if e := s.store.db.QueryRow(`SELECT username FROM users WHERE id = ?`, id).Scan(&uname); e == nil {
			s.store.db.Exec(`DELETE FROM media_tokens WHERE username = ?`, uname)
			s.store.db.Exec(`DELETE FROM sessions WHERE username = ?`, uname)
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	if sub != "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "operación no soportada"})
		return
	}

	// DELETE /api/admin/users/{id}
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo DELETE"})
		return
	}
	if id == me.ID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no puedes borrarte a ti mismo"})
		return
	}
	var username string
	var isAdmin int
	switch err := s.store.db.QueryRow(`SELECT username, is_admin FROM users WHERE id = ?`, id).
		Scan(&username, &isAdmin); {
	case err == sql.ErrNoRows:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "usuario no encontrado"})
		return
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// No dejar la instalación sin ningún admin. (Con el carril máquina se podría
	// recuperar, pero mejor no llegar ahí.)
	if isAdmin == 1 {
		var admins int
		s.store.db.QueryRow(`SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&admins)
		if admins <= 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no puedes borrar al último admin"})
			return
		}
	}
	// Purga transaccional del estado personal + sesiones (auditoría H-1), y solo
	// entonces la fila de la cuenta. Si el cascade falla, no borramos el usuario:
	// mejor dejarlo entero que a medias.
	if err := s.store.DeleteUserData(username); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, err := s.store.db.Exec(`DELETE FROM users WHERE id = ?`, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ── Errores de entrada ─────────────────────────────────────────────────────

type inputError struct{ msg string }

func (e inputError) Error() string { return e.msg }
func errBadInput(m string) error   { return inputError{m} }

func writeInputError(w http.ResponseWriter, err error) {
	if _, ok := err.(inputError); ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

// ── Rate-limit de login (H-6) ──────────────────────────────────────────────
//
// Backoff incremental por IP, en memoria (se pierde al reiniciar, aceptable: el
// objetivo es frenar el fuerza-bruta sostenido, no persistir baneos). No sustituye
// a NimShield en la capa NimOS; es la defensa local del propio shim.
//
// Umbral: los primeros `loginFreeTries` fallos no penalizan (dedazos legítimos).
// A partir de ahí, cada fallo impone una espera creciente, tope `loginMaxWait`.
// Un login correcto limpia el contador de la IP.

const (
	loginFreeTries = 5
	loginMaxWait   = 5 * time.Minute
)

type loginAttempt struct {
	fails int
	next  time.Time // instante a partir del cual se permite el siguiente intento
	last  time.Time // permite retirar IPs antiguas y acotar el mapa en memoria
}

type ipLimiter struct {
	mu sync.Mutex
	m  map[string]*loginAttempt
}

var loginLimiter = &ipLimiter{m: make(map[string]*loginAttempt)}
var passwordLimiter = &ipLimiter{m: make(map[string]*loginAttempt)}

// blocked indica si la IP debe esperar, y cuánto.
func (l *ipLimiter) blocked(ip string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.m[ip]
	if a == nil && len(l.m) >= 4096 {
		a = l.m["__overflow__"]
	}
	if a == nil {
		return 0, false
	}
	if wait := time.Until(a.next); wait > 0 {
		return wait, true
	}
	return 0, false
}

// fail registra un fallo y programa la próxima ventana con backoff exponencial
// suave (2^(fails-freeTries) segundos), acotado a loginMaxWait.
func (l *ipLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.m) >= 1024 {
		for key, old := range l.m {
			if now.Sub(old.last) > 30*time.Minute {
				delete(l.m, key)
			}
		}
	}
	a := l.m[ip]
	if a == nil {
		// Límite duro: una oleada de IPs no puede hacer crecer el proceso sin fin.
		// Las direcciones adicionales comparten un bucket, pero no quedan libres.
		if len(l.m) >= 4096 {
			ip = "__overflow__"
			a = l.m[ip]
		}
		if a == nil {
			a = &loginAttempt{}
			l.m[ip] = a
		}
	}
	a.last = now
	a.fails++
	over := a.fails - loginFreeTries
	if over <= 0 {
		return // aún dentro de los intentos libres
	}
	backoff := time.Duration(1<<min(over, 10)) * time.Second
	if backoff > loginMaxWait {
		backoff = loginMaxWait
	}
	a.next = time.Now().Add(backoff)
}

// reset limpia el contador de una IP (login correcto).
func (l *ipLimiter) reset(ip string) {
	l.mu.Lock()
	delete(l.m, ip)
	l.mu.Unlock()
}

// clientIP solo confía en X-Forwarded-For cuando el despliegue declara
// TRUST_PROXY=1. Aceptarlo de cualquiera permitía rotar una cabecera falsa para
// saltarse el limitador y llenar su mapa de IPs.
func clientIP(r *http.Request) string {
	if os.Getenv("TRUST_PROXY") == "1" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i >= 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
