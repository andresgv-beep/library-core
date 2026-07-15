// sidecar.go — generador de metadatos post-descarga.
//
// Cuando un job del download.Manager llega a `done`, este módulo escribe, junto
// al fichero recién bajado, una ficha `<fichero>.json` (schema CONTENT-TEMPLATES
// §6, generalizado) para que Library pueda MOSTRARLO offline con su plantilla:
// vídeo→video, pdf→visor, imagen→galería. Si la carpeta aún no tiene
// `collection.json`, también lo crea (CONTENT-TEMPLATES §2: "la carpeta ES la
// colección").
//
// Las plantillas se nombran por TIPO de contenido (video, pdf, gallery, audio,
// reader), NUNCA por superficie. La ficha .json es la fuente de la verdad y la
// rellena el usuario (carga manual desde el Panel); este módulo solo siembra un
// esqueleto best-effort a partir del nombre del fichero.
//
// El download.Manager es genérico y no sabe de plantillas (DOWNLOADS-CONTRACT
// §1); por eso esto vive en el shim y se engancha vía su callback onEvent.
// Nunca pisa un JSON existente: el usuario es dueño del render, el JSON es la
// fuente de la verdad.
package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/andresgv-beep/nimos-library/download"
)

// sidecarWriter escribe fichas post-descarga. root es DOWNLOAD_ROOT
// (informativo: el dest_path del job ya viene anclado y validado por el handler).
type sidecarWriter struct {
	root string
}

// sidecar = ficha por item (CONTENT-TEMPLATES §6). Campos comunes a todas las
// plantillas; los específicos de vídeo (duration, chapters, subtitles) los
// aporta el usuario en la carga manual (o, más adelante, ffprobe §8, §10).
type sidecar struct {
	Template    string         `json:"template"`
	Title       string         `json:"title"`
	Media       string         `json:"media"` // nombre del fichero, junto a este .json
	Author      string         `json:"author,omitempty"`
	Date        string         `json:"date,omitempty"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`        // visibles (chips filtrables)
	Keywords    []string       `json:"keywords,omitempty"`    // invisibles (combustible del buscador)
	Source      string         `json:"source"`                // carril: "manual" | …
	SourceID    string         `json:"source_id,omitempty"`   // identificador de procedencia
	SourceURL   string         `json:"source_url,omitempty"`  // ficha original (procedencia)
	Language    string         `json:"language,omitempty"`    // idioma legible (English, Español…)
	Contributor string         `json:"contributor,omitempty"` // p. ej. "Biblioteca Pública"
	License     string         `json:"license,omitempty"`     // etiqueta humana: "CC BY 3.0"
	Cover       string         `json:"cover,omitempty"`       // portada local (<base>.cover.jpg, junto al media)
	Text        string         `json:"text,omitempty"`        // texto OCR completo local (<base>.txt, junto al media)
	Tracks      []sidecarTrack `json:"tracks,omitempty"`      // pistas locales de un audiolibro
	// Campos de vídeo. Aditivos y omitempty: los demás tipos no los usan.
	Duration      int              `json:"duration,omitempty"`       // segundos
	Subtitles     []sidecarSub     `json:"subtitles,omitempty"`      // pistas .vtt locales
	Chapters      []sidecarChapter `json:"chapters,omitempty"`       // marcadores de tiempo
	ChannelAvatar string           `json:"channel_avatar,omitempty"` // imagen del canal/autor (channel.jpg, en la carpeta)
}

// sidecarSub = una pista de subtítulos local (fichero .vtt junto al media).
type sidecarSub struct {
	Lang string `json:"lang"`
	File string `json:"file"`
}

// sidecarChapter = un marcador de capítulo (segundo de inicio + título).
type sidecarChapter struct {
	Start float64 `json:"start"`
	Title string  `json:"title"`
}

// sidecarTrack = una pista de audiolibro local (nombres de fichero relativos a
// la carpeta del sidecar).
type sidecarTrack struct {
	Title    string `json:"title"`
	Media    string `json:"media"`
	Waveform string `json:"waveform,omitempty"`
}

// collectionMeta declara el tipo/plantilla/título de una carpeta-colección
// (CONTENT-TEMPLATES §2).
type collectionMeta struct {
	Type        string `json:"type"`
	Template    string `json:"template"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	SourceID    string `json:"source_id,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
}

// onJobEvent es el callback que se pasa a download.NewManager. El Manager lo
// llama en cada emit (progreso incluido); solo actuamos cuando el job llega a
// `done`, y escribimos en una goroutine para no bloquear el emit del Manager.
func (sw *sidecarWriter) onJobEvent(job download.Job) {
	if job.Status != download.StatusDone {
		return
	}
	go sw.writeForJob(job)
}

// writeForJob genera la ficha del fichero recién bajado y, si procede, el
// collection.json de su carpeta. Best-effort: los errores se loguean, no rompen
// la descarga (que ya terminó bien). La ficha nace mínima (título derivado del
// nombre); el usuario la completa desde el Panel (carga manual).
func (sw *sidecarWriter) writeForJob(job download.Job) {
	template, collType := templateForExt(job.DestPath)

	sc := sidecar{
		Template: template,
		Media:    filepath.Base(job.DestPath),
		Title:    titleFromFilename(job.DestPath),
		Source:   sourceLabel(job.OwnerKind),
	}

	scPath := sidecarPathFor(job.DestPath)
	if written, err := writeJSONFileIfAbsent(scPath, sc); err != nil {
		log.Printf("sidecar: no se pudo escribir %s: %v", scPath, err)
	} else if written {
		log.Printf("sidecar: %s", scPath)
	}

	// collection.json de la carpeta, si aún no existe y sabemos el tipo.
	if collType != "" {
		dir := filepath.Dir(job.DestPath)
		collPath := filepath.Join(dir, "collection.json")
		coll := collectionMeta{
			Type:     collType,
			Template: template,
			Title:    filepath.Base(dir),
			Source:   sc.Source,
		}
		if _, err := writeJSONFileIfAbsent(collPath, coll); err != nil {
			log.Printf("sidecar: no se pudo escribir %s: %v", collPath, err)
		}
	}
}

// templateForExt elige plantilla y tipo de colección por la extensión REAL del
// fichero (decisión Andrés: más preciso que fiarse de un mediatype declarado).
// Devuelve ("","") si la extensión no la reproduce ninguna plantilla; en ese
// caso se escribe igualmente la ficha (metadatos útiles) pero sin collection.json.
func templateForExt(path string) (template, collType string) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".webm", ".m4v", ".mov":
		return "video", "video"
	case ".mp3", ".ogg", ".oga", ".flac", ".m4a", ".wav":
		return "audio", "audio"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return "gallery", "images"
	case ".pdf":
		return "pdf", "pdf"
	case ".epub":
		return "reader", "documents"
	case ".md", ".markdown", ".txt":
		return "reader", "documents"
	}
	return "", ""
}

// sourceLabel mapea el owner_kind del job al carril de origen del sidecar.
func sourceLabel(ownerKind string) string {
	if ownerKind == "" {
		return "manual"
	}
	return ownerKind
}

// sidecarPathFor devuelve la ruta de la ficha: mismo nombre, extensión .json
// (CONTENT-TEMPLATES §2: cosmos-ep1.mp4 → cosmos-ep1.json).
func sidecarPathFor(dest string) string {
	ext := filepath.Ext(dest)
	return strings.TrimSuffix(dest, ext) + ".json"
}

// titleFromFilename deriva un título legible del nombre de fichero: quita la
// extensión y convierte separadores en espacios. Best-effort; el usuario lo
// pisa desde el Panel con el título real.
func titleFromFilename(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.NewReplacer("_", " ", "-", " ").Replace(name)
	return strings.Join(strings.Fields(name), " ")
}

// keywordsFromSubjects normaliza los tags a keywords (minúsculas, sin
// duplicados) como combustible del buscador FTS5 (§7).
func keywordsFromSubjects(subjects []string) []string {
	if len(subjects) == 0 {
		return nil
	}
	out := make([]string, 0, len(subjects))
	seen := map[string]bool{}
	for _, s := range subjects {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

// writeJSONFileIfAbsent escribe v como JSON indentado en path, SOLO si el
// fichero no existe ya (no pisa ediciones del usuario: el JSON es su fuente de
// la verdad). Escritura atómica: fichero temporal + rename. Devuelve written=true
// si de verdad escribió (false si ya existía).
func writeJSONFileIfAbsent(path string, v any) (written bool, err error) {
	if _, statErr := os.Stat(path); statErr == nil {
		return false, nil // ya existe: no tocar
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return false, err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return false, err
	}
	return true, nil
}
