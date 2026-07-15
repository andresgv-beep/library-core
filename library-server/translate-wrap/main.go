// translate-wrap — sidecar HTTP fino sobre translateLocally (TRANSLATE.md §1/§2).
//
// El binario translateLocally (motor Bergamot/Marian, offline) tiene CLI headless
// pero NO servidor REST. Este wrapper lo expone en el contrato que el shim ya
// consume, para que la traducción sea "una pieza madura detrás del shim":
//
//	GET  /languages  -> {"pairs":[{"from":"en","to":"es"}, ...]}
//	POST /translate  -> {"texts":[...]}
//	     body: {"from":"en","to":"es","texts":[...]}
//
// Cada request lanza translateLocally una vez (carga modelo ~0.5s) traduciendo
// TODOS los textos en lote (una línea por texto). La caché vive en el shim, así
// que un segmento se traduce una sola vez.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var (
	bin   string
	sem   chan struct{}
	mu    sync.RWMutex        // protege pairs/model (loadModels reescribe en caliente)
	pairs []map[string]string // [{from,to}] instalados
	model = map[string]string{} // "en-es" -> "en-es-tiny"
	reMdl = regexp.MustCompile(`-m\s+([a-z]{2,3})-([a-z]{2,3})-(\w+)`)
	reWS  = regexp.MustCompile(`\s+`)
	reSeg = regexp.MustCompile(`(?s)<p data-tw="(\d+)">(.*?)</p>`)
	// Línea de modelo tanto en -l ("To invoke do -m id") como en -a ("To download do -d id").
	reModelLine = regexp.MustCompile(`(?m)^(.+?) type: (\S+) version: \d+; To \w+ do -[dm] (\S+)\s*$`)
	reModelID   = regexp.MustCompile(`^[a-z]{2,4}-[a-z]{2,4}-\w+$`)
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// loadModels ejecuta `translateLocally -l` y arma pares + mapa (from-to)->modelId.
// Construye copias locales y las intercambia bajo lock: seguro con traducciones
// en curso y refleja bien los modelos añadidos/quitados en caliente.
func loadModels() error {
	out, err := exec.Command(bin, "-l").Output()
	if err != nil {
		return err
	}
	newPairs := []map[string]string{}
	newModel := map[string]string{}
	for _, m := range reMdl.FindAllStringSubmatch(string(out), -1) {
		from, to, id := m[1], m[2], m[1]+"-"+m[2]+"-"+m[3]
		key := from + "-" + to
		if _, dup := newModel[key]; dup {
			continue
		}
		newModel[key] = id
		newPairs = append(newPairs, map[string]string{"from": from, "to": to})
	}
	mu.Lock()
	pairs, model = newPairs, newModel
	mu.Unlock()
	return nil
}

func handleLanguages(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	p := pairs
	mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"pairs": p})
}

// ── Gestión de modelos (Panel de Control) ──────────────────────────────────

type modelInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	From      string `json:"from"`
	To        string `json:"to"`
	Installed bool   `json:"installed"`
}

func parseModelLines(out string) []modelInfo {
	var res []modelInfo
	for _, m := range reModelLine.FindAllStringSubmatch(out, -1) {
		id := m[3]
		mi := modelInfo{ID: id, Name: strings.TrimSpace(m[1]), Type: m[2]}
		if p := strings.Split(id, "-"); len(p) >= 3 {
			mi.From, mi.To = p[0], p[1]
		}
		res = append(res, mi)
	}
	return res
}

func installedIDs() map[string]bool {
	mu.RLock()
	defer mu.RUnlock()
	ids := make(map[string]bool, len(model))
	for _, id := range model {
		ids[id] = true
	}
	return ids
}

// GET /models/available — modelos descargables (translateLocally -a), marcando
// los ya instalados.
func handleModelsAvailable(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command(bin, "-a").CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "no se pudo listar disponibles", "detail": string(out)})
		return
	}
	models := parseModelLines(string(out))
	inst := installedIDs()
	for i := range models {
		models[i].Installed = inst[models[i].ID]
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

// POST /models/download {id} — descarga un modelo y recarga la lista.
func handleModelsDownload(w http.ResponseWriter, r *http.Request) {
	handleModelOp(w, r, "-d", "descargar")
}

// POST /models/remove {id} — borra un modelo local y recarga la lista.
func handleModelsRemove(w http.ResponseWriter, r *http.Request) {
	handleModelOp(w, r, "-r", "borrar")
}

func handleModelOp(w http.ResponseWriter, r *http.Request, flag, verb string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !reModelID.MatchString(req.ID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id de modelo inválido"})
		return
	}
	out, err := exec.Command(bin, flag, req.ID).CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "no se pudo " + verb + " " + req.ID, "detail": strings.TrimSpace(string(out))})
		return
	}
	if lerr := loadModels(); lerr != nil {
		log.Printf("loadModels tras %s %s: %v", verb, req.ID, lerr)
	}
	mu.RLock()
	p := pairs
	mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID, "pairs": p})
}

type translateReq struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	HTML  bool     `json:"html"` // los textos son fragmentos HTML → preservar tags (enlaces)
	Texts []string `json:"texts"`
}

func handleTranslate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req translateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	mu.RLock()
	id, ok := model[req.From+"-"+req.To]
	mu.RUnlock()
	if !ok {
		http.Error(w, "modelo no instalado: "+req.From+"-"+req.To, http.StatusBadRequest)
		return
	}

	// Solo se mandan al motor los textos no vacíos; los vacíos se devuelven igual.
	// Cada texto = una línea (los saltos internos se colapsan a espacio para que
	// N líneas de entrada = N líneas de salida).
	out := make([]string, len(req.Texts))
	var idx []int
	var lines []string
	for i, t := range req.Texts {
		clean := strings.TrimSpace(reWS.ReplaceAllString(t, " "))
		if clean == "" {
			out[i] = t
			continue
		}
		idx = append(idx, i)
		lines = append(lines, clean)
	}

	if len(lines) > 0 {
		sem <- struct{}{}
		res, err := runEngine(id, lines, req.HTML)
		<-sem
		if err != nil {
			log.Printf("engine error (%s): %v", id, err)
			http.Error(w, "engine error", http.StatusBadGateway)
			return
		}
		if len(res) != len(lines) {
			log.Printf("desalineado: in=%d out=%d (modelo %s)", len(lines), len(res), id)
			http.Error(w, "engine output mismatch", http.StatusBadGateway)
			return
		}
		for j, i := range idx {
			out[i] = res[j]
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"texts": out})
}

// runEngine traduce las líneas. En modo texto (html=false) va por líneas
// (N entra = N sale). En modo html envuelve cada fragmento en un marcador
// <p data-tw="i"> y parsea la salida por ese marcador — así NO depende del
// conteo de líneas (un <br> o bloque interno partiría la salida y desalinearía).
func runEngine(modelID string, lines []string, html bool) ([]string, error) {
	if html {
		return runEngineHTML(modelID, lines)
	}
	cmd := exec.Command(bin, "-m", modelID)
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var res []string
	sc := bufio.NewScanner(&stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		res = append(res, sc.Text())
	}
	return res, sc.Err()
}

func runEngineHTML(modelID string, lines []string) ([]string, error) {
	var sb strings.Builder
	for i, l := range lines {
		fmt.Fprintf(&sb, `<p data-tw="%d">%s</p>`, i, l)
	}
	cmd := exec.Command(bin, "-m", modelID, "--html")
	cmd.Stdin = strings.NewReader(sb.String())
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	res := make([]string, len(lines))
	found := 0
	for _, m := range reSeg.FindAllStringSubmatch(stdout.String(), -1) {
		idx, err := strconv.Atoi(m[1])
		if err != nil || idx < 0 || idx >= len(lines) || res[idx] != "" {
			continue
		}
		res[idx] = strings.TrimSpace(m[2])
		found++
	}
	if found != len(lines) {
		return nil, fmt.Errorf("html: %d/%d segmentos parseados", found, len(lines))
	}
	return res, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func main() {
	bin = env("TRANSLATE_BIN", `C:\Users\asus\nimos-tools\translate\translateLocally.exe`)
	port := env("PORT", "8091")
	bind := env("BIND", "127.0.0.1")

	conc := runtime.NumCPU() / 2
	if conc < 1 {
		conc = 1
	}
	if conc > 4 {
		conc = 4
	}
	sem = make(chan struct{}, conc)

	if err := loadModels(); err != nil {
		log.Fatalf("no se pudo listar modelos con %s: %v", bin, err)
	}
	log.Printf("translate-wrap → http://%s:%s  ·  motor: %s  ·  pares: %v  ·  concurrencia %d",
		bind, port, bin, pairs, conc)

	mux := http.NewServeMux()
	mux.HandleFunc("/languages", handleLanguages)
	mux.HandleFunc("/translate", handleTranslate)
	mux.HandleFunc("/models/available", handleModelsAvailable)
	mux.HandleFunc("/models/download", handleModelsDownload)
	mux.HandleFunc("/models/remove", handleModelsRemove)
	log.Fatal(http.ListenAndServe(bind+":"+port, mux))
}
