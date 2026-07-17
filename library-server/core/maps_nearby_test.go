package main

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNearbyCategory(t *testing.T) {
	cases := map[string]string{"restaurant": "Restaurante", "cafe": "Cafetería", "fuel": "Gasolinera", "supermarket": "Comercio", "pharmacy": "Salud", "unknown": ""}
	for kind, want := range cases {
		if got := nearbyCategory(kind); got != want {
			t.Fatalf("nearbyCategory(%q)=%q, want %q", kind, got, want)
		}
	}
}

func TestNearbyCategoryIncludesStableCode(t *testing.T) {
	code, label := nearbyCategoryInfo("museum")
	if code != "culture" || label != "Cultura y ocio" {
		t.Fatalf("categoria inesperada: %q %q", code, label)
	}
}

func TestNearbyRadiusZeroDoesNotOpenMap(t *testing.T) {
	m := &mapManager{root: t.TempDir()}
	hits, err := m.nearby(context.Background(), 40.4168, -3.7038, "missing.pmtiles", 0)
	if err != nil || len(hits) != 0 {
		t.Fatalf("radius=0 debe ser vacio sin abrir mapa: hits=%+v err=%v", hits, err)
	}
}

func TestHandleNearbyAcceptsDefaultRadius(t *testing.T) {
	m := &mapManager{root: t.TempDir()}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/maps/nearby?lat=41.38879&lon=2.15899&map=missing.pmtiles", nil)
	m.handleNearby(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("el radio por defecto debe pasar la validacion: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestNearbyTilePlanIsBounded(t *testing.T) {
	for _, latitude := range []float64{0, 40.4168, 70, 84} {
		for _, radius := range []int{500, 2500, 5000} {
			zoom, tileRadius := nearbyTilePlan(latitude, radius, 15)
			tiles := (2*tileRadius + 1) * (2*tileRadius + 1)
			if zoom > 15 || tiles > 121 {
				t.Fatalf("plan fuera de limites para lat=%.1f %dm: zoom=%d tiles=%d", latitude, radius, zoom, tiles)
			}
		}
	}
}

func TestLonLatTileAndDistance(t *testing.T) {
	x, y := lonLatTile(2.17, 41.38, 14)
	if x != 8290 || y != 6119 {
		t.Fatalf("tesela Barcelona inesperada: %d/%d", x, y)
	}
	if d := geoDistanceMeters(41.38, 2.17, 41.381, 2.17); math.Abs(d-111) > 2 {
		t.Fatalf("distancia inesperada: %.1f", d)
	}
}

func TestGeoHouseNumber(t *testing.T) {
	if got := geoHouseNumber("Carrer de Mallorca 401, Barcelona"); got != "401" {
		t.Fatalf("portal=%q", got)
	}
	if got := geoHouseNumber("08013 Barcelona"); got != "" {
		t.Fatalf("el codigo postal no es portal: %q", got)
	}
}
