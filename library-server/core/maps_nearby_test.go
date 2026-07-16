package main

import (
	"math"
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
