//go:build windows

package palsave

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/oodle"
)

// loadLevelWorld decodes the gitignored world fixture into a loaded World.
func loadLevelWorld(t *testing.T) *World {
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
	w, err := NewWorld(f, gvas.PalworldOptions())
	if err != nil {
		t.Fatalf("NewWorld: %v", err)
	}
	if err := w.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return w
}

// Camp storage is only useful if both ends of the link hold: the camp must be
// one the save knows, and the container must actually exist. A GUID-shaped run
// of bytes that satisfies neither would sail past a weaker check.
func TestCampStoragesLinkRealCampsAndContainers(t *testing.T) {
	w := loadLevelWorld(t)

	camps := map[gvas.GUID]bool{}
	for _, c := range w.BaseCamps() {
		camps[c.ID] = true
	}
	if len(camps) == 0 {
		t.Fatal("fixture has no base camps")
	}

	got := w.CampStorages()
	if len(got) == 0 {
		t.Fatal("no camp storage found; the offsets in storage.go are wrong")
	}

	seen := map[gvas.GUID]bool{}
	for _, s := range got {
		if !camps[s.CampID] {
			t.Errorf("%s: camp %v is not a base camp in this save", s.Kind, s.CampID)
		}
		if _, ok := w.Container(s.ContainerID); !ok {
			t.Errorf("%s: container %v does not exist", s.Kind, s.ContainerID)
		}
		if s.Kind == "" {
			t.Errorf("storage in camp %v has no kind", s.CampID)
		}
		if seen[s.ContainerID] {
			t.Errorf("container %v claimed by two structures", s.ContainerID)
		}
		seen[s.ContainerID] = true
	}
	t.Logf("%d storage structures across %d camps", len(got), len(camps))
}

// The world's own loot chests must not leak in. They outnumber camp storage
// roughly twenty to one, so if the camp filter broke, this is what would
// flood the UI.
func TestCampStoragesExcludeWorldChests(t *testing.T) {
	w := loadLevelWorld(t)
	for _, s := range w.CampStorages() {
		if s.Kind == "TreasureBox" {
			t.Fatalf("world loot chest leaked into camp storage (camp %v)", s.CampID)
		}
	}
}

// findDuplicateStacks looks for a container holding one item in two slots.
func findDuplicateStacks(w *World) (gvas.GUID, string, []ItemStack) {
	for _, s := range w.CampStorages() {
		byItem := map[string][]ItemStack{}
		for _, st := range w.ContainerContents(s.ContainerID) {
			byItem[st.ItemID] = append(byItem[st.ItemID], st)
		}
		for id, stacks := range byItem {
			if len(stacks) > 1 {
				return s.ContainerID, id, stacks
			}
		}
	}
	return gvas.GUID{}, "", nil
}

// A container holding the same item in two slots is ordinary — a chest with
// 9,999 crude oil and another 500 of it. SetItemCount cannot express "this
// one": it rewrites every matching stack, so editing the small one destroys
// the big one. SetSlotCount is what a click on a specific stack must use.
func TestSetSlotCountTouchesOnlyThatSlot(t *testing.T) {
	w := loadLevelWorld(t)

	cid, item, stacks := findDuplicateStacks(w)
	if len(stacks) < 2 {
		t.Skip("fixture has no container holding one item in two slots")
	}
	target, other := stacks[0], stacks[1]
	if other.Count == target.Count {
		t.Skipf("both %s stacks hold %d; the bug needs different counts", item, target.Count)
	}
	const want = 77

	if err := w.SetSlotCount(cid, target.Index, want); err != nil {
		t.Fatalf("SetSlotCount: %v", err)
	}

	got := map[int32]int32{}
	for _, s := range w.ContainerContents(cid) {
		got[s.Index] = s.Count
	}
	if got[target.Index] != want {
		t.Errorf("target slot %d holds %d, want %d", target.Index, got[target.Index], want)
	}
	if got[other.Index] != other.Count {
		t.Errorf("slot %d changed from %d to %d; SetSlotCount hit a slot it was not given",
			other.Index, other.Count, got[other.Index])
	}
}

// The old id-addressed call is kept for the toolbar, so pin what it actually
// does rather than leaving the difference implicit.
func TestSetItemCountRewritesEveryMatchingStack(t *testing.T) {
	w := loadLevelWorld(t)

	cid, item, stacks := findDuplicateStacks(w)
	if len(stacks) < 2 {
		t.Skip("fixture has no container holding one item in two slots")
	}
	n, err := w.SetItemCount(cid, item, 5)
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("SetItemCount reported %d stacks, expected at least 2", n)
	}
	for _, s := range w.ContainerContents(cid) {
		if s.ItemID == item && s.Count != 5 {
			t.Errorf("slot %d holds %d, expected every %s stack to be 5", s.Index, s.Count, item)
		}
	}
}

// Zero empties the slot rather than needing a separate remove call.
func TestSetSlotCountZeroEmptiesTheSlot(t *testing.T) {
	w := loadLevelWorld(t)
	for _, s := range w.CampStorages() {
		got := w.ContainerContents(s.ContainerID)
		if len(got) == 0 {
			continue
		}
		idx := got[0].Index
		if err := w.SetSlotCount(s.ContainerID, idx, 0); err != nil {
			t.Fatalf("SetSlotCount 0: %v", err)
		}
		for _, st := range w.ContainerContents(s.ContainerID) {
			if st.Index == idx {
				t.Fatalf("slot %d still listed with %d after being zeroed", idx, st.Count)
			}
		}
		return
	}
	t.Skip("no camp storage with contents")
}

// An index the container does not have must fail rather than silently do
// nothing, or a stale UI would report success for an edit that never landed.
func TestSetSlotCountRejectsUnknownSlot(t *testing.T) {
	w := loadLevelWorld(t)
	camps := w.CampStorages()
	if len(camps) == 0 {
		t.Skip("no camp storage")
	}
	if err := w.SetSlotCount(camps[0].ContainerID, 30000, 1); err == nil {
		t.Error("expected an error for a slot the container does not have")
	}
}
