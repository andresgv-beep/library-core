// storage.go — Pool de almacenamiento (POOL-CONTRACT.md).
//
// El pool es la raíz única de datos (POOL_ROOT) con un layout conocido: zim,
// models, downloads, maps, db. Este módulo resuelve las rutas del pool con la
// precedencia del contrato y expone el inventario read-only que consume el
// Panel de Control (GET /api/storage). No muta nada: describe qué cuelga de la
// raíz, cuánto ocupa y qué motor lo sirve.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// resolvePoolPath resuelve la ruta de un componente del pool con la precedencia
// del contrato (POOL-CONTRACT.md §5): env var explícita > derivada de POOL_ROOT >
// default legacy. `sub` es la subruta dentro del pool (p. ej. "db/library.db").
// Con POOL_ROOT vacío y sin env, devuelve el default de hoy → cero regresión.
func resolvePoolPath(envKey, poolRoot, sub, legacyDefault string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if poolRoot != "" {
		return filepath.Join(poolRoot, filepath.FromSlash(sub))
	}
	return legacyDefault
}

// sectionSpec: una subruta del pool con su motor responsable (POOL-CONTRACT.md §2).
type sectionSpec struct {
	key, engine, path string
}

// poolInfo describe el pool para el inventario del Panel.
type poolInfo struct {
	root              string
	provider          string
	configPath        string
	externallyManaged bool
	sections          []sectionSpec
}

type storageConfig struct {
	ContentRoot string `json:"contentRoot"`
}

type storageVolume struct {
	Path string `json:"path"`
}

// storageSection es la forma JSON de una sección (POOL-CONTRACT.md §6).
type storageSection struct {
	Key    string `json:"key"`
	Path   string `json:"path"`
	Engine string `json:"engine"`
	Items  int    `json:"items"`
	Bytes  int64  `json:"bytes"`
	Exists bool   `json:"exists"`
}

// handleStorage: GET /api/storage — inventario read-only del pool.
func (p *poolInfo) handleStorage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		p.handleStorageRootUpdate(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, PUT")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "metodo no permitido"})
		return
	}
	sections := make([]storageSection, 0, len(p.sections))
	var used int64
	for _, s := range p.sections {
		sec := storageSection{Key: s.key, Path: s.path, Engine: s.engine}
		if s.path != "" {
			sec.Bytes, sec.Items, sec.Exists = dirUsage(s.path)
			used += sec.Bytes
		}
		sections = append(sections, sec)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"root":         p.root,
		"provider":     p.provider,
		"configurable": p.configPath != "" && !p.externallyManaged,
		"volumes":      listStorageVolumes(),
		"usedBytes":    used,
		"sections":     sections,
	})
}

func (p *poolInfo) handleStorageRootUpdate(w http.ResponseWriter, r *http.Request) {
	if p.externallyManaged {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "la ubicacion esta gestionada por el entorno del servidor"})
		return
	}
	if p.configPath == "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "este servidor no dispone de configuracion persistente"})
		return
	}
	var input storageConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalido"})
		return
	}
	root, err := prepareStorageRoot(input.ContentRoot)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := writeStorageConfig(p.configPath, storageConfig{ContentRoot: root}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo guardar la ubicacion: " + err.Error()})
		return
	}
	status := http.StatusOK
	restarting := false
	if os.Getenv("LIBRARY_SUPERVISED") == "1" {
		status = http.StatusAccepted
		restarting = scheduleSupervisedRestart()
	}
	writeJSON(w, status, map[string]any{"root": root, "restartRequired": true, "restarting": restarting})
}

func prepareStorageRoot(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("indica una carpeta")
	}
	root, err := filepath.Abs(raw)
	if err != nil || !filepath.IsAbs(raw) {
		return "", fmt.Errorf("la ruta debe ser absoluta")
	}
	root = filepath.Clean(root)
	volume := filepath.VolumeName(root)
	if root == string(filepath.Separator) || (volume != "" && strings.EqualFold(root, volume+string(filepath.Separator))) {
		return "", fmt.Errorf("elige una carpeta dentro del disco, no la raiz del disco")
	}
	for _, protected := range []string{os.Getenv("WINDIR"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if protected != "" && pathInside(root, protected) {
			return "", fmt.Errorf("esa ubicacion pertenece al sistema; elige una carpeta de datos")
		}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("no se pudo crear la carpeta: %w", err)
	}
	probe, err := os.CreateTemp(root, ".nimos-write-test-*")
	if err != nil {
		return "", fmt.Errorf("el servicio no puede escribir en esa carpeta: %w", err)
	}
	probePath := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(probePath)
		return "", fmt.Errorf("no se pudo comprobar la carpeta: %w", err)
	}
	_ = os.Remove(probePath)
	for _, child := range []string{"zim", "downloads", "models", "maps"} {
		if err := os.MkdirAll(filepath.Join(root, child), 0o755); err != nil {
			return "", fmt.Errorf("no se pudo preparar %s: %w", child, err)
		}
	}
	return root, nil
}

func pathInside(path, parent string) bool {
	path = strings.TrimRight(filepath.Clean(path), string(filepath.Separator))
	parent = strings.TrimRight(filepath.Clean(parent), string(filepath.Separator))
	return strings.EqualFold(path, parent) || strings.HasPrefix(strings.ToLower(path), strings.ToLower(parent+string(filepath.Separator)))
}

func writeStorageConfig(path string, cfg storageConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

// dirUsage suma recursivamente el tamaño de un directorio y cuenta sus entradas
// de primer nivel (los "items": nº de ZIMs, modelos, colecciones…). Solo stat de
// ficheros, no lee contenido. exists=false si no es un directorio.
func dirUsage(path string) (bytes int64, items int, exists bool) {
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return 0, 0, false
	}
	if entries, err := os.ReadDir(path); err == nil {
		items = len(entries)
	}
	filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, ierr := d.Info(); ierr == nil {
			bytes += info.Size()
		}
		return nil
	})
	return bytes, items, true
}
