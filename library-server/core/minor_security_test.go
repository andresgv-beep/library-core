package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSanitizeFilenameRemovesWindowsADS(t *testing.T) {
	if got := sanitizeFilename(`video.mp4:payload.exe`); got != "video.mp4payload.exe" {
		t.Fatalf("sanitizeFilename dejó caracteres ADS: %q", got)
	}
	for _, raw := range []string{`a<b>.pdf`, `a|b?.pdf`, `a*b.pdf`} {
		if got := sanitizeFilename(raw); got == raw {
			t.Errorf("no limpió nombre reservado de Windows %q", raw)
		}
	}
}

func TestMutationOriginPolicy(t *testing.T) {
	t.Setenv("CLIENT_ORIGINS", "")
	t.Setenv("DEV_CORS", "")
	same := httptest.NewRequest(http.MethodPost, "http://library.local/api/admin/test", nil)
	same.Host = "library.local"
	same.Header.Set("Origin", "http://library.local")
	if !requestOriginAllowed(same) {
		t.Fatal("rechazó el mismo origen")
	}
	cross := same.Clone(same.Context())
	cross.Header = same.Header.Clone()
	cross.Header.Set("Origin", "https://evil.example")
	if requestOriginAllowed(cross) {
		t.Fatal("aceptó un Origin cruzado")
	}
	withoutOrigin := httptest.NewRequest(http.MethodPost, "http://library.local/api/admin/test", nil)
	if !requestOriginAllowed(withoutOrigin) {
		t.Fatal("rompió clientes CLI sin Origin")
	}
	withoutOrigin.Header.Set("Sec-Fetch-Site", "cross-site")
	if requestOriginAllowed(withoutOrigin) {
		t.Fatal("aceptó Sec-Fetch-Site cross-site")
	}
}

func TestMachineTokenMinimumLength(t *testing.T) {
	if validateMachineToken("") != nil {
		t.Fatal("el token opcional vacío debe seguir permitido")
	}
	if validateMachineToken("demasiado-corto") == nil {
		t.Fatal("aceptó un NIMOS_TOKEN corto")
	}
	if validateMachineToken("0123456789abcdef0123456789abcdef") != nil {
		t.Fatal("rechazó un token de 32 caracteres")
	}
}

func TestIllustrationRequiresCollectionAccess(t *testing.T) {
	s := testAuthServer(t, "")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/catalog/v2/illustration/privada", nil)
	s.handleIllustration(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("ilustración bloqueada devolvió %d; quería 403", w.Code)
	}
}
