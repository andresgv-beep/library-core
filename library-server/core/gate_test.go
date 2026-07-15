package main

// Regresión de la puerta de búsquedas v1.1 (cola de espera acotada).
// Cubre: camino rápido, espera que despierta al liberar slot, desbordamiento
// de cola → 429 inmediato, y cancelación por desconexión del cliente.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func gateServer(gate, _ int) *Server {
	return &Server{searchGate: make(chan struct{}, gate)}
}

func req(ctx context.Context) *http.Request {
	r := httptest.NewRequest("GET", "/api/search?q=x", nil)
	return r.WithContext(ctx)
}

// Camino rápido: con slot libre entra sin esperar.
func TestGateFastPath(t *testing.T) {
	s := gateServer(2, 4)
	w := httptest.NewRecorder()
	if !s.acquireSearch(w, req(context.Background())) {
		t.Fatal("con gate vacío debería entrar")
	}
	s.releaseSearch()
}

// Espera: gate lleno, el esperante entra en cuanto un activo libera.
func TestGateWaitsForSlot(t *testing.T) {
	s := gateServer(1, 4)
	if !s.acquireSearch(httptest.NewRecorder(), req(context.Background())) {
		t.Fatal("primer acquire debería entrar")
	}

	got := make(chan bool, 1)
	go func() {
		got <- s.acquireSearch(httptest.NewRecorder(), req(context.Background()))
	}()

	time.Sleep(50 * time.Millisecond) // asegurar que ya está esperando
	s.releaseSearch()

	select {
	case ok := <-got:
		if !ok {
			t.Fatal("el esperante debería haber recibido el slot")
		}
		s.releaseSearch()
	case <-time.After(2 * time.Second):
		t.Fatal("el esperante no despertó al liberarse el slot")
	}
}

// Desbordamiento: gate lleno + cola llena → 429 inmediato para el siguiente.
func TestGateQueueOverflow(t *testing.T) {
	s := gateServer(1, 4)
	if !s.acquireSearch(httptest.NewRecorder(), req(context.Background())) {
		t.Fatal("primer acquire debería entrar")
	}

	// Llenar la cola con searchQueueMax esperantes bloqueados.
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < searchQueueMax; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.acquireSearch(httptest.NewRecorder(), req(ctx))
		}()
	}
	// Esperar a que los esperantes estén contados.
	deadline := time.Now().Add(2 * time.Second)
	for s.searchWaiters.Load() < int32(searchQueueMax) {
		if time.Now().After(deadline) {
			t.Fatalf("la cola no se llenó: %d esperantes", s.searchWaiters.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}

	// El siguiente no cabe: 429 sin esperar.
	w := httptest.NewRecorder()
	start := time.Now()
	if s.acquireSearch(w, req(context.Background())) {
		t.Fatal("con cola llena debería rechazar")
	}
	if time.Since(start) > time.Second {
		t.Fatal("el rechazo por cola llena debe ser inmediato")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("esperaba 429, recibí %d", w.Code)
	}

	cancel() // liberar a los esperantes por contexto
	wg.Wait()
	s.releaseSearch()
}

// Desconexión: un esperante cuyo cliente se va suelta su hueco al instante.
func TestGateWaiterClientGone(t *testing.T) {
	s := gateServer(1, 4)
	if !s.acquireSearch(httptest.NewRecorder(), req(context.Background())) {
		t.Fatal("primer acquire debería entrar")
	}

	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan bool, 1)
	go func() {
		got <- s.acquireSearch(httptest.NewRecorder(), req(ctx))
	}()

	time.Sleep(50 * time.Millisecond)
	cancel() // el cliente cierra la conexión

	select {
	case ok := <-got:
		if ok {
			t.Fatal("un esperante cancelado no debe recibir slot")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el esperante cancelado no se soltó")
	}
	if w := s.searchWaiters.Load(); w != 0 {
		t.Fatalf("la cola debería quedar vacía, hay %d", w)
	}
	s.releaseSearch()
}
