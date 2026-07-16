package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServiceControlRequiresSupervisor(t *testing.T) {
	t.Setenv("LIBRARY_SUPERVISED", "")
	restartRequested.Store(false)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/service", nil)
	(&Server{}).handleServiceControl(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d; esperado %d", recorder.Code, http.StatusConflict)
	}
}

func TestServiceControlRequestsReservedExit(t *testing.T) {
	t.Setenv("LIBRARY_SUPERVISED", "1")
	restartRequested.Store(false)
	exited := make(chan int, 1)
	originalExit := exitProcess
	exitProcess = func(code int) { exited <- code }
	t.Cleanup(func() { exitProcess = originalExit; restartRequested.Store(false) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/service", nil)
	(&Server{}).handleServiceControl(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d; esperado %d", recorder.Code, http.StatusAccepted)
	}
	select {
	case code := <-exited:
		if code != supervisedRestartExitCode {
			t.Fatalf("exit = %d; esperado %d", code, supervisedRestartExitCode)
		}
	case <-time.After(time.Second):
		t.Fatal("no se solicito la salida al supervisor")
	}
}
