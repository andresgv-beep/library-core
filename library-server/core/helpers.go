// helpers.go — utilidades pequeñas compartidas entre módulos del shim.
package main

import "strings"

// firstNonEmpty devuelve el primer valor no vacío (tras TrimSpace) de la lista.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// channelSlug normaliza el nombre de un canal/autor a un identificador seguro para
// nombre de fichero: minúsculas, alfanumérico ASCII, el resto → '-'. Así el logo
// del canal se guarda por CANAL (channel-<slug>.jpg), no por carpeta: vídeos del
// mismo canal lo comparten; canales distintos no se pisan.
func channelSlug(author string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(author)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// channelAvatarName es el nombre del fichero del logo de un canal (o "" si no hay
// autor con el que derivar un slug).
func channelAvatarName(author string) string {
	slug := channelSlug(author)
	if slug == "" {
		return ""
	}
	return "channel-" + slug + ".jpg"
}
