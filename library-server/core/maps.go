package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Plugin Maps — geocoder offline. Índice SQLite FTS5 de calles/lugares (sacados de
// OSM vía Overpass) con nuestro ranking. Mismo patrón "capa Nimos" que la búsqueda
// de Library: el motor da datos, nosotros ponemos la relevancia.

// ─── Datos crudos de Overpass ─────────────────────────────────────────────────
type opElement struct {
	Type   string  `json:"type"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Center *struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"center"`
	Tags map[string]string `json:"tags"`
}
type opResult struct {
	Elements []opElement `json:"elements"`
}

// geoRow: una entrada del índice. `context` se indexa (busca); `display` no (solo
// se muestra, p.ej. el pueblo de un código postal, para no ensuciar búsquedas).
type geoRow struct {
	name, kind, context, display string
	lat, lon                     float64
}

// buildGeoIndex: Overpass JSON (+ opcional GeoNames CP) → geo.db (FTS5).
// cpPrefixes: prefijos de provincia de los códigos postales a indexar,
// separados por coma ("08,17,25,43" = Catalunya). Parametrizado para que los
// futuros packs de región (MAPS.md §4) reutilicen el mismo builder.
func buildGeoIndex(jsonPath, dbPath, geonamesPath, cpPrefixes string) error {
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}
	var res opResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return err
	}

	os.Remove(dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	// remove_diacritics 2 → escribir "passeig" encuentra "Passeig" (sin acentos).
	if _, err := db.Exec(`CREATE VIRTUAL TABLE geo USING fts5(
		name, kind UNINDEXED, lat UNINDEXED, lon UNINDEXED, context, display UNINDEXED,
		tokenize='unicode61 remove_diacritics 2');`); err != nil {
		return err
	}

	// Fase 1: recoger lugares y calles por separado (dedup por nombre+coord).
	var places, streets []geoRow
	seen := make(map[string]bool)
	for _, e := range res.Elements {
		name := strings.TrimSpace(e.Tags["name"])
		if name == "" {
			continue
		}
		var lat, lon float64
		var kind, context string
		switch {
		case e.Type == "node" && e.Tags["place"] != "":
			lat, lon, kind = e.Lat, e.Lon, "place:"+e.Tags["place"]
		case e.Type == "way" && e.Tags["highway"] != "" && e.Center != nil:
			lat, lon, kind = e.Center.Lat, e.Center.Lon, "street"
			context = e.Tags["addr:city"] // rara vez presente → se rellena en fase 2
		default:
			continue
		}
		key := strings.ToLower(name) + "|" + fmt.Sprintf("%.2f,%.2f", lat, lon)
		if seen[key] {
			continue
		}
		seen[key] = true
		row := geoRow{name: name, kind: kind, context: context, lat: lat, lon: lon}
		if kind == "street" {
			streets = append(streets, row)
		} else {
			places = append(places, row)
		}
	}

	// Fase 2: contexto de cada calle = los ~3 lugares cercanos + el municipio.
	// Así casan tanto barrios pequeños ("la Floresta") como el pueblo/ciudad,
	// sin depender de acertar "el" lugar. Rejilla espacial de ~5km (barato).
	const cell = 0.05
	ckey := func(lat, lon float64) [2]int { return [2]int{int(math.Floor(lat / cell)), int(math.Floor(lon / cell))} }
	grid := make(map[[2]int][]int)
	for i, p := range places {
		grid[ckey(p.lat, p.lon)] = append(grid[ckey(p.lat, p.lon)], i)
	}
	// Lugares diminutos que no ayudan como contexto; y municipios (siempre útiles).
	tiny := map[string]bool{"place:isolated_dwelling": true, "place:farm": true, "place:islet": true, "place:plot": true, "place:square": true}
	big := map[string]bool{"place:city": true, "place:town": true, "place:village": true, "place:municipality": true}
	nearContext := func(lat, lon float64) string {
		base := ckey(lat, lon)
		coslat := math.Cos(lat * math.Pi / 180)
		type cand struct {
			d    float64
			name string
			big  bool
		}
		var cands []cand
		for dx := -2; dx <= 2; dx++ { // bloque 5×5 (~25km) → suficiente en Catalunya
			for dy := -2; dy <= 2; dy++ {
				for _, idx := range grid[[2]int{base[0] + dx, base[1] + dy}] {
					p := places[idx]
					if tiny[p.kind] {
						continue
					}
					dLat, dLon := p.lat-lat, (p.lon-lon)*coslat
					cands = append(cands, cand{dLat*dLat + dLon*dLon, p.name, big[p.kind]})
				}
			}
		}
		sort.Slice(cands, func(i, j int) bool { return cands[i].d < cands[j].d })
		seen := map[string]bool{}
		var names []string
		add := func(nm string) {
			if k := strings.ToLower(nm); !seen[k] {
				seen[k] = true
				names = append(names, nm)
			}
		}
		for _, c := range cands { // los 3 lugares más cercanos (cualquiera)
			if len(names) >= 3 {
				break
			}
			add(c.name)
		}
		for _, c := range cands { // + el municipio más cercano (aunque no esté en el top 3)
			if c.big {
				add(c.name)
				break
			}
		}
		return strings.Join(names, " ")
	}
	for i := range streets {
		if streets[i].context == "" {
			streets[i].context = nearContext(streets[i].lat, streets[i].lon)
		}
	}

	// Códigos postales (GeoNames, opcional): buscables por el código; el pueblo va
	// en `display` (no indexado) para no aparecer al buscar el pueblo por nombre.
	var postcodes []geoRow
	if geonamesPath != "" {
		postcodes, err = loadPostcodes(geonamesPath, cpPrefixes)
		if err != nil {
			return err
		}
	}

	// Fase 3: insertar todo en el FTS.
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO geo(name, kind, lat, lon, context, display) VALUES(?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	n := 0
	for _, rows := range [][]geoRow{places, streets, postcodes} {
		for _, g := range rows {
			if _, err := stmt.Exec(g.name, g.kind, g.lat, g.lon, g.context, g.display); err != nil {
				return err
			}
			n++
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("geo index: %d entradas (%d lugares, %d calles, %d CP) → %s", n, len(places), len(streets), len(postcodes), dbPath)
	return nil
}

// loadPostcodes: TSV de GeoNames (país) → entradas de CP de las provincias
// indicadas (prefijos separados por coma; vacío = todas las del fichero).
// Columnas: 0 país · 1 código · 2 lugar · … · 9 lat · 10 lon.
func loadPostcodes(path, prefixes string) ([]geoRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wanted []string
	for _, p := range strings.Split(prefixes, ",") {
		if p = strings.TrimSpace(p); p != "" {
			wanted = append(wanted, p)
		}
	}
	matches := func(code string) bool {
		if len(wanted) == 0 {
			return true
		}
		for _, p := range wanted {
			if strings.HasPrefix(code, p) {
				return true
			}
		}
		return false
	}
	var out []geoRow
	for _, line := range strings.Split(string(raw), "\n") {
		c := strings.Split(line, "\t")
		if len(c) < 11 {
			continue
		}
		code := strings.TrimSpace(c[1])
		if len(code) != 5 || !matches(code) {
			continue
		}
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(c[9]), 64)
		lon, err2 := strconv.ParseFloat(strings.TrimSpace(c[10]), 64)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, geoRow{name: code, kind: "postcode", display: strings.TrimSpace(c[2]), lat: lat, lon: lon})
	}
	return out, nil
}

// ─── /api/maps/geocode ────────────────────────────────────────────────────────
type GeoHit struct {
	Name    string  `json:"name"`
	Kind    string  `json:"kind"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Context string  `json:"context,omitempty"`
}

func (s *Server) handleGeocode(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || s.geo == nil {
		writeJSON(w, http.StatusOK, []GeoHit{})
		return
	}
	toks := geoTokens(q)
	if len(toks) == 0 {
		writeJSON(w, http.StatusOK, []GeoHit{})
		return
	}
	// Intento estricto (todos los tokens). Si no hay nada, reintento soltando el
	// primer token (suele ser el genérico "carrer/calle/avinguda…" que puede no
	// casar el idioma). Los números de portal ya se quitaron en geoTokens.
	for start := 0; start < len(toks) && start < 2; start++ {
		if hits := s.geoSearch(matchExpr(toks[start:]), q); len(hits) > 0 {
			writeJSON(w, http.StatusOK, hits)
			return
		}
	}
	writeJSON(w, http.StatusOK, []GeoHit{})
}

// geoSearch: ejecuta la consulta FTS con el ranking Nimos (exacta > empieza-por >
// bm25; a igualdad, lugares antes que calles).
func (s *Server) geoSearch(match, raw string) []GeoHit {
	rows, err := s.geo.Query(`
		SELECT name, kind, lat, lon, context, display
		FROM geo WHERE geo MATCH ?
		ORDER BY
		  CASE WHEN lower(name)=lower(?) THEN 0
		       WHEN lower(name) LIKE lower(?)||'%' THEN 1 ELSE 2 END,
		  CASE WHEN kind LIKE 'place%' THEN 0 ELSE 1 END,
		  bm25(geo)
		LIMIT 12`, match, raw, raw)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []GeoHit{}
	for rows.Next() {
		var h GeoHit
		var display string
		if err := rows.Scan(&h.Name, &h.Kind, &h.Lat, &h.Lon, &h.Context, &display); err == nil {
			if h.Context == "" {
				h.Context = display // p.ej. el pueblo de un código postal
			}
			out = append(out, h)
		}
	}
	return out
}

// geoTokens: limpia la consulta y quita los tokens que son solo número (portales).
func geoTokens(q string) []string {
	q = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`"*():^-,.`, r) {
			return ' '
		}
		return r
	}, q)
	out := []string{}
	for _, t := range strings.Fields(q) {
		// Quita números de portal (≤4 dígitos) pero conserva códigos postales (5).
		if isAllDigits(t) && len(t) != 5 {
			continue
		}
		out = append(out, t)
	}
	return out
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// matchExpr: expresión FTS5 con prefijo en el último token (autocompletar).
func matchExpr(toks []string) string {
	if len(toks) == 0 {
		return `""`
	}
	cp := append([]string{}, toks...)
	cp[len(cp)-1] += "*"
	return strings.Join(cp, " ")
}
