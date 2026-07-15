package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// API de gestión persistida (DESIGN §7). Escrituras (POST/PUT/DELETE) exigen
// token si NIMOS_TOKEN está configurado (auth por canal, §6 · lo hace el middleware).
//
// El estado personal se scopea por usuario: cada cuenta ve/edita el suyo; el
// anónimo comparte el "invitado" (currentUsername → "").

func errmap(err error) map[string]string { return map[string]string{"error": err.Error()} }
func now() int64                         { return time.Now().Unix() }

// ─── /api/favorites ───────────────────────────────────────────────────────────
func (s *Server) handleFavorites(w http.ResponseWriter, r *http.Request) {
	user := s.currentUsername(r)
	switch r.Method {
	case http.MethodGet:
		favs, err := s.store.ListFavorites(user)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, favs)
	case http.MethodPut, http.MethodPost:
		var f Fav
		if err := json.NewDecoder(r.Body).Decode(&f); err != nil || (f.ItemID == "" && (f.Lib == "" || f.Path == "")) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "favorito inválido"})
			return
		}
		if err := s.store.PutFavorite(user, f, now()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case http.MethodDelete:
		lib, path, itemID := r.URL.Query().Get("lib"), r.URL.Query().Get("path"), r.URL.Query().Get("itemId")
		if itemID == "" && (lib == "" || path == "") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "faltan lib y path"})
			return
		}
		if err := s.store.DeleteFavorite(user, lib, path, itemID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// ─── /api/notes ───────────────────────────────────────────────────────────────
func (s *Server) handleNotes(w http.ResponseWriter, r *http.Request) {
	user := s.currentUsername(r)
	switch r.Method {
	case http.MethodGet:
		lib, path, itemID := r.URL.Query().Get("lib"), r.URL.Query().Get("path"), r.URL.Query().Get("itemId")
		if itemID != "" || (lib != "" && path != "") {
			n, err := s.store.GetNote(user, lib, path, itemID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errmap(err))
				return
			}
			writeJSON(w, http.StatusOK, n) // null si no hay nota
			return
		}
		notes, err := s.store.ListNotes(user)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, notes)
	case http.MethodPut, http.MethodPost:
		var n Note
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil || (n.ItemID == "" && (n.Lib == "" || n.Path == "")) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nota inválida"})
			return
		}
		var err error
		if strings.TrimSpace(n.Body) == "" {
			err = s.store.DeleteNote(user, n.Lib, n.Path, n.ItemID) // nota vacía = borrar
		} else {
			err = s.store.PutNote(user, n, now())
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case http.MethodDelete:
		lib, path, itemID := r.URL.Query().Get("lib"), r.URL.Query().Get("path"), r.URL.Query().Get("itemId")
		if err := s.store.DeleteNote(user, lib, path, itemID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// ─── /api/history ─────────────────────────────────────────────────────────────
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	user := s.currentUsername(r)
	switch r.Method {
	case http.MethodGet:
		hist, err := s.store.ListHistory(user, 200)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, hist)
	case http.MethodPost:
		var v Visit
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil || (v.ItemID == "" && (v.Lib == "" || v.Path == "")) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "visita inválida"})
			return
		}
		if err := s.store.AddHistory(user, v, now()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case http.MethodDelete:
		// Borrado manual: ?id=N (una fila) · ?lib=&path= (una página) · sin params (todo).
		q := r.URL.Query()
		var err error
		if idStr := q.Get("id"); idStr != "" {
			id, perr := strconv.ParseInt(idStr, 10, 64)
			if perr != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id inválido"})
				return
			}
			err = s.store.DeleteHistoryID(user, id)
		} else if itemID := q.Get("itemId"); itemID != "" {
			err = s.store.DeleteHistoryPath(user, "", "", itemID)
		} else if lib := q.Get("lib"); lib != "" {
			err = s.store.DeleteHistoryPath(user, lib, q.Get("path"), "")
		} else {
			err = s.store.ClearHistory(user)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// ─── /api/recent ──────────────────────────────────────────────────────────────
func (s *Server) handleRecent(w http.ResponseWriter, r *http.Request) {
	recent, err := s.store.ListRecent(s.currentUsername(r), 40)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errmap(err))
		return
	}
	writeJSON(w, http.StatusOK, recent)
}

// ─── /api/tags ────────────────────────────────────────────────────────────────
func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	user := s.currentUsername(r)
	q := r.URL.Query()
	switch r.Method {
	case http.MethodGet:
		switch {
		case q.Get("itemId") != "" || (q.Get("lib") != "" && q.Get("path") != ""):
			tags, err := s.store.PageTags(user, q.Get("lib"), q.Get("path"), q.Get("itemId"))
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errmap(err))
				return
			}
			writeJSON(w, http.StatusOK, tags)
		case q.Get("tag") != "": // páginas de una etiqueta
			pages, err := s.store.ListTagPages(user, q.Get("tag"))
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errmap(err))
				return
			}
			writeJSON(w, http.StatusOK, pages)
		case q.Get("keys") != "": // "lib\npath" de páginas con etiqueta (marca botón)
			keys, err := s.store.TaggedKeys(user)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errmap(err))
				return
			}
			writeJSON(w, http.StatusOK, keys)
		default: // nube de etiquetas con conteos
			tags, err := s.store.ListTags(user)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errmap(err))
				return
			}
			writeJSON(w, http.StatusOK, tags)
		}
	case http.MethodPut, http.MethodPost:
		var t Tag
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "etiqueta inválida"})
			return
		}
		t.Tag = strings.TrimSpace(t.Tag)
		if (t.ItemID == "" && (t.Lib == "" || t.Path == "")) || t.Tag == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "faltan itemId o lib/path, o tag"})
			return
		}
		if err := s.store.AddTag(user, t, now()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case http.MethodDelete:
		tag := q.Get("tag")
		if tag == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falta tag"})
			return
		}
		var err error
		if itemID := q.Get("itemId"); itemID != "" {
			err = s.store.RemoveTag(user, "", "", itemID, tag)
		} else if lib := q.Get("lib"); lib != "" { // quitar la etiqueta de una página
			err = s.store.RemoveTag(user, lib, q.Get("path"), "", tag)
		} else { // borrar la etiqueta de todas las páginas
			err = s.store.DeleteTag(user, tag)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errmap(err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
