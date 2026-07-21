// Package oodle handles the outer container of a Palworld .sav file:
// the 12-byte header plus the compressed payload behind it.
//
// Two compression families exist. PlZ/CNK are zlib and are handled here in
// pure Go. PlM is Oodle (Kraken) and needs a native codec — see oodle_windows.go.
package oodle

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// SaveType is the byte at offset 11. The values are ASCII digits, which is why
// PlM saves report save_type 49 ('1') rather than something more obvious.
type SaveType byte

const (
	SaveTypeCNK SaveType = 48 // '0'
	SaveTypePLM SaveType = 49 // '1'
	SaveTypePLZ SaveType = 50 // '2'
)

func (s SaveType) String() string {
	switch s {
	case SaveTypeCNK:
		return "CNK"
	case SaveTypePLM:
		return "PlM"
	case SaveTypePLZ:
		return "PlZ"
	default:
		return fmt.Sprintf("unknown(0x%02X)", byte(s))
	}
}

// Valid reports whether s is a save type this package understands.
func (s SaveType) Valid() bool {
	return s == SaveTypeCNK || s == SaveTypePLM || s == SaveTypePLZ
}

// Magic returns the 3-byte magic that pairs with this save type.
func (s SaveType) Magic() []byte {
	switch s {
	case SaveTypeCNK:
		return []byte("CNK")
	case SaveTypePLM:
		return []byte("PlM")
	case SaveTypePLZ:
		return []byte("PlZ")
	default:
		return nil
	}
}

var (
	ErrTooSmall     = errors.New("oodle: file too small to contain a save header")
	ErrUnknownMagic = errors.New("oodle: unknown magic bytes")
	ErrSizeMismatch = errors.New("oodle: decompressed size does not match header")
)

// Header is the parsed outer header of a .sav file.
type Header struct {
	UncompressedLen uint32
	CompressedLen   uint32
	Magic           [3]byte
	Type            SaveType
	// DataOffset is where the compressed payload starts: 12 normally,
	// 24 for CNK, which carries a second header inline.
	DataOffset int
}

const (
	headerSize    = 12
	cnkHeaderSize = 24
)

// ParseHeader reads the outer header. It does not touch the payload.
func ParseHeader(data []byte) (Header, error) {
	var h Header
	// Only CNK needs the larger header; PlM and PlZ are valid at 12 bytes
	// plus payload, which small saves really do hit.
	if len(data) < headerSize {
		return h, ErrTooSmall
	}

	h.UncompressedLen = binary.LittleEndian.Uint32(data[0:4])
	h.CompressedLen = binary.LittleEndian.Uint32(data[4:8])
	copy(h.Magic[:], data[8:11])
	h.Type = SaveType(data[11])
	h.DataOffset = headerSize

	// CNK nests a second header immediately after the first.
	if string(h.Magic[:]) == "CNK" {
		if len(data) < cnkHeaderSize {
			return h, ErrTooSmall
		}
		h.UncompressedLen = binary.LittleEndian.Uint32(data[12:16])
		h.CompressedLen = binary.LittleEndian.Uint32(data[16:20])
		copy(h.Magic[:], data[20:23])
		h.Type = SaveType(data[23])
		h.DataOffset = cnkHeaderSize
	}

	switch string(h.Magic[:]) {
	case "PlZ", "PlM", "CNK":
	default:
		return h, fmt.Errorf("%w: %q", ErrUnknownMagic, h.Magic[:])
	}

	if got, want := len(data)-h.DataOffset, int(h.CompressedLen); got < want {
		return h, fmt.Errorf("oodle: payload truncated: have %d bytes, header claims %d", got, want)
	}
	return h, nil
}

// Payload returns the compressed bytes described by h.
func (h Header) Payload(data []byte) []byte {
	return data[h.DataOffset : h.DataOffset+int(h.CompressedLen)]
}

// BuildSav assembles a .sav from already-compressed data.
//
// Note the header always uses the 12-byte form: we never emit CNK.
func BuildSav(compressed []byte, uncompressedLen int, t SaveType) ([]byte, error) {
	magic := t.Magic()
	if magic == nil {
		return nil, fmt.Errorf("oodle: cannot build sav: %w: %v", ErrUnknownMagic, t)
	}
	out := make([]byte, 0, headerSize+len(compressed))
	out = binary.LittleEndian.AppendUint32(out, uint32(uncompressedLen))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(compressed)))
	out = append(out, magic...)
	out = append(out, byte(t))
	out = append(out, compressed...)
	return out, nil
}
