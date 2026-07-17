package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	Category     string  `json:"category"`
	CategoryCode string  `json:"categoryCode"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Distance     int     `json:"distance"`
}

// handleNearby obtiene puntos de interes directamente del mapa descargado. No
// necesita Internet ni un segundo indice: abre las teselas que rodean a la
// posicion elegida y devuelve los lugares mas cercanos.
func (m *mapManager) handleNearby(w http.ResponseWriter, r *http.Request) {
	lat, errLat := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon, errLon := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	mapFile := filepath.Base(strings.TrimSpace(r.URL.Query().Get("map")))
	radius := 2500 // valor por defecto de Maps; valido en la escala 0-5 km
	if rawRadius := strings.TrimSpace(r.URL.Query().Get("radius")); rawRadius != "" {
		parsed, parseErr := strconv.Atoi(rawRadius)
		if parseErr != nil || parsed < 0 || parsed > 5000 || parsed%500 != 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "radio no valido"})
			return
		}
		radius = parsed
	}
	if errLat != nil || errLon != nil || lat < -85 || lat > 85 || lon < -180 || lon > 180 || mapFile == "" || mapFile != r.URL.Query().Get("map") || !strings.HasSuffix(strings.ToLower(mapFile), ".pmtiles") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "posicion o mapa no valido"})
		return
	}
	hits, err := m.nearby(r.Context(), lat, lon, mapFile, radius)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "mapa no encontrado"})
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}
	writeJSON(w, http.StatusOK, hits)
}

func (m *mapManager) nearby(ctx context.Context, lat, lon float64, mapFile string, radius int) ([]nearbyHit, error) {
	if radius < 0 || radius > 5000 || radius%500 != 0 {
		return nil, fmt.Errorf("radio no valido")
	}
	if radius == 0 {
		return []nearbyHit{}, nil
	}
	cacheKey := fmt.Sprintf("%s|%.4f|%.4f|%d", mapFile, lat, lon, radius)
	if m.nearbyCache != nil {
		if raw, ok := m.nearbyCache.get(cacheKey); ok {
			var cached []nearbyHit
			if json.Unmarshal(raw, &cached) == nil {
				return cached, nil
			}
		}
	}
	f, err := os.Open(filepath.Join(m.root, mapFile))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	headerBytes := make([]byte, pmHeaderLen)
	if _, err = f.ReadAt(headerBytes, 0); err != nil {
		return nil, fmt.Errorf("no se pudo leer el mapa")
	}
	header, err := readPMHeader(headerBytes)
	if err != nil || header.tileType != pmMVT {
		return nil, fmt.Errorf("mapa vectorial no compatible")
	}
	zoom, tileRadius := nearbyTilePlan(lat, radius, header.maxZoom)
	if zoom < 12 {
		return []nearbyHit{}, nil
	}
	cx, cy := lonLatTile(lon, lat, zoom)
	limit := int64(uint64(1) << zoom)
	seen := make(map[string]bool)
	hits := make([]nearbyHit, 0, 64)
	for dy := -tileRadius; dy <= tileRadius; dy++ {
		for dx := -tileRadius; dx <= tileRadius; dx++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
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
					categoryCode, category := nearbyCategoryInfo(kind)
					if name == "" || categoryCode == "" || feature.Geometry == nil {
						continue
					}
					center := feature.Geometry.Bound().Center()
					distance := geoDistanceMeters(lat, lon, center[1], center[0])
					if distance > float64(radius) {
						continue
					}
					key := strings.ToLower(name) + "|" + kind + "|" + strconv.Itoa(int(math.Round(center[0]*10000))) + "|" + strconv.Itoa(int(math.Round(center[1]*10000)))
					if seen[key] {
						continue
					}
					seen[key] = true
					hits = append(hits, nearbyHit{Name: name, Kind: kind, Category: category, CategoryCode: categoryCode, Lat: center[1], Lon: center[0], Distance: int(math.Round(distance))})
				}
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Distance < hits[j].Distance })
	if len(hits) > 18 {
		hits = hits[:18]
	}
	if m.nearbyCache != nil {
		if raw, marshalErr := json.Marshal(hits); marshalErr == nil {
			m.nearbyCache.set(cacheKey, raw)
		}
	}
	return hits, nil
}

// nearbyTilePlan baja de zoom cuando sea necesario para que incluso 5 km se
// resuelva con una ventana acotada. El filtro Haversine posterior conserva el
// radio circular exacto.
func nearbyTilePlan(lat float64, radius int, maxZoom uint8) (uint8, int64) {
	zoom := maxZoom
	if zoom > 15 {
		zoom = 15
	}
	for {
		metersPerTile := math.Cos(lat*math.Pi/180) * 2 * math.Pi * 6371000 / math.Exp2(float64(zoom))
		tileRadius := int64(math.Ceil(float64(radius)/metersPerTile)) + 1
		if tileRadius < 1 {
			tileRadius = 1
		}
		if (2*tileRadius+1)*(2*tileRadius+1) <= 121 || zoom == 0 {
			return zoom, tileRadius
		}
		zoom--
	}
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
	_, label := nearbyCategoryInfo(kind)
	return label
}

func nearbyCategoryInfo(kind string) (string, string) {
	switch kind {
	case "restaurant", "fast_food", "food_court":
		return "restaurant", "Restaurante"
	case "cafe", "bakery", "ice_cream":
		return "cafe", "Cafetería"
	case "bar", "pub", "nightclub":
		return "bar", "Bar"
	case "fuel", "charging_station":
		return "fuel", "Gasolinera"
	case "supermarket", "convenience", "department_store", "mall", "marketplace", "clothes", "books", "electronics", "beauty", "hardware", "furniture", "florist", "gift", "jewelry", "mobile_phone", "shoes", "sports", "toys":
		return "shop", "Comercio"
	case "pharmacy", "hospital", "clinic", "doctors", "dentist":
		return "health", "Salud"
	case "hotel", "hostel", "motel", "guest_house":
		return "lodging", "Alojamiento"
	case "parking", "parking_entrance":
		return "parking", "Aparcamiento"
	case "bank", "atm":
		return "bank", "Banco"
	case "station", "bus_stop", "ferry_terminal":
		return "transport", "Transporte"
	case "museum", "attraction", "cinema", "theatre", "artwork", "library":
		return "culture", "Cultura y ocio"
	case "park", "garden", "playground":
		return "park", "Parque"
	default:
		return "", ""
	}
}
