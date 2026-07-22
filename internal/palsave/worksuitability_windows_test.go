//go:build windows

package palsave

import (
	"bytes"
	"testing"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// Adding a job to a pal that has never had a book used on it means creating
// the array from nothing. That is the same shape of work that broke item slots
// once, so it is asserted against a real save rather than a synthetic one.
func TestSetWorkSuitabilityRoundTrips(t *testing.T) {
	world := loadWorld(t)
	m := mapProp(t, world, "CharacterSaveParameterMap")
	opts := gvas.PalworldOptions()

	var withNone, withSome *Pal
	var blobNone, blobSome []byte

	for _, e := range m.Entries {
		sv := e.Value.Struct.(*gvas.StructProperties)
		blob, ok := rawDataBytes(sv.Props)
		if !ok {
			continue
		}
		c, err := DecodeCharacter(blob, opts)
		if err != nil {
			t.Fatal(err)
		}
		p, err := NewPal(c)
		if err != nil {
			t.Fatal(err)
		}
		if p.IsPlayer() {
			continue
		}
		if len(p.WorkSuitabilityBonuses()) == 0 {
			if withNone == nil {
				withNone, blobNone = p, blob
			}
		} else if withSome == nil {
			withSome, blobSome = p, blob
		}
		if withNone != nil && withSome != nil {
			break
		}
	}
	if withNone == nil || withSome == nil {
		t.Skip("save lacks both a pal with and without work suitability bonuses")
	}
	_ = blobNone
	_ = blobSome

	t.Run("create on a pal with none", func(t *testing.T) {
		if err := withNone.SetWorkSuitability("Handcraft", 7); err != nil {
			t.Fatalf("SetWorkSuitability: %v", err)
		}
		out := reencode(t, withNone, opts)
		if got := out.WorkSuitabilityBonuses()["EPalWorkSuitability::Handcraft"]; got != 7 {
			t.Errorf("after round trip Handcraft = %d, want 7", got)
		}
	})

	t.Run("qualified name is accepted too", func(t *testing.T) {
		if err := withNone.SetWorkSuitability("EPalWorkSuitability::Mining", 3); err != nil {
			t.Fatalf("SetWorkSuitability: %v", err)
		}
		out := reencode(t, withNone, opts)
		if got := out.WorkSuitabilityBonuses()["EPalWorkSuitability::Mining"]; got != 3 {
			t.Errorf("Mining = %d, want 3", got)
		}
		// The first job must survive the second write.
		if got := out.WorkSuitabilityBonuses()["EPalWorkSuitability::Handcraft"]; got != 7 {
			t.Errorf("Handcraft = %d after adding Mining, want 7", got)
		}
	})

	t.Run("update an existing job", func(t *testing.T) {
		before := withSome.WorkSuitabilityBonuses()
		var kind string
		for k := range before {
			kind = k
			break
		}
		if err := withSome.SetWorkSuitability(kind, 9); err != nil {
			t.Fatalf("SetWorkSuitability: %v", err)
		}
		out := reencode(t, withSome, opts)
		if got := out.WorkSuitabilityBonuses()[kind]; got != 9 {
			t.Errorf("%s = %d, want 9", kind, got)
		}
		if len(out.WorkSuitabilityBonuses()) != len(before) {
			t.Errorf("job count changed: %d -> %d", len(before), len(out.WorkSuitabilityBonuses()))
		}
	})

	t.Run("rank 0 removes the entry", func(t *testing.T) {
		if err := withNone.SetWorkSuitability("Handcraft", 0); err != nil {
			t.Fatalf("SetWorkSuitability: %v", err)
		}
		out := reencode(t, withNone, opts)
		if _, present := out.WorkSuitabilityBonuses()["EPalWorkSuitability::Handcraft"]; present {
			t.Error("Handcraft should be gone after setting rank 0")
		}
		if got := out.WorkSuitabilityBonuses()["EPalWorkSuitability::Mining"]; got != 3 {
			t.Errorf("Mining = %d after removing Handcraft, want 3", got)
		}
	})

	t.Run("out of range is refused", func(t *testing.T) {
		for _, bad := range []int{-1, 11, 999} {
			if err := withSome.SetWorkSuitability("Mining", bad); err == nil {
				t.Errorf("rank %d should have been refused", bad)
			}
		}
	})
}

// reencode writes a pal back to bytes and reads it again, which is the only
// way to know a mutation survives the encoder rather than merely looking right
// in memory.
func reencode(t *testing.T, p *Pal, opts *gvas.Options) *Pal {
	t.Helper()
	b, err := p.Raw.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	raw, err := DecodeCharacter(b, opts)
	if err != nil {
		t.Fatalf("DecodeCharacter: %v", err)
	}
	out, err := NewPal(raw)
	if err != nil {
		t.Fatalf("NewPal: %v", err)
	}
	return out
}

// Touching one pal must not disturb any other. This is the property that makes
// an editor safe to point at a shared server save.
func TestWorkSuitabilityLeavesOtherPalsAlone(t *testing.T) {
	world := loadWorld(t)
	m := mapProp(t, world, "CharacterSaveParameterMap")
	opts := gvas.PalworldOptions()

	type sample struct {
		pal  *Pal
		blob []byte
	}
	var pals []sample
	for _, e := range m.Entries {
		sv := e.Value.Struct.(*gvas.StructProperties)
		blob, ok := rawDataBytes(sv.Props)
		if !ok {
			continue
		}
		c, err := DecodeCharacter(blob, opts)
		if err != nil {
			t.Fatal(err)
		}
		p, err := NewPal(c)
		if err != nil {
			t.Fatal(err)
		}
		if p.IsPlayer() {
			continue
		}
		pals = append(pals, sample{p, blob})
		if len(pals) == 40 {
			break
		}
	}
	if len(pals) < 2 {
		t.Skip("not enough pals in the fixture")
	}

	// Edit exactly one.
	if err := pals[0].pal.SetWorkSuitability("Transport", 5); err != nil {
		t.Fatal(err)
	}

	for i, s := range pals[1:] {
		got, err := s.pal.Raw.Encode()
		if err != nil {
			t.Fatalf("pal %d: %v", i+1, err)
		}
		if !bytes.Equal(got, s.blob) {
			t.Fatalf("pal %d changed despite not being edited", i+1)
		}
	}
}
