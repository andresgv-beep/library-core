package main

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type mapSearchLocation struct {
	GeoHit
	MatchQuality string `json:"matchQuality"`
}

type mapSearchMap struct {
	Name    string     `json:"name"`
	File    string     `json:"file"`
	BBox    [4]float64 `json:"bbox"`
	MaxZoom int        `json:"maxZoom"`
	Style   string     `json:"style"`
	Tiles   string     `json:"tiles"`
}

// mapSearchResponse es el contrato agregado que consume Library.
type mapSearchResponse struct {
	Available    bool                `json:"available"`
	Reason       string              `json:"reason"`
	Query        string              `json:"query"`
	Radius       int                 `json:"radius"`
	Location     *mapSearchLocation  `json:"location"`
	Alternatives []mapSearchLocation `json:"alternatives"`
	POIs         []nearbyHit         `json:"pois"`
	Map          *mapSearchMap       `json:"map"`
}

func emptyMapSearch(query string, radius int, reason string) mapSearchResponse {
	return mapSearchResponse{
		Available: false, Reason: reason, Query: query, Radius: radius,
		Alternatives: []mapSearchLocation{}, POIs: []nearbyHit{},
	}
}

func (s *Server) handleMapSearch(m *mapManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo GET"})
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falta q"})
			return
		}
		radius := 2500
		if raw := strings.TrimSpace(r.URL.Query().Get("radius")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 || parsed > 5000 || parsed%500 != 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "radius debe estar entre 0 y 5000 en pasos de 500"})
				return
			}
			radius = parsed
		}

		state, err := m.loadState()
		if err != nil || state.Active == nil {
			writeJSON(w, http.StatusOK, emptyMapSearch(query, radius, "no_map"))
			return
		}
		active := state.Active
		if active.File == "" || filepath.Base(active.File) != active.File {
			writeJSON(w, http.StatusOK, emptyMapSearch(query, radius, "no_map"))
			return
		}
		if info, statErr := os.Stat(filepath.Join(m.root, active.File)); statErr != nil || info.IsDir() {
			writeJSON(w, http.StatusOK, emptyMapSearch(query, radius, "no_map"))
			return
		}
		if s.geocoder() == nil {
			writeJSON(w, http.StatusOK, emptyMapSearch(query, radius, "no_geocoder"))
			return
		}

		hits := s.searchGeo(query, active.File, &active.BBox)
		locations := make([]mapSearchLocation, 0, len(hits))
		for _, hit := range hits {
			quality := geoMatchQuality(query, hit)
			if quality == "exact" || quality == "strong" {
				locations = append(locations, mapSearchLocation{GeoHit: hit, MatchQuality: quality})
			}
		}
		rankMapSearchLocations(locations)
		if len(locations) == 0 {
			writeJSON(w, http.StatusOK, emptyMapSearch(query, radius, "no_match"))
			return
		}

		alternatives := []mapSearchLocation{}
		if len(locations) > 1 {
			end := len(locations)
			if end > 4 {
				end = 4
			}
			alternatives = append(alternatives, locations[1:end]...)
		}
		location := locations[0]
		pois := []nearbyHit{}
		if radius > 0 {
			pois, err = m.nearby(r.Context(), location.Lat, location.Lon, active.File, radius)
			if err != nil {
				reason := "map_incompatible"
				if os.IsNotExist(err) {
					reason = "no_map"
				}
				writeJSON(w, http.StatusOK, emptyMapSearch(query, radius, reason))
				return
			}
		}
		writeJSON(w, http.StatusOK, mapSearchResponse{
			Available: true, Reason: "", Query: query, Radius: radius,
			Location: &location, Alternatives: alternatives, POIs: pois,
			Map: &mapSearchMap{
				Name: active.Name, File: active.File, BBox: active.BBox, MaxZoom: active.MaxZoom,
				Style: "/maps/style-light.json",
				Tiles: "/api/maps/tiles/" + url.PathEscape(active.File) + "/{z}/{x}/{y}.mvt",
			},
		})
	}
}

// En Library una consulta desnuda como "Madrid" expresa normalmente una
// localidad, no una calle homonima. El buscador de Maps conserva su ranking
// detallado; este agregado solo adapta el orden a la intencion de Library.
func rankMapSearchLocations(locations []mapSearchLocation) {
	sort.SliceStable(locations, func(i, j int) bool {
		left, right := locations[i], locations[j]
		if left.MatchQuality != right.MatchQuality {
			return left.MatchQuality == "exact"
		}
		leftPlace := strings.HasPrefix(left.Kind, "place")
		rightPlace := strings.HasPrefix(right.Kind, "place")
		if leftPlace != rightPlace {
			return leftPlace
		}
		return false
	})
}

func geoMatchQuality(query string, hit GeoHit) string {
	queryNorm := normalizeText(query)
	nameNorm := normalizeText(hit.Name)
	if queryNorm != "" && queryNorm == nameNorm {
		return "exact"
	}
	queryTokens := geoTokens(queryNorm)
	if len(queryTokens) == 0 {
		return "weak"
	}
	coveredBy := make(map[string]bool)
	for _, token := range geoTokens(normalizeText(hit.Name + " " + hit.Context)) {
		coveredBy[token] = true
	}
	for _, token := range queryTokens {
		if !coveredBy[token] {
			return "weak"
		}
	}
	return "strong"
}
