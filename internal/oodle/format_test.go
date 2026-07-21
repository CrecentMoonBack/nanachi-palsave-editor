package oodle

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// realSave loads the gitignored fixture pulled from the live server.
// Tests that need it skip when it is absent so a fresh clone still builds.
func realSave(t *testing.T) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "Level.sav")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("fixture %s not present: %v", p, err)
	}
	return b
}

func TestParseHeaderRealSave(t *testing.T) {
	data := realSave(t)

	h, err := ParseHeader(data)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}

	if string(h.Magic[:]) != "PlM" {
		t.Errorf("magic = %q, want PlM", h.Magic[:])
	}
	if h.Type != SaveTypePLM {
		t.Errorf("save type = %v (%d), want PlM (49)", h.Type, h.Type)
	}
	if h.DataOffset != 12 {
		t.Errorf("data offset = %d, want 12", h.DataOffset)
	}
	// The header's compressed length must account for every byte after it.
	if got := len(data) - h.DataOffset; got != int(h.CompressedLen) {
		t.Errorf("payload = %d bytes, header says %d", got, h.CompressedLen)
	}
	// Sanity: this save is known to expand roughly 17x.
	if h.UncompressedLen < h.CompressedLen {
		t.Errorf("uncompressed (%d) < compressed (%d)", h.UncompressedLen, h.CompressedLen)
	}
	t.Logf("PlM save: %d compressed -> %d uncompressed", h.CompressedLen, h.UncompressedLen)
}

func TestBuildSavRoundTripsHeader(t *testing.T) {
	payload := []byte("not really compressed, but the header does not care")

	sav, err := BuildSav(payload, 123456, SaveTypePLM)
	if err != nil {
		t.Fatalf("BuildSav: %v", err)
	}

	h, err := ParseHeader(sav)
	if err != nil {
		t.Fatalf("ParseHeader of built sav: %v", err)
	}
	if h.UncompressedLen != 123456 {
		t.Errorf("uncompressed len = %d, want 123456", h.UncompressedLen)
	}
	if h.CompressedLen != uint32(len(payload)) {
		t.Errorf("compressed len = %d, want %d", h.CompressedLen, len(payload))
	}
	if h.Type != SaveTypePLM {
		t.Errorf("type = %v, want PlM", h.Type)
	}
	if !bytes.Equal(h.Payload(sav), payload) {
		t.Error("payload did not survive the round trip")
	}
}

func TestParseHeaderRejectsGarbage(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"too small", make([]byte, 20)},
		{"bad magic", func() []byte {
			b := make([]byte, 64)
			copy(b[8:11], "XXX")
			return b
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseHeader(tc.data); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

func TestSaveTypeMagicPairs(t *testing.T) {
	for _, st := range []SaveType{SaveTypeCNK, SaveTypePLM, SaveTypePLZ} {
		if !st.Valid() {
			t.Errorf("%v should be valid", st)
		}
		if got := string(st.Magic()); got != st.String() {
			t.Errorf("%v: magic %q != String() %q", byte(st), got, st.String())
		}
	}
	if bad := SaveType(0x99); bad.Valid() || bad.Magic() != nil {
		t.Error("unknown save type should be invalid and have no magic")
	}
}
