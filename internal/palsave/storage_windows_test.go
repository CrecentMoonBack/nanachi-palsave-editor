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
