package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/paulmach/orb/encoding/mvt"
)

const defaultMapSource = "https://data.source.coop/protomaps/openstreetmap/v4.pmtiles"
const defaultGeoNamesSource = "https://download.geonames.org/export/dump/cities500.zip"

type mapRegion struct {
	ID       string     `json:"id"`
	Category string     `json:"category"`
	Name     string     `json:"name"`
	BBox     [4]float64 `json:"bbox"`
	Center   [2]float64 `json:"center"`
}

// Recortes deliberadamente solapados: el usuario elige una zona humana, no
// tiene que conocer coordenadas. Los continentes completos se recomiendan con
// zoom 10; para calles, una subregion con zoom 13/15.
var mapCatalog = []mapRegion{
	{ID: "europe", Category: "Europa", Name: "Europa completa", BBox: [4]float64{-25, 34, 45, 72}, Center: [2]float64{10, 52}},
	{ID: "iberia", Category: "Europa", Name: "España y Portugal", BBox: [4]float64{-10.2, 35.5, 4.8, 44.3}, Center: [2]float64{-3.5, 40.2}},
	{ID: "france-benelux", Category: "Europa", Name: "Francia y Benelux", BBox: [4]float64{-5.5, 41, 11, 53.8}, Center: [2]float64{2.5, 47.5}},
	{ID: "british-isles", Category: "Europa", Name: "Islas Britanicas", BBox: [4]float64{-11, 49, 2.5, 61}, Center: [2]float64{-3, 55}},
	{ID: "central-europe", Category: "Europa", Name: "Europa central", BBox: [4]float64{5, 44, 25, 56}, Center: [2]float64{14, 50}},
	{ID: "nordics", Category: "Europa", Name: "Paises nordicos", BBox: [4]float64{4, 54, 32, 72}, Center: [2]float64{17, 63}},
	{ID: "balkans", Category: "Europa", Name: "Balcanes y Grecia", BBox: [4]float64{12, 34, 30, 48}, Center: [2]float64{21, 42}},
	{ID: "eastern-europe", Category: "Europa", Name: "Europa oriental", BBox: [4]float64{20, 43, 45, 60}, Center: [2]float64{31, 51}},
	{ID: "asia", Category: "Asia", Name: "Asia completa", BBox: [4]float64{25, -11, 180, 81}, Center: [2]float64{90, 38}},
	{ID: "middle-east", Category: "Asia", Name: "Oriente Medio", BBox: [4]float64{25, 12, 64, 43}, Center: [2]float64{45, 29}},
	{ID: "central-asia", Category: "Asia", Name: "Asia central", BBox: [4]float64{45, 34, 90, 56}, Center: [2]float64{67, 45}},
	{ID: "south-asia", Category: "Asia", Name: "Asia meridional", BBox: [4]float64{60, 5, 93, 38}, Center: [2]float64{77, 22}},
	{ID: "southeast-asia", Category: "Asia", Name: "Sudeste asiatico", BBox: [4]float64{92, -11, 142, 29}, Center: [2]float64{116, 10}},
	{ID: "east-asia", Category: "Asia", Name: "China, Mongolia y Corea", BBox: [4]float64{73, 18, 135, 54}, Center: [2]float64{105, 36}},
	{ID: "japan", Category: "Asia", Name: "Japon", BBox: [4]float64{128, 29, 146, 46}, Center: [2]float64{138, 37}},
	{ID: "africa", Category: "Africa", Name: "Africa completa", BBox: [4]float64{-19, -36, 52, 38}, Center: [2]float64{20, 2}},
	{ID: "north-africa", Category: "Africa", Name: "Africa del norte", BBox: [4]float64{-18, 18, 37, 38}, Center: [2]float64{10, 29}},
	{ID: "west-africa", Category: "Africa", Name: "Africa occidental", BBox: [4]float64{-19, 0, 17, 21}, Center: [2]float64{-1, 10}},
	{ID: "central-africa", Category: "Africa", Name: "Africa central", BBox: [4]float64{8, -14, 35, 13}, Center: [2]float64{22, 0}},
	{ID: "east-africa", Category: "Africa", Name: "Africa oriental", BBox: [4]float64{28, -13, 52, 18}, Center: [2]float64{39, 2}},
	{ID: "southern-africa", Category: "Africa", Name: "Africa austral", BBox: [4]float64{10, -36, 41, -10}, Center: [2]float64{25, -24}},
	{ID: "north-america", Category: "America del Norte", Name: "America del Norte", BBox: [4]float64{-170, 7, -50, 84}, Center: [2]float64{-105, 48}},
	{ID: "canada", Category: "America del Norte", Name: "Canada", BBox: [4]float64{-142, 41, -50, 84}, Center: [2]float64{-96, 58}},
	{ID: "usa", Category: "America del Norte", Name: "Estados Unidos", BBox: [4]float64{-125, 24, -66, 50}, Center: [2]float64{-98, 39}},
	{ID: "mexico-central", Category: "America del Norte", Name: "Mexico y Centroamerica", BBox: [4]float64{-118, 7, -77, 33}, Center: [2]float64{-96, 19}},
	{ID: "caribbean", Category: "America del Norte", Name: "Caribe", BBox: [4]float64{-90, 9, -58, 28}, Center: [2]float64{-74, 18}},
	{ID: "south-america", Category: "America del Sur", Name: "America del Sur", BBox: [4]float64{-82, -56, -34, 14}, Center: [2]float64{-60, -18}},
	{ID: "andes", Category: "America del Sur", Name: "Region andina", BBox: [4]float64{-82, -24, -63, 13}, Center: [2]float64{-73, -5}},
	{ID: "brazil", Category: "America del Sur", Name: "Brasil", BBox: [4]float64{-74, -34, -34, 6}, Center: [2]float64{-52, -14}},
	{ID: "southern-cone", Category: "America del Sur", Name: "Cono Sur", BBox: [4]float64{-76, -56, -51, -17}, Center: [2]float64{-64, -37}},
	{ID: "oceania", Category: "Oceania", Name: "Oceania", BBox: [4]float64{110, -50, 180, 0}, Center: [2]float64{145, -25}},
	{ID: "australia", Category: "Oceania", Name: "Australia", BBox: [4]float64{112, -44, 154, -10}, Center: [2]float64{134, -26}},
	{ID: "new-zealand", Category: "Oceania", Name: "Nueva Zelanda", BBox: [4]float64{165, -48, 179.9, -33}, Center: [2]float64{172, -41}},
}

type mapInstall struct {
	File          string     `json:"file"`
	RegionID      string     `json:"regionId"`
	Name          string     `json:"name"`
	BBox          [4]float64 `json:"bbox"`
	Center        [2]float64 `json:"center"`
	MaxZoom       int        `json:"maxZoom"`
	Bytes         int64      `json:"bytes"`
	StreetIndexed bool       `json:"streetIndexed"`
	StreetBytes   int64      `json:"streetBytes,omitempty"`
}

type mapState struct {
	Active *mapInstall `json:"active,omitempty"`
}

type mapJob struct {
	RegionID string `json:"regionId"`
	Name     string `json:"name"`
	File     string `json:"file"`
	Status   string `json:"status"`
	MaxZoom  int    `json:"maxZoom"`
	Bytes    int64  `json:"bytes"`
	Error    string `json:"error,omitempty"`
	partPath string
}

type geoIndexJob struct {
	Status   string `json:"status"`
	Bytes    int64  `json:"bytes"`
	Entries  int    `json:"entries,omitempty"`
	Error    string `json:"error,omitempty"`
	partPath string
}

type streetIndexJob struct {
	File       string `json:"file"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Tiles      int64  `json:"tiles"`
	TotalTiles int64  `json:"totalTiles"`
	Streets    int64  `json:"streets"`
	Zoom       int    `json:"zoom"`
	Error      string `json:"error,omitempty"`
}

type mapManager struct {
	root, tool, source string
	geoPath, geoSource string
	mu                 sync.Mutex
	job                *mapJob
	geoJob             *geoIndexJob
	streetJob          *streetIndexJob
	streetCancel       context.CancelFunc
	cancel             context.CancelFunc
}

func newMapManager(root, geoPath string) *mapManager {
	return &mapManager{
		root: root, geoPath: geoPath, tool: findMapTool(),
		source:    env("MAP_SOURCE_URL", defaultMapSource),
		geoSource: env("MAP_GEONAMES_URL", defaultGeoNamesSource),
	}
}

func findMapTool() string {
	for _, name := range []string{"pmtiles", "go-pmtiles"} {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		path := filepath.Join(siblingDir(""), name)
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
	}
	return ""
}

func (m *mapManager) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo GET"})
		return
	}
	m.mu.Lock()
	job := m.job
	if job != nil && job.partPath != "" {
		copyJob := *job
		if st, err := os.Stat(job.partPath); err == nil {
			copyJob.Bytes = st.Size()
		}
		job = &copyJob
	}
	geoJob := m.geoJob
	if geoJob != nil {
		copyGeo := *geoJob
		if geoJob.partPath != "" {
			if st, err := os.Stat(geoJob.partPath); err == nil {
				copyGeo.Bytes = st.Size()
			}
		}
		geoJob = &copyGeo
	}
	streetJob := m.streetJob
	if streetJob != nil {
		copyStreet := *streetJob
		streetJob = &copyStreet
	}
	m.mu.Unlock()
	state, _ := m.loadState()
	geoInstalled, geoBytes := false, int64(0)
	if st, err := os.Stat(m.geoPath); err == nil && !st.IsDir() {
		geoInstalled, geoBytes = true, st.Size()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"catalog": mapCatalog, "installed": m.installed(), "active": state.Active,
		"job": job, "available": m.tool != "",
		"geocoder":  map[string]any{"installed": geoInstalled, "bytes": geoBytes, "job": geoJob},
		"streetJob": streetJob,
	})
}

func (m *mapManager) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	if m.tool == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "extractor PMTiles no instalado"})
		return
	}
	var input struct {
		RegionID string `json:"regionId"`
		MaxZoom  int    `json:"maxZoom"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalido"})
		return
	}
	region, ok := catalogRegion(input.RegionID)
	if !ok || input.MaxZoom < 8 || input.MaxZoom > 15 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "region o detalle no valido"})
		return
	}
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	file := region.ID + "-z" + strconv.Itoa(input.MaxZoom) + ".pmtiles"
	part := filepath.Join(m.root, file+".part")
	m.mu.Lock()
	if m.job != nil && (m.job.Status == "downloading" || m.job.Status == "starting") {
		m.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "ya hay un mapa descargandose"})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.job = &mapJob{RegionID: region.ID, Name: region.Name, File: file, Status: "starting", MaxZoom: input.MaxZoom, partPath: part}
	m.mu.Unlock()
	go m.extract(ctx, region, input.MaxZoom, file, part)
	// El primer mapa instala también el buscador mundial ligero. Son unos
	// 12 MB comprimidos y queda compartido por todos los mapas.
	_ = m.startGeocoder()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "descarga iniciada"})
}

func (m *mapManager) handleGeocoder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	if err := m.startGeocoder(); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "indice iniciado"})
}

func (m *mapManager) startGeocoder() error {
	if _, err := os.Stat(m.geoPath); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.geoPath), 0o755); err != nil {
		return err
	}
	part := filepath.Join(filepath.Dir(m.geoPath), "cities500.zip.part")
	m.mu.Lock()
	if m.geoJob != nil && (m.geoJob.Status == "downloading" || m.geoJob.Status == "indexing") {
		m.mu.Unlock()
		return nil
	}
	m.geoJob = &geoIndexJob{Status: "downloading", partPath: part}
	m.mu.Unlock()
	go m.downloadGeocoder(part)
	return nil
}

func (m *mapManager) downloadGeocoder(part string) {
	_ = os.Remove(part)
	resp, err := http.Get(m.geoSource)
	if err == nil && resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("GeoNames: HTTP %d", resp.StatusCode)
	}
	if err == nil {
		var f *os.File
		f, err = os.Create(part)
		if err == nil {
			_, err = io.Copy(f, resp.Body)
			if closeErr := f.Close(); err == nil {
				err = closeErr
			}
		}
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		m.finishGeoError(err, part)
		return
	}
	m.mu.Lock()
	m.geoJob.Status = "indexing"
	m.mu.Unlock()
	entries, err := buildGeoNamesCitiesIndex(part, m.geoPath)
	if err != nil {
		m.finishGeoError(err, part)
		return
	}
	_ = os.Remove(part)
	m.mu.Lock()
	m.geoJob.Status, m.geoJob.Entries, m.geoJob.partPath = "done", entries, ""
	if st, statErr := os.Stat(m.geoPath); statErr == nil {
		m.geoJob.Bytes = st.Size()
	}
	m.mu.Unlock()
}

func (m *mapManager) finishGeoError(err error, part string) {
	_ = os.Remove(part)
	m.mu.Lock()
	m.geoJob.Status, m.geoJob.Error, m.geoJob.partPath = "error", err.Error(), ""
	m.mu.Unlock()
}

func (m *mapManager) extract(ctx context.Context, region mapRegion, maxZoom int, file, part string) {
	_ = os.Remove(part)
	bbox := fmt.Sprintf("%g,%g,%g,%g", region.BBox[0], region.BBox[1], region.BBox[2], region.BBox[3])
	m.mu.Lock()
	m.job.Status = "downloading"
	m.mu.Unlock()
	cmd := exec.CommandContext(ctx, m.tool, "extract", m.source, part, "--bbox="+bbox, "--maxzoom="+strconv.Itoa(maxZoom), "--quiet")
	output, err := cmd.CombinedOutput()
	if err == nil {
		err = os.Rename(part, filepath.Join(m.root, file))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.job.Status = "error"
		if ctx.Err() != nil {
			m.job.Status, m.job.Error = "cancelled", "descarga cancelada"
		} else {
			m.job.Error = strings.TrimSpace(string(output))
			if m.job.Error == "" {
				m.job.Error = err.Error()
			}
		}
		_ = os.Remove(part)
		return
	}
	st, _ := os.Stat(filepath.Join(m.root, file))
	m.job.Status = "done"
	if st != nil {
		m.job.Bytes = st.Size()
	}
	state, _ := m.loadState()
	if state.Active == nil {
		install := installForFile(file, st)
		state.Active = &install
		_ = m.saveState(state)
	}
}

func (m *mapManager) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "solo POST"})
		return
	}
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "cancelando"})
}

func (m *mapManager) handleActivate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		File string `json:"file"`
	}
	if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&input) != nil || filepath.Base(input.File) != input.File {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "peticion no valida"})
		return
	}
	st, err := os.Stat(filepath.Join(m.root, input.File))
	if err != nil || st.IsDir() || !strings.HasSuffix(strings.ToLower(input.File), ".pmtiles") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mapa no encontrado"})
		return
	}
	install := installForFile(input.File, st)
	if err := m.saveState(mapState{Active: &install}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active": install})
}

func (m *mapManager) handleDelete(w http.ResponseWriter, r *http.Request) {
	var input struct {
		File string `json:"file"`
	}
	if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&input) != nil || filepath.Base(input.File) != input.File {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "peticion no valida"})
		return
	}
	if err := os.Remove(filepath.Join(m.root, input.File)); err != nil && !os.IsNotExist(err) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = os.Remove(streetIndexPath(m.root, input.File))
	state, _ := m.loadState()
	if state.Active != nil && state.Active.File == input.File {
		state.Active = nil
		installed := m.installed()
		if len(installed) > 0 {
			state.Active = &installed[0]
		}
		_ = m.saveState(state)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (m *mapManager) handlePublicConfig(w http.ResponseWriter, r *http.Request) {
	state, _ := m.loadState()
	if state.Active == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no hay mapa activo"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": state.Active.Name, "file": state.Active.File, "url": "/mapdata/" + state.Active.File, "bbox": state.Active.BBox, "center": state.Active.Center, "maxZoom": state.Active.MaxZoom})
}

func (m *mapManager) handlePublicTile(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/maps/tiles/"
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if r.Method != http.MethodGet || len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	file := parts[0]
	if file == "" || filepath.Base(file) != file || !strings.HasSuffix(strings.ToLower(file), ".pmtiles") {
		http.NotFound(w, r)
		return
	}
	z64, errZ := strconv.ParseUint(parts[1], 10, 8)
	x64, errX := strconv.ParseUint(parts[2], 10, 32)
	y64, errY := strconv.ParseUint(strings.TrimSuffix(parts[3], ".mvt"), 10, 32)
	if errZ != nil || errX != nil || errY != nil || !strings.HasSuffix(parts[3], ".mvt") {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(m.root, file))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	headerBytes := make([]byte, pmHeaderLen)
	if _, err = io.ReadFull(f, headerBytes); err != nil {
		http.Error(w, "mapa no valido", http.StatusInternalServerError)
		return
	}
	header, err := readPMHeader(headerBytes)
	if err != nil || header.tileType != pmMVT {
		http.Error(w, "mapa no compatible", http.StatusInternalServerError)
		return
	}
	tile, found, err := readPMTile(f, header, uint8(z64), uint32(x64), uint32(y64))
	if err != nil {
		http.Error(w, "no se pudo leer la tesela", http.StatusInternalServerError)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// Planetiler puede producir geometrías perfectamente recuperables pero que
	// ciertos decodificadores WebGL descartan completas. Pasarlas por el lector
	// MVT de Core normaliza comandos y anillos antes de entregarlas al cliente.
	layers, err := mvt.Unmarshal(tile)
	if err != nil {
		http.Error(w, "tesela vectorial no valida", http.StatusInternalServerError)
		return
	}
	normalized := layers[:0]
	for _, layer := range layers {
		// Los centros urbanos pueden superar 300 POI por tesela. WebView2 deja
		// de dibujar el tile entero al construir tantos símbolos; Planetiler los
		// ordena por prioridad, así que conservamos los 160 más relevantes.
		if layer.Name == "pois" && len(layer.Features) > 160 {
			layer.Features = layer.Features[:160]
		}
		normalized = append(normalized, layer)
	}
	tile, err = mvt.Marshal(normalized)
	if err != nil {
		http.Error(w, "no se pudo normalizar la tesela", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.mapbox-vector-tile")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(tile)
}

func (m *mapManager) installed() []mapInstall {
	entries, _ := os.ReadDir(m.root)
	result := make([]mapInstall, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".pmtiles") {
			continue
		}
		st, err := entry.Info()
		if err == nil {
			install := installForFile(entry.Name(), st)
			if streets, streetErr := os.Stat(streetIndexPath(m.root, entry.Name())); streetErr == nil {
				install.StreetIndexed, install.StreetBytes = true, streets.Size()
			}
			result = append(result, install)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func installForFile(file string, st os.FileInfo) mapInstall {
	base := strings.TrimSuffix(file, filepath.Ext(file))
	id := base
	zoom := 0
	if marker := strings.LastIndex(base, "-z"); marker > 0 {
		if parsed, err := strconv.Atoi(base[marker+2:]); err == nil {
			zoom = parsed
			id = base[:marker]
		}
	}
	region, ok := catalogRegion(id)
	if !ok {
		region = mapRegion{ID: id, Name: id}
	}
	return mapInstall{File: file, RegionID: id, Name: region.Name, BBox: region.BBox, Center: region.Center, MaxZoom: zoom, Bytes: st.Size()}
}

func catalogRegion(id string) (mapRegion, bool) {
	for _, region := range mapCatalog {
		if region.ID == id {
			return region, true
		}
	}
	return mapRegion{}, false
}

func (m *mapManager) statePath() string { return filepath.Join(m.root, "maps.json") }
func (m *mapManager) loadState() (mapState, error) {
	var state mapState
	raw, err := os.ReadFile(m.statePath())
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(raw, &state)
	return state, err
}
func (m *mapManager) saveState(state mapState) error {
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(state, "", "  ")
	return os.WriteFile(m.statePath(), append(raw, '\n'), 0o600)
}
