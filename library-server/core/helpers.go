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
