package main

// Lector mínimo PMTiles v3 para la indexación local. Implementa solamente
// cabecera, directorios e IDs Hilbert; no incluye servidores ni backends cloud.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"sort"
)

const pmHeaderLen = 127
const (
	pmNone = 1
	pmGzip = 2
	pmMVT  = 1
)

type pmHeader struct {
	rootOffset, rootLength, leafOffset, tileOffset uint64
	internalCompression, tileCompression, tileType uint8
	maxZoom                                        uint8
}

type pmEntry struct {
	tileID, offset uint64
	length, run    uint32
}

func readPMHeader(d []byte) (pmHeader, error) {
	var h pmHeader
	if len(d) < pmHeaderLen || string(d[:7]) != "PMTiles" || d[7] != 3 {
		return h, fmt.Errorf("archivo PMTiles v3 no valido")
	}
	h.rootOffset = binary.LittleEndian.Uint64(d[8:16])
	h.rootLength = binary.LittleEndian.Uint64(d[16:24])
	h.leafOffset = binary.LittleEndian.Uint64(d[40:48])
	h.tileOffset = binary.LittleEndian.Uint64(d[56:64])
	h.internalCompression = d[97]
	h.tileCompression = d[98]
	h.tileType = d[99]
	h.maxZoom = d[101]
	return h, nil
}

func readPMDirectory(r io.ReaderAt, offset, length uint64, compression uint8) ([]pmEntry, error) {
	data := make([]byte, length)
	if _, err := r.ReadAt(data, int64(offset)); err != nil {
		return nil, err
	}
	var source io.Reader = bytes.NewReader(data)
	if compression == pmGzip {
		gz, err := gzip.NewReader(source)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		source = gz
	} else if compression != pmNone {
		return nil, fmt.Errorf("compresion de directorio PMTiles no compatible")
	}
	br := bufio.NewReader(source)
	count, err := binary.ReadUvarint(br)
	if err != nil || count > 20_000_000 {
		return nil, fmt.Errorf("directorio PMTiles no valido")
	}
	entries := make([]pmEntry, count)
	var last uint64
	for i := range entries {
		delta, err := binary.ReadUvarint(br)
		if err != nil {
			return nil, err
		}
		last += delta
		entries[i].tileID = last
	}
	for i := range entries {
		v, err := binary.ReadUvarint(br)
		if err != nil {
			return nil, err
		}
		entries[i].run = uint32(v)
	}
	for i := range entries {
		v, err := binary.ReadUvarint(br)
		if err != nil || v > uint64(^uint32(0)) {
			return nil, fmt.Errorf("longitud de tesela no valida")
		}
		entries[i].length = uint32(v)
	}
	for i := range entries {
		v, err := binary.ReadUvarint(br)
		if err != nil {
			return nil, err
		}
		if i > 0 && v == 0 {
			entries[i].offset = entries[i-1].offset + uint64(entries[i-1].length)
		} else {
			if v == 0 {
				return nil, fmt.Errorf("offset PMTiles no valido")
			}
			entries[i].offset = v - 1
		}
	}
	return entries, nil
}

func iteratePMEntries(r io.ReaderAt, h pmHeader, operation func(pmEntry) error) error {
	var walk func(uint64, uint64, int) error
	walk = func(offset, length uint64, depth int) error {
		if depth > 4 {
			return fmt.Errorf("directorio PMTiles demasiado profundo")
		}
		entries, err := readPMDirectory(r, offset, length, h.internalCompression)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.run > 0 {
				if err := operation(entry); err != nil {
					return err
				}
			} else if err := walk(h.leafOffset+entry.offset, uint64(entry.length), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(h.rootOffset, h.rootLength, 0)
}

// readPMTile busca y devuelve una tesela descomprimida. Mantener esta lectura
// en Core evita que cada WebView/navegador tenga que resolver rangos dentro de
// archivos PMTiles de varios GB.
func readPMTile(r io.ReaderAt, h pmHeader, z uint8, x, y uint32) ([]byte, bool, error) {
	if z > h.maxZoom || x >= uint32(1)<<z || y >= uint32(1)<<z {
		return nil, false, nil
	}
	id := pmZxyToID(z, x, y)
	offset, length := h.rootOffset, h.rootLength
	for depth := 0; depth < 5; depth++ {
		entries, err := readPMDirectory(r, offset, length, h.internalCompression)
		if err != nil {
			return nil, false, err
		}
		i := sort.Search(len(entries), func(i int) bool { return entries[i].tileID > id }) - 1
		if i < 0 {
			return nil, false, nil
		}
		entry := entries[i]
		if entry.run == 0 {
			offset, length = h.leafOffset+entry.offset, uint64(entry.length)
			continue
		}
		if id >= entry.tileID+uint64(entry.run) {
			return nil, false, nil
		}
		data := make([]byte, entry.length)
		if _, err := r.ReadAt(data, int64(h.tileOffset+entry.offset)); err != nil {
			return nil, false, err
		}
		if h.tileCompression == pmNone {
			return data, true, nil
		}
		if h.tileCompression != pmGzip {
			return nil, false, fmt.Errorf("compresion de tesela PMTiles no compatible")
		}
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, false, err
		}
		plain, err := io.ReadAll(gz)
		closeErr := gz.Close()
		if err != nil {
			return nil, false, err
		}
		if closeErr != nil {
			return nil, false, closeErr
		}
		return plain, true, nil
	}
	return nil, false, fmt.Errorf("directorio PMTiles demasiado profundo")
}

func pmRotate(n, x, y, rx, ry uint32) (uint32, uint32) {
	if ry == 0 {
		if rx != 0 {
			x, y = n-1-x, n-1-y
		}
		return y, x
	}
	return x, y
}

func pmIDToZxy(id uint64) (uint8, uint32, uint32) {
	z := uint8(bits.Len64(3*id+1)-1) / 2
	start := (uint64(1)<<(z*2) - 1) / 3
	t := id - start
	var x, y uint32
	for a := uint8(0); a < z; a++ {
		s := uint32(1) << a
		rx := 1 & (uint32(t) >> 1)
		ry := 1 & (uint32(t) ^ rx)
		x, y = pmRotate(s, x, y, rx, ry)
		x += rx << a
		y += ry << a
		t >>= 2
	}
	return z, x, y
}

func pmZxyToID(z uint8, x, y uint32) uint64 {
	if z == 0 {
		return 0
	}
	var d uint64
	for s := uint32(1) << (z - 1); s > 0; s >>= 1 {
		var rx, ry uint32
		if x&s != 0 {
			rx = 1
		}
		if y&s != 0 {
			ry = 1
		}
		d += uint64(s) * uint64(s) * uint64((3*rx)^ry)
		x, y = pmRotate(s, x, y, rx, ry)
	}
	return (uint64(1)<<(2*z)-1)/3 + d
}
