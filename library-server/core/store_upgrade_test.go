package main

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestUpgradeFromOldSchema simula una actualización in-place: una DB creada por
// la versión anterior (tablas SIN item_id) reabierta por el nuevo openStore.
// Regresión de: "SQL logic error: no such column: item_id" al crear el índice
// de item_id antes de que migrateItemIDs añada la columna.
func TestUpgradeFromOldSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "old.db")

	old, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	oldSchema := `
CREATE TABLE favorites (lib TEXT NOT NULL, path TEXT NOT NULL, title TEXT, book TEXT, on_home INTEGER NOT NULL DEFAULT 0, created INTEGER NOT NULL, PRIMARY KEY (lib, path));
CREATE TABLE notes (lib TEXT NOT NULL, path TEXT NOT NULL, title TEXT, book TEXT, body TEXT NOT NULL, updated INTEGER NOT NULL, PRIMARY KEY (lib, path));
CREATE TABLE history (id INTEGER PRIMARY KEY AUTOINCREMENT, lib TEXT NOT NULL, path TEXT NOT NULL, title TEXT, book TEXT, visited INTEGER NOT NULL);
CREATE TABLE tags (lib TEXT NOT NULL, path TEXT NOT NULL, tag TEXT NOT NULL, title TEXT, book TEXT, created INTEGER NOT NULL, PRIMARY KEY (lib, path, tag));
`
	if _, err := old.Exec(oldSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`INSERT INTO favorites (lib, path, title, book, on_home, created) VALUES ('wiki','A/Foo','Foo','wiki',0,1)`); err != nil {
		t.Fatal(err)
	}
	old.Close()

	st, err := openStore(dbPath)
	if err != nil {
		t.Fatalf("openStore falló en upgrade in-place: %v", err)
	}
	defer st.db.Close()

	// Tras el upgrade, los datos legacy quedan bajo el usuario invitado ("").
	favs, err := st.ListFavorites("")
	if err != nil {
		t.Fatalf("ListFavorites tras upgrade: %v", err)
	}
	if len(favs) != 1 {
		t.Fatalf("esperaba 1 favorito preservado, got %d", len(favs))
	}
	// Un artículo ZIM conserva su identidad por lib/path: item_id vacío.
	if favs[0].ItemID != "" {
		t.Fatalf("favorito ZIM no debería tener itemId, got %q", favs[0].ItemID)
	}
}
