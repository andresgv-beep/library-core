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
