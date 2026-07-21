// Package gvas reads and writes Unreal Engine GVAS save archives.
//
// The wire format is little-endian throughout. Strings carry an int32 length
// prefix whose sign selects the encoding, and every value is self-describing
// by a type name string, which is what makes the tree dynamic.
//
// The guiding rule for this package: Encode(Decode(b)) must equal b, byte for
// byte. Anything that cannot round-trip is a bug, not a detail.
package gvas

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf16"
)

var (
	ErrUnexpectedEOF = errors.New("gvas: unexpected end of archive")
	ErrBadMagic      = errors.New("gvas: not a GVAS archive")
	ErrUnsupported   = errors.New("gvas: unsupported archive version")
)

// GUID is a 16-byte identifier stored exactly as it appears on the wire.
//
// Unreal writes each of the four 32-bit groups little-endian, so the textual
// form is a swizzle of the raw bytes rather than a straight hex dump. We keep
// the raw bytes as the source of truth and only swizzle for display, which
// means a GUID always round-trips even if we render it oddly.
type GUID [16]byte

func (g GUID) String() string {
	b := g
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%04x%08x",
		uint32(b[3])<<24|uint32(b[2])<<16|uint32(b[1])<<8|uint32(b[0]),
		uint32(b[7])<<8|uint32(b[6]),
		uint32(b[5])<<8|uint32(b[4]),
		uint32(b[11])<<8|uint32(b[10]),
		uint32(b[9])<<8|uint32(b[8]),
		uint32(b[15])<<24|uint32(b[14])<<16|uint32(b[13])<<8|uint32(b[12]),
	)
}

// IsZero reports whether this is the all-zero GUID, which Palworld uses in
// place of "no value" in several places.
func (g GUID) IsZero() bool {
	return g == GUID{}
}

// ParseGUID is the inverse of GUID.String, so callers can name an entity by the
// same text the editor displays.
func ParseGUID(s string) (GUID, error) {
	var g GUID
	var hex [32]byte
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '-' {
			continue
		}
		if n == 32 {
			return g, fmt.Errorf("gvas: %q has too many hex digits for a GUID", s)
		}
		var v byte
		switch {
		case c >= '0' && c <= '9':
			v = c - '0'
		case c >= 'a' && c <= 'f':
			v = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v = c - 'A' + 10
		default:
			return g, fmt.Errorf("gvas: %q is not a GUID", s)
		}
		hex[n] = v
		n++
	}
	if n != 32 {
		return g, fmt.Errorf("gvas: %q has %d hex digits, want 32", s, n)
	}

	// Undo the display swizzle: nibble pairs are read in textual order, then
	// each 32-bit group is byte-reversed back into wire order.
	var flat [16]byte
	for i := range flat {
		flat[i] = hex[i*2]<<4 | hex[i*2+1]
	}
	order := [16]int{3, 2, 1, 0, 7, 6, 5, 4, 11, 10, 9, 8, 15, 14, 13, 12}
	for textPos, wirePos := range order {
		g[wirePos] = flat[textPos]
	}
	return g, nil
}

// Reader walks a GVAS byte slice.
type Reader struct {
	buf []byte
	pos int
}

func NewReader(b []byte) *Reader { return &Reader{buf: b} }

func (r *Reader) Pos() int       { return r.pos }
func (r *Reader) Len() int       { return len(r.buf) }
func (r *Reader) Remaining() int { return len(r.buf) - r.pos }
func (r *Reader) EOF() bool      { return r.pos >= len(r.buf) }

func (r *Reader) take(n int) ([]byte, error) {
	if n < 0 || r.pos+n > len(r.buf) {
		return nil, fmt.Errorf("%w: wanted %d bytes at offset %d of %d",
			ErrUnexpectedEOF, n, r.pos, len(r.buf))
	}
	b := r.buf[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

func (r *Reader) Bytes(n int) ([]byte, error) {
	b, err := r.take(n)
	if err != nil {
		return nil, err
	}
	out := make([]byte, n)
	copy(out, b)
	return out, nil
}

func (r *Reader) Skip(n int) error {
	_, err := r.take(n)
	return err
}

// Rest returns everything not yet consumed.
func (r *Reader) Rest() []byte {
	out := make([]byte, len(r.buf)-r.pos)
	copy(out, r.buf[r.pos:])
	r.pos = len(r.buf)
	return out
}

func (r *Reader) U8() (uint8, error) {
	b, err := r.take(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (r *Reader) Bool() (bool, error) {
	v, err := r.U8()
	return v > 0, err
}

func (r *Reader) U16() (uint16, error) {
	b, err := r.take(2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b), nil
}

func (r *Reader) I16() (int16, error) {
	v, err := r.U16()
	return int16(v), err
}

func (r *Reader) U32() (uint32, error) {
	b, err := r.take(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (r *Reader) I32() (int32, error) {
	v, err := r.U32()
	return int32(v), err
}

func (r *Reader) U64() (uint64, error) {
	b, err := r.take(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}

func (r *Reader) I64() (int64, error) {
	v, err := r.U64()
	return int64(v), err
}

func (r *Reader) F32() (float32, error) {
	v, err := r.U32()
	return math.Float32frombits(v), err
}

func (r *Reader) F64() (float64, error) {
	v, err := r.U64()
	return math.Float64frombits(v), err
}

func (r *Reader) GUID() (GUID, error) {
	var g GUID
	b, err := r.take(16)
	if err != nil {
		return g, err
	}
	copy(g[:], b)
	return g, nil
}

// OptionalGUID reads a presence flag followed by a GUID when the flag is set.
func (r *Reader) OptionalGUID() (*GUID, error) {
	present, err := r.U8()
	if err != nil {
		return nil, err
	}
	if present == 0 {
		return nil, nil
	}
	g, err := r.GUID()
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// String reads a length-prefixed string.
//
// A negative length means UTF-16LE and counts code units; positive means
// ASCII/Latin-1 and counts bytes. Both include a trailing NUL in the count.
//
// The empty string is length 0 with no payload, which is distinct from a
// length-1 string holding just the NUL — the two must not be conflated or the
// re-encoded bytes will differ.
func (r *Reader) String() (String, error) {
	n, err := r.I32()
	if err != nil {
		return String{}, err
	}
	switch {
	case n == 0:
		return String{}, nil

	case n < 0:
		count := int(-n)
		b, err := r.take(count * 2)
		if err != nil {
			return String{}, err
		}
		units := make([]uint16, count)
		for i := 0; i < count; i++ {
			units[i] = binary.LittleEndian.Uint16(b[i*2:])
		}
		// Drop the trailing NUL unit.
		return String{Value: string(utf16.Decode(units[:count-1])), Wide: true}, nil

	default:
		b, err := r.take(int(n))
		if err != nil {
			return String{}, err
		}
		// Drop the trailing NUL byte.
		return String{Value: string(b[:n-1])}, nil
	}
}

// String is a GVAS string plus the encoding it arrived in.
//
// Carrying Wide matters for round-tripping: a pure-ASCII string that the game
// happened to store as UTF-16 must be written back as UTF-16, or the bytes
// change even though the text does not.
type String struct {
	Value string
	Wide  bool
}

func (s String) IsEmpty() bool  { return s.Value == "" && !s.Wide }
func (s String) String() string { return s.Value }

// Str builds an ASCII String, for constructing values in code.
func Str(s string) String { return String{Value: s} }

// Writer builds a GVAS byte slice.
type Writer struct {
	buf []byte
}

func NewWriter() *Writer { return &Writer{} }

func (w *Writer) Bytes() []byte { return w.buf }
func (w *Writer) Len() int      { return len(w.buf) }

func (w *Writer) Raw(b []byte) { w.buf = append(w.buf, b...) }
func (w *Writer) U8(v uint8)   { w.buf = append(w.buf, v) }
func (w *Writer) Bool(v bool) {
	if v {
		w.U8(1)
	} else {
		w.U8(0)
	}
}
func (w *Writer) U16(v uint16)  { w.buf = binary.LittleEndian.AppendUint16(w.buf, v) }
func (w *Writer) I16(v int16)   { w.U16(uint16(v)) }
func (w *Writer) U32(v uint32)  { w.buf = binary.LittleEndian.AppendUint32(w.buf, v) }
func (w *Writer) I32(v int32)   { w.U32(uint32(v)) }
func (w *Writer) U64(v uint64)  { w.buf = binary.LittleEndian.AppendUint64(w.buf, v) }
func (w *Writer) I64(v int64)   { w.U64(uint64(v)) }
func (w *Writer) F32(v float32) { w.U32(math.Float32bits(v)) }
func (w *Writer) F64(v float64) { w.U64(math.Float64bits(v)) }

func (w *Writer) GUID(g GUID) { w.Raw(g[:]) }

func (w *Writer) OptionalGUID(g *GUID) {
	if g == nil {
		w.U8(0)
		return
	}
	w.U8(1)
	w.GUID(*g)
}

func (w *Writer) String(s String) {
	if s.Value == "" && !s.Wide {
		w.I32(0)
		return
	}
	if s.Wide {
		units := utf16.Encode([]rune(s.Value))
		w.I32(-int32(len(units) + 1))
		for _, u := range units {
			w.U16(u)
		}
		w.U16(0)
		return
	}
	w.I32(int32(len(s.Value) + 1))
	w.Raw([]byte(s.Value))
	w.U8(0)
}

// Placeholder reserves space for a value patched in later, which the format
// needs because several sizes are only known after their payload is written.
type Placeholder struct {
	w      *Writer
	offset int
}

// ReserveU64 writes a zero placeholder and returns a handle to fill it in.
func (w *Writer) ReserveU64() Placeholder {
	p := Placeholder{w: w, offset: len(w.buf)}
	w.U64(0)
	return p
}

// SetU64 patches the reserved slot.
func (p Placeholder) SetU64(v uint64) {
	binary.LittleEndian.PutUint64(p.w.buf[p.offset:], v)
}

// End returns the offset just past the placeholder, which is where the payload
// it measures begins.
func (p Placeholder) End() int { return p.offset + 8 }
