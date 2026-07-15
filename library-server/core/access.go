// access.go — Control de acceso por colección (nivel + edad) y enforcement.
//
// Cada colección tiene un nivel (open/login/blocked, por defecto blocked) y una
// edad mínima. La regla canSee combina ambos con el usuario de la sesión. El
// admin lo ve y lo cambia desde el Panel; el lector solo ve lo que le toca.

package main

import (
	"encoding/json"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// flexInt acepta un entero venga como número o como string ("12" o 12) en JSON.
// El input numérico del navegador puede mandar string; no queremos un 400 por eso.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}

type accessCfg struct {
	Access string `json:"access"` // "open" | "login" | "blocked"
	MinAge int    `json:"minAge"`
	// AllowDownload: si es true, cualquiera que pueda VER la colección puede además
	// DESCARGAR el fichero sin cuenta. Si es false (default), ver puede ser público
	// pero descargar exige sesión (modelo web: navegas libre, para bajar te registras).
	// La descarga NUNCA salta el gate de ver: es un permiso ADICIONAL, no alternativo.
	AllowDownload bool `json:"allowDownload"`
}

func validAccess(a string) bool {
	return a == "open" || a == "login" || a == "blocked"
}

// collectionAccess devuelve la config de una colección. Sin fila → blocked/0
// (todo lo que añade el admin queda bloqueado hasta que lo abra).
func (s *Server) collectionAccess(id string) accessCfg {
	cfg := accessCfg{Access: "blocked"}
	if s.store == nil || s.store.db == nil {
		return cfg // sin store no se puede comprobar nada → se cierra, no se abre
	}
	var dl int
	s.store.db.QueryRow(`SELECT access, min_age, allow_download FROM collection_access WHERE collection_id = ?`, id).
		Scan(&cfg.Access, &cfg.MinAge, &dl)
	cfg.AllowDownload = dl == 1
	return cfg
}

// accessMap carga TODA la config de acceso de una vez (una query). Los filtros de
// listado/búsqueda (filterMediaItems, filterSearchResults, visibleLibs…) llamaban
// collectionAccess por cada item → N+1 sobre SQLite, que en la Pi con listados de
// cientos de items se nota (auditoría O-1). Con el mapa en mano, canSeeCached
// resuelve en memoria. La tabla es pequeña (una fila por colección), así que cargarla
// entera es barato. Sin fila → blocked (misma regla que collectionAccess).
func (s *Server) accessMap() map[string]accessCfg {
	m := map[string]accessCfg{}
	if s.store == nil || s.store.db == nil {
		return m
	}
	rows, err := s.store.db.Query(`SELECT collection_id, access, min_age, allow_download FROM collection_access`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var cfg accessCfg
		var dl int
		if rows.Scan(&id, &cfg.Access, &cfg.MinAge, &dl) == nil {
			cfg.AllowDownload = dl == 1
			m[id] = cfg
		}
	}
	return m
}

// canSeeCached resuelve el acceso contra un mapa ya cargado (sin tocar SQLite).
// Sin fila en el mapa → blocked, igual que collectionAccess/canSee.
func canSeeCached(u *User, m map[string]accessCfg, collectionID string) bool {
	cfg, ok := m[collectionID]
	if !ok {
		cfg = accessCfg{Access: "blocked"}
	}
	return canSee(u, cfg)
}

// canSee aplica el modelo acordado con Andrés:
//   - admin: todo.
//   - blocked: nadie (salvo admin).
//   - edad mínima > 0: exige cuenta (sin sesión no se comprueba la edad) y edad ≥ mín.
//   - open sin edad: todos. login sin edad: cualquiera con sesión.
func canSee(u *User, cfg accessCfg) bool {
	if u != nil && u.IsAdmin {
		return true
	}
	switch cfg.Access {
	case "open":
		if cfg.MinAge > 0 {
			return u != nil && u.Age >= cfg.MinAge
		}
		return true
	case "login":
		if u == nil {
			return false
		}
		if cfg.MinAge > 0 {
			return u.Age >= cfg.MinAge
		}
		return true
	default: // blocked (o desconocido)
		return false
	}
}

// canDownload decide si un usuario puede DESCARGAR el fichero (no solo verlo). La
// descarga es un permiso ADICIONAL al de ver: primero tiene que poder ver, y
// además:
//   - admin: siempre.
//   - usuario con sesión: siempre (los registrados pueden bajar lo que ven).
//   - anónimo: solo si la colección tiene allow_download (descarga anónima permitida).
//
// Traducción del modelo web que pidió Andrés: navegas/ves libre, pero para
// BAJARTE el fichero te registras — salvo que el admin marque esa colección como
// de descarga pública.
func canDownload(u *User, cfg accessCfg) bool {
	if !canSee(u, cfg) {
		return false // ni siquiera puede ver → menos aún descargar
	}
	if u != nil {
		return true // admin o usuario con sesión
	}
	return cfg.AllowDownload // anónimo: depende del flag de la colección
}
func (s *Server) filterCollections(u *User, cols []Collection) []Collection {
	am := s.accessMap()
	out := make([]Collection, 0, len(cols))
	for _, c := range cols {
		if canSeeCached(u, am, c.ID) {
			out = append(out, c)
		}
	}
	return out
}

// canSeeZim resuelve el acceso de una colección ZIM por su identificador de
// contenido (lib.ID = lo que va en /content/{zim}/…).
func (s *Server) canSeeZim(u *User, zimID string) bool {
	return canSee(u, s.collectionAccess(collectionIDForZIM(zimID)))
}

// ── El gate universal ──────────────────────────────────────────────────────
//
// Todo lo que Library sirve (Item, Collection, SearchResult) lleva CollectionID.
// Así que hay UNA sola pregunta: ¿puede este usuario ver esta colección? Da igual
// si detrás hay un ZIM o un PDF bajado del pool.
//
// Antes el gate solo cubría el carril ZIM: /content/* y la lista de colecciones.
// El contenido descargado (/api/media, /media/*, /api/items/*) se servía a
// cualquiera, incluso sin cuenta — o sea que la edad mínima protegía la
// Wikipedia y dejaba abierta la carpeta de vídeos.

func (s *Server) canSeeCollectionID(u *User, collectionID string) bool {
	return canSee(u, s.collectionAccess(collectionID))
}

// mediaCollectionForRel: ruta relativa de un fichero → carpeta-colección, con el
// MISMO criterio que media.go/toItem (`filepath.Dir` relativo a la raíz). Ojo:
// un fichero en la raíz da ".", no "" — y así es como se guarda su ID.
func mediaCollectionForRel(rel string) string {
	clean := strings.Trim(path.Clean(filepath.ToSlash(rel)), "/")
	return path.Dir(clean) // "Libros/Novela/x.pdf" → "Libros/Novela"; "x.pdf" → "."
}

// canSeeMediaPath: gate para /media/<rel>. La carpeta manda: la portada, los
// subtítulos y las pistas viven junto al fichero, así que se cubren todos.
func (s *Server) canSeeMediaPath(u *User, rel string) bool {
	return s.canSeeCollectionID(u, collectionIDForMedia(mediaCollectionForRel(rel)))
}

// canDownloadMediaPath: ¿puede este usuario DESCARGAR (no solo ver) este fichero?
// Mismo criterio de carpeta que canSeeMediaPath, pero con la regla de descarga.
func (s *Server) canDownloadMediaPath(u *User, rel string) bool {
	return canDownload(u, s.collectionAccess(collectionIDForMedia(mediaCollectionForRel(rel))))
}

// filterMediaItems deja solo los items cuya colección puede ver el usuario.
func (s *Server) filterMediaItems(u *User, items []mediaItem) []mediaItem {
	am := s.accessMap()
	out := make([]mediaItem, 0, len(items))
	for _, it := range items {
		if canSeeCached(u, am, collectionIDForMedia(it.Collection)) {
			out = append(out, it)
		}
	}
	return out
}

// filterSearchResults: mismo criterio para la búsqueda federada. Sin esto, el
// buscador enseña el título y el snippet de lo que el usuario no puede abrir.
func (s *Server) filterSearchResults(u *User, res []FederatedSearchResult) []FederatedSearchResult {
	am := s.accessMap()
	out := make([]FederatedSearchResult, 0, len(res))
	for _, r := range res {
		if r.CollectionID == "" || canSeeCached(u, am, r.CollectionID) {
			out = append(out, r)
		}
	}
	return out
}

// visibleLibs devuelve el catálogo ZIM filtrado por lo que el usuario puede ver.
func (s *Server) visibleLibs(u *User) ([]Library, error) {
	libs, err := s.fetchLibraries()
	if err != nil {
		return nil, err
	}
	am := s.accessMap()
	out := make([]Library, 0, len(libs))
	for _, lib := range libs {
		if canSeeCached(u, am, collectionIDForZIM(lib.ID)) {
			out = append(out, lib)
		}
	}
	return out, nil
}

// contentZim extrae el {zim} de una ruta /content/{zim}/… ("" si no lo es).
func contentZim(path string) string {
	rest := strings.TrimPrefix(path, "/content/")
	if rest == path {
		return ""
	}
	zim, _, _ := strings.Cut(rest, "/")
	return zim
}

// ── Admin: leer/escribir la config de acceso ───────────────────────────────

func (s *Server) registerAccessRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/admin/collections/access", s.handleCollectionsAccess)
}

func (s *Server) handleCollectionsAccess(w http.ResponseWriter, r *http.Request) {
	me := s.currentUser(r)
	if me == nil || !me.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "solo admin"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"access": s.accessMap()})
	case http.MethodPut:
		var req struct {
			CollectionID  string  `json:"collectionId"`
			Access        string  `json:"access"`
			MinAge        flexInt `json:"minAge"`
			AllowDownload bool    `json:"allowDownload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CollectionID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body inválido"})
			return
		}
		if !validAccess(req.Access) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nivel inválido (open|login|blocked)"})
			return
		}
		age := int(req.MinAge)
		if age < 0 || age > 18 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "edad mínima fuera de rango (0-18)"})
			return
		}
		dl := 0
		if req.AllowDownload {
			dl = 1
		}
		_, err := s.store.db.Exec(`
			INSERT INTO collection_access (collection_id, access, min_age, allow_download, updated) VALUES (?,?,?,?,?)
			ON CONFLICT(collection_id) DO UPDATE SET access=excluded.access, min_age=excluded.min_age, allow_download=excluded.allow_download, updated=excluded.updated`,
			req.CollectionID, req.Access, age, dl, time.Now().Unix())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
	}
}
