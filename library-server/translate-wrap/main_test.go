package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEngineCommandUsesModelsDir(t *testing.T) {
	modelsDir = t.TempDir()
	bin = "translateLocally"
	cmd := engineCommand("-l")
	if cmd.Dir != modelsDir {
		t.Fatalf("directorio de trabajo = %q; esperado %q", cmd.Dir, modelsDir)
	}
	key := "XDG_DATA_HOME"
	if os.PathSeparator == '\\' {
		key = "APPDATA"
	}
	found := false
	for _, entry := range cmd.Env {
		if entry == key+"="+modelsDir {
			found = true
		}
	}
	if !found {
		t.Fatalf("falta %s en el entorno del motor", key)
	}
}

func TestMigrateLegacyModelsCopiesMissingAndPreservesExisting(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "old")
	destination := filepath.Join(base, "new")
	if err := os.MkdirAll(filepath.Join(source, "pair"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(destination, "pair"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "pair", "model.bin"), []byte("model"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "pair", "keep.bin"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "pair", "keep.bin"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	copied, err := migrateLegacyModels(source, destination)
	if err != nil {
		t.Fatal(err)
	}
	if copied != 1 {
		t.Fatalf("copiados = %d; esperado 1", copied)
	}
	for path, want := range map[string]string{
		filepath.Join(source, "pair", "model.bin"):      "model",
		filepath.Join(destination, "pair", "model.bin"): "model",
		filepath.Join(destination, "pair", "keep.bin"):  "new",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; esperado %q", path, got, err, want)
		}
	}
}
