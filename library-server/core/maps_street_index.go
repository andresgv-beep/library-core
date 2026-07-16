package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/maptile"
)

func streetIndexPath(root, mapFile string) string {
	return filepath.Join(root, filepath.Base(mapFile)+".streets.db")
}

func (m *mapManager) handleStreetIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	var input struct {
		File string `json:"file"`
	}
	if jsonDecodeSmall(w, r, &input) != nil || input.File == "" || filepath.Base(input.File) != input.File || !strings.HasSuffix(strings.ToLower(input.File), ".pmtiles") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mapa no valido"})
		return
	}
	mapPath := filepath.Join(m.root, input.File)
	st, err := os.Stat(mapPath)
	if err != nil || st.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mapa no encontrado"})
		return
	}
	m.mu.Lock()
	if m.streetJob != nil && m.streetJob.Status == "indexing" {
		m.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "ya hay un mapa indexandose"})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.streetCancel = cancel
	install := installForFile(input.File, st)
	m.streetJob = &streetIndexJob{File: input.File, Name: install.Name, Status: "indexing"}
	m.mu.Unlock()
	go m.runStreetIndex(ctx, input.File, mapPath)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "indexacion iniciada"})
}

func jsonDecodeSmall(w http.ResponseWriter, r *http.Request, dst any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(dst)
}

func (m *mapManager) handleStreetIndexCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	m.mu.Lock()
	if m.streetCancel != nil {
		m.streetCancel()
	}
	m.mu.Unlock()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "cancelando"})
}

func (m *mapManager) runStreetIndex(ctx context.Context, file, mapPath string) {
	progress := func(tiles, total, streets int64, zoom int) {
		m.mu.Lock()
		if m.streetJob != nil && m.streetJob.File == file {
			m.streetJob.Tiles, m.streetJob.TotalTiles, m.streetJob.Streets, m.streetJob.Zoom = tiles, total, streets, zoom
		}
		m.mu.Unlock()
	}
	streets, err := buildStreetIndexFromPMTiles(ctx, mapPath, streetIndexPath(m.root, file), m.geoPath, progress)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.streetJob == nil || m.streetJob.File != file {
		return
	}
	if err != nil {
		m.streetJob.Status = "error"
		if ctx.Err() != nil {
			m.streetJob.Status, m.streetJob.Error = "cancelled", "indexacion cancelada"
		} else {
			m.streetJob.Error = err.Error()
		}
		return
	}
	m.streetJob.Status, m.streetJob.Streets = "done", streets
}

type placeGrid struct {
	cell  float64
	cells map[[2]int][]geoRow
}

func loadPlaceGrid(geoPath string) *placeGrid {
	g := &placeGrid{cell: 0.1, cells: make(map[[2]int][]geoRow)}
	db, err := sql.Open("sqlite", geoPath)
	if err != nil {
		return g
	}
	defer db.Close()
	rows, err := db.Query(`SELECT name, lat, lon FROM geo WHERE kind LIKE 'place:%'`)
	if err != nil {
		return g
	}
	defer rows.Close()
	for rows.Next() {
		var p geoRow
		if rows.Scan(&p.name, &p.lat, &p.lon) == nil {
			key := [2]int{int(math.Floor(p.lat / g.cell)), int(math.Floor(p.lon / g.cell))}
			g.cells[key] = append(g.cells[key], p)
		}
	}
	return g
}

func (g *placeGrid) nearest(lat, lon float64) string {
	base := [2]int{int(math.Floor(lat / g.cell)), int(math.Floor(lon / g.cell))}
	best, bestD := "", math.Inf(1)
	coslat := math.Cos(lat * math.Pi / 180)
	for dx := -2; dx <= 2; dx++ {
		for dy := -2; dy <= 2; dy++ {
			for _, p := range g.cells[[2]int{base[0] + dx, base[1] + dy}] {
				dlat, dlon := p.lat-lat, (p.lon-lon)*coslat
				if d := dlat*dlat + dlon*dlon; d < bestD {
					best, bestD = p.name, d
				}
			}
		}
	}
	return best
}

func buildStreetIndexFromPMTiles(ctx context.Context, mapPath, dbPath, geoPath string, progress func(int64, int64, int64, int)) (int64, error) {
	f, err := os.Open(mapPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	headerBytes := make([]byte, pmHeaderLen)
	if _, err = io.ReadFull(f, headerBytes); err != nil {
		return 0, err
	}
	header, err := readPMHeader(headerBytes)
	if err != nil {
		return 0, err
	}
	if header.tileType != pmMVT || (header.tileCompression != pmGzip && header.tileCompression != pmNone) {
		return 0, fmt.Errorf("el mapa no contiene teselas vectoriales compatibles")
	}
	zoom := int(header.maxZoom)
	if zoom > 14 {
		zoom = 14 // calles completas sin multiplicar por cuatro el coste de z15
	}
	var total int64
	if err = iteratePMEntries(f, header, func(entry pmEntry) error {
		z, _, _ := pmIDToZxy(entry.tileID)
		if int(z) == zoom {
			total++
		}
		return nil
	}); err != nil {
		return 0, err
	}
	progress(0, total, 0, zoom)

	tmp := dbPath + ".new"
	_ = os.Remove(tmp)
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		return 0, err
	}
	cleanup := func() { _ = db.Close(); _ = os.Remove(tmp) }
	if _, err = db.Exec(`PRAGMA journal_mode=OFF; PRAGMA synchronous=OFF; PRAGMA temp_store=MEMORY;
		CREATE VIRTUAL TABLE geo USING fts5(
		name, kind UNINDEXED, lat UNINDEXED, lon UNINDEXED, context, display UNINDEXED, popularity UNINDEXED,
		tokenize='unicode61 remove_diacritics 2');`); err != nil {
		cleanup()
		return 0, err
	}
	tx, err := db.Begin()
	if err != nil {
		cleanup()
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT INTO geo(name, kind, lat, lon, context, display, popularity) VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		_ = tx.Rollback()
		cleanup()
		return 0, err
	}
	places := loadPlaceGrid(geoPath)
	seen := make(map[string]bool)
	var processed, streetCount int64
	var processErr error
	err = iteratePMEntries(f, header, func(entry pmEntry) error {
		if processErr != nil {
			return processErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		z, x, y := pmIDToZxy(entry.tileID)
		if int(z) != zoom {
			return nil
		}
		data := make([]byte, entry.length)
		if _, readErr := f.ReadAt(data, int64(header.tileOffset+entry.offset)); readErr != nil {
			processErr = readErr
			return readErr
		}
		var layers mvt.Layers
		if header.tileCompression == pmGzip {
			layers, processErr = mvt.UnmarshalGzipped(data)
		} else {
			layers, processErr = mvt.Unmarshal(data)
		}
		if processErr != nil {
			return processErr
		}
		for _, layer := range layers {
			if layer.Name != "roads" {
				continue
			}
			layer.ProjectToWGS84(maptile.New(x, y, maptile.Zoom(z)))
			for _, feature := range layer.Features {
				name, _ := feature.Properties["name"].(string)
				name = strings.TrimSpace(name)
				if name == "" || feature.Geometry == nil {
					continue
				}
				center := feature.Geometry.Bound().Center()
				contextName := places.nearest(center[1], center[0])
				key := strings.ToLower(name) + "|" + strings.ToLower(contextName)
				if seen[key] {
					continue
				}
				seen[key] = true
				kind, _ := feature.Properties["kind"].(string)
				if _, insertErr := stmt.Exec(name, "street:"+kind, center[1], center[0], contextName, "", 0); insertErr != nil {
					processErr = insertErr
					return insertErr
				}
				streetCount++
			}
		}
		processed++
		if processed%100 == 0 {
			progress(processed, total, streetCount, zoom)
		}
		return nil
	})
	if err == nil {
		err = processErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = stmt.Close()
	}
	if err == nil {
		err = tx.Commit()
	} else {
		_ = stmt.Close()
		_ = tx.Rollback()
	}
	if err != nil {
		cleanup()
		return 0, err
	}
	if err = db.Close(); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	_ = os.Remove(dbPath)
	if err = os.Rename(tmp, dbPath); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	progress(processed, total, streetCount, zoom)
	return streetCount, nil
}
