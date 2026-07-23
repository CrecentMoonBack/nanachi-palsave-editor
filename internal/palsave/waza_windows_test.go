//go:build windows

package palsave

import (
	"testing"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// firstPalWithWaza returns a pal that has an EquipWaza array.
func firstPalWithWaza(t *testing.T, w *World) *Pal {
	t.Helper()
	for _, c := range w.Chars() {
		if !c.Pal.IsPlayer() && len(c.Pal.EquipWaza()) > 0 {
			return c.Pal
		}
	}
	t.Skip("no pal with equipped skills")
	return nil
}

func TestEquipWazaRoundTrips(t *testing.T) {
	w := loadLevelWorld(t)
	p := firstPalWithWaza(t, w)

	// Read strips the prefix.
	for _, id := range p.EquipWaza() {
		if len(id) > len(wazaPrefix) && id[:len(wazaPrefix)] == wazaPrefix {
			t.Errorf("EquipWaza returned a prefixed id: %q", id)
		}
	}

	want := []string{"AcidRain", "IceMissile", "PowerBall"}
	if err := p.SetEquipWaza(want); err != nil {
		t.Fatal(err)
	}
	// Equipped skills must also be mastered.
	mastered := map[string]bool{}
	for _, m := range p.MasteredWaza() {
		mastered[m] = true
	}
	for _, s := range want {
		if !mastered[s] {
			t.Errorf("%s equipped but not mastered", s)
		}
	}

	// Round trip through the encoder.
	out := reencode(t, p, gvas.PalworldOptions())
	got := out.EquipWaza()
	if len(got) != len(want) {
		t.Fatalf("after reopen have %d skills, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("skill %d is %q, want %q", i, got[i], want[i])
		}
	}
	// And they are stored qualified.
	raw := out.wazaStrings("EquipWaza")
	for _, s := range raw {
		if len(s.Value) < len(wazaPrefix) || s.Value[:len(wazaPrefix)] != wazaPrefix {
			t.Errorf("stored value is not qualified: %q", s.Value)
		}
	}
}

func TestSetEquipWazaRejectsTooMany(t *testing.T) {
	w := loadLevelWorld(t)
	p := firstPalWithWaza(t, w)
	if err := p.SetEquipWaza([]string{"A", "B", "C", "D"}); err == nil {
		t.Error("expected rejection of 4 skills")
	}
}
