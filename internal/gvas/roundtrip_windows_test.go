//go:build windows

package gvas

import (
	"bytes"
	"testing"
)

// This is the test the whole package exists to pass: decode the real 46MB
// save and re-encode it to the same bytes. Anything less means an edit could
// corrupt data we never intended to touch.
func TestRealSaveRoundTripsByteIdentical(t *testing.T) {
	raw := realGvas(t)

	f, err := Decode(raw, PalworldOptions())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	out, err := Encode(f)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	if len(out) != len(raw) {
		t.Errorf("re-encoded %d bytes, original was %d (diff %+d)",
			len(out), len(raw), len(out)-len(raw))
	}
	if !bytes.Equal(out, raw) {
		n := len(out)
		if len(raw) < n {
			n = len(raw)
		}
		for i := 0; i < n; i++ {
			if out[i] != raw[i] {
				lo := max(0, i-16)
				t.Fatalf("first difference at byte %d of %d\n  got  % x\n  want % x",
					i, len(raw), out[lo:min(i+16, len(out))], raw[lo:min(i+16, len(raw))])
			}
		}
		t.Fatalf("outputs differ only in length: %d vs %d", len(out), len(raw))
	}

	t.Logf("round-tripped %d bytes exactly", len(raw))
	t.Logf("root properties: %d", f.Root.Len())
	t.Logf("trailer: % x", f.Trailer)
}

// A quick census, useful both as documentation of what a save contains and as
// a smoke test that the tree really was walked rather than skipped.
func TestRealSaveStructure(t *testing.T) {
	raw := realGvas(t)

	f, err := Decode(raw, PalworldOptions())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	world, ok := f.Root.Get("worldSaveData")
	if !ok {
		t.Fatal("no worldSaveData in root")
	}
	sp, ok := world.(*StructProperty)
	if !ok {
		t.Fatalf("worldSaveData is %T, want *StructProperty", world)
	}
	inner, ok := sp.Value.(*StructProperties)
	if !ok {
		t.Fatalf("worldSaveData value is %T, want *StructProperties", sp.Value)
	}

	for _, name := range []string{
		"CharacterSaveParameterMap",
		"ItemContainerSaveData",
		"CharacterContainerSaveData",
		"GroupSaveDataMap",
		"BaseCampSaveData",
	} {
		v, ok := inner.Props.Get(name)
		if !ok {
			t.Errorf("worldSaveData is missing %s", name)
			continue
		}
		switch p := v.(type) {
		case *MapProperty:
			t.Logf("%-28s MapProperty, %d entries", name, len(p.Entries))
		case *CustomProperty:
			t.Logf("%-28s custom blob, %d bytes", name, len(p.Raw))
		default:
			t.Logf("%-28s %T", name, v)
		}
	}
}
