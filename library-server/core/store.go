package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // SQLite puro Go (sin CGO → binario estático, §5.5)
)

// Store = capa de gestión de la app (favoritos, notas, historial). Es NUESTRA,
// no la toca kiwix-serve (DESIGN §2.2, D2). SQLite propio, esquema portable.
type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS favorites (
  item_id TEXT NOT NULL DEFAULT '',
  lib     TEXT NOT NULL,
  path    TEXT NOT NULL,
  title   TEXT,
  book    TEXT,
  on_home INTEGER NOT NULL DEFAULT 1,
  created INTEGER NOT NULL,
  PRIMARY KEY (lib, path)
);
CREATE TABLE IF NOT EXISTS notes (
  item_id TEXT NOT NULL DEFAULT '',
  lib     TEXT NOT NULL,
  path    TEXT NOT NULL,
  title   TEXT,
  book    TEXT,
  body    TEXT NOT NULL,
  updated INTEGER NOT NULL,
  PRIMARY KEY (lib, path)
);
CREATE TABLE IF NOT EXISTS history (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  item_id TEXT NOT NULL DEFAULT '',
  lib     TEXT NOT NULL,
  path    TEXT NOT NULL,
  title   TEXT,
  book    TEXT,
  visited INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_history_visited ON history(visited DESC);
CREATE TABLE IF NOT EXISTS tags (
  item_id TEXT NOT NULL DEFAULT '',
  lib     TEXT NOT NULL,
  path    TEXT NOT NULL,
  tag     TEXT NOT NULL,
  title   TEXT,
  book    TEXT,
  created INTEGER NOT NULL,
  PRIMARY KEY (lib, path, tag)
);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
CREATE TABLE IF NOT EXISTS translations (
  lib      TEXT NOT NULL,
  path     TEXT NOT NULL,
  to_lang  TEXT NOT NULL,
  seg_id   TEXT NOT NULL,   -- id estable del segmento (índice de párrafo)
  src_hash TEXT NOT NULL,   -- hash del texto ORIGEN (invalida si el ZIM cambia)
  text     TEXT NOT NULL,   -- segmento traducido
  created  INTEGER NOT NULL,
  PRIMARY KEY (lib, path, to_lang, seg_id)
);
CREATE TABLE IF NOT EXISTS users (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  username  TEXT NOT NULL UNIQUE,
  pass_hash TEXT NOT NULL,
  age       INTEGER NOT NULL DEFAULT 0,
  is_admin  INTEGER NOT NULL DEFAULT 0,
  created   INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  token    TEXT PRIMARY KEY,
  username TEXT NOT NULL,
  created  INTEGER NOT NULL
);
-- Acceso por colección (Panel · usuarios/18+). Sin fila = 'blocked' por defecto.
CREATE TABLE IF NOT EXISTS collection_access (
  collection_id TEXT PRIMARY KEY,
  access        TEXT NOT NULL DEFAULT 'blocked',  -- 'open' | 'login' | 'blocked'
  min_age       INTEGER NOT NULL DEFAULT 0,
  allow_download INTEGER NOT NULL DEFAULT 0,       -- 0 = descargar exige cuenta; 1 = descarga anónima permitida
  updated       INTEGER NOT NULL
);
`

// Los índices sobre item_id NO van en el schema de arriba: en un upgrade in-place
// las tablas ya existen (CREATE TABLE IF NOT EXISTS es no-op) y la columna item_id
// aún no está, así que crear el índice aquí reventaría el arranque con
// "no such column: item_id". Se crean en migrateItemIDs, tras ensureColumn.

func openStore(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		os.MkdirAll(dir, 0o755)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite: una conexión evita "database is locked"
	// WAL: escrituras que no bloquean lecturas y menos fsyncs (durabilidad sana
	// en la Pi con SD). busy_timeout por si algún día hay más de una conexión.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA synchronous=NORMAL;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	st := &Store{db: db}
	if err := st.migrateUsers(); err != nil { // scoping por usuario (antes de los índices)
		db.Close()
		return nil, err
	}
	if err := st.migrateItemIDs(); err != nil {
		db.Close()
		return nil, err
	}
	// allow_download en collection_access (permiso de descarga separado del de ver).
	// Idempotente: si ya existe la columna, ensureColumn no hace nada.
	if err := st.ensureColumn("collection_access", "allow_download", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) hasColumn(table, column string) (bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var def any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// migrateUsers añade el scoping por usuario al estado personal. favorites/notes/
// tags se recrean con `user` en la PK (los datos previos pasan a "invitado"="");
// history solo gana una columna user. Idempotente: si ya existe la columna, no
// hace nada. Corre ANTES de migrateItemIDs (que crea los índices por usuario).
func (s *Store) migrateUsers() error {
	has, err := s.hasColumn("favorites", "user")
	if err != nil || has {
		return err
	}
	// Puede venir de un esquema MUY viejo (anterior a item_id): migrateUsers corre
	// antes que migrateItemIDs, así que las tablas origen podrían no tener aún la
	// columna item_id. Garantizarla antes de copiarla a las _v2 (si no, el
	// `SELECT item_id` de abajo peta con "no such column: item_id" al arrancar).
	for _, table := range []string{"favorites", "notes", "tags"} {
		if err := s.ensureColumn(table, "item_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	stmts := []string{
		`CREATE TABLE favorites_v2 (user TEXT NOT NULL DEFAULT '', item_id TEXT NOT NULL DEFAULT '', lib TEXT NOT NULL, path TEXT NOT NULL, title TEXT, book TEXT, on_home INTEGER NOT NULL DEFAULT 1, created INTEGER NOT NULL, PRIMARY KEY (user, lib, path))`,
		`INSERT INTO favorites_v2 (user,item_id,lib,path,title,book,on_home,created) SELECT '',item_id,lib,path,title,book,on_home,created FROM favorites`,
		`DROP TABLE favorites`,
		`ALTER TABLE favorites_v2 RENAME TO favorites`,

		`CREATE TABLE notes_v2 (user TEXT NOT NULL DEFAULT '', item_id TEXT NOT NULL DEFAULT '', lib TEXT NOT NULL, path TEXT NOT NULL, title TEXT, book TEXT, body TEXT NOT NULL, updated INTEGER NOT NULL, PRIMARY KEY (user, lib, path))`,
		`INSERT INTO notes_v2 (user,item_id,lib,path,title,book,body,updated) SELECT '',item_id,lib,path,title,book,body,updated FROM notes`,
		`DROP TABLE notes`,
		`ALTER TABLE notes_v2 RENAME TO notes`,

		`CREATE TABLE tags_v2 (user TEXT NOT NULL DEFAULT '', item_id TEXT NOT NULL DEFAULT '', lib TEXT NOT NULL, path TEXT NOT NULL, tag TEXT NOT NULL, title TEXT, book TEXT, created INTEGER NOT NULL, PRIMARY KEY (user, lib, path, tag))`,
		`INSERT INTO tags_v2 (user,item_id,lib,path,tag,title,book,created) SELECT '',item_id,lib,path,tag,title,book,created FROM tags`,
		`DROP TABLE tags`,
		`ALTER TABLE tags_v2 RENAME TO tags`,
		`CREATE INDEX IF NOT EXISTS idx_tags_user_tag ON tags(user, tag)`,

		`ALTER TABLE history ADD COLUMN user TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_history_user_visited ON history(user, visited DESC)`,
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, st := range stmts {
		if _, err := tx.Exec(st); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrateUsers: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) migrateItemIDs() error {
	for _, table := range []string{"favorites", "notes", "history", "tags"} {
		if err := s.ensureColumn(table, "item_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_favorites_item_id ON favorites(user, item_id) WHERE item_id <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_notes_item_id ON notes(user, item_id) WHERE item_id <> ''`,
		`CREATE INDEX IF NOT EXISTS idx_history_item_visited ON history(user, item_id, visited DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_item_tag ON tags(user, item_id, tag) WHERE item_id <> ''`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	for _, table := range []string{"favorites", "notes", "history", "tags"} {
		if err := s.backfillItemID(table); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureColumn(table, column, decl string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var def any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + decl)
	return err
}

func (s *Store) backfillItemID(table string) error {
	rows, err := s.db.Query(`SELECT rowid, lib, path FROM ` + table + ` WHERE item_id='' AND lib<>'' AND path<>''`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		rowid int64
		lib   string
		path  string
	}
	pending := []row{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.rowid, &r.lib, &r.path); err != nil {
			return err
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range pending {
		if _, err := s.db.Exec(`UPDATE `+table+` SET item_id=? WHERE rowid=?`, canonicalItemID("", r.lib, r.path), r.rowid); err != nil {
			return err
		}
	}
	return nil
}

const itemLegacyLib = "__item__"

func canonicalItemID(itemID, lib, path string) string {
	itemID = strings.TrimSpace(itemID)
	if itemID != "" {
		return itemID
	}
	if lib == itemLegacyLib {
		return path
	}
	// Un artículo ZIM SIN itemId explícito conserva su identidad por lib/path
	// (item_id vacío). No derivamos aquí un "zim:<base64>": el cliente solo tiene
	// lib/path durante la navegación normal del iframe, así que si el Core
	// canonicalizara ZIM a itemId, la estrella/nota/etiqueta no coincidirían al
	// recargar. El contrato itemId es para contenido nativo de Item (media).
	return ""
}

func storageKey(itemID, lib, path string) (string, string, string) {
	itemID = canonicalItemID(itemID, lib, path)
	if strings.TrimSpace(lib) == "" && strings.TrimSpace(path) == "" && itemID != "" {
		return itemID, itemLegacyLib, itemID
	}
	return itemID, lib, path
}

// publicKeys oculta el centinela interno itemLegacyLib al salir por la API: el
// contenido nativo de Item se expone SOLO por itemId; su lib/path ("__item__" /
// itemId) son artefactos de almacenamiento y no deben llegar a la UI (si no,
// bookName("__item__") pintaría basura). Los artículos ZIM salen intactos.
func publicKeys(lib, path string) (string, string) {
	if lib == itemLegacyLib {
		return "", ""
	}
	return lib, path
}

// ─── Favoritos ────────────────────────────────────────────────────────────────

type Fav struct {
	ItemID string `json:"itemId,omitempty"`
	Lib    string `json:"lib"`
	Path   string `json:"path"`
	Title  string `json:"title"`
	Book   string `json:"book"`
	OnHome bool   `json:"onHome"`
}

func (s *Store) ListFavorites(user string) ([]Fav, error) {
	rows, err := s.db.Query(`SELECT item_id, lib, path, title, book, on_home FROM favorites WHERE user=? ORDER BY created DESC`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Fav{}
	for rows.Next() {
		var f Fav
		var onHome int
		if err := rows.Scan(&f.ItemID, &f.Lib, &f.Path, &f.Title, &f.Book, &onHome); err != nil {
			return nil, err
		}
		f.OnHome = onHome != 0
		f.Lib, f.Path = publicKeys(f.Lib, f.Path)
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) PutFavorite(user string, f Fav, now int64) error {
	f.ItemID, f.Lib, f.Path = storageKey(f.ItemID, f.Lib, f.Path)
	onHome := 0
	if f.OnHome {
		onHome = 1
	}
	if f.ItemID != "" {
		res, err := s.db.Exec(`UPDATE favorites SET lib=?, path=?, title=?, book=?, on_home=? WHERE user=? AND item_id=?`,
			f.Lib, f.Path, f.Title, f.Book, onHome, user, f.ItemID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			return nil
		}
	}
	_, err := s.db.Exec(
		`INSERT INTO favorites (user, item_id, lib, path, title, book, on_home, created) VALUES (?,?,?,?,?,?,?,?)
		 ON CONFLICT(user, lib, path) DO UPDATE SET item_id=excluded.item_id, title=excluded.title, book=excluded.book, on_home=excluded.on_home`,
		user, f.ItemID, f.Lib, f.Path, f.Title, f.Book, onHome, now)
	return err
}

func (s *Store) DeleteFavorite(user, lib, path, itemID string) error {
	itemID = canonicalItemID(itemID, lib, path)
	if itemID != "" {
		_, err := s.db.Exec(`DELETE FROM favorites WHERE user=? AND item_id=?`, user, itemID)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM favorites WHERE user=? AND lib=? AND path=?`, user, lib, path)
	return err
}

// ─── Notas (una por artículo) ─────────────────────────────────────────────────

type Note struct {
	ItemID  string `json:"itemId,omitempty"`
	Lib     string `json:"lib"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	Book    string `json:"book"`
	Body    string `json:"body"`
	Updated int64  `json:"updated"`
}

func (s *Store) ListNotes(user string) ([]Note, error) {
	rows, err := s.db.Query(`SELECT item_id, lib, path, title, book, body, updated FROM notes WHERE user=? ORDER BY updated DESC`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ItemID, &n.Lib, &n.Path, &n.Title, &n.Book, &n.Body, &n.Updated); err != nil {
			return nil, err
		}
		n.Lib, n.Path = publicKeys(n.Lib, n.Path)
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) GetNote(user, lib, path, itemID string) (*Note, error) {
	var n Note
	itemID = canonicalItemID(itemID, lib, path)
	var err error
	if itemID != "" {
		err = s.db.QueryRow(`SELECT item_id, lib, path, title, book, body, updated FROM notes WHERE user=? AND item_id=?`, user, itemID).
			Scan(&n.ItemID, &n.Lib, &n.Path, &n.Title, &n.Book, &n.Body, &n.Updated)
	} else {
		err = s.db.QueryRow(`SELECT item_id, lib, path, title, book, body, updated FROM notes WHERE user=? AND lib=? AND path=?`, user, lib, path).
			Scan(&n.ItemID, &n.Lib, &n.Path, &n.Title, &n.Book, &n.Body, &n.Updated)
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	n.Lib, n.Path = publicKeys(n.Lib, n.Path)
	return &n, nil
}

func (s *Store) PutNote(user string, n Note, now int64) error {
	n.ItemID, n.Lib, n.Path = storageKey(n.ItemID, n.Lib, n.Path)
	if n.ItemID != "" {
		res, err := s.db.Exec(`UPDATE notes SET lib=?, path=?, title=?, book=?, body=?, updated=? WHERE user=? AND item_id=?`,
			n.Lib, n.Path, n.Title, n.Book, n.Body, now, user, n.ItemID)
		if err != nil {
			return err
		}
		if rows, _ := res.RowsAffected(); rows > 0 {
			return nil
		}
	}
	_, err := s.db.Exec(
		`INSERT INTO notes (user, item_id, lib, path, title, book, body, updated) VALUES (?,?,?,?,?,?,?,?)
		 ON CONFLICT(user, lib, path) DO UPDATE SET item_id=excluded.item_id, title=excluded.title, book=excluded.book, body=excluded.body, updated=excluded.updated`,
		user, n.ItemID, n.Lib, n.Path, n.Title, n.Book, n.Body, now)
	return err
}

func (s *Store) DeleteNote(user, lib, path, itemID string) error {
	itemID = canonicalItemID(itemID, lib, path)
	if itemID != "" {
		_, err := s.db.Exec(`DELETE FROM notes WHERE user=? AND item_id=?`, user, itemID)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM notes WHERE user=? AND lib=? AND path=?`, user, lib, path)
	return err
}

// ─── Historial / Recientes ────────────────────────────────────────────────────

type Visit struct {
	ID      int64  `json:"id,omitempty"` // fila concreta del historial (para borrar una a una); 0 en Recientes
	ItemID  string `json:"itemId,omitempty"`
	Lib     string `json:"lib"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	Book    string `json:"book"`
	Visited int64  `json:"visited"`
}

func (s *Store) AddHistory(user string, v Visit, now int64) error {
	v.ItemID, v.Lib, v.Path = storageKey(v.ItemID, v.Lib, v.Path)
	_, err := s.db.Exec(`INSERT INTO history (user, item_id, lib, path, title, book, visited) VALUES (?,?,?,?,?,?,?)`,
		user, v.ItemID, v.Lib, v.Path, v.Title, v.Book, now)
	return err
}

// ListHistory: registro cronológico completo (con repeticiones); trae el id de cada fila.
func (s *Store) ListHistory(user string, limit int) ([]Visit, error) {
	rows, err := s.db.Query(`SELECT id, item_id, lib, path, title, book, visited FROM history WHERE user=? ORDER BY visited DESC LIMIT ?`, user, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Visit{}
	for rows.Next() {
		var v Visit
		if err := rows.Scan(&v.ID, &v.ItemID, &v.Lib, &v.Path, &v.Title, &v.Book, &v.Visited); err != nil {
			return nil, err
		}
		v.Lib, v.Path = publicKeys(v.Lib, v.Path)
		out = append(out, v)
	}
	return out, rows.Err()
}

// ListRecent: artículos distintos por su última visita (para "Reciente"). Sin id (fila agregada).
func (s *Store) ListRecent(user string, limit int) ([]Visit, error) {
	rows, err := s.db.Query(
		`SELECT item_id, lib, path, title, book, MAX(visited) AS v
		   FROM history WHERE user=?
		  GROUP BY CASE WHEN item_id<>'' THEN item_id ELSE lib || char(10) || path END
		  ORDER BY v DESC LIMIT ?`, user, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Visit{}
	for rows.Next() {
		var v Visit
		if err := rows.Scan(&v.ItemID, &v.Lib, &v.Path, &v.Title, &v.Book, &v.Visited); err != nil {
			return nil, err
		}
		v.Lib, v.Path = publicKeys(v.Lib, v.Path)
		out = append(out, v)
	}
	return out, rows.Err()
}

// Borrado manual (sin auto-limpieza): una fila, todas las visitas de una página, o todo.
func (s *Store) DeleteHistoryID(user string, id int64) error {
	_, err := s.db.Exec(`DELETE FROM history WHERE user=? AND id=?`, user, id)
	return err
}

func (s *Store) DeleteHistoryPath(user, lib, path, itemID string) error {
	itemID = canonicalItemID(itemID, lib, path)
	if itemID != "" {
		_, err := s.db.Exec(`DELETE FROM history WHERE user=? AND item_id=?`, user, itemID)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM history WHERE user=? AND lib=? AND path=?`, user, lib, path)
	return err
}

func (s *Store) ClearHistory(user string) error {
	_, err := s.db.Exec(`DELETE FROM history WHERE user=?`, user)
	return err
}

// DeleteUserData borra TODO el estado personal de un usuario (favoritos, notas,
// historial, tags) y sus sesiones, en una sola transacción. Se llama al borrar la
// cuenta: sin esto, las filas quedaban huérfanas y, como el estado se scopea por
// `username` (texto reutilizable), recrear una cuenta con el mismo nombre heredaba
// el historial de lectura del usuario anterior (auditoría H-1). El borrado de la
// fila de `users` se hace aparte, en el handler, tras las comprobaciones de negocio
// (último admin, etc.).
func (s *Store) DeleteUserData(username string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		`DELETE FROM sessions  WHERE username = ?`,
		`DELETE FROM favorites WHERE user = ?`,
		`DELETE FROM notes     WHERE user = ?`,
		`DELETE FROM history   WHERE user = ?`,
		`DELETE FROM tags      WHERE user = ?`,
	} {
		if _, err := tx.Exec(stmt, username); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ─── Etiquetas (una página puede tener varias) ────────────────────────────────

type Tag struct {
	ItemID  string `json:"itemId,omitempty"`
	Lib     string `json:"lib"`
	Path    string `json:"path"`
	Tag     string `json:"tag"`
	Title   string `json:"title"`
	Book    string `json:"book"`
	Created int64  `json:"created"`
}

// TagCount: nombre de etiqueta + cuántas páginas la llevan (nube de etiquetas).
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ListTags: todas las etiquetas distintas con su conteo (para la vista Etiquetas).
func (s *Store) ListTags(user string) ([]TagCount, error) {
	rows, err := s.db.Query(`SELECT tag, COUNT(*) FROM tags WHERE user=? GROUP BY tag ORDER BY tag COLLATE NOCASE`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TagCount{}
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Tag, &tc.Count); err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// ListTagPages: páginas que llevan una etiqueta concreta.
func (s *Store) ListTagPages(user, tag string) ([]Tag, error) {
	rows, err := s.db.Query(`SELECT item_id, lib, path, tag, title, book, created FROM tags WHERE user=? AND tag=? ORDER BY created DESC`, user, tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ItemID, &t.Lib, &t.Path, &t.Tag, &t.Title, &t.Book, &t.Created); err != nil {
			return nil, err
		}
		t.Lib, t.Path = publicKeys(t.Lib, t.Path)
		out = append(out, t)
	}
	return out, rows.Err()
}

// PageTags: etiquetas de una página concreta (para el editor del artículo).
func (s *Store) PageTags(user, lib, path, itemID string) ([]string, error) {
	itemID = canonicalItemID(itemID, lib, path)
	var rows *sql.Rows
	var err error
	if itemID != "" {
		rows, err = s.db.Query(`SELECT tag FROM tags WHERE user=? AND item_id=? ORDER BY tag COLLATE NOCASE`, user, itemID)
	} else {
		rows, err = s.db.Query(`SELECT tag FROM tags WHERE user=? AND lib=? AND path=? ORDER BY tag COLLATE NOCASE`, user, lib, path)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TaggedKeys: item_id si existe; si no, "lib\npath" legacy (marca el boton).
func (s *Store) TaggedKeys(user string) ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT item_id, lib, path FROM tags WHERE user=?`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var itemID, lib, path string
		if err := rows.Scan(&itemID, &lib, &path); err != nil {
			return nil, err
		}
		if itemID != "" {
			out = append(out, itemID)
		} else {
			out = append(out, lib+"\n"+path)
		}
	}
	return out, rows.Err()
}

func (s *Store) AddTag(user string, t Tag, now int64) error {
	t.ItemID, t.Lib, t.Path = storageKey(t.ItemID, t.Lib, t.Path)
	if t.ItemID != "" {
		if _, err := s.db.Exec(`DELETE FROM tags WHERE user=? AND item_id=? AND tag=?`, user, t.ItemID, t.Tag); err != nil {
			return err
		}
	}
	_, err := s.db.Exec(
		`INSERT INTO tags (user, item_id, lib, path, tag, title, book, created) VALUES (?,?,?,?,?,?,?,?)
		 ON CONFLICT(user, lib, path, tag) DO UPDATE SET item_id=excluded.item_id, title=excluded.title, book=excluded.book`,
		user, t.ItemID, t.Lib, t.Path, t.Tag, t.Title, t.Book, now)
	return err
}

func (s *Store) RemoveTag(user, lib, path, itemID, tag string) error {
	itemID = canonicalItemID(itemID, lib, path)
	if itemID != "" {
		_, err := s.db.Exec(`DELETE FROM tags WHERE user=? AND item_id=? AND tag=?`, user, itemID, tag)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM tags WHERE user=? AND lib=? AND path=? AND tag=?`, user, lib, path, tag)
	return err
}

// DeleteTag: borra una etiqueta de TODAS las páginas del usuario.
func (s *Store) DeleteTag(user, tag string) error {
	_, err := s.db.Exec(`DELETE FROM tags WHERE user=? AND tag=?`, user, tag)
	return err
}

// ─── Traducciones (caché por segmento, TRANSLATE.md §5) ────────────────────────

// CachedSeg es una entrada de traducción cacheada de un segmento.
type CachedSeg struct {
	SrcHash string // hash del texto origen: si cambió el ZIM, la caché no casa
	Text    string // segmento ya traducido
}

// GetTranslations trae todos los segmentos cacheados de un artículo a un idioma.
// El llamante valida src_hash contra el texto origen actual (así una edición del
// ZIM invalida solo los segmentos que cambiaron).
func (s *Store) GetTranslations(lib, path, toLang string) (map[string]CachedSeg, error) {
	rows, err := s.db.Query(
		`SELECT seg_id, src_hash, text FROM translations WHERE lib=? AND path=? AND to_lang=?`,
		lib, path, toLang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]CachedSeg{}
	for rows.Next() {
		var segID string
		var c CachedSeg
		if err := rows.Scan(&segID, &c.SrcHash, &c.Text); err != nil {
			return nil, err
		}
		out[segID] = c
	}
	return out, rows.Err()
}

// PutTranslation guarda (o refresca) un segmento traducido.
func (s *Store) PutTranslation(lib, path, toLang, segID, srcHash, text string, now int64) error {
	_, err := s.db.Exec(
		`INSERT INTO translations (lib, path, to_lang, seg_id, src_hash, text, created) VALUES (?,?,?,?,?,?,?)
		 ON CONFLICT(lib, path, to_lang, seg_id) DO UPDATE SET src_hash=excluded.src_hash, text=excluded.text, created=excluded.created`,
		lib, path, toLang, segID, srcHash, text, now)
	return err
}
