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

// loadPlayerSave decodes the gitignored player fixture.
func loadPlayerSave(t *testing.T) (*PlayerSave, []byte) {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "player_950F9B77.sav")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("fixture %s not present: %v", p, err)
	}
	if err := oodle.Available(); err != nil {
		t.Skipf("native codec unavailable: %v", err)
	}
	raw, _, err := oodle.DecompressSav(data)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	f, err := gvas.Decode(raw, gvas.PalworldOptions())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	ps, err := NewPlayerSave(f)
	if err != nil {
		t.Fatalf("NewPlayerSave: %v", err)
	}
	return ps, raw
}

// reencodePlayer writes the save back and reads it again, which is the only
// way to know a change survived the encoder.
func reencodePlayer(t *testing.T, ps *PlayerSave) *PlayerSave {
	t.Helper()
	b, err := ps.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	f, err := gvas.Decode(b, gvas.PalworldOptions())
	if err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	out, err := NewPlayerSave(f)
	if err != nil {
		t.Fatalf("NewPlayerSave: %v", err)
	}
	return out
}

// The untouched player save must round-trip byte for byte, the same contract
// the world save holds.
func TestPlayerSaveRoundTripsByteIdentical(t *testing.T) {
	ps, raw := loadPlayerSave(t)

	out, err := ps.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(out, raw) {
		t.Fatalf("player save changed on a no-op round trip (%d bytes in, %d out)",
			len(raw), len(out))
	}
	t.Logf("round-tripped %d bytes exactly", len(raw))
}

func TestRelicsRead(t *testing.T) {
	ps, _ := loadPlayerSave(t)

	got := ps.Relics()
	if len(got) == 0 {
		t.Fatal("player has no relic entries")
	}
	for _, k := range ps.RelicOrder() {
		t.Logf("  %-40s %d", k, got[k])
	}

	// The standalone copy must agree with the map, which is the assumption
	// SetRelic maintains.
	if n, cap := ps.RelicPossessNum(), got[RelicCapturePower]; n != cap {
		t.Errorf("RelicPossessNum = %d but map CapturePower = %d", n, cap)
	}
}

func TestSetRelicRoundTrips(t *testing.T) {
	ps, _ := loadPlayerSave(t)

	t.Run("update an existing type", func(t *testing.T) {
		if err := ps.SetRelic("EPalRelicType::MoveSpeed", 42); err != nil {
			t.Fatalf("SetRelic: %v", err)
		}
		out := reencodePlayer(t, ps)
		if got := out.Relics()["EPalRelicType::MoveSpeed"]; got != 42 {
			t.Errorf("MoveSpeed = %d, want 42", got)
		}
	})

	t.Run("bare name is accepted", func(t *testing.T) {
		if err := ps.SetRelic("ClimbSpeed", 9); err != nil {
			t.Fatalf("SetRelic: %v", err)
		}
		out := reencodePlayer(t, ps)
		if got := out.Relics()["EPalRelicType::ClimbSpeed"]; got != 9 {
			t.Errorf("ClimbSpeed = %d, want 9", got)
		}
	})

	// The one that matters: the duplicated field must not drift.
	t.Run("capture power keeps RelicPossessNum in step", func(t *testing.T) {
		if err := ps.SetRelic(RelicCapturePower, 25); err != nil {
			t.Fatalf("SetRelic: %v", err)
		}
		out := reencodePlayer(t, ps)
		if got := out.Relics()[RelicCapturePower]; got != 25 {
			t.Errorf("map CapturePower = %d, want 25", got)
		}
		if got := out.RelicPossessNum(); got != 25 {
			t.Errorf("RelicPossessNum = %d, want 25 — the copy drifted", got)
		}
	})

	t.Run("add a type the player lacks", func(t *testing.T) {
		const novel = "EPalRelicType::GliderSpeed"
		before := len(ps.RelicOrder())
		if err := ps.SetRelic(novel, 4); err != nil {
			t.Fatalf("SetRelic: %v", err)
		}
		out := reencodePlayer(t, ps)
		if got := out.Relics()[novel]; got != 4 {
			t.Errorf("%s = %d, want 4", novel, got)
		}
		if after := len(out.RelicOrder()); after != before+1 {
			t.Errorf("entry count %d -> %d, want one more", before, after)
		}
	})

	t.Run("out of range is refused", func(t *testing.T) {
		if err := ps.SetRelic(RelicCapturePower, -1); err == nil {
			t.Error("negative should be refused")
		}
		if err := ps.SetRelic(RelicCapturePower, MaxRelicCount+1); err == nil {
			t.Errorf("above %d should be refused", MaxRelicCount)
		}
	})
}

// Editing relics must leave the rest of the player save alone — the container
// ids in particular, since losing those would orphan the inventory.
func TestSetRelicLeavesContainersAlone(t *testing.T) {
	ps, _ := loadPlayerSave(t)

	type ref struct {
		name string
		id   gvas.GUID
	}
	var before []ref
	for _, n := range []string{
		ContainerCommon, ContainerEssential, ContainerWeapon,
		ContainerArmor, ContainerFood, ContainerDropSlot,
	} {
		if id, ok := ps.InventoryContainer(n); ok {
			before = append(before, ref{n, id})
		}
	}
	palbox, hadPalbox := ps.PalStorageContainer()

	if err := ps.SetRelic(RelicCapturePower, 30); err != nil {
		t.Fatal(err)
	}
	out := reencodePlayer(t, ps)

	for _, b := range before {
		got, ok := out.InventoryContainer(b.name)
		if !ok {
			t.Errorf("%s went missing", b.name)
			continue
		}
		if got != b.id {
			t.Errorf("%s changed: %s -> %s", b.name, b.id, got)
		}
	}
	if hadPalbox {
		got, ok := out.PalStorageContainer()
		if !ok || got != palbox {
			t.Errorf("palbox container changed: %s -> %s", palbox, got)
		}
	}
	if uid, ok := out.PlayerUID(); !ok || uid.IsZero() {
		t.Error("player uid lost")
	}
}
