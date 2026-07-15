package main

import "testing"

// Modelo pedido por Andrés: ver puede ser público, pero descargar exige cuenta
// salvo que el admin marque la colección con allow_download.

func TestCanDownloadRules(t *testing.T) {
	admin := &User{IsAdmin: true}
	registered := &User{ID: 5, Username: "ana", Age: 30}

	// open + descarga NO permitida: se ve, pero el anónimo NO descarga.
	openNoDL := accessCfg{Access: "open", AllowDownload: false}
	if !canSee(nil, openNoDL) {
		t.Fatal("anónimo debería VER 'open'")
	}
	if canDownload(nil, openNoDL) {
		t.Fatal("anónimo NO debería descargar si allow_download=false")
	}
	if !canDownload(registered, openNoDL) {
		t.Fatal("usuario registrado SÍ debería descargar lo que ve")
	}
	if !canDownload(admin, openNoDL) {
		t.Fatal("admin siempre descarga")
	}

	// open + descarga permitida: el anónimo también baja.
	openDL := accessCfg{Access: "open", AllowDownload: true}
	if !canDownload(nil, openDL) {
		t.Fatal("anónimo SÍ debería descargar si allow_download=true")
	}

	// blocked: nadie ve → nadie descarga, ni con allow_download activo.
	blockedDL := accessCfg{Access: "blocked", AllowDownload: true}
	if canDownload(nil, blockedDL) {
		t.Fatal("blocked no debe descargarse aunque allow_download=true")
	}
	if canDownload(registered, blockedDL) {
		t.Fatal("blocked no se descarga ni con sesión (no puede ver)")
	}

	// login: el anónimo no ve → no descarga; el registrado sí.
	login := accessCfg{Access: "login", AllowDownload: true}
	if canDownload(nil, login) {
		t.Fatal("anónimo no debería descargar 'login' (no tiene sesión para ver)")
	}
	if !canDownload(registered, login) {
		t.Fatal("registrado debería descargar 'login'")
	}
}

// La descarga NUNCA salta el gate de ver: es aditivo, no alternativo.
func TestDownloadNeverBypassesView(t *testing.T) {
	// open con edad mínima 18: un menor no ve → tampoco descarga aunque allow_download.
	adultOnly := accessCfg{Access: "open", MinAge: 18, AllowDownload: true}
	minor := &User{ID: 9, Username: "nino", Age: 12}
	if canSee(minor, adultOnly) {
		t.Fatal("un menor no debería ver contenido 18+")
	}
	if canDownload(minor, adultOnly) {
		t.Fatal("un menor no debería descargar contenido 18+ ni con allow_download")
	}
}
