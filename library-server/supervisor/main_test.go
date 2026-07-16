package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMergeEnvReplacesValues(t *testing.T) {
	result := mergeEnv([]string{"A=one", "B=two"}, map[string]string{"a": "changed", "C": "three"})
	joined := strings.ToUpper(strings.Join(result, "\n"))
	if strings.Count(joined, "A=") != 1 || !strings.Contains(joined, "A=CHANGED") || !strings.Contains(joined, "C=THREE") {
		t.Fatalf("entorno inesperado: %v", result)
	}
}

func TestFindExecutable(t *testing.T) {
	dir := t.TempDir()
	name := "library-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findExecutable(dir, "missing", "library-server"); got != path {
		t.Fatalf("findExecutable = %q; esperado %q", got, path)
	}
}
