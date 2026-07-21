package gvas

import (
	"bytes"
	"testing"
)

func TestPrimitiveRoundTrip(t *testing.T) {
	w := NewWriter()
	w.U8(0xAB)
	w.Bool(true)
	w.Bool(false)
	w.U16(0x1234)
	w.I16(-2)
	w.U32(0xDEADBEEF)
	w.I32(-100000)
	w.U64(0x0123456789ABCDEF)
	w.I64(-9000000000)
	w.F32(3.5)
	w.F64(-2.25)
	g := GUID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	w.GUID(g)
	w.OptionalGUID(nil)
	w.OptionalGUID(&g)

	r := NewReader(w.Bytes())
	check := func(name string, got, want any, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}

	v8, err := r.U8()
	check("U8", v8, uint8(0xAB), err)
	b1, err := r.Bool()
	check("Bool true", b1, true, err)
	b2, err := r.Bool()
	check("Bool false", b2, false, err)
	v16, err := r.U16()
	check("U16", v16, uint16(0x1234), err)
	i16, err := r.I16()
	check("I16", i16, int16(-2), err)
	v32, err := r.U32()
	check("U32", v32, uint32(0xDEADBEEF), err)
	i32, err := r.I32()
	check("I32", i32, int32(-100000), err)
	v64, err := r.U64()
	check("U64", v64, uint64(0x0123456789ABCDEF), err)
	i64, err := r.I64()
	check("I64", i64, int64(-9000000000), err)
	f32, err := r.F32()
	check("F32", f32, float32(3.5), err)
	f64, err := r.F64()
	check("F64", f64, -2.25, err)
	gg, err := r.GUID()
	check("GUID", gg, g, err)

	og, err := r.OptionalGUID()
	if err != nil {
		t.Fatalf("OptionalGUID nil: %v", err)
	}
	if og != nil {
		t.Errorf("OptionalGUID = %v, want nil", og)
	}
	og2, err := r.OptionalGUID()
	if err != nil {
		t.Fatalf("OptionalGUID set: %v", err)
	}
	if og2 == nil || *og2 != g {
		t.Errorf("OptionalGUID = %v, want %v", og2, g)
	}

	if !r.EOF() {
		t.Errorf("%d bytes left unread", r.Remaining())
	}
}

// Strings are the fiddliest primitive: the sign of the length selects the
// encoding, and empty is its own case.
func TestStringRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   String
	}{
		{"empty", String{}},
		{"ascii", Str("PalSummon_NightLady_Dark")},
		{"ascii single char", Str("x")},
		{"wide korean", String{Value: "벨라루주의 석판", Wide: true}},
		{"wide but ascii content", String{Value: "Level", Wide: true}},
		{"wide empty-ish", String{Value: "", Wide: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWriter()
			w.String(tc.in)
			encoded := w.Bytes()

			r := NewReader(encoded)
			got, err := r.String()
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if got.Value != tc.in.Value {
				t.Errorf("value = %q, want %q", got.Value, tc.in.Value)
			}
			if got.Wide != tc.in.Wide {
				t.Errorf("wide = %v, want %v", got.Wide, tc.in.Wide)
			}
			if !r.EOF() {
				t.Errorf("%d bytes left unread", r.Remaining())
			}

			// Re-encoding what we read must reproduce the same bytes.
			w2 := NewWriter()
			w2.String(got)
			if !bytes.Equal(encoded, w2.Bytes()) {
				t.Errorf("re-encode differs:\n got %v\nwant %v", w2.Bytes(), encoded)
			}
		})
	}
}

// An ASCII string stored as UTF-16 must stay UTF-16, or the bytes change even
// though the text does not.
func TestStringPreservesEncodingChoice(t *testing.T) {
	w := NewWriter()
	w.String(String{Value: "Level", Wide: true})
	wide := w.Bytes()

	w2 := NewWriter()
	w2.String(Str("Level"))
	narrow := w2.Bytes()

	if bytes.Equal(wide, narrow) {
		t.Fatal("wide and narrow encodings of the same text should differ")
	}
	if len(wide) <= len(narrow) {
		t.Errorf("wide encoding (%d bytes) should be longer than narrow (%d)", len(wide), len(narrow))
	}
}

func TestReaderRejectsOverread(t *testing.T) {
	r := NewReader([]byte{1, 2, 3})
	if _, err := r.U64(); err == nil {
		t.Error("expected an error reading 8 bytes from a 3 byte buffer")
	}
	if _, err := r.String(); err == nil {
		t.Error("expected an error reading a string from a short buffer")
	}
}

func TestGUIDStringMatchesUnrealSwizzle(t *testing.T) {
	// Wire bytes for 2b2e5a63-e8c5-4c46-95e4-189292181c4b, a pal instance id
	// observed in the live save. Each 32-bit group is stored little-endian,
	// so the bytes are not in textual order.
	g := GUID{
		0x63, 0x5a, 0x2e, 0x2b,
		0x46, 0x4c, 0xc5, 0xe8,
		0x92, 0x18, 0xe4, 0x95,
		0x4b, 0x1c, 0x18, 0x92,
	}
	const want = "2b2e5a63-e8c5-4c46-95e4-189292181c4b"
	if got := g.String(); got != want {
		t.Errorf("GUID.String() = %s, want %s", got, want)
	}
	if !(GUID{}).IsZero() {
		t.Error("zero GUID should report IsZero")
	}
	if g.IsZero() {
		t.Error("non-zero GUID should not report IsZero")
	}
}

// Parsing and formatting must be exact inverses, which also guards the
// hand-derived byte order in the test above.
func TestGUIDParseRoundTrip(t *testing.T) {
	for _, s := range []string{
		"2b2e5a63-e8c5-4c46-95e4-189292181c4b",
		"00000000-0000-0000-0000-000000000000",
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
		"c404c6a4-56a5-406f-a97f-c800ce6c6de0",
	} {
		g, err := ParseGUID(s)
		if err != nil {
			t.Fatalf("ParseGUID(%s): %v", s, err)
		}
		if got := g.String(); got != s {
			t.Errorf("round trip: %s -> %s", s, got)
		}
	}

	// And bytes -> text -> bytes.
	orig := GUID{0x63, 0x5a, 0x2e, 0x2b, 0x46, 0x4c, 0xc5, 0xe8, 0x92, 0x18, 0xe4, 0x95, 0x4b, 0x1c, 0x18, 0x92}
	back, err := ParseGUID(orig.String())
	if err != nil {
		t.Fatal(err)
	}
	if back != orig {
		t.Errorf("bytes round trip: %v -> %v", orig, back)
	}
}

func TestParseGUIDRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "not-a-guid", "2b2e5a63", "zzzzzzzz-e8c5-4c46-95e4-189292181c4b",
		"2b2e5a63-e8c5-4c46-95e4-189292181c4b00"} {
		if _, err := ParseGUID(s); err == nil {
			t.Errorf("ParseGUID(%q) should have failed", s)
		}
	}
}

func TestPlaceholderPatches(t *testing.T) {
	w := NewWriter()
	w.U32(1)
	p := w.ReserveU64()
	start := p.End()
	w.Raw([]byte("payload"))
	p.SetU64(uint64(w.Len() - start))

	r := NewReader(w.Bytes())
	if _, err := r.U32(); err != nil {
		t.Fatal(err)
	}
	size, err := r.U64()
	if err != nil {
		t.Fatal(err)
	}
	if size != uint64(len("payload")) {
		t.Errorf("patched size = %d, want %d", size, len("payload"))
	}
}
