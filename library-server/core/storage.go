// storage.go — Pool de almacenamiento (POOL-CONTRACT.md).
//
// El pool es la raíz única de datos (POOL_ROOT) con un layout conocido: zim,
// models, downloads, maps, db. Este módulo resuelve las rutas del pool con la
// precedencia del contrato y expone el inventario read-only que consume el
// Panel de Control (GET /api/storage). No muta nada: describe qué cuelga de la
// raíz, cuánto ocupa y qué motor lo sirve.

package main

import (
	"net/http"
	"os"
	"path/filepath"
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
	root     string
	provider string
	sections []sectionSpec
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
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo GET"})
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
		"root":      p.root,
		"provider":  p.provider,
		"usedBytes": used,
		"sections":  sections,
	})
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
