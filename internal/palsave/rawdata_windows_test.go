//go:build windows

package palsave

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/oodle"
)

// loadWorld decodes the live-server fixture down to worldSaveData.
func loadWorld(t *testing.T) *gvas.Properties {
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
		t.Fatalf("decompress: %v", err)
	}
	f, err := gvas.Decode(raw, gvas.PalworldOptions())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	world, ok := f.Root.Get("worldSaveData")
	if !ok {
		t.Fatal("no worldSaveData")
	}
	sp := world.(*gvas.StructProperty)
	return sp.Value.(*gvas.StructProperties).Props
}

// mapProp fetches a named MapProperty out of worldSaveData.
func mapProp(t *testing.T, world *gvas.Properties, name string) *gvas.MapProperty {
	t.Helper()
	v, ok := world.Get(name)
	if !ok {
		t.Fatalf("worldSaveData has no %s", name)
	}
	m, ok := v.(*gvas.MapProperty)
	if !ok {
		t.Fatalf("%s is %T, want *gvas.MapProperty", name, v)
	}
	return m
}

// rawDataBytes pulls the ByteProperty payload of a "RawData" property out of a
// nested property block.
func rawDataBytes(props *gvas.Properties) ([]byte, bool) {
	v, ok := props.Get("RawData")
	if !ok {
		return nil, false
	}
	a, ok := v.(*gvas.ArrayProperty)
	if !ok {
		return nil, false
	}
	return a.Values.Bytes, true
}

// Every character blob in the save must decode and re-encode unchanged. This
// is the same contract internal/gvas holds, one level further in.
func TestAllCharacterBlobsRoundTrip(t *testing.T) {
	world := loadWorld(t)
	m := mapProp(t, world, "CharacterSaveParameterMap")
	opts := gvas.PalworldOptions()

	var decoded, players, pals int
	for i, e := range m.Entries {
		sv, ok := e.Value.Struct.(*gvas.StructProperties)
		if !ok {
			t.Fatalf("entry %d value is %T", i, e.Value.Struct)
		}
		blob, ok := rawDataBytes(sv.Props)
		if !ok {
			t.Fatalf("entry %d has no RawData", i)
		}

		c, err := DecodeCharacter(blob, opts)
		if err != nil {
			t.Fatalf("entry %d: DecodeCharacter: %v", i, err)
		}
		out, err := c.Encode()
		if err != nil {
			t.Fatalf("entry %d: Encode: %v", i, err)
		}
		if !bytes.Equal(out, blob) {
			t.Fatalf("entry %d: blob changed (%d bytes in, %d out)", i, len(blob), len(out))
		}
		decoded++

		p, err := NewPal(c)
		if err != nil {
			t.Fatalf("entry %d: NewPal: %v", i, err)
		}
		if p.IsPlayer() {
			players++
		} else {
			pals++
		}
	}

	t.Logf("round-tripped %d character blobs: %d players, %d pals", decoded, players, pals)
	if decoded == 0 {
		t.Fatal("no character blobs found")
	}
}

// Item slots are the other blob the editor writes to.
func TestAllItemSlotBlobsRoundTrip(t *testing.T) {
	world := loadWorld(t)
	m := mapProp(t, world, "ItemContainerSaveData")

	var containers, slots, occupied int
	for i, e := range m.Entries {
		sv, ok := e.Value.Struct.(*gvas.StructProperties)
		if !ok {
			t.Fatalf("entry %d value is %T", i, e.Value.Struct)
		}
		containers++

		// Container-level blob.
		if blob, ok := rawDataBytes(sv.Props); ok && len(blob) > 0 {
			c, err := DecodeItemContainer(blob)
			if err != nil {
				t.Fatalf("container %d: %v", i, err)
			}
			if out := c.Encode(); !bytes.Equal(out, blob) {
				t.Fatalf("container %d blob changed (%d in, %d out)", i, len(blob), len(out))
			}
		}

		// Per-slot blobs.
		sv2, ok := sv.Props.Get("Slots")
		if !ok {
			continue
		}
		arr, ok := sv2.(*gvas.ArrayProperty)
		if !ok || arr.Structs == nil {
			continue
		}
		for j, elem := range arr.Structs.Values {
			ep, ok := elem.(*gvas.StructProperties)
			if !ok {
				continue
			}
			blob, ok := rawDataBytes(ep.Props)
			if !ok {
				continue
			}
			slots++

			s, err := DecodeItemSlot(blob)
			if err != nil {
				t.Fatalf("container %d slot %d: %v", i, j, err)
			}
			if out := s.Encode(); !bytes.Equal(out, blob) {
				t.Fatalf("container %d slot %d blob changed (%d in, %d out)", i, j, len(blob), len(out))
			}
			if !s.IsEmpty() {
				occupied++
			}
		}
	}

	t.Logf("round-tripped %d containers, %d slots (%d occupied)", containers, slots, occupied)
}

// Pal container slots hold the palbox/party/base-camp rosters.
func TestAllCharacterContainerSlotsRoundTrip(t *testing.T) {
	world := loadWorld(t)
	m := mapProp(t, world, "CharacterContainerSaveData")

	var slots int
	for i, e := range m.Entries {
		sv, ok := e.Value.Struct.(*gvas.StructProperties)
		if !ok {
			continue
		}
		v, ok := sv.Props.Get("Slots")
		if !ok {
			continue
		}
		arr, ok := v.(*gvas.ArrayProperty)
		if !ok || arr.Structs == nil {
			continue
		}
		for j, elem := range arr.Structs.Values {
			ep, ok := elem.(*gvas.StructProperties)
			if !ok {
				continue
			}
			blob, ok := rawDataBytes(ep.Props)
			if !ok {
				continue
			}
			slots++

			s, err := DecodeCharacterContainerSlot(blob)
			if err != nil {
				t.Fatalf("container %d slot %d: %v", i, j, err)
			}
			if out := s.Encode(); !bytes.Equal(out, blob) {
				t.Fatalf("container %d slot %d blob changed", i, j)
			}
		}
	}
	t.Logf("round-tripped %d pal container slots", slots)
}

// Reproduce the census the Python tooling produced for player 지나가는 형준,
// which is a strong check that the typed accessors read the same values.
func TestPalAccessorsMatchKnownSave(t *testing.T) {
	world := loadWorld(t)
	m := mapProp(t, world, "CharacterSaveParameterMap")
	opts := gvas.PalworldOptions()

	owner, err := gvas.ParseGUID("950f9b77-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatal(err)
	}

	species := map[string]int{}
	levels := map[int]int{}
	var mine int

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
		got, ok := p.OwnerPlayerUID()
		if !ok || got != owner {
			continue
		}
		mine++
		switch p.Species() {
		case "IceHorse_Dark", "IceNarwhal_Fire", "BlackGriffon":
			species[p.Species()]++
			levels[p.Level()]++
		}
	}

	t.Logf("player owns %d pals", mine)
	for k, v := range species {
		t.Logf("  %-18s %d", k, v)
	}
	t.Logf("levels of those: %v", levels)

	// The Python run reported 345 owned pals, of which 54 흑천마, 25 홍등고래
	// and 20 제노그리프.
	if mine != 345 {
		t.Errorf("owned pals = %d, want 345", mine)
	}
	for id, want := range map[string]int{
		"IceHorse_Dark":   54,
		"IceNarwhal_Fire": 25,
		"BlackGriffon":    20,
	} {
		if species[id] != want {
			t.Errorf("%s = %d, want %d", id, species[id], want)
		}
	}
}
