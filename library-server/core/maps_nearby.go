package main

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/maptile"
)

type nearbyHit struct {
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	Category string  `json:"category"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Distance int     `json:"distance"`
}

// handleNearby obtiene puntos de interes directamente del mapa descargado. No
// necesita Internet ni un segundo indice: abre las teselas que rodean a la
// posicion elegida y devuelve los lugares mas cercanos.
func (m *mapManager) handleNearby(w http.ResponseWriter, r *http.Request) {
	lat, errLat := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, errLon := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	mapFile := filepath.Base(strings.TrimSpace(r.URL.Query().Get("map")))
	if errLat != nil || errLon != nil || lat < -85 || lat > 85 || lon < -180 || lon > 180 || mapFile == "" || mapFile != r.URL.Query().Get("map") || !strings.HasSuffix(strings.ToLower(mapFile), ".pmtiles") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "posicion o mapa no valido"})
		return
	}
	f, err := os.Open(filepath.Join(m.root, mapFile))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mapa no encontrado"})
		return
	}
	defer f.Close()
	headerBytes := make([]byte, pmHeaderLen)
	if _, err = f.ReadAt(headerBytes, 0); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo leer el mapa"})
		return
	}
	header, err := readPMHeader(headerBytes)
	if err != nil || header.tileType != pmMVT {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mapa vectorial no compatible"})
		return
	}

	zoom := header.maxZoom
	if zoom > 15 {
		zoom = 15
	}
	if zoom < 12 {
		writeJSON(w, http.StatusOK, []nearbyHit{})
		return
	}
	cx, cy := lonLatTile(lon, lat, zoom)
	limit := int64(uint64(1) << zoom)
	seen := make(map[string]bool)
	hits := make([]nearbyHit, 0, 64)
	for dy := int64(-2); dy <= 2; dy++ {
		for dx := int64(-2); dx <= 2; dx++ {
			x, y := int64(cx)+dx, int64(cy)+dy
			if x < 0 || y < 0 || x >= limit || y >= limit {
				continue
			}
			data, ok, readErr := readPMTile(f, header, zoom, uint32(x), uint32(y))
			if readErr != nil || !ok {
				continue
			}
			layers, decodeErr := mvt.Unmarshal(data)
			if decodeErr != nil {
				continue
			}
			for _, layer := range layers {
				if layer.Name != "pois" {
					continue
				}
				layer.ProjectToWGS84(maptile.New(uint32(x), uint32(y), maptile.Zoom(zoom)))
				for _, feature := range layer.Features {
					name, _ := feature.Properties["name"].(string)
					kind, _ := feature.Properties["kind"].(string)
					name, kind = strings.TrimSpace(name), strings.TrimSpace(kind)
					category := nearbyCategory(kind)
					if name == "" || category == "" || feature.Geometry == nil {
						continue
					}
					center := feature.Geometry.Bound().Center()
					distance := geoDistanceMeters(lat, lon, center[1], center[0])
					if distance > 2200 {
						continue
					}
					key := strings.ToLower(name) + "|" + kind + "|" + strconv.Itoa(int(math.Round(center[0]*10000))) + "|" + strconv.Itoa(int(math.Round(center[1]*10000)))
					if seen[key] {
						continue
					}
					seen[key] = true
					hits = append(hits, nearbyHit{Name: name, Kind: kind, Category: category, Lat: center[1], Lon: center[0], Distance: int(math.Round(distance))})
				}
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Distance < hits[j].Distance })
	if len(hits) > 18 {
		hits = hits[:18]
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(hits)
}

func lonLatTile(lon, lat float64, zoom uint8) (uint32, uint32) {
	n := math.Exp2(float64(zoom))
	x := math.Floor((lon + 180) / 360 * n)
	y := math.Floor((1 - math.Asinh(math.Tan(lat*math.Pi/180))/math.Pi) / 2 * n)
	x = math.Max(0, math.Min(n-1, x))
	y = math.Max(0, math.Min(n-1, y))
	return uint32(x), uint32(y)
}

func geoDistanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earth = 6371000.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp, dl := (lat2-lat1)*math.Pi/180, (lon2-lon1)*math.Pi/180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return earth * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func nearbyCategory(kind string) string {
	switch kind {
	case "restaurant", "fast_food", "food_court":
		return "Restaurante"
	case "cafe", "bakery", "ice_cream":
		return "Cafetería"
	case "bar", "pub", "nightclub":
		return "Bar"
	case "fuel", "charging_station":
		return "Gasolinera"
	case "supermarket", "convenience", "department_store", "mall", "marketplace", "clothes", "books", "electronics", "beauty", "hardware", "furniture", "florist", "gift", "jewelry", "mobile_phone", "shoes", "sports", "toys":
		return "Comercio"
	case "pharmacy", "hospital", "clinic", "doctors", "dentist":
		return "Salud"
	case "hotel", "hostel", "motel", "guest_house":
		return "Alojamiento"
	case "parking", "parking_entrance":
		return "Aparcamiento"
	case "bank", "atm":
		return "Banco"
	case "station", "bus_stop", "ferry_terminal":
		return "Transporte"
	case "museum", "attraction", "cinema", "theatre", "artwork", "library":
		return "Cultura y ocio"
	case "park", "garden", "playground":
		return "Parque"
	default:
		return ""
	}
}
