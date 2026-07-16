package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorageRootUpdateCreatesPersistsAndRestarts(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Library content")
	configPath := filepath.Join(base, "config.json")
	t.Setenv("LIBRARY_SUPERVISED", "1")
	restartRequested.Store(false)
	exited := make(chan int, 1)
	originalExit := exitProcess
	exitProcess = func(code int) { exited <- code }
	t.Cleanup(func() { exitProcess = originalExit; restartRequested.Store(false) })

	body, _ := json.Marshal(storageConfig{ContentRoot: root})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/storage", bytes.NewReader(body))
	(&poolInfo{configPath: configPath}).handleStorage(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	for _, child := range []string{"zim", "downloads", "models", "maps"} {
		if st, err := os.Stat(filepath.Join(root, child)); err != nil || !st.IsDir() {
			t.Fatalf("no se creo %s: %v", child, err)
		}
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg storageConfig
	if err := json.Unmarshal(raw, &cfg); err != nil || cfg.ContentRoot != root {
		t.Fatalf("config inesperada: %+v, err=%v", cfg, err)
	}
	select {
	case code := <-exited:
		if code != supervisedRestartExitCode {
			t.Fatalf("exit = %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("no se solicito reinicio")
	}
}

func TestStorageRootUpdateRejectsRelativePath(t *testing.T) {
	body := bytes.NewBufferString(`{"contentRoot":"relative-folder"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/storage", body)
	(&poolInfo{configPath: filepath.Join(t.TempDir(), "config.json")}).handleStorage(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
}
