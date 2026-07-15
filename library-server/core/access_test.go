package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// El agujero original: el gate de edad protegía los ZIM (/content/*) pero el
// contenido descargado se servía a cualquiera. Estos tests lo fijan.

func accessTestServer(t *testing.T, root string) (*Server, *mediaDeps) {
	t.Helper()
	st, err := openStore(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	t.Cleanup(func() { st.db.Close() })
	return &Server{store: st}, &mediaDeps{root: root}
}

func setAccess(t *testing.T, s *Server, collection, level string, minAge int) {
	t.Helper()
	_, err := s.store.db.Exec(
		`INSERT INTO collection_access (collection_id, access, min_age, updated) VALUES (?,?,?,?)
		 ON CONFLICT(collection_id) DO UPDATE SET access=excluded.access, min_age=excluded.min_age`,
		collectionIDForMedia(collection), level, minAge, time.Now().Unix())
	if err != nil {
		t.Fatalf("setAccess: %v", err)
	}
}

func getAs(h http.Handler, path string, c *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if c != nil {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// Los BYTES: /media/<ruta> de una colección bloqueada no se sirven ni al anónimo
// ni a un usuario sin edad. Esto es lo que estaba abierto de par en par.
func TestMediaFileRespectsCollectionAccess(t *testing.T) {
	root := t.TempDir()
	seedCollection(t, root, "Adultos", "peli.mp4", "fake mp4",
		sidecar{Template: "video", Title: "Peli"})
	s, md := accessTestServer(t, root)

	mux := http.NewServeMux()
	s.registerMediaRoutes(mux, md)
	const file = "/media/Adultos/peli.mp4"

	// Sin fila de acceso → blocked por defecto.
	if code := getAs(mux, file, nil).Code; code != http.StatusForbidden {
		t.Fatalf("anónimo · colección bloqueada: quiero 403, tengo %d", code)
	}

	// open + edad mínima 18: el crío (12) no pasa, el adulto (40) sí.
	setAccess(t, s, "Adultos", "open", 18)
	nino := sessionFor(t, s, "critico", 12, false)
	adulto := sessionFor(t, s, "andres", 40, false)

	if code := getAs(mux, file, nino).Code; code != http.StatusForbidden {
		t.Fatalf("usuario de 12 en colección 18+: quiero 403, tengo %d", code)
	}
	if code := getAs(mux, file, nil).Code; code != http.StatusForbidden {
		t.Fatalf("anónimo en colección con edad mínima: quiero 403, tengo %d", code)
	}
	if code := getAs(mux, file, adulto).Code; code != http.StatusOK {
		t.Fatalf("usuario de 40 en colección 18+: quiero 200, tengo %d", code)
	}
}

// La LISTA no debe enseñar ni el título de lo que no se puede abrir.
func TestMediaListFiltersByAccess(t *testing.T) {
	root := t.TempDir()
	seedCollection(t, root, "Publico", "doc.pdf", "fake pdf", sidecar{Template: "pdf", Title: "Manual"})
	seedCollection(t, root, "Adultos", "peli.mp4", "fake mp4", sidecar{Template: "video", Title: "Peli"})
	s, md := accessTestServer(t, root)
	setAccess(t, s, "Publico", "open", 0)
	setAccess(t, s, "Adultos", "open", 18)

	mux := http.NewServeMux()
	s.registerMediaRoutes(mux, md)

	decode := func(rec *httptest.ResponseRecorder) []mediaItem {
		var payload struct {
			Items []mediaItem `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return payload.Items
	}

	items := decode(getAs(mux, "/api/media", nil))
	if len(items) != 1 || items[0].Title != "Manual" {
		t.Fatalf("anónimo debería ver solo 'Manual', veo: %+v", items)
	}

	nino := sessionFor(t, s, "critico", 12, false)
	if items := decode(getAs(mux, "/api/media", nino)); len(items) != 1 {
		t.Fatalf("usuario de 12: quiero 1 item, tengo %d", len(items))
	}

	adulto := sessionFor(t, s, "andres", 40, false)
	if items := decode(getAs(mux, "/api/media", adulto)); len(items) != 2 {
		t.Fatalf("usuario de 40: quiero 2 items, tengo %d", len(items))
	}
}

// Pedir la colección por su ID (que es adivinable: base64 del nombre) era la
// puerta de atrás del filtro de la lista.
func TestCollectionByIDRespectsAccess(t *testing.T) {
	root := t.TempDir()
	seedCollection(t, root, "Adultos", "peli.mp4", "fake mp4", sidecar{Template: "video", Title: "Peli"})
	s, md := accessTestServer(t, root)
	s.kiwixSem = make(chan struct{}, 1)
	s.searchGate = make(chan struct{}, 1)

	mux := http.NewServeMux()
	s.registerItemRoutes(mux, md)

	id := collectionIDForMedia("Adultos")
	if code := getAs(mux, "/api/collections/"+id, nil).Code; code != http.StatusForbidden {
		t.Fatalf("GET colección bloqueada por ID: quiero 403, tengo %d", code)
	}
	if code := getAs(mux, "/api/collections/"+id+"/items", nil).Code; code != http.StatusForbidden {
		t.Fatalf("GET items de colección bloqueada: quiero 403, tengo %d", code)
	}
}

// /api/items/{id}/open devuelve la URL del fichero: si no se comprueba aquí, el
// usuario se lleva la dirección exacta de lo que no puede ver.
func TestItemOpenRespectsAccess(t *testing.T) {
	root := t.TempDir()
	seedCollection(t, root, "Adultos", "peli.mp4", "fake mp4", sidecar{Template: "video", Title: "Peli"})
	s, md := accessTestServer(t, root)

	items, err := md.scan("")
	if err != nil || len(items) != 1 {
		t.Fatalf("scan: %v (%d items)", err, len(items))
	}
	item := mediaToItem(items[0])

	mux := http.NewServeMux()
	s.registerItemRoutes(mux, md)

	if code := getAs(mux, "/api/items/"+item.ID+"/open", nil).Code; code != http.StatusForbidden {
		t.Fatalf("open de item bloqueado: quiero 403, tengo %d", code)
	}

	setAccess(t, s, "Adultos", "open", 0)
	if code := getAs(mux, "/api/items/"+item.ID+"/open", nil).Code; code != http.StatusOK {
		t.Fatalf("open de item abierto: quiero 200, tengo %d", code)
	}
}

// Un fichero suelto en la raíz de DOWNLOAD_ROOT: su "colección" es ".", y tiene
// que resolverse igual que la calcula media.go/toItem (si no, el gate miraría
// una fila que no existe y se abriría o cerraría por accidente).
func TestMediaCollectionForRelMatchesToItem(t *testing.T) {
	cases := map[string]string{
		"Biblioteca/Libros/linux.pdf": "Biblioteca/Libros",
		"Publico/doc.pdf":           "Publico",
		"suelto.pdf":                ".",
	}
	for rel, want := range cases {
		if got := mediaCollectionForRel(rel); got != want {
			t.Fatalf("mediaCollectionForRel(%q) = %q, quiero %q", rel, got, want)
		}
	}
}
