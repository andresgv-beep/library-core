// sidecars.go — arranque y cierre de los procesos que la app necesita.
//
// La app de escritorio es dueña de su backend: al abrir levanta translate-wrap
// (:8091) y el core (:8090); mientras el core no responde /api/health sirve un
// splash con auto-refresh; al cerrar mata lo que arrancó. Si ya hay un core en
// marcha (dev), se engancha a él y NO lo mata al salir.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"
)

const (
	corePort      = "8090"
	translatePort = "8091"
	coreURL       = "http://127.0.0.1:" + corePort
	healthPath    = "/api/health"
	bootTimeout   = 60 * time.Second
)

// shell es el handler de la webview: hace de reverse-proxy al core y gestiona el
// ciclo de vida de los sidecars.
type shell struct {
	target   *url.URL
	proxy    *httputil.ReverseProxy
	binDir   string
	poolRoot string

	ready atomic.Bool
	owned []*exec.Cmd // procesos que arrancamos nosotros (a matar al cerrar)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func newShell() (*shell, error) {
	target, err := url.Parse(coreURL)
	if err != nil {
		return nil, err
	}
	s := &shell{
		target:   target,
		proxy:    httputil.NewSingleHostReverseProxy(target),
		binDir:   resolveBinDir(),
		poolRoot: env("POOL_ROOT", `C:\Users\asus\nimos-library-pool`),
	}
	// Si el core cae en mitad de sesión no reventamos la webview: 503 corto.
	s.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "Library Server no disponible", http.StatusServiceUnavailable)
	}
	return s, nil
}

// ServeHTTP recibe TODAS las peticiones de la webview. Hasta que el core está
// listo devuelve el splash (auto-refresh); después, proxy transparente.
func (s *shell) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		serveSplash(w)
		return
	}
	s.proxy.ServeHTTP(w, r)
}

// onStartup lanza el arranque en segundo plano para no bloquear la UI.
func (s *shell) onStartup(_ context.Context) {
	go s.boot()
}

func (s *shell) boot() {
	if s.healthy() {
		log.Print("core ya en marcha; me engancho (no lo gestiono)")
		s.ready.Store(true)
		return
	}
	log.Printf("arrancando sidecars desde %s (pool=%s)", s.binDir, s.poolRoot)
	s.spawn("translate-wrap", []string{"PORT=" + translatePort, "BIND=127.0.0.1"})
	s.spawn("core", s.coreEnv())

	deadline := time.Now().Add(bootTimeout)
	for time.Now().Before(deadline) {
		if s.healthy() {
			s.ready.Store(true)
			log.Print("core listo → cargando lector")
			return
		}
		time.Sleep(400 * time.Millisecond)
	}
	log.Printf("TIMEOUT: el core no respondió en %s", bootTimeout)
}

// onShutdown mata en árbol lo que arrancamos (translateLocally se cuelga de
// translate-wrap, de ahí el /T en Windows).
func (s *shell) onShutdown(_ context.Context) {
	for _, c := range s.owned {
		if c == nil || c.Process == nil {
			continue
		}
		if runtime.GOOS == "windows" {
			_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(c.Process.Pid)).Run()
		} else {
			_ = c.Process.Kill()
		}
	}
}

func (s *shell) coreEnv() []string {
	zim := filepath.Join(s.poolRoot, "zim")
	return []string{
		"POOL_ROOT=" + s.poolRoot,
		"ZIM_DIR=" + zim,
		"LIBRARY_XML=" + filepath.Join(zim, "library.xml"),
		"PORT=" + corePort,
		"BIND=127.0.0.1",
		"TRANSLATE_URL=http://127.0.0.1:" + translatePort,
	}
}

// spawn arranca un binario del binDir con env extra; su cwd es binDir para que
// siblingDir (www-client/www-panel) resuelva junto al exe.
func (s *shell) spawn(name string, extraEnv []string) {
	exe := filepath.Join(s.binDir, name)
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	cmd := exec.Command(exe)
	cmd.Dir = s.binDir
	cmd.Env = append(os.Environ(), extraEnv...)
	if lf, err := logFile(name); err == nil {
		cmd.Stdout, cmd.Stderr = lf, lf
	}
	noWindow(cmd)
	if err := cmd.Start(); err != nil {
		log.Printf("no pude arrancar %s: %v", name, err)
		return
	}
	superviseChild(cmd) // muere con la app aunque la maten a la fuerza (Job Object)
	s.owned = append(s.owned, cmd)
	log.Printf("sidecar %s pid=%d", name, cmd.Process.Pid)
}

// logFile abre un log por sidecar bajo %TEMP%/nimos-library (siempre escribible,
// la app corre sin consola donde volcar stdout).
func logFile(name string) (*os.File, error) {
	dir := filepath.Join(os.TempDir(), "nimos-library")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return os.Create(filepath.Join(dir, name+".log"))
}

func (s *shell) healthy() bool {
	c := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := c.Get(coreURL + healthPath)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// resolveBinDir busca la carpeta con los binarios de los sidecars: bin/ junto al
// exe (empaquetado) o, en dev, junto al propio exe.
func resolveBinDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if st, err := os.Stat(filepath.Join(dir, "bin")); err == nil && st.IsDir() {
			return filepath.Join(dir, "bin")
		}
		return dir
	}
	return "bin"
}
