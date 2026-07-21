//go:build windows

package gvas

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wo420/nanachi-palsave-editor/internal/oodle"
)

// realGvas decompresses the live-server fixture down to raw GVAS bytes.
func realGvas(t *testing.T) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "Level.sav")
	sav, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("fixture %s not present: %v", p, err)
	}
	if err := oodle.Available(); err != nil {
		t.Skipf("native codec unavailable: %v", err)
	}
	raw, _, err := oodle.DecompressSav(sav)
	if err != nil {
		t.Fatalf("decompress fixture: %v", err)
	}
	return raw
}

func TestReadRealHeader(t *testing.T) {
	raw := realGvas(t)

	r := NewReader(raw)
	h, err := ReadHeader(r)
	if err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}

	if h.Magic != gvasMagic {
		t.Errorf("magic = 0x%08x, want 0x%08x", uint32(h.Magic), uint32(gvasMagic))
	}
	if h.SaveGameVersion != 3 {
		t.Errorf("save game version = %d, want 3", h.SaveGameVersion)
	}
	if h.CustomVersionFormat != 3 {
		t.Errorf("custom version format = %d, want 3", h.CustomVersionFormat)
	}
	if len(h.CustomVersions) == 0 {
		t.Error("expected a non-empty custom version table")
	}
	if h.SaveGameClassName.Value == "" {
		t.Error("expected a save game class name")
	}

	t.Logf("engine   %s", h.EngineVersion())
	t.Logf("class    %s", h.SaveGameClassName.Value)
	t.Logf("versions %d entries", len(h.CustomVersions))
	t.Logf("header   %d bytes", r.Pos())
}

// The header is the first thing that has to survive a write, and it exercises
// strings, GUIDs and a counted table all at once.
func TestRealHeaderRoundTripsByteIdentical(t *testing.T) {
	raw := realGvas(t)

	r := NewReader(raw)
	h, err := ReadHeader(r)
	if err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	consumed := r.Pos()

	w := NewWriter()
	h.Write(w)

	if got, want := w.Bytes(), raw[:consumed]; !bytes.Equal(got, want) {
		t.Errorf("header re-encode differs (%d bytes read, %d written)", consumed, len(got))
		for i := 0; i < len(got) && i < len(want); i++ {
			if got[i] != want[i] {
				t.Fatalf("first difference at byte %d: got 0x%02x, want 0x%02x", i, got[i], want[i])
			}
		}
	}
}
