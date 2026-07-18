package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type zimTrust struct {
	FileName string
	Stamp    string
	Source   string
	Enabled  bool
}

type zimTrustState struct {
	Interactive bool
	Official    bool
	Stale       bool
}

func zimFileStamp(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("no es un archivo")
	}
	return fmt.Sprintf("%d:%d", st.Size(), st.ModTime().UnixNano()), nil
}

func (a *adminZim) readTrust(id string) (zimTrust, bool, error) {
	var t zimTrust
	var enabled int
	err := a.store.db.QueryRow(`SELECT file_name, file_stamp, source, enabled FROM zim_content_trust WHERE collection_id = ?`, id).
		Scan(&t.FileName, &t.Stamp, &t.Source, &enabled)
	if err == sql.ErrNoRows {
		return t, false, nil
	}
	if err != nil {
		return t, false, err
	}
	t.Enabled = enabled != 0
	return t, true, nil
}

func (a *adminZim) trustState(id string) zimTrustState {
	if a == nil || a.store == nil {
		return zimTrustState{}
	}
	t, ok, err := a.readTrust(id)
	if err != nil || !ok {
		return zimTrustState{}
	}
	stamp, err := zimFileStamp(filepath.Join(a.zimDir, t.FileName))
	if err != nil || stamp != t.Stamp {
		return zimTrustState{Stale: true}
	}
	return zimTrustState{Interactive: t.Enabled, Official: t.Source == "official"}
}

func (a *adminZim) interactiveAllowed(id string) bool {
	return a.trustState(id).Interactive
}

// zimTrustKey deriva la CLAVE del trust a partir del nombre de fichero: el id
// público del ZIM (nombre sin extensión), el MISMO que usan las URLs de /content
// y /api/libraries (zim_native.go: TrimSuffix(Base(path), ext)) y, por tanto, el
// que recibe interactiveAllowed(contentZim(url)) en handleContent. Antes el trust
// se llaveaba por b.ID (el UUID del library.xml): la escritura del Panel nunca
// casaba con la lectura del handler y el desbloqueo se ignoraba (TED en blanco).
func zimTrustKey(file string) string {
	return file[:len(file)-len(filepath.Ext(file))]
}

func (a *adminZim) writeTrust(id, file, source string, enabled bool, overwrite bool) error {
	stamp, err := zimFileStamp(filepath.Join(a.zimDir, file))
	if err != nil {
		return err
	}
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	if overwrite {
		_, err = a.store.db.Exec(`INSERT INTO zim_content_trust (collection_id,file_name,file_stamp,source,enabled,updated)
			VALUES (?,?,?,?,?,?) ON CONFLICT(collection_id) DO UPDATE SET
			file_name=excluded.file_name,file_stamp=excluded.file_stamp,source=excluded.source,
			enabled=excluded.enabled,updated=excluded.updated`, id, file, stamp, source, enabledInt, time.Now().Unix())
	} else {
		_, err = a.store.db.Exec(`INSERT OR IGNORE INTO zim_content_trust
			(collection_id,file_name,file_stamp,source,enabled,updated) VALUES (?,?,?,?,?,?)`,
			id, file, stamp, source, enabledInt, time.Now().Unix())
	}
	return err
}

func (a *adminZim) bookFile(id string) (string, bool) {
	books, err := a.readLibrary()
	if err != nil {
		return "", false
	}
	for _, b := range books {
		if b.ID == id {
			return filepath.Base(b.Path), true
		}
	}
	return "", false
}

func (a *adminZim) handleInteractive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	var req struct {
		ID          string `json:"id"`
		Enabled     bool   `json:"enabled"`
		Acknowledge bool   `json:"acknowledge"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body invalido"})
		return
	}
	file, ok := a.bookFile(req.ID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "coleccion no encontrada"})
		return
	}
	key := zimTrustKey(file)
	state := a.trustState(key)
	if req.Enabled && !state.Official && !req.Acknowledge {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "debes aceptar el aviso de contenido no verificado"})
		return
	}
	source := "manual"
	if state.Official {
		source = "official"
	}
	if err := a.writeTrust(key, file, source, req.Enabled, true); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "interactive": req.Enabled, "official": source == "official"})
}
