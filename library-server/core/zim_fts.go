// zim_fts.go — Full-text NATIVO (Fase C2, Milestone 2 · INDEXER.md).
//
// Detrás del mismo toggle ZIM_ENGINE=native, la búsqueda global deja de llamar
// al /search de kiwix (Xapian) y consulta el índice bleve de cada colección: un
// directorio `<nombre>.bleve/` AL LADO del .zim, con manifiesto verificado al
// abrir (fts.Open exige que uuid/entryCount/schema casen con el ZIM — FTS-AUDIT
// BUG-1). Sin índice, la colección sigue buscándose por título/suggest — regla
// de los tres pisos (INDEXER.md §0): el FTS es una capacidad que aparece, no un
// requisito.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/andresgv-beep/zim-engine/fts"
	"github.com/andresgv-beep/zim-engine/zim"

	zimhtml "golang.org/x/net/html"
)

// ftsDirFor: convención del pool — el índice vive junto al .zim, mismo nombre,
// extensión .bleve. Es la forma en que un índice precocinado en el PC se copia a
// la Pi (INDEXER.md §2): dos ficheros al lado, cero configuración.
func ftsDirFor(zimPath string) string {
	return strings.TrimSuffix(zimPath, filepath.Ext(zimPath)) + ".bleve"
}

// ftsState: índice abierto de una colección. En el registro solo viven
// POSITIVOS: las negativas ("aún no hay índice", "está a medio construir",
// "el manifiesto no casa") NO se cachean — fts.Open falla BARATO en todos esos
// casos (lee el manifiesto antes de abrir bleve), y así el índice "aparece"
// solo en cuanto el job o una copia lo dejan completo en el pool, sin
// reiniciar nada. Tres pisos: el FTS es una capacidad que aparece.
type ftsState struct {
	idx *fts.Index
}

// ftsFor devuelve el índice FTS de una colección, abriéndolo (y verificándolo
// contra su ZIM) la primera vez. nil = no hay índice → piso 1: solo título.
func (n *nativeZims) ftsFor(id string) *fts.Index {
	n.ftsMu.Lock()
	if st, ok := n.fts[id]; ok {
		n.ftsMu.Unlock()
		return st.idx
	}
	n.ftsMu.Unlock()

	// Abrir fuera del mutex: get() tiene el suyo y fts.Open puede tardar ms.
	na, err := n.get(id)
	if err != nil {
		return nil // colección no resoluble: que lo reporte el camino de contenido
	}
	dir := ftsDirFor(na.path)
	idx, err := fts.Open(dir, na.arc)
	if err != nil {
		// Sin dir o sin manifiesto = sin índice (o build en curso): silencio.
		// Manifiesto presente pero inválido = problema real: se loguea, pero
		// solo cuando el motivo cambia (nada de un log por tecleo).
		if !errors.Is(err, os.ErrNotExist) {
			n.ftsMu.Lock()
			if n.ftsErr[id] != err.Error() {
				n.ftsErr[id] = err.Error()
				log.Printf("fts: índice de %s descartado: %v", id, err)
			}
			n.ftsMu.Unlock()
		}
		return nil
	}

	n.ftsMu.Lock()
	defer n.ftsMu.Unlock()
	if prev, ok := n.fts[id]; ok { // carrera benigna: otro goroutine llegó antes
		if idx != prev.idx {
			idx.Close()
		}
		return prev.idx
	}
	n.fts[id] = ftsState{idx: idx}
	delete(n.ftsErr, id)
	if dc, derr := idx.DocCount(); derr == nil {
		log.Printf("fts: índice de %s abierto (%d docs, %s)", id, dc, filepath.Base(dir))
	}
	return idx
}

// closeFTS cierra los índices abiertos (lo llama invalidate, junto a los archives).
func (n *nativeZims) closeFTS() {
	n.ftsMu.Lock()
	old := n.fts
	n.fts = make(map[string]ftsState)
	n.ftsErr = make(map[string]string)
	n.ftsMu.Unlock()
	for _, st := range old {
		if st.idx != nil {
			st.idx.Close()
		}
	}
}

// nativeSearchHits: los hits full-text de una colección para la búsqueda global,
// más el total de coincidencias (el "Total" del grupo). Path sale en la MISMA
// forma pública que el suggest (zimEntryPath): sin namespace en el esquema
// moderno, "A/…" en el legacy.
func (s *Server) nativeSearchHits(libID, q string, limit int) ([]SearchHit, int) {
	idx := s.zimNative.ftsFor(libID)
	if idx == nil {
		return nil, 0
	}
	hits, total, err := idx.Search(q, limit)
	if err != nil {
		log.Printf("fts: búsqueda en %s: %v", libID, err)
		return nil, 0
	}
	out := make([]SearchHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, SearchHit{
			Title:   h.Title,
			Path:    ftsPublicPath(h.Path),
			Snippet: ftsSnippet(h.Snippet),
		})
	}
	return out, int(total)
}

// ftsPublicPath convierte el FullPath del índice ("C/Saturno", "A/Богъ") a la
// forma pública de la URL (la de zimEntryPath).
func ftsPublicPath(full string) string {
	if len(full) > 2 && full[1] == '/' && full[0] == 'C' {
		return full[2:]
	}
	return full // legacy: el namespace viaja en la URL
}

// ftsSnippet limpia el fragmento resaltado de bleve (<mark>…</mark> + HTML
// escapado) a texto plano, que es lo que espera el pipeline de snippets del
// shim. Con StoreBody=false (el default de producción) no hay fragmento: el
// snippet lo rellena fillMissingPreviews leyendo el artículo del motor.
func ftsSnippet(frag string) string {
	if frag == "" {
		return ""
	}
	frag = strings.NewReplacer("<mark>", "", "</mark>", "", "…", " ").Replace(frag)
	return stripSnippetNoise(frag)
}

// ── Previews sin kiwix ──────────────────────────────────────────────────────

// articleDoc: un artículo parseado a DOM, del motor nativo o de kiwix según el
// toggle. Es la costura que deja articlePreview (search.go) y firstImage
// (images.go) sin saber de dónde vienen los bytes. Devuelve también la ruta
// FINAL tras redirects, para que las imágenes relativas resuelvan contra la URL
// del artículo de destino.
func (s *Server) articleDoc(ctx context.Context, lib, path string) (*zimhtml.Node, string, error) {
	if s.zimNative != nil {
		return s.nativeArticleDoc(ctx, lib, path)
	}
	resp, err := s.kget(s.contentURL(lib, path))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("artículo %s/%s: status %d", lib, path, resp.StatusCode)
	}
	doc, err := zimhtml.Parse(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, "", err
	}
	final := path
	if resp.Request != nil && resp.Request.URL != nil {
		if fp := strings.TrimPrefix(resp.Request.URL.Path, "/content/"+lib+"/"); fp != "" {
			if dec, derr := url.PathUnescape(fp); derr == nil {
				fp = dec
			}
			final = fp
		}
	}
	return doc, final, nil
}

// nativeArticleDoc lee el artículo directamente del archive. Sigue los redirects
// del ZIM con tope (§14), igual que el camino HTTP seguía los 302 de kiwix.
func (s *Server) nativeArticleDoc(ctx context.Context, lib, path string) (*zimhtml.Node, string, error) {
	na, err := s.zimNative.get(lib)
	if err != nil {
		return nil, "", err
	}
	e, err := zimResolveEntry(na.arc, path)
	if err != nil {
		return nil, "", err
	}
	final := path
	for i := 0; i < 4 && e.IsRedirect(); i++ {
		tgt, ok := e.RedirectTarget()
		if !ok {
			return nil, "", zim.ErrCorrupt
		}
		if e, err = na.arc.EntryAt(tgt); err != nil {
			return nil, "", err
		}
		final = zimEntryPath(tgt)
	}
	rc, _, err := e.Open(ctx)
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()
	doc, err := zimhtml.Parse(io.LimitReader(rc, 1024*1024))
	if err != nil {
		return nil, "", err
	}
	return doc, final, nil
}
