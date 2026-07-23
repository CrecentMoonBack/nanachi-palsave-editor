package main

import (
	"encoding/json"
	"os"
	"regexp"
	"path/filepath"
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

// TestBaseCampsAreGuildScopedAndComplete pins the two ways deriving camps from
// pals got them wrong.
//
// Camps used to be found by grouping ownerless pals by container. That looks
// equivalent and is not: it cannot see a camp with no workers, and it has no
// idea whose camp it is, so on a shared server every guild's camps showed up
// in everyone's list. The fixture has two guilds — four camps and two — and
// one of the four is empty, so both faults are visible in it.
func TestBaseCampsAreGuildScopedAndComplete(t *testing.T) {
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("no save fixture; see scripts/setup.sh --all")
	}
	a := NewApp()
	if _, err := a.OpenSave(fixture); err != nil {
		t.Fatal(err)
	}
	players, err := a.Players()
	if err != nil {
		t.Fatal(err)
	}

	seen := map[int]bool{} // camp counts observed, keyed by count
	var sawEmpty bool      // a camp with no workers survived the trip
	for _, p := range players {
		camps, err := a.BaseCamps(p.UID)
		if err != nil {
			t.Fatalf("BaseCamps(%s): %v", p.Name, err)
		}
		pals, err := a.BasePals(p.UID)
		if err != nil {
			t.Fatalf("BasePals(%s): %v", p.Name, err)
		}

		total := 0
		for _, c := range camps {
			total += c.PalCount
			if c.PalCount == 0 {
				sawEmpty = true
			}
		}
		if total != len(pals) {
			t.Errorf("%s: camps hold %d pals but BasePals returned %d",
				p.Name, total, len(pals))
		}
		// Every pal must land in one of the camps offered, or the filter and
		// the listing disagree about which guild is being shown.
		for _, x := range pals {
			if x.Camp < 1 || x.Camp > len(camps) {
				t.Errorf("%s: pal %s has camp %d, outside 1..%d",
					p.Name, x.Name, x.Camp, len(camps))
			}
		}
		seen[len(camps)] = true
	}

	if !sawEmpty {
		t.Error("no empty camp in the fixture; a camp with no workers is the case that used to vanish")
	}
	if len(seen) < 2 {
		t.Errorf("every player saw the same number of camps (%v); the fixture has two guilds with different camp counts, so this is not scoping by guild", seen)
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

// TestBaseStoragesAreGuildScopedAndAddressable covers the storage view the
// same way camps are covered: it must show this guild's boxes and no one
// else's, and every box it offers must be one the item calls can actually
// write to.
func TestBaseStoragesAreGuildScopedAndAddressable(t *testing.T) {
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("no save fixture; see scripts/setup.sh --all")
	}
	a := NewApp()
	if _, err := a.OpenSave(fixture); err != nil {
		t.Fatal(err)
	}
	players, err := a.Players()
	if err != nil {
		t.Fatal(err)
	}

	counts := map[int]bool{}
	sawItems := false
	for _, p := range players {
		camps, err := a.BaseCamps(p.UID)
		if err != nil {
			t.Fatal(err)
		}
		boxes, err := a.BaseStorages(p.UID)
		if err != nil {
			t.Fatalf("BaseStorages(%s): %v", p.Name, err)
		}
		for _, b := range boxes {
			if b.Camp < 1 || b.Camp > len(camps) {
				t.Errorf("%s: box %s in camp %d, outside 1..%d",
					p.Name, b.Kind, b.Camp, len(camps))
			}
			if b.Kind == "" {
				t.Errorf("%s: box in camp %d has no kind", p.Name, b.Camp)
			}
			// The id the UI is handed must round-trip, or every edit from the
			// storage view fails at the first click.
			if _, err := a.namedContainer(b.ContainerID); err != nil {
				t.Errorf("%s: box %s id %q not addressable: %v",
					p.Name, b.Kind, b.ContainerID, err)
			}
			if len(b.Items) > 0 {
				sawItems = true
			}
		}
		counts[len(boxes)] = true
	}

	if !sawItems {
		t.Error("no camp box held any items; the container link is probably wrong")
	}
	if len(counts) < 2 {
		t.Errorf("every player saw the same number of boxes (%v); the fixture has two guilds, so this is not scoped by guild", counts)
	}
}

// copyFixture puts the save and its Players folder in a temp dir so a test can
// write to it. SaveToDisk overwrites the file it opened, so this is what keeps
// the real fixture untouched.
func copyFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	level := filepath.Join(dir, "Level.sav")
	b, err := os.ReadFile(fixture)
	if err != nil {
		t.Skipf("no save fixture: %v", err)
	}
	if err := os.WriteFile(level, b, 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join("testdata", "Players")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Skipf("no player fixtures: %v", err)
	}
	dst := filepath.Join(dir, "Players")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return level
}

// TestCampStorageEditSurvivesSaveAndReopen is the round trip that reading the
// save back in my own process does not prove: write an item into a base camp
// box, save, and open the file fresh. Everything up to Flush can look right
// while the change never reaches the disk.
func TestCampStorageEditSurvivesSaveAndReopen(t *testing.T) {
	if err := oodle.Available(); err != nil {
		t.Skipf("native codec unavailable: %v", err)
	}
	level := copyFixture(t)

	const item = "PalSphere"
	const want = 4321

	a := NewApp()
	if _, err := a.OpenSave(level); err != nil {
		t.Fatal(err)
	}
	players, err := a.Players()
	if err != nil {
		t.Fatal(err)
	}
	var target StorageInfo
	var owner string
	for _, p := range players {
		boxes, err := a.BaseStorages(p.UID)
		if err != nil || len(boxes) == 0 {
			continue
		}
		target, owner = boxes[0], p.UID
		break
	}
	if owner == "" {
		t.Skip("fixture has no base camp storage")
	}

	before := int32(0)
	for _, it := range target.Items {
		if it.ItemID == item {
			before += it.Count
		}
	}

	if _, err := a.GiveContainerItem(target.ContainerID, item, want); err != nil {
		t.Fatalf("GiveContainerItem: %v", err)
	}
	if _, err := a.SaveToDisk(); err != nil {
		t.Fatalf("SaveToDisk: %v", err)
	}

	// A brand new App, reading the bytes that actually landed on disk.
	b := NewApp()
	if _, err := b.OpenSave(level); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	boxes, err := b.BaseStorages(owner)
	if err != nil {
		t.Fatal(err)
	}
	var got int32
	found := false
	for _, box := range boxes {
		if box.ContainerID != target.ContainerID {
			continue
		}
		found = true
		for _, it := range box.Items {
			if it.ItemID == item {
				got += it.Count
			}
		}
	}
	if !found {
		t.Fatalf("box %s vanished after reopen", target.ContainerID)
	}
	// The delta, not the total: a box that already held these would let a
	// no-op write pass a "want at least" check.
	if got-before != want {
		t.Errorf("after reopen the box gained %d %s, want exactly %d (before=%d after=%d)",
			got-before, item, want, before, got)
	}
}

// TestSetSlotCountEditsOneStackOfTwo is the reported bug at the app layer: a
// box holding the same item in two slots could not be edited, because the
// id-addressed call rewrote both and destroyed the larger stack.
func TestSetSlotCountEditsOneStackOfTwo(t *testing.T) {
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("no save fixture; see scripts/setup.sh --all")
	}
	a := NewApp()
	if _, err := a.OpenSave(fixture); err != nil {
		t.Fatal(err)
	}
	players, err := a.Players()
	if err != nil {
		t.Fatal(err)
	}

	// Find a box holding one item twice.
	var box StorageInfo
	var dup []ItemInfo
	for _, p := range players {
		boxes, err := a.BaseStorages(p.UID)
		if err != nil {
			continue
		}
		for _, b := range boxes {
			byItem := map[string][]ItemInfo{}
			for _, it := range b.Items {
				byItem[it.ItemID] = append(byItem[it.ItemID], it)
			}
			for _, stacks := range byItem {
				if len(stacks) > 1 && stacks[0].Count != stacks[1].Count {
					box, dup = b, stacks
					break
				}
			}
			if dup != nil {
				break
			}
		}
		if dup != nil {
			break
		}
	}
	if dup == nil {
		t.Skip("fixture has no box holding one item in two differently sized stacks")
	}

	target, other := dup[0], dup[1]
	const want = 77
	if err := a.SetSlotCount(box.ContainerID, int(target.Slot), want); err != nil {
		t.Fatalf("SetSlotCount: %v", err)
	}

	got := map[int32]int32{}
	for _, it := range a.describeContainer(mustGUID(t, box.ContainerID)) {
		got[it.Slot] = it.Count
	}
	if got[target.Slot] != want {
		t.Errorf("slot %d holds %d, want %d", target.Slot, got[target.Slot], want)
	}
	if got[other.Slot] != other.Count {
		t.Errorf("the other stack in slot %d went from %d to %d; editing one stack must not touch its twin",
			other.Slot, other.Count, got[other.Slot])
	}
}

func mustGUID(t *testing.T, s string) gvas.GUID {
	t.Helper()
	g, err := gvas.ParseGUID(s)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// TestAddPalSurvivesSaveAndReopen is the test the whole feature rests on.
//
// Every in-memory check can pass while the encoder drops the new map entry, so
// this writes the file and opens it with a fresh App. It adds several pals at
// once because the failure being guarded against — every new slot entry
// inheriting the donor's SlotIndex — only shows up with more than one: the
// game keeps a single pal at the contested slot and deletes the rest, along
// with whatever was already there.
func TestAddPalSurvivesSaveAndReopen(t *testing.T) {
	if err := oodle.Available(); err != nil {
		t.Skipf("native codec unavailable: %v", err)
	}
	level := copyFixture(t)

	a := NewApp()
	if _, err := a.OpenSave(level); err != nil {
		t.Fatal(err)
	}
	players, err := a.Players()
	if err != nil {
		t.Fatal(err)
	}

	// A player whose palbox we can reach and that has room.
	var uid, name string
	for _, p := range players {
		if n, err := a.PalboxSpace(p.UID); err == nil && n > 10 {
			uid, name = p.UID, p.Name
			break
		}
	}
	if uid == "" {
		t.Skip("no player with a reachable palbox that has room")
	}

	beforeIDs := map[string]bool{}
	for _, c := range a.world.Chars() {
		beforeIDs[c.InstanceID.String()] = true
	}
	beforeSpace, _ := a.PalboxSpace(uid)

	const n = 5
	want := map[string]bool{}
	for i := 0; i < n; i++ {
		id, err := a.AddPal(uid, "SheepBall", 30, 3,
			map[string]int{"Talent_HP": 80}, nil, false, "")
		if err != nil {
			t.Fatalf("AddPal %d: %v", i, err)
		}
		want[id] = true
	}
	if len(want) != n {
		t.Fatalf("only %d distinct instance ids from %d adds", len(want), n)
	}
	if got, _ := a.PalboxSpace(uid); got != beforeSpace-n {
		t.Errorf("palbox space went %d -> %d, want %d", beforeSpace, got, beforeSpace-n)
	}

	if _, err := a.SaveToDisk(); err != nil {
		t.Fatalf("SaveToDisk: %v", err)
	}

	// Fresh process-equivalent: read back what actually landed on disk.
	b := NewApp()
	if _, err := b.OpenSave(level); err != nil {
		t.Fatalf("reopen: %v", err)
	}

	afterIDs := map[string]bool{}
	slotOf := map[string]int32{}
	for _, c := range b.world.Chars() {
		afterIDs[c.InstanceID.String()] = true
	}
	for id := range beforeIDs {
		if !afterIDs[id] {
			t.Errorf("pal %s was lost across save and reopen", id)
		}
	}
	for id := range want {
		if !afterIDs[id] {
			t.Errorf("added pal %s did not survive save and reopen", id)
		}
	}

	// Every slot in the palbox must still be claimed by exactly one entry.
	pf := b.players[uid]
	box, _ := pf.save.PalStorageContainer()
	c, ok := b.world.PalContainerByID(box)
	if !ok {
		t.Fatal("palbox missing after reopen")
	}
	seen := map[int32]bool{}
	for _, s := range c.Slots {
		if seen[s.Index] {
			t.Errorf("after reopen, slot %d is claimed twice", s.Index)
		}
		seen[s.Index] = true
		if s.Slot != nil {
			slotOf[s.Slot.InstanceID.String()] = s.Index
		}
	}
	for id := range want {
		if _, ok := slotOf[id]; !ok {
			t.Errorf("added pal %s has no slot entry after reopen", id)
		}
	}

	// And the pals must read back as what was asked for.
	for _, ch := range b.world.Chars() {
		if !want[ch.InstanceID.String()] {
			continue
		}
		if got := ch.Pal.CharacterID(); got != "SheepBall" {
			t.Errorf("%s: species %q after reopen", ch.InstanceID, got)
		}
		if got := ch.Pal.Level(); got != 30 {
			t.Errorf("%s: level %d after reopen, want 30", ch.InstanceID, got)
		}
		if got := ch.Pal.Rank(); got != 3 {
			t.Errorf("%s: rank %d after reopen, want 3", ch.InstanceID, got)
		}
	}
	t.Logf("%s: added %d pals, %d characters before, %d after",
		name, n, len(beforeIDs), len(afterIDs))
}

// The species picker opens on whatever SearchPals returns first, so ordinary
// pals have to come before the tower and raid bosses. Several tower bosses
// share a Korean name, and an unsorted list opens on a wall of them.
func TestSearchPalsPutsOrdinaryPalsFirst(t *testing.T) {
	a := NewApp()
	got := a.SearchPals("")
	if len(got) < 10 {
		t.Fatalf("only %d results for an empty query", len(got))
	}
	for i, c := range got[:10] {
		p, ok := paldata.LookupPal(c.ID)
		if !ok {
			t.Errorf("result %d (%s) is not in the pal table", i, c.ID)
			continue
		}
		if p.IsTowerBoss || p.IsRaidBoss {
			t.Errorf("result %d is a boss (%s); ordinary pals must come first", i, c.ID)
		}
		if !p.IsPal {
			t.Errorf("result %d (%s) is not a pal", i, c.ID)
		}
	}
}

// nullField finds a JSON field whose value is null.
var nullField = regexp.MustCompile(`"(\w+)":null`)

// nullCollector returns a sink for one endpoint's result. Go only spreads a
// multi-value call when it is the sole argument, so the label is bound first
// and the (value, error) pair is passed on its own.
func nullCollector(t *testing.T, found map[string]int, what string) func(any, error) {
	t.Helper()
	return func(v any, err error) {
		if err != nil {
			return
		}
		b, e := json.Marshal(v)
		if e != nil {
			t.Errorf("%s: marshal: %v", what, e)
			return
		}
		for _, m := range nullField.FindAllStringSubmatch(string(b), -1) {
			found[what+"."+m[1]]++
		}
	}
}

// TestNoAPIReturnsNullSlices is the bug that turned the window black.
//
// A nil Go slice marshals to JSON null. The generated TypeScript still types
// the field as an array, so the UI calls .length on it, React throws during
// render, and the entire app blanks — not just the pal that was clicked.
// Fifty pals in the live save carry no passive skills, so every one of them
// was a black screen waiting to happen.
//
// Checking one field would only fix the one already found, so this walks the
// JSON of every read endpoint and fails on any null at all.
func TestNoAPIReturnsNullSlices(t *testing.T) {
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("no save fixture; see scripts/setup.sh --all")
	}
	a := NewApp()
	if _, err := a.OpenSave(fixture); err != nil {
		t.Fatal(err)
	}
	players, err := a.Players()
	if err != nil {
		t.Fatal(err)
	}

	found := map[string]int{}
	sink := func(what string) func(any, error) { return nullCollector(t, found, what) }
	for _, p := range players {
		sink("Pals")(a.Pals(p.UID))
		sink("PalSpecies")(a.PalSpecies(p.UID))
		sink("BaseCamps")(a.BaseCamps(p.UID))
		sink("BasePals")(a.BasePals(p.UID))
		sink("BaseStorages")(a.BaseStorages(p.UID))
		sink("Inventory")(a.Inventory(p.UID))
		sink("PlayerDetail")(a.PlayerDetail(p.UID))
		sink("Relics")(a.Relics(p.UID))
	}
	sink("SearchPals")(a.SearchPals(""), nil)
	sink("SearchItems")(a.SearchItems("wood"), nil)
	sink("SearchPassives")(a.SearchPassives(""), nil)
	sink("Presets")(a.Presets())

	if len(found) == 0 {
		return
	}
	keys := make([]string, 0, len(found))
	for k := range found {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Errorf("%s serialises as null %d times; the UI treats it as an array", k, found[k])
	}
}

// TestBasePalsIsEmptyNotNullForAStranger covers the crash a user hit:
// "TypeError: g is not iterable" on clicking a player.
//
// The UI does [...roster, ...basePals], and a nil Go slice marshals to JSON
// null, which is not iterable. BasePals returned nil whenever the player's
// guild had no base pals — my own fixture has some for everyone, which is why
// the earlier null-slice test passed while users still crashed. An unknown uid
// reaches the same empty path deterministically.
func TestBasePalsIsEmptyNotNullForAStranger(t *testing.T) {
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("no save fixture; see scripts/setup.sh --all")
	}
	a := NewApp()
	if _, err := a.OpenSave(fixture); err != nil {
		t.Fatal(err)
	}

	const stranger = "00000000-0000-4000-8000-000000000000"
	got, err := a.BasePals(stranger)
	if err != nil {
		t.Fatalf("BasePals: %v", err)
	}
	if got == nil {
		t.Fatal("BasePals returned nil; it marshals to null and the UI spreads it")
	}
	b, _ := json.Marshal(got)
	if string(b) != "[]" {
		t.Errorf("BasePals serialised as %s, want []", b)
	}

	boxes, err := a.BaseStorages(stranger)
	if err != nil {
		t.Fatal(err)
	}
	if boxes == nil {
		t.Error("BaseStorages returned nil")
	}
}

// An alpha created through the app must still be an alpha after the save has
// been written and reopened, and must remain the species that was asked for —
// the BOSS_ prefix changes CharacterID, which is what the reference tables and
// the icon lookup key on.
func TestAddAlphaPalSurvivesSaveAndReopen(t *testing.T) {
	if err := oodle.Available(); err != nil {
		t.Skipf("native codec unavailable: %v", err)
	}
	level := copyFixture(t)

	a := NewApp()
	if _, err := a.OpenSave(level); err != nil {
		t.Fatal(err)
	}
	players, err := a.Players()
	if err != nil {
		t.Fatal(err)
	}
	var uid string
	for _, p := range players {
		if n, err := a.PalboxSpace(p.UID); err == nil && n > 2 {
			uid = p.UID
			break
		}
	}
	if uid == "" {
		t.Skip("no player with a reachable palbox")
	}

	id, err := a.AddPal(uid, "SheepBall", 25, 2, nil, nil, true, "Female")
	if err != nil {
		t.Fatalf("AddPal alpha: %v", err)
	}
	if _, err := a.SaveToDisk(); err != nil {
		t.Fatal(err)
	}

	b := NewApp()
	if _, err := b.OpenSave(level); err != nil {
		t.Fatal(err)
	}
	for _, c := range b.world.Chars() {
		if c.InstanceID.String() != id {
			continue
		}
		if !c.Pal.IsBoss() {
			t.Errorf("not an alpha after reopen: CharacterID=%q", c.Pal.CharacterID())
		}
		if got := c.Pal.Species(); got != "SheepBall" {
			t.Errorf("species is %q after reopen, want SheepBall", got)
		}
		if got := c.Pal.Gender(); got != "Female" {
			t.Errorf("gender is %q after reopen, want Female", got)
		}
		// The UI must still be able to name and draw it.
		info := b.describePal(uid, c)
		if info.Name == "" || info.Name == info.SpeciesID {
			t.Errorf("alpha has no Korean name: %q", info.Name)
		}
		if !info.IsBoss {
			t.Error("describePal does not report it as an alpha")
		}
		return
	}
	t.Fatal("the alpha is not in the reopened save")
}

// Active skill editing must show up in Pals, apply, and survive a reopen.
func TestSetPalSkillsRoundTripsThroughApp(t *testing.T) {
	if err := oodle.Available(); err != nil {
		t.Skipf("codec unavailable: %v", err)
	}
	level := copyFixture(t)

	a := NewApp()
	if _, err := a.OpenSave(level); err != nil {
		t.Fatal(err)
	}
	players, _ := a.Players()

	// Find a pal that has equipped skills, via the API the UI uses.
	var uid, inst string
	for _, p := range players {
		pals, err := a.Pals(p.UID)
		if err != nil {
			continue
		}
		for _, x := range pals {
			if x.Skills == nil {
				t.Fatalf("%s: Skills is nil (marshals to null, crashes UI)", x.SpeciesID)
			}
			if len(x.Skills) > 0 && uid == "" {
				uid, inst = p.UID, x.InstanceID
			}
		}
	}
	if inst == "" {
		t.Skip("no pal with equipped skills")
	}

	want := []string{"AcidRain", "PowerBall", "IceMissile"}
	if err := a.SetPalSkills(inst, want); err != nil {
		t.Fatalf("SetPalSkills: %v", err)
	}
	if err := a.SetPalSkills(inst, []string{"NopeNotASkill"}); err == nil {
		t.Error("unknown skill should be refused")
	}
	if _, err := a.SaveToDisk(); err != nil {
		t.Fatal(err)
	}

	b := NewApp()
	if _, err := b.OpenSave(level); err != nil {
		t.Fatal(err)
	}
	pals, _ := b.Pals(uid)
	for _, x := range pals {
		if x.InstanceID != inst {
			continue
		}
		got := make([]string, len(x.Skills))
		for i, s := range x.Skills {
			got[i] = s.ID
			if s.Name == "" {
				t.Errorf("skill %s has no display name", s.ID)
			}
		}
		if len(got) != 3 {
			t.Fatalf("after reopen %d skills, want 3: %v", len(got), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("skill %d is %q, want %q", i, got[i], want[i])
			}
		}
		return
	}
	t.Fatal("edited pal not found after reopen")
}
