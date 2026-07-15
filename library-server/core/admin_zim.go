// admin_zim.go — Acciones de gestión de colecciones ZIM para el Panel de Control.
//
// El Panel "enchufa" a la biblioteca los .zim que ya viven en el pool (zim/):
// los ficheros llegan por descarga/copia/share, y aquí se registran. Desde la
// retirada de kiwix (§8, 2026-07-14) el registro es NATIVO: el servidor lee la
// identidad y metadata del .zim con el motor propio y edita library.xml él
// mismo (admin_zim_native.go) — kiwix-manage ya no se usa ni hace falta en la
// máquina. El formato del xml sigue siendo compatible con kiwix (rollback).
//
// Quitar = desregistrar de library.xml. NO borra el fichero del disco (borrar
// datos será una acción explícita aparte).

package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type adminZim struct {
	libraryXML string // ruta a library.xml
	zimDir     string // carpeta de ZIMs del pool

	// onLibraryChange: se invoca tras una alta/baja exitosa en library.xml. El
	// motor nativo lo usa para invalidar su registro de archives abiertos (§23).
	onLibraryChange func()
}

// libraryChanged notifica el cambio de library.xml a quien esté suscrito.
func (a *adminZim) libraryChanged() {
	if a.onLibraryChange != nil {
		a.onLibraryChange()
	}
}

// ── Parseo de library.xml ──────────────────────────────────────────────────

type xmlLibrary struct {
	Books []xmlBook `xml:"book"`
}
type xmlBook struct {
	ID       string `xml:"id,attr"`
	Path     string `xml:"path,attr"`
	Title    string `xml:"title,attr"`
	Name     string `xml:"name,attr"`
	Language string `xml:"language,attr"`
}

func (a *adminZim) readLibrary() ([]xmlBook, error) {
	data, err := os.ReadFile(a.libraryXML)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // sin library.xml todavía: biblioteca vacía
		}
		return nil, err
	}
	var lib xmlLibrary
	if err := xml.Unmarshal(data, &lib); err != nil {
		return nil, err
	}
	return lib.Books, nil
}

// ── Tipos de respuesta ─────────────────────────────────────────────────────

type registeredZim struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Name     string `json:"name,omitempty"`
	Language string `json:"language,omitempty"`
	File     string `json:"file"`
	Bytes    int64  `json:"bytes"`
	Present  bool   `json:"present"` // el fichero existe en disco
}

type unregisteredZim struct {
	File  string `json:"file"`
	Bytes int64  `json:"bytes"`
}

// ── GET /api/admin/zim ─────────────────────────────────────────────────────

func (a *adminZim) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo GET"})
		return
	}
	books, err := a.readLibrary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "library.xml: " + err.Error()})
		return
	}

	// Ficheros .zim presentes en el pool → tamaño por nombre.
	sizes := map[string]int64{}
	if entries, derr := os.ReadDir(a.zimDir); derr == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".zim") {
				continue
			}
			if info, ierr := e.Info(); ierr == nil {
				sizes[e.Name()] = info.Size()
			}
		}
	}

	registered := make([]registeredZim, 0, len(books))
	claimed := map[string]bool{}
	for _, b := range books {
		file := filepath.Base(b.Path)
		claimed[file] = true
		sz, present := sizes[file]
		registered = append(registered, registeredZim{
			ID: b.ID, Title: b.Title, Name: b.Name, Language: b.Language,
			File: file, Bytes: sz, Present: present,
		})
	}

	unregistered := make([]unregisteredZim, 0)
	for file, sz := range sizes {
		if !claimed[file] {
			unregistered = append(unregistered, unregisteredZim{File: file, Bytes: sz})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"canManage":    true, // gestión nativa: siempre disponible (sin kiwix-manage)
		"libraryXML":   a.libraryXML,
		"registered":   registered,
		"unregistered": unregistered,
	})
}

// ── POST /api/admin/zim/register  {file} ───────────────────────────────────

func (a *adminZim) handleRegister(w http.ResponseWriter, r *http.Request) {
	file, ok := a.readFileArg(w, r)
	if !ok {
		return
	}
	abs := filepath.Join(a.zimDir, file)
	if st, err := os.Stat(abs); err != nil || st.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "el fichero no existe en el pool"})
		return
	}
	id, err := a.addBook(file)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "registrar: " + err.Error()})
		return
	}
	a.libraryChanged() // que el motor nativo relea library.xml a la próxima petición
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "file": file, "id": id})
}

// ── POST /api/admin/zim/unregister  {id} ───────────────────────────────────

func (a *adminZim) handleUnregister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body inválido"})
		return
	}
	id := req.ID
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falta id"})
		return
	}
	if err := a.removeBook(id); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "no está en la biblioteca") {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	a.libraryChanged() // cerrar el archive nativo desregistrado (§23) y dejar de servirlo
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "note": "fichero conservado en el pool"})
}

// ── Helpers ────────────────────────────────────────────────────────────────

// readFileArg lee y sanea el argumento {file}: nombre simple .zim, sin rutas
// (anti path-traversal). Escribe la respuesta de error y devuelve ok=false.
func (a *adminZim) readFileArg(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req struct {
		File string `json:"file"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body inválido"})
		return "", false
	}
	file := req.File
	if file == "" || filepath.Base(file) != file || strings.ContainsAny(file, `/\`) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nombre de fichero inválido"})
		return "", false
	}
	if !strings.EqualFold(filepath.Ext(file), ".zim") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "debe ser un .zim"})
		return "", false
	}
	return file, true
}

// registerDownloaded registra un .zim recién descargado al pool (auto-registro
// del catálogo). Idempotente: addBook no duplica si ya está en library.xml.
func (a *adminZim) registerDownloaded(destPath string) error {
	file := filepath.Base(destPath)
	if !strings.EqualFold(filepath.Ext(file), ".zim") {
		return fmt.Errorf("no es un .zim: %s", file)
	}
	if st, err := os.Stat(filepath.Join(a.zimDir, file)); err != nil || st.IsDir() {
		return fmt.Errorf("fichero no encontrado en el pool: %s", file)
	}
	if _, err := a.addBook(file); err != nil {
		return err
	}
	a.libraryChanged() // auto-registro del catálogo: que el motor nativo lo recoja
	return nil
}

func (a *adminZim) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/zim", a.handleList)
	mux.HandleFunc("/api/admin/zim/register", a.handleRegister)
	mux.HandleFunc("/api/admin/zim/unregister", a.handleUnregister)
}
