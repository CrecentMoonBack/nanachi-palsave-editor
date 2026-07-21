//go:build windows

package oodle

import (
	"bytes"
	"testing"
)

func requireCodec(t *testing.T) {
	t.Helper()
	if err := Available(); err != nil {
		t.Skipf("native codec unavailable: %v", err)
	}
}

// The load-bearing test: the real save must decompress to exactly the length
// its header promises, and the result must actually be GVAS.
func TestDecompressRealSave(t *testing.T) {
	requireCodec(t)
	data := realSave(t)

	h, err := ParseHeader(data)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}

	gvas, st, err := DecompressSav(data)
	if err != nil {
		t.Fatalf("DecompressSav: %v", err)
	}
	if st != SaveTypePLM {
		t.Errorf("save type = %v, want PlM", st)
	}
	if len(gvas) != int(h.UncompressedLen) {
		t.Fatalf("decompressed %d bytes, header said %d", len(gvas), h.UncompressedLen)
	}
	if got := string(gvas[:4]); got != "GVAS" {
		t.Errorf("decompressed data starts with %q, want GVAS", got)
	}
	t.Logf("decompressed %d -> %d bytes", h.CompressedLen, len(gvas))
}

// Compression is only useful if our own output reads back identically. This is
// the property that matters; matching the game's byte layout is not required
// and does not hold.
func TestCompressRoundTrip(t *testing.T) {
	requireCodec(t)
	data := realSave(t)

	orig, st, err := DecompressSav(data)
	if err != nil {
		t.Fatalf("DecompressSav: %v", err)
	}

	packed, err := CompressSav(orig, st)
	if err != nil {
		t.Fatalf("CompressSav: %v", err)
	}

	again, st2, err := DecompressSav(packed)
	if err != nil {
		t.Fatalf("DecompressSav of our own output: %v", err)
	}
	if st2 != st {
		t.Errorf("save type changed: %v -> %v", st, st2)
	}
	if !bytes.Equal(orig, again) {
		t.Fatalf("round trip altered the data (%d bytes in, %d out)", len(orig), len(again))
	}

	t.Logf("original sav %d bytes, ours %d bytes (%.1f%% of original), payload identical",
		len(data), len(packed), 100*float64(len(packed))/float64(len(data)))
}

func TestCompressRoundTripSmallInputs(t *testing.T) {
	requireCodec(t)

	cases := map[string][]byte{
		"tiny":       []byte("GVAS"),
		"repetitive": bytes.Repeat([]byte("palworld"), 4096),
		"incompressible": func() []byte {
			b := make([]byte, 8192)
			for i := range b {
				b[i] = byte(i*7 + i/3)
			}
			return b
		}(),
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			packed, err := CompressSav(in, SaveTypePLM)
			if err != nil {
				t.Fatalf("CompressSav: %v", err)
			}
			out, _, err := DecompressSav(packed)
			if err != nil {
				t.Fatalf("DecompressSav: %v", err)
			}
			if !bytes.Equal(in, out) {
				t.Errorf("round trip altered %d bytes of input", len(in))
			}
		})
	}
}

func TestZlibPathRoundTrips(t *testing.T) {
	// PlZ needs no native codec — it is pure stdlib.
	in := bytes.Repeat([]byte("older palworld save format"), 512)

	packed, err := CompressSav(in, SaveTypePLZ)
	if err != nil {
		t.Fatalf("CompressSav(PlZ): %v", err)
	}
	if h, err := ParseHeader(packed); err != nil {
		t.Fatalf("ParseHeader: %v", err)
	} else if h.Type != SaveTypePLZ {
		t.Errorf("type = %v, want PlZ", h.Type)
	}

	out, st, err := DecompressSav(packed)
	if err != nil {
		t.Fatalf("DecompressSav(PlZ): %v", err)
	}
	if st != SaveTypePLZ {
		t.Errorf("type = %v, want PlZ", st)
	}
	if !bytes.Equal(in, out) {
		t.Error("PlZ round trip altered the data")
	}
}
