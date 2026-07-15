package main

import "testing"

// Verifica la pregunta de Andrés: un usuario ANÓNIMO (sin registrar, u=nil) NO
// puede ver contenido bloqueado en NINGÚN carril (ZIM, media local).
// El anónimo es u=nil; blocked es el default (sin fila en collection_access).

func TestAnonymousCannotSeeBlocked(t *testing.T) {
	// Sin fila configurada → blocked por defecto. Es el caso más peligroso: si el
	// admin no ha tocado nada, TODO está bloqueado para el anónimo.
	blocked := accessCfg{Access: "blocked"}
	if canSee(nil, blocked) {
		t.Fatal("anónimo NO debería ver contenido blocked")
	}

	// login: exige sesión. El anónimo (nil) no la tiene.
	login := accessCfg{Access: "login"}
	if canSee(nil, login) {
		t.Fatal("anónimo NO debería ver contenido 'login' (no tiene sesión)")
	}

	// open con edad mínima: exige cuenta para comprobar la edad → anónimo fuera.
	openAdult := accessCfg{Access: "open", MinAge: 18}
	if canSee(nil, openAdult) {
		t.Fatal("anónimo NO debería ver 'open' con edad mínima (no hay edad que comprobar)")
	}

	// Lo único que el anónimo SÍ ve: open sin edad (contenido público a propósito).
	openFree := accessCfg{Access: "open"}
	if !canSee(nil, openFree) {
		t.Fatal("anónimo SÍ debería ver 'open' sin restricción")
	}
}

// Prueba de extremo a extremo por los helpers reales de cada carril, con un Server
// que tiene store: el default sin fila debe cerrar a los tres.
func TestAnonymousBlockedAcrossRails(t *testing.T) {
	s := testAuthServer(t, "")

	// Carril ZIM: /content/{zim}/…
	if s.canSeeZim(nil, "wikipedia_es") {
		t.Fatal("ZIM sin configurar debería estar bloqueado para el anónimo")
	}
	// Carril media: /media/Biblioteca/Libros/x.pdf
	if s.canSeeMediaPath(nil, "Biblioteca/Libros/x.pdf") {
		t.Fatal("la colección local sin configurar debería estar bloqueada para el anónimo")
	}
	// Carril vídeo: /media/Videos/<canal>/<id>.mp4
	if s.canSeeMediaPath(nil, "Videos/CanalX/abc123.mp4") {
		t.Fatal("la colección de vídeo sin configurar debería estar bloqueado para el anónimo")
	}

	// Ahora el admin ABRE solo el ZIM (open). El resto sigue cerrado.
	s.store.db.Exec(`INSERT INTO collection_access (collection_id, access, min_age, updated) VALUES (?,?,?,?)`,
		collectionIDForZIM("wikipedia_es"), "open", 0, now())
	if !s.canSeeZim(nil, "wikipedia_es") {
		t.Fatal("tras abrir el ZIM como 'open', el anónimo debería verlo")
	}
	if s.canSeeMediaPath(nil, "Videos/CanalX/abc123.mp4") {
		t.Fatal("abrir el ZIM NO debe abrir otras colecciones (colecciones independientes)")
	}
}
