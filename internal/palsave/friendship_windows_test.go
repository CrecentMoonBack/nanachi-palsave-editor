//go:build windows

package palsave

import (
	"bytes"
	"testing"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// This reads the friendship values the live save actually holds, which is the
// only source we have for what a reasonable range looks like.
func TestFriendshipRead(t *testing.T) {
	world := loadWorld(t)
	m := mapProp(t, world, "CharacterSaveParameterMap")
	opts := gvas.PalworldOptions()

	var have, zero, max int
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
		f := p.Friendship()
		if f > 0 {
			have++
		} else {
			zero++
		}
		if f > max {
			max = f
		}
	}
	t.Logf("pals with friendship > 0: %d, at zero: %d, highest observed: %d", have, zero, max)
}

func TestSetFriendshipRoundTrips(t *testing.T) {
	world := loadWorld(t)
	m := mapProp(t, world, "CharacterSaveParameterMap")
	opts := gvas.PalworldOptions()

	var withProp, without *Pal
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
		if _, ok := p.Params().Get("FriendshipPoint"); ok {
			if withProp == nil {
				withProp = p
			}
		} else if without == nil {
			without = p
		}
		if withProp != nil && without != nil {
			break
		}
	}

	t.Run("update an existing value", func(t *testing.T) {
		if withProp == nil {
			t.Skip("no pal with a FriendshipPoint property")
		}
		if err := withProp.SetFriendship(150000); err != nil {
			t.Fatalf("SetFriendship: %v", err)
		}
		out := reencode(t, withProp, opts)
		if got := out.Friendship(); got != 150000 {
			t.Errorf("friendship = %d, want 150000", got)
		}
	})

	t.Run("create on a pal with none", func(t *testing.T) {
		if without == nil {
			t.Skip("every pal already has a FriendshipPoint property")
		}
		if got := without.Friendship(); got != 0 {
			t.Fatalf("a pal without the property should read 0, got %d", got)
		}
		if err := without.SetFriendship(12345); err != nil {
			t.Fatalf("SetFriendship: %v", err)
		}
		out := reencode(t, without, opts)
		if got := out.Friendship(); got != 12345 {
			t.Errorf("friendship = %d, want 12345", got)
		}
	})

	t.Run("out of range is refused", func(t *testing.T) {
		if withProp == nil {
			t.Skip("no pal to test against")
		}
		if err := withProp.SetFriendship(-1); err == nil {
			t.Error("negative should be refused")
		}
		if err := withProp.SetFriendship(MaxFriendship + 1); err == nil {
			t.Error("above the cap should be refused")
		}
	})
}

// Editing one pal's friendship must leave the rest byte-identical.
func TestFriendshipLeavesOtherPalsAlone(t *testing.T) {
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

	if err := pals[0].pal.SetFriendship(177777); err != nil {
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
