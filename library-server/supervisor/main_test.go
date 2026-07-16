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

func TestCoreEnvLoadsConfiguredContentRootAndKeepsStateDB(t *testing.T) {
	base := t.TempDir()
	content := filepath.Join(base, "large-library")
	t.Setenv("NIMOS_LIBRARY_DATA", base)
	t.Setenv("POOL_ROOT", "")
	configPath := filepath.Join(base, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"contentRoot":"`+strings.ReplaceAll(content, `\`, `\\`)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &supervisor{}
	joined := strings.Join(s.coreEnv(), "\n")
	for _, want := range []string{
		"POOL_ROOT=" + content,
		"POOL_PROVIDER=configured",
		"DB_PATH=" + filepath.Join(base, "data", "db", "library.db"),
		"DOWNLOADS_DB=" + filepath.Join(base, "data", "db", "downloads.db"),
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("falta %q en entorno:\n%s", want, joined)
		}
	}
}
