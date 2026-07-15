// admin_zim_native.go — Alta/baja de ZIM en library.xml SIN kiwix-manage.
//
// Última pieza de la retirada de kiwix (§8): el registro deja de orquestar un
// binario externo. El servidor abre el .zim con el motor propio (UUID + M/*),
// y edita library.xml directamente:
//
//   - El formato se mantiene COMPATIBLE con kiwix (root <library version=…>,
//     book id = UUID en 8-4-4-4-12, path relativo al xml): el rollback
//     ZIM_ENGINE=kiwix puede seguir leyendo el fichero. Los atributos extra de
//     books existentes (favicon, tags…) se PRESERVAN tal cual al reescribir.
//   - Escritura atómica (tmp + rename): un corte no deja un xml a medias.
//   - library.xml sigue siendo la fuente de verdad del registro (el paso a
//     SQLite propio queda para el gate final §8, si llega a hacer falta).
package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andresgv-beep/zim-engine/zim"
)

// rawLibrary/rawBook: representación SIN pérdidas de library.xml — los books son
// elementos de solo-atributos y se conservan todos, conozcamos su nombre o no.
type rawLibrary struct {
	XMLName xml.Name   `xml:"library"`
	Attrs   []xml.Attr `xml:",any,attr"`
	Books   []rawBook  `xml:"book"`
}

type rawBook struct {
	XMLName xml.Name   `xml:"book"`
	Attrs   []xml.Attr `xml:",any,attr"`
}

func (b rawBook) attr(name string) string {
	for _, a := range b.Attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func readRawLibrary(path string) (*rawLibrary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Biblioteca nueva: root con la versión que escribe kiwix, por
			// compatibilidad byte-a-byte de formato.
			return &rawLibrary{Attrs: []xml.Attr{
				{Name: xml.Name{Local: "version"}, Value: "20110515"},
			}}, nil
		}
		return nil, err
	}
	var lib rawLibrary
	if err := xml.Unmarshal(data, &lib); err != nil {
		return nil, fmt.Errorf("library.xml corrupto: %w", err)
	}
	return &lib, nil
}

// writeRawLibrary reescribe library.xml de forma atómica.
func writeRawLibrary(path string, lib *rawLibrary) error {
	out, err := xml.MarshalIndent(lib, "", "  ")
	if err != nil {
		return err
	}
	data := append([]byte(xml.Header), out...)
	data = append(data, '\n')
	tmp := fmt.Sprintf("%s.tmp-%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	os.Remove(path) // Windows: rename no pisa
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// zimUUIDString: el UUID del header en el formato 8-4-4-4-12 que kiwix usa como
// book id (verificado contra library.xml reales).
func zimUUIDString(u [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// addBook registra un .zim del pool en library.xml leyendo su identidad y
// metadata con el motor propio. Idempotente por UUID y por path: registrar dos
// veces no duplica. Devuelve el book id.
func (a *adminZim) addBook(file string) (string, error) {
	abs := filepath.Join(a.zimDir, file)

	arc, err := zim.Open(context.Background(), abs, nil)
	if err != nil {
		return "", fmt.Errorf("no es un ZIM válido: %w", err)
	}
	id := zimUUIDString(arc.UUID())
	meta := func(name string) string { v, _ := arc.Metadata(name); return v }
	attrs := []xml.Attr{
		{Name: xml.Name{Local: "id"}, Value: id},
		{Name: xml.Name{Local: "path"}, Value: file}, // relativo al xml, como kiwix-manage
	}
	for _, m := range []struct{ attrName, metaName string }{
		{"title", "Title"},
		{"description", "Description"},
		{"language", "Language"},
		{"creator", "Creator"},
		{"publisher", "Publisher"},
		{"name", "Name"},
		{"date", "Date"},
		{"tags", "Tags"},
	} {
		if v := meta(m.metaName); v != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: m.attrName}, Value: v})
		}
	}
	arc.Close() // solo hacía falta para identidad+metadata; el registro lo reabre al servir

	lib, err := readRawLibrary(a.libraryXML)
	if err != nil {
		return "", err
	}
	for _, b := range lib.Books {
		if b.attr("id") == id || strings.EqualFold(filepath.Base(b.attr("path")), file) {
			return id, nil // ya registrado: idempotente
		}
	}
	lib.Books = append(lib.Books, rawBook{Attrs: attrs})
	if err := writeRawLibrary(a.libraryXML, lib); err != nil {
		return "", err
	}
	return id, nil
}

// removeBook desregistra por book id. El fichero .zim del pool NO se toca.
func (a *adminZim) removeBook(id string) error {
	lib, err := readRawLibrary(a.libraryXML)
	if err != nil {
		return err
	}
	kept := lib.Books[:0]
	found := false
	for _, b := range lib.Books {
		if b.attr("id") == id {
			found = true
			continue
		}
		kept = append(kept, b)
	}
	if !found {
		return fmt.Errorf("id no está en la biblioteca")
	}
	lib.Books = kept
	return writeRawLibrary(a.libraryXML, lib)
}
