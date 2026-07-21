package main

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/icons"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/oodle"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/paldata"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/palsave"
)

// fixture is a real server save. It is gitignored (it holds other players'
// Steam ids), so every test using it skips when it is absent.
const fixture = "testdata/Level.sav"

// TestRealSaveEveryPalIsNamedAndDrawn is the end-to-end guard for the pal
// grid, and it exists because two separate bugs got past everything else.
//
// The first drew a generic hooded silhouette for 169 pals, and passed every
// "does the artwork resolve?" check because the wrong file genuinely existed.
// The second left five species unnamed and unillustrated because the save
// spells ids differently from the table (Boss_ vs BOSS_, SheepBall vs
// Sheepball). Counting resolved ids caught neither: the first resolved to the
// wrong picture, the second was a rounding error at 5 of 216 species.
//
// So this asserts the two things a user actually sees — a Korean name and a
// species-specific picture — for every pal in a real save.
func TestRealSaveEveryPalIsNamedAndDrawn(t *testing.T) {
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("no save fixture; see scripts/setup.sh --all")
	}
	if !icons.Available() {
		t.Skip("no artwork; see scripts/fetch-icons.sh")
	}

	counts := speciesInSave(t)

	var unnamed, undrawn, generic []string
	for species := range counts {
		if _, ok := paldata.PalName(species); !ok {
			unnamed = append(unnamed, species)
		}
		icon := palIcon(species)
		if icon == "" {
			undrawn = append(undrawn, species)
			continue
		}
		// The generic portrait is only wrong on a real pal. A save also holds
		// human NPCs — merchants, the tamer at a respawn point — and for those
		// it is the correct picture and usually the only one they have.
		entry, known := paldata.LookupPal(species)
		if known && entry.IsPal && strings.HasPrefix(icon, "t_commonhuman") {
			generic = append(generic, species)
		}
	}
	sort.Strings(unnamed)
	sort.Strings(undrawn)
	sort.Strings(generic)

	if len(unnamed) > 0 {
		t.Errorf("%d species have no Korean name: %v", len(unnamed), unnamed)
	}
	if len(generic) > 0 {
		t.Errorf("%d species draw the generic human portrait: %v", len(generic), generic)
	}
	// Artwork genuinely absent upstream is tolerated — the UI falls back to a
	// text badge — but a jump here means a mapping broke, not that Pocketpair
	// shipped new pals.
	const knownUndrawn = 0
	if len(undrawn) > knownUndrawn {
		t.Errorf("%d species have no artwork, expected at most %d: %v",
			len(undrawn), knownUndrawn, undrawn)
	}
}

// speciesInSave decodes the fixture and returns every non-player species with
// a count.
func speciesInSave(t *testing.T) map[string]int {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := oodle.DecompressSav(data)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	opts := gvas.PalworldOptions()
	f, err := gvas.Decode(raw, opts)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	w, err := palsave.NewWorld(f, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Load(); err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for _, c := range w.Chars() {
		if !c.Pal.IsPlayer() {
			out[c.Pal.Species()]++
		}
	}
	if len(out) == 0 {
		t.Fatal("fixture holds no pals")
	}
	return out
}

func TestValidatePassives(t *testing.T) {
	cases := []struct {
		name    string
		ids     []string
		wantErr string // substring, empty means the list must be accepted
	}{
		{name: "empty clears the list", ids: nil},
		{
			name: "a full legitimate set",
			ids:  []string{"WorldTree_CraftSpeed", "CraftSpeed_up3", "PAL_CorporateSlave", "Vampire"},
		},
		{
			name:    "one over the cap",
			ids:     []string{"Legend", "Vampire", "CraftSpeed_up3", "CraftSpeed_up2", "PAL_CorporateSlave"},
			wantErr: "최대",
		},
		{
			name:    "duplicates",
			ids:     []string{"Legend", "Legend"},
			wantErr: "중복",
		},
		{
			name:    "unknown id",
			ids:     []string{"NotARealPassive"},
			wantErr: "알 수 없는",
		},
		{
			// Gear-only passives are deliberately absent from the table, so a
			// pal must not be able to hold one even though the game defines it.
			name:    "gear-only passive",
			ids:     []string{"AirDash_1"},
			wantErr: "알 수 없는",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validatePassives(c.ids)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("validatePassives(%v) = %v, want nil", c.ids, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validatePassives(%v) = nil, want error containing %q", c.ids, c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("validatePassives(%v) = %q, want it to mention %q", c.ids, err, c.wantErr)
			}
		})
	}
}

// TestDescribePassiveUnknownIDSurvives covers the case a save edited elsewhere
// produces: the trait must still come back with its raw id, because the UI
// writes back what it was given and a dropped entry would lose the trait.
func TestDescribePassiveUnknownIDSurvives(t *testing.T) {
	got := describePassive("SomeTraitFromAFutureUpdate")
	if got.ID != "SomeTraitFromAFutureUpdate" {
		t.Errorf("ID = %q, want the raw id back", got.ID)
	}
	if got.Name != "SomeTraitFromAFutureUpdate" {
		t.Errorf("Name = %q, want it to fall back to the raw id", got.Name)
	}
	if got.Known {
		t.Error("Known = true for an id the table does not have")
	}

	known := describePassive("WorldTree_CraftSpeed")
	if !known.Known || known.Name != "악마의 손" {
		t.Errorf("describePassive(WorldTree_CraftSpeed) = %+v, want 악마의 손 and Known", known)
	}
}

// TestEditableNamesAreRecognised pins the allow-lists to the property names
// palsave actually reads, so a rename on either side fails here rather than
// silently writing a junk property onto a pal.
func TestEditableNamesAreRecognised(t *testing.T) {
	for name := range allowedTalent {
		if !strings.HasPrefix(name, "Talent_") {
			t.Errorf("allowedTalent has %q, which is not a Talent_ property", name)
		}
	}
	for name := range allowedRankBonus {
		if !strings.HasPrefix(name, "Rank_") {
			t.Errorf("allowedRankBonus has %q, which is not a Rank_ property", name)
		}
	}
	if len(allowedTalent) != 4 || len(allowedRankBonus) != 4 {
		t.Errorf("want 4 talents and 4 soul stats, got %d and %d",
			len(allowedTalent), len(allowedRankBonus))
	}
}
