// upload.go — carga manual de contenido al pool (sustituto de los motores de
// descarga). El admin sube UN fichero + metadatos desde el Panel; guardamos el
// fichero bajo la carpeta de su app (Moments/ o Cabinet/) y escribimos el mismo
// sidecar `<fichero>.json` que consume el escáner (media.go). Sin red.
//
//	POST /api/admin/upload  (multipart, admin) → guarda + ficha → 200 {ok, item}
//
// Reusa el andamiaje existente: templateForExt (tipo por extensión),
// writeJSONFileIfAbsent (atómico), sanitizeFilename, collectionMeta.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// uploadDeps: raíz del pool (misma que descargas/media). Se inyecta desde main.
type uploadDeps struct {
	root string
}

// appDirFor mapea el carril de la app a su carpeta en el pool. Las superficies
// del lector (Moments/Cabinet) y brand.js esperan estos prefijos exactos.
func appDirFor(source string) (dir string, ok bool) {
	switch source {
	case "moments":
		return "Moments", true
	case "cabinet":
		return "Cabinet", true
	}
	return "", false
}

// dest ancla root/<app>/<colección>/<fichero> dentro del pool (defensa en
// profundidad frente a "..").
func (u *uploadDeps) dest(appDir, collection, filename string) (string, error) {
	if u.root == "" {
		return "", fmt.Errorf("DOWNLOAD_ROOT no configurado")
	}
	full := filepath.Clean(filepath.Join(u.root, appDir, collection, filename))
	root := strings.TrimRight(u.root, string(filepath.Separator))
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fmt.Errorf("destino fuera del pool")
	}
	return full, nil
}

// resolve ancla una ruta relativa (id del item = ruta del sidecar) dentro del pool.
func (u *uploadDeps) resolve(rel string) (string, error) {
	if u.root == "" {
		return "", fmt.Errorf("DOWNLOAD_ROOT no configurado")
	}
	full := filepath.Clean(filepath.Join(u.root, filepath.FromSlash(rel)))
	root := strings.TrimRight(u.root, string(filepath.Separator))
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fmt.Errorf("ruta fuera del pool")
	}
	return full, nil
}

// handleMediaUpdate edita la ficha de un item ya en el pool (metadatos + imágenes
// + visibilidad). El fichero de media NO se cambia (para eso, borrar y re-subir).
func (s *Server) handleMediaUpdate(u *uploadDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
			return
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "formulario inválido"})
			return
		}
		id := strings.TrimSpace(r.FormValue("id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falta id"})
			return
		}
		scPath, err := u.resolve(id)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		raw, err := os.ReadFile(scPath)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item no encontrado"})
			return
		}
		var sc sidecar
		if json.Unmarshal(raw, &sc) != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ficha ilegible"})
			return
		}

		// Metadatos (se pisan con lo que venga; título vacío conserva el actual).
		if v := strings.TrimSpace(r.FormValue("title")); v != "" {
			sc.Title = v
		}
		sc.Author = strings.TrimSpace(r.FormValue("author"))
		sc.Description = strings.TrimSpace(r.FormValue("description"))
		tags := splitTags(r.FormValue("tags"))
		sc.Tags = tags
		sc.Keywords = keywordsFromSubjects(tags)
		sc.Date = strings.TrimSpace(r.FormValue("date"))
		sc.Language = strings.TrimSpace(r.FormValue("language"))
		sc.Contributor = strings.TrimSpace(r.FormValue("contributor"))
		sc.License = strings.TrimSpace(r.FormValue("license"))
		if d, derr := strconv.Atoi(strings.TrimSpace(r.FormValue("duration"))); derr == nil && d > 0 {
			sc.Duration = d
		} else if r.FormValue("duration") != "" {
			sc.Duration = 0
		}

		dir := filepath.Dir(scPath)
		// Nuevas imágenes (opcionales): reemplazan las existentes.
		if cf, ch, cerr := r.FormFile("cover"); cerr == nil {
			base := strings.TrimSuffix(sc.Media, filepath.Ext(sc.Media))
			name := base + ".cover" + strings.ToLower(filepath.Ext(sanitizeFilename(ch.Filename)))
			if name == base+".cover" {
				name = base + ".cover.jpg"
			}
			dst := filepath.Join(dir, name)
			_ = os.Remove(dst)
			if saveMultipart(cf, dst) == nil {
				sc.Cover = name
			}
			cf.Close()
		}
		if af, _, aerr := r.FormFile("channel_avatar"); aerr == nil {
			dst := filepath.Join(dir, "channel.jpg")
			_ = os.Remove(dst)
			if saveMultipart(af, dst) == nil {
				sc.ChannelAvatar = "channel.jpg"
			}
			af.Close()
		}

		if err := writeJSONFileOverwrite(scPath, sc); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo guardar la ficha"})
			return
		}

		// Visibilidad (opcional): pisa el nivel de la colección.
		if s.store != nil && s.store.db != nil {
			if access := strings.TrimSpace(r.FormValue("access")); validAccess(access) {
				cid := collectionIDForMedia(strings.Trim(filepath.ToSlash(strings.TrimPrefix(dir, u.root)), "/"))
				_, _ = s.store.db.Exec(`
					INSERT INTO collection_access (collection_id, access, min_age, allow_download, updated) VALUES (?,?,0,0,?)
					ON CONFLICT(collection_id) DO UPDATE SET access=excluded.access, updated=excluded.updated`,
					cid, access, time.Now().Unix())
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "title": sc.Title})
	}
}

// writeJSONFileOverwrite escribe v como JSON indentado, PISANDO el fichero (atómico).
func writeJSONFileOverwrite(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func (s *Server) handleUpload(u *uploadDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "método no permitido"})
			return
		}
		// Hasta 2 GB por subida (el grueso se streamea a disco temporal; solo 32 MB
		// en memoria). El vídeo casero cabe de sobra.
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "formulario inválido: " + err.Error()})
			return
		}

		source := strings.TrimSpace(r.FormValue("source"))
		appDir, ok := appDirFor(source)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "app inválida (moments|cabinet)"})
			return
		}
		collection := sanitizeSegment(r.FormValue("collection"))
		if collection == "" {
			collection = "General"
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "falta el fichero"})
			return
		}
		defer file.Close()

		filename := sanitizeFilename(header.Filename)
		if filename == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nombre de fichero inválido"})
			return
		}
		template, collType := templateForExt(filename)
		if template == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tipo de fichero no soportado"})
			return
		}

		dst, err := u.dest(appDir, collection, filename)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if _, err := os.Stat(dst); err == nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "ya existe un fichero con ese nombre en la colección"})
			return
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo crear la carpeta"})
			return
		}
		if err := saveMultipart(file, dst); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo guardar el fichero: " + err.Error()})
			return
		}

		// Portada opcional → <base>.cover.jpg junto al media.
		cover := ""
		if cf, ch, cerr := r.FormFile("cover"); cerr == nil {
			base := strings.TrimSuffix(filename, filepath.Ext(filename))
			coverName := base + ".cover" + strings.ToLower(filepath.Ext(sanitizeFilename(ch.Filename)))
			if coverName == base+".cover" {
				coverName = base + ".cover.jpg"
			}
			if cdst, derr := u.dest(appDir, collection, coverName); derr == nil {
				if saveMultipart(cf, cdst) == nil {
					cover = coverName
				}
			}
			cf.Close()
		}

		// Logo del canal / autor (carril vídeo) → channel.jpg de la carpeta, como el
		// patrón antiguo. Es distinto de la miniatura del vídeo (cover).
		channelAvatar := ""
		if af, _, aerr := r.FormFile("channel_avatar"); aerr == nil {
			if adst, derr := u.dest(appDir, collection, "channel.jpg"); derr == nil {
				_ = os.Remove(adst) // el admin lo reemplaza a voluntad
				if saveMultipart(af, adst) == nil {
					channelAvatar = "channel.jpg"
				}
			}
			af.Close()
		}

		tags := splitTags(r.FormValue("tags"))
		duration := 0
		if d, derr := strconv.Atoi(strings.TrimSpace(r.FormValue("duration"))); derr == nil && d > 0 {
			duration = d
		}
		sc := sidecar{
			Template:    template,
			Media:       filename,
			Title:       firstNonEmpty(strings.TrimSpace(r.FormValue("title")), titleFromFilename(filename)),
			Author:      strings.TrimSpace(r.FormValue("author")),
			Date:        strings.TrimSpace(r.FormValue("date")),
			Description: strings.TrimSpace(r.FormValue("description")),
			Tags:        tags,
			Keywords:    keywordsFromSubjects(tags),
			Source:      source,
			SourceURL:   strings.TrimSpace(r.FormValue("source_url")),
			Language:    strings.TrimSpace(r.FormValue("language")),
			Contributor: strings.TrimSpace(r.FormValue("contributor")),
			License:     strings.TrimSpace(r.FormValue("license")),
			Cover:         cover,
			ChannelAvatar: channelAvatar,
			Duration:      duration,
		}
		scPath := sidecarPathFor(dst)
		if _, err := writeJSONFileIfAbsent(scPath, sc); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo escribir la ficha: " + err.Error()})
			return
		}

		// collection.json de la carpeta, si aún no existe.
		collPath := filepath.Join(filepath.Dir(dst), "collection.json")
		_, _ = writeJSONFileIfAbsent(collPath, collectionMeta{
			Type:     collType,
			Template: template,
			Title:    collection,
			Source:   source,
		})

		// Visibilidad: la colección nueva nace con el nivel elegido (por defecto
		// ABIERTA, para que lo subido se vea sin pasar por Colecciones). Si la
		// colección ya tenía nivel (subidas previas), se respeta (DO NOTHING).
		if s.store != nil && s.store.db != nil {
			access := strings.TrimSpace(r.FormValue("access"))
			if !validAccess(access) {
				access = "open"
			}
			cid := collectionIDForMedia(appDir + "/" + collection)
			_, _ = s.store.db.Exec(`
				INSERT INTO collection_access (collection_id, access, min_age, allow_download, updated) VALUES (?,?,0,0,?)
				ON CONFLICT(collection_id) DO NOTHING`,
				cid, access, time.Now().Unix())
		}

		rel := strings.TrimPrefix(filepath.ToSlash(strings.TrimPrefix(dst, u.root)), "/")
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"collection": appDir + "/" + collection,
			"media":      rel,
			"title":      sc.Title,
			"template":   template,
		})
	}
}

// saveMultipart vuelca el fichero subido a dst de forma atómica (.part + rename).
func saveMultipart(src io.Reader, dst string) error {
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// sanitizeSegment limpia un nombre de carpeta (colección): sin separadores ni "..".
func sanitizeSegment(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", " ")
	name = strings.ReplaceAll(name, "\\", " ")
	name = strings.ReplaceAll(name, "..", "")
	name = strings.Trim(name, ". ")
	return strings.Join(strings.Fields(name), " ")
}

// splitTags parte "a, b, c" en tags limpios, sin vacíos ni duplicados.
func splitTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	return out
}
