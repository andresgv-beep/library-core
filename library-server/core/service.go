package main

import (
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

const supervisedRestartExitCode = 75

var (
	restartRequested atomic.Bool
	exitProcess      = os.Exit
)

// handleServiceControl solo se monta dentro de adminMux. La interfaz solicita
// el reinicio; no toca procesos. El supervisor observa el codigo 75 y levanta
// una instancia nueva del Core.
func (s *Server) handleServiceControl(w http.ResponseWriter, r *http.Request) {
	supervised := os.Getenv("LIBRARY_SUPERVISED") == "1"
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{
			"supervised": supervised,
			"pid":        os.Getpid(),
		})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo no permitido"})
		return
	}
	if !supervised {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Library Server no esta bajo supervision"})
		return
	}
	if !scheduleSupervisedRestart() {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "reinicio ya solicitado"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "reiniciando"})
}

func scheduleSupervisedRestart() bool {
	if !restartRequested.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		exitProcess(supervisedRestartExitCode)
	}()
	return true
}
