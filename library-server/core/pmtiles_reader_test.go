package main

import "testing"

func TestPMTileIDRoundTrip(t *testing.T) {
	for z := uint8(0); z <= 15; z++ {
		n := uint32(1) << z
		points := [][2]uint32{{0, 0}, {n - 1, 0}, {0, n - 1}, {n - 1, n - 1}, {n / 2, n / 2}}
		for _, point := range points {
			id := pmZxyToID(z, point[0], point[1])
			gotZ, gotX, gotY := pmIDToZxy(id)
			if gotZ != z || gotX != point[0] || gotY != point[1] {
				t.Fatalf("roundtrip z%d/%d/%d = z%d/%d/%d", z, point[0], point[1], gotZ, gotX, gotY)
			}
		}
	}
}
