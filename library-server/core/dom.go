package main

import (
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// Helpers de DOM y URLs compartidos por search.go, images.go y catalog.go.
// (Antes vivían en article.go, retirado tras cerrar el spike de render en
// iframe: el HTML "domesticado" de /api/article quedó sin consumidores.
// Si un cliente futuro lo necesita, está en el historial de git.)

// contentURL construye la URL del motor para un artículo de una colección.
func (s *Server) contentURL(lib, p string) string {
	u := *s.kiwix
	u.Path = "/content/" + lib + "/" + p
	u.RawPath = ""
	return u.String()
}

// kget hace un GET al motor pasando por el semáforo global (KIWIX_CONCURRENCY):
// limita cuántas peticiones simultáneas recibe kiwix-serve, que en la Pi se
// satura rápido con el fan-out de la búsqueda (suggest × variantes × colecciones).
//
// CONTRATO EXACTO (que el yo del futuro no lo redescubra como bug): el slot se
// libera cuando llegan las CABECERAS de la respuesta (el defer corre al retornar
// Get), así que el streaming del body ocurre FUERA del semáforo. Es aceptable
// porque el coste real que acotamos es Xapian, que trabaja antes de emitir
// cabeceras; los bodies (snippets, thumbs) son I/O barato con los ZIMs en el
// pool BTRFS (HDD/SSD — nadie sirve 50-100GB de biblioteca desde una SD). Si
// las mediciones en la Pi dijeran lo contrario, el fix es envolver resp.Body
// en un ReadCloser que libere el slot en Close() (backlog, prioridad baja).
func (s *Server) kget(u string) (*http.Response, error) {
	s.kiwixSem <- struct{}{}
	defer func() { <-s.kiwixSem }()
	return s.http.Get(u)
}

// absResource resuelve un recurso relativo del ZIM contra la URL del artículo.
func absResource(href string, base *url.URL) string {
	if href == "" || isExternal(href) {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).EscapedPath()
}

func isExternal(href string) bool {
	l := strings.ToLower(href)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") ||
		strings.HasPrefix(l, "//") || strings.HasPrefix(l, "mailto:") ||
		strings.HasPrefix(l, "data:") || strings.HasPrefix(l, "tel:") ||
		strings.HasPrefix(l, "javascript:")
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var f func(*html.Node)
	f = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return strings.TrimSpace(b.String())
}

func urlq(s string) string { return url.QueryEscape(s) }
