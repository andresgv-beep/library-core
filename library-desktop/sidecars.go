// sidecars.go gestiona los modos local (todo en uno) y gateway remoto.
package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

const (
	corePort     = "8090"
	localCoreURL = "http://127.0.0.1:" + corePort
	healthPath   = "/api/health"
	bootTimeout  = 60 * time.Second
)

// distributionMode se fija a "remote" por ldflags en el build del cliente
// ligero. El valor por defecto conserva el paquete todo-en-uno existente.
var distributionMode = "all-in-one"

type shell struct {
	mu        sync.RWMutex
	target    *url.URL
	proxy     *httputil.ReverseProxy
	remote    bool
	startPath string

	configured atomic.Bool
	ready      atomic.Bool
	booting    atomic.Bool
}

func newShell() (*shell, error) {
	target, remote, configured, err := resolveShellTarget()
	if err != nil {
		return nil, err
	}
	s := &shell{
		remote: remote,
	}
	if interfaceMode == "panel" {
		s.startPath = "/panel/"
	}
	s.configured.Store(configured)
	if configured {
		s.installProxy(target)
	}
	return s, nil
}

func (s *shell) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == gatewayConfigPath {
		s.serveGatewayConfig(w, r)
		return
	}
	if s.remote && !s.configured.Load() {
		serveSetup(w, "")
		return
	}
	if !s.ready.Load() {
		s.startBoot()
		serveSplash(w, s.remote, s.targetString())
		return
	}
	if s.startPath != "" && r.URL.Path == "/" {
		r.URL.Path = s.startPath
		r.URL.RawPath = ""
	}

	s.mu.RLock()
	proxy := s.proxy
	s.mu.RUnlock()
	if proxy == nil {
		http.Error(w, "Library Server no configurado", http.StatusServiceUnavailable)
		return
	}
	proxy.ServeHTTP(w, r)
}

func (s *shell) onStartup(_ context.Context) { s.startBoot() }

func (s *shell) startBoot() {
	if !s.configured.Load() || !s.booting.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer s.booting.Store(false)
		s.boot()
	}()
}

func (s *shell) boot() {
	if s.healthy() {
		log.Printf("Library Server disponible en %s", s.targetString())
		s.ready.Store(true)
		return
	}

	log.Printf("esperando Library Server en %s", s.targetString())

	deadline := time.Now().Add(bootTimeout)
	for time.Now().Before(deadline) {
		if s.healthy() {
			s.ready.Store(true)
			log.Print("Library Server listo; cargando lector")
			return
		}
		time.Sleep(400 * time.Millisecond)
	}
	log.Printf("TIMEOUT: Library Server no respondio en %s", bootTimeout)
}

func (s *shell) onShutdown(_ context.Context) {}

func (s *shell) healthy() bool {
	target := s.targetString()
	if target == "" {
		return false
	}
	c := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := c.Get(target + healthPath)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
