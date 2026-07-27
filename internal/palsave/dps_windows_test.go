//go:build windows

package palsave

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/oodle"
)

// loadDPS decodes the gitignored DPS fixture.
func loadDPS(t *testing.T) *DPSStore {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "dps.sav")
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
	d, err := NewDPSStore(f, gvas.PalworldOptions())
	if err != nil {
		t.Fatalf("NewDPSStore: %v", err)
	}
	return d
}

// firstPalbox returns a palbox container with a free slot.
func firstPalbox(t *testing.T, w *World) *PalContainer {
	t.Helper()
	for _, c := range w.PalContainers() {
		if c.Capacity >= 900 {
			if _, ok := c.FreeSlotIndex(); ok {
				return c
			}
		}
	}
	t.Fatal("no palbox with a free slot")
	return nil
}

func reloadDPS(t *testing.T, d *DPSStore) *DPSStore {
	t.Helper()
	b, err := d.Encode()
	if err != nil {
		t.Fatalf("dps encode: %v", err)
	}
	f, err := gvas.Decode(b, gvas.PalworldOptions())
	if err != nil {
		t.Fatalf("dps re-decode: %v", err)
	}
	d2, err := NewDPSStore(f, gvas.PalworldOptions())
	if err != nil {
		t.Fatalf("dps re-wrap: %v", err)
	}
	return d2
}

func reloadWorld(t *testing.T, w *World) *World {
	t.Helper()
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	b, err := gvas.Encode(w.File)
	if err != nil {
		t.Fatalf("world encode: %v", err)
	}
	f, err := gvas.Decode(b, gvas.PalworldOptions())
	if err != nil {
		t.Fatalf("world re-decode: %v", err)
	}
	w2, err := NewWorld(f, gvas.PalworldOptions())
	if err != nil {
		t.Fatalf("world re-wrap: %v", err)
	}
	if err := w2.Load(); err != nil {
		t.Fatalf("world reload: %v", err)
	}
	return w2
}

func hasChar(w *World, instance gvas.GUID) bool {
	for _, c := range w.chars {
		if c.InstanceID == instance {
			return true
		}
	}
	return false
}

func lockerLen(w *World) int {
	set, ok := w.lockerSet()
	if !ok {
		return -1
	}
	return len(set.Structs)
}

func TestMoveFromDPSRestoresPalAndSurvivesRoundTrip(t *testing.T) {
	w := loadLevelWorld(t)
	d := loadDPS(t)

	pals := d.Pals()
	if len(pals) == 0 {
		t.Skip("DPS fixture has no stored pals")
	}
	dp := pals[0]
	instance := dp.InstanceID
	species := dp.Pal.CharacterID()
	t.Logf("restoring %s (%s), owner=%s", species, instance, dp.Owner)

	box := firstPalbox(t, w)
	beforeChars := len(w.chars)
	beforeLocker := lockerLen(w)

	if err := w.MoveFromDPS(d, instance, box.ID); err != nil {
		t.Fatalf("MoveFromDPS: %v", err)
	}

	if got := len(d.Pals()); got != len(pals)-1 {
		t.Errorf("DPS pal count = %d, want %d", got, len(pals)-1)
	}
	if got := lockerLen(w); got != beforeLocker-1 {
		t.Errorf("locker len = %d, want %d", got, beforeLocker-1)
	}
	if len(w.chars) != beforeChars+1 {
		t.Errorf("world chars = %d, want %d", len(w.chars), beforeChars+1)
	}
	if !hasChar(w, instance) {
		t.Error("restored instance not in world")
	}

	// both files must survive a full re-encode / re-decode.
	w2 := reloadWorld(t, w)
	if !hasChar(w2, instance) {
		t.Error("restored instance missing after world round-trip")
	}
	// the pal's species must be intact — awakening and all.
	var found *Pal
	for _, c := range w2.chars {
		if c.InstanceID == instance {
			found = c.Pal
		}
	}
	if found == nil {
		t.Fatal("restored pal gone after round-trip")
	}
	if found.CharacterID() != species {
		t.Errorf("species = %q, want %q", found.CharacterID(), species)
	}
	if !found.IsAwakened() {
		t.Error("restored blizzender lost its awakening")
	}

	d2 := reloadDPS(t, d)
	if got := len(d2.Pals()); got != len(pals)-1 {
		t.Errorf("DPS pals after round-trip = %d, want %d", got, len(pals)-1)
	}
	if _, ok := d2.find(instance); ok {
		t.Error("restored instance still in DPS after round-trip")
	}
}

func TestMoveToDPSStoresPalAndSurvivesRoundTrip(t *testing.T) {
	w := loadLevelWorld(t)
	d := loadDPS(t)

	// pick an ordinary owned pal that sits in a palbox.
	var instance gvas.GUID
	var species string
	for _, c := range w.chars {
		if c.Pal.IsPlayer() || c.Pal.IsBoss() {
			continue
		}
		if _, ok := c.Pal.OwnerPlayerUID(); !ok {
			continue
		}
		instance = c.InstanceID
		species = c.Pal.CharacterID()
		break
	}
	if instance == (gvas.GUID{}) {
		t.Skip("no suitable pal to store")
	}
	t.Logf("storing %s (%s)", species, instance)

	beforeDPS := len(d.Pals())
	beforeChars := len(w.chars)
	beforeLocker := lockerLen(w)

	if err := w.MoveToDPS(d, instance); err != nil {
		t.Fatalf("MoveToDPS: %v", err)
	}

	if got := len(d.Pals()); got != beforeDPS+1 {
		t.Errorf("DPS pals = %d, want %d", got, beforeDPS+1)
	}
	if got := lockerLen(w); got != beforeLocker+1 {
		t.Errorf("locker len = %d, want %d", got, beforeLocker+1)
	}
	if len(w.chars) != beforeChars-1 {
		t.Errorf("world chars = %d, want %d", len(w.chars), beforeChars-1)
	}
	if hasChar(w, instance) {
		t.Error("stored instance still in world")
	}

	w2 := reloadWorld(t, w)
	if hasChar(w2, instance) {
		t.Error("stored instance back in world after round-trip")
	}
	d2 := reloadDPS(t, d)
	dp, ok := d2.find(instance)
	if !ok {
		t.Fatal("stored instance not in DPS after round-trip")
	}
	if dp.Pal.CharacterID() != species {
		t.Errorf("stored species = %q, want %q", dp.Pal.CharacterID(), species)
	}
}

func TestMoveRoundTripsBackToStart(t *testing.T) {
	w := loadLevelWorld(t)
	d := loadDPS(t)
	pals := d.Pals()
	if len(pals) == 0 {
		t.Skip("no stored pals")
	}
	instance := pals[0].InstanceID
	box := firstPalbox(t, w)

	if err := w.MoveFromDPS(d, instance, box.ID); err != nil {
		t.Fatalf("out: %v", err)
	}
	if err := w.MoveToDPS(d, instance); err != nil {
		t.Fatalf("back: %v", err)
	}
	if len(d.Pals()) != len(pals) {
		t.Errorf("DPS count = %d, want %d after out-and-back", len(d.Pals()), len(pals))
	}
	if _, ok := d.find(instance); !ok {
		t.Error("pal not back in DPS after out-and-back")
	}
	if hasChar(w, instance) {
		t.Error("pal still in world after out-and-back")
	}
	reloadWorld(t, w)
	reloadDPS(t, d)
}
