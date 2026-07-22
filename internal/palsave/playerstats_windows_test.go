//go:build windows

package palsave

import (
	"bytes"
	"testing"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// findPlayer returns the first player record in the fixture, along with the
// blob it came from.
func findPlayer(t *testing.T) (*Pal, []byte, *gvas.Options) {
	t.Helper()
	world := loadWorld(t)
	m := mapProp(t, world, "CharacterSaveParameterMap")
	opts := gvas.PalworldOptions()

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
			return p, blob, opts
		}
	}
	t.Skip("fixture holds no player record")
	return nil, nil, nil
}

func TestPlayerStatusPointsRead(t *testing.T) {
	p, _, _ := findPlayer(t)

	got := p.StatusPoints(StatusPointList)
	if len(got) == 0 {
		t.Fatal("player has no GotStatusPointList entries")
	}
	order := p.StatusPointOrder(StatusPointList)
	if len(order) != len(got) {
		t.Errorf("order has %d names, map has %d", len(order), len(got))
	}
	t.Logf("unused: %d", p.UnusedStatusPoint())
	for _, n := range order {
		t.Logf("  %-24s %d", n, got[n])
	}

	ex := p.StatusPoints(ExStatusPointList)
	t.Logf("ex list: %d entries", len(ex))
}

func TestPlayerStatusPointsRoundTrip(t *testing.T) {
	p, _, opts := findPlayer(t)

	order := p.StatusPointOrder(StatusPointList)
	if len(order) == 0 {
		t.Skip("no status entries to edit")
	}
	target := order[0]

	t.Run("update an existing status", func(t *testing.T) {
		if err := p.SetStatusPoint(StatusPointList, target, 42); err != nil {
			t.Fatalf("SetStatusPoint: %v", err)
		}
		out := reencode(t, p, opts)
		if got := out.StatusPoints(StatusPointList)[target]; got != 42 {
			t.Errorf("%s = %d, want 42", target, got)
		}
		// The other entries must be untouched.
		if len(out.StatusPointOrder(StatusPointList)) != len(order) {
			t.Errorf("entry count changed: %d -> %d",
				len(order), len(out.StatusPointOrder(StatusPointList)))
		}
	})

	t.Run("zero is kept, not removed", func(t *testing.T) {
		if err := p.SetStatusPoint(StatusPointList, target, 0); err != nil {
			t.Fatal(err)
		}
		out := reencode(t, p, opts)
		names := out.StatusPointOrder(StatusPointList)
		found := false
		for _, n := range names {
			if n == target {
				found = true
			}
		}
		if !found {
			t.Error("a zeroed status should stay in the list")
		}
	})

	t.Run("add a status the player lacks", func(t *testing.T) {
		const novel = "テスト用ステータス"
		if err := p.SetStatusPoint(StatusPointList, novel, 7); err != nil {
			t.Fatalf("SetStatusPoint: %v", err)
		}
		out := reencode(t, p, opts)
		if got := out.StatusPoints(StatusPointList)[novel]; got != 7 {
			t.Errorf("new status = %d, want 7", got)
		}
	})

	t.Run("unused pool", func(t *testing.T) {
		if err := p.SetUnusedStatusPoint(123); err != nil {
			t.Fatal(err)
		}
		out := reencode(t, p, opts)
		if got := out.UnusedStatusPoint(); got != 123 {
			t.Errorf("unused = %d, want 123", got)
		}
	})

	t.Run("out of range is refused", func(t *testing.T) {
		if err := p.SetStatusPoint(StatusPointList, target, -1); err == nil {
			t.Error("negative should be refused")
		}
		if err := p.SetStatusPoint(StatusPointList, target, MaxStatusPoint+1); err == nil {
			t.Errorf("above %d should be refused", MaxStatusPoint)
		}
	})
}

// Editing a player must not disturb the pals sharing the save.
func TestPlayerEditLeavesPalsAlone(t *testing.T) {
	world := loadWorld(t)
	m := mapProp(t, world, "CharacterSaveParameterMap")
	opts := gvas.PalworldOptions()

	type sample struct {
		pal  *Pal
		blob []byte
	}
	var player *Pal
	var others []sample

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
		if p.IsPlayer() && player == nil {
			player = p
			continue
		}
		if len(others) < 30 {
			others = append(others, sample{p, blob})
		}
	}
	if player == nil || len(others) == 0 {
		t.Skip("fixture lacks a player or pals")
	}

	if err := player.SetUnusedStatusPoint(99); err != nil {
		t.Fatal(err)
	}
	if order := player.StatusPointOrder(StatusPointList); len(order) > 0 {
		if err := player.SetStatusPoint(StatusPointList, order[0], 15); err != nil {
			t.Fatal(err)
		}
	}

	for i, s := range others {
		got, err := s.pal.Raw.Encode()
		if err != nil {
			t.Fatalf("pal %d: %v", i, err)
		}
		if !bytes.Equal(got, s.blob) {
			t.Fatalf("pal %d changed while editing the player", i)
		}
	}
}
