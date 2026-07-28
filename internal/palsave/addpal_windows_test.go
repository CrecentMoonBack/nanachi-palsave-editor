//go:build windows

package palsave

import (
	"testing"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// palboxOf finds a player and their palbox: the container their existing pals
// are actually kept in.
func palboxOf(t *testing.T, w *World) (gvas.GUID, *PalContainer) {
	t.Helper()
	for _, p := range w.Players() {
		owned := w.PalsOwnedBy(p.PlayerUID)
		counts := map[gvas.GUID]int{}
		for _, pal := range owned {
			if cid, ok := pal.Pal.ContainerID(); ok {
				counts[cid]++
			}
		}
		var best gvas.GUID
		n := 0
		for cid, c := range counts {
			if c > n {
				best, n = cid, c
			}
		}
		if n == 0 {
			continue
		}
		if c, ok := w.PalContainerByID(best); ok && c.Capacity > int32(len(c.Slots)) {
			return p.PlayerUID, c
		}
	}
	t.Skip("no player with a palbox that has room")
	return gvas.GUID{}, nil
}

// The failure this guards against deletes pals. Adding several in a row used
// to leave exactly one alive, always at the same slot, because each new slot
// entry inherited the donor's SlotIndex and the game resolved the collision by
// throwing the others away — along with whatever already sat there.
//
// Counting is not enough to catch that, so this compares the *set* of instance
// ids before and after: every pal that existed must still exist.
func TestAddPalKeepsEveryExistingPal(t *testing.T) {
	w := loadLevelWorld(t)
	owner, box := palboxOf(t, w)

	before := map[gvas.GUID]bool{}
	for _, c := range w.Chars() {
		before[c.InstanceID] = true
	}
	beforeSlots := len(box.Slots)

	const n = 5
	added := map[gvas.GUID]bool{}
	indices := map[int32]bool{}
	for i := 0; i < n; i++ {
		id, err := w.AddPal(NewPalSpec{
			SpeciesID: "SheepBall",
			Level:     10,
			Rank:      1,
			Owner:     owner,
			Container: box.ID,
		})
		if err != nil {
			t.Fatalf("AddPal %d: %v", i, err)
		}
		if added[id] {
			t.Fatalf("AddPal returned instance id %v twice", id)
		}
		added[id] = true
	}

	// Re-read the container: the slots we just wrote have to be visible.
	box, ok := w.PalContainerByID(box.ID)
	if !ok {
		t.Fatal("the palbox vanished")
	}
	if got := len(box.Slots) - beforeSlots; got != n {
		t.Errorf("container gained %d slots, want %d", got, n)
	}

	// No two slots may claim the same number. This is the actual bug.
	for _, s := range box.Slots {
		if indices[s.Index] {
			t.Errorf("slot index %d is claimed twice", s.Index)
		}
		indices[s.Index] = true
	}

	after := map[gvas.GUID]bool{}
	for _, c := range w.Chars() {
		after[c.InstanceID] = true
	}
	for id := range before {
		if !after[id] {
			t.Errorf("pal %v disappeared when pals were added", id)
		}
	}
	for id := range added {
		if !after[id] {
			t.Errorf("newly added pal %v is not in the save", id)
		}
	}
}

// The slot entry and the pal's own SlotId have to agree. If they drift, the
// game trusts one of them and the pal is either invisible or fighting for a
// slot that belongs to somebody else.
func TestAddPalWritesTheSameSlotIndexInBothPlaces(t *testing.T) {
	w := loadLevelWorld(t)
	owner, box := palboxOf(t, w)

	id, err := w.AddPal(NewPalSpec{
		SpeciesID: "SheepBall",
		Level:     1,
		Rank:      1,
		Owner:     owner,
		Container: box.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	var slotIdx int32 = -1
	box, _ = w.PalContainerByID(box.ID)
	for _, s := range box.Slots {
		if s.Slot != nil && s.Slot.InstanceID == id {
			slotIdx = s.Index
		}
	}
	if slotIdx < 0 {
		t.Fatal("the new pal has no slot entry in the container")
	}

	for _, c := range w.Chars() {
		if c.InstanceID != id {
			continue
		}
		got, ok := palSlotIndex(c.Pal)
		if !ok {
			t.Fatal("the new pal has no SlotId")
		}
		if got != slotIdx {
			t.Errorf("slot entry says %d, the pal's own SlotId says %d", slotIdx, got)
		}
		cid, _ := c.Pal.ContainerID()
		if cid != box.ID {
			t.Errorf("the pal points at container %v, want %v", cid, box.ID)
		}
		return
	}
	t.Fatal("the new pal is not in the character map")
}

// palSlotIndex reads a pal's own idea of which slot it is in.
func palSlotIndex(p *Pal) (int32, bool) {
	v, ok := p.Params().Get("SlotId")
	if !ok {
		return 0, false
	}
	sp, ok := v.(*gvas.StructProperty)
	if !ok {
		return 0, false
	}
	inner, ok := sp.Value.(*gvas.StructProperties)
	if !ok {
		return 0, false
	}
	iv, ok := inner.Props.Get("SlotIndex")
	if !ok {
		return 0, false
	}
	ip, ok := iv.(*gvas.IntProperty)
	if !ok {
		return 0, false
	}
	return ip.Value, true
}

// The free slot must come from the slot entries. A pal that has left the box
// keeps a stale SlotId pointing back into it, so deriving free slots from the
// pals would skip numbers that are actually empty — and, worse, could hand out
// one that is occupied.
func TestFreeSlotIndexIsNotTakenFromPals(t *testing.T) {
	w := loadLevelWorld(t)
	_, box := palboxOf(t, w)

	free, ok := box.FreeSlotIndex()
	if !ok {
		t.Skip("palbox is full")
	}
	for _, s := range box.Slots {
		if s.Index == free {
			t.Fatalf("FreeSlotIndex returned %d, which a slot entry already holds", free)
		}
	}
	if free >= box.Capacity {
		t.Errorf("FreeSlotIndex returned %d, beyond capacity %d", free, box.Capacity)
	}
}

// A pal is only owned once the guild roster knows about it.
func TestAddPalRegistersInTheGuild(t *testing.T) {
	w := loadLevelWorld(t)
	owner, box := palboxOf(t, w)

	g, ok := w.GuildOf(owner)
	if !ok {
		t.Skip("that player is in no guild")
	}
	before := len(g.Members)

	if _, err := w.AddPal(NewPalSpec{
		SpeciesID: "SheepBall",
		Level:     1,
		Rank:      1,
		Owner:     owner,
		Container: box.ID,
	}); err != nil {
		t.Fatal(err)
	}

	g2, ok := w.GuildOf(owner)
	if !ok {
		t.Fatal("the guild stopped parsing after a pal was added")
	}
	// The roster is re-read from the blob, so a broken rewrite shows up as a
	// parse failure or a lost member rather than a silent no-op.
	if len(g2.Members) < before {
		t.Errorf("guild went from %d members to %d", before, len(g2.Members))
	}
	if g2.ID != g.ID {
		t.Errorf("guild id changed from %v to %v", g.ID, g2.ID)
	}
}

// The new pal has to be a pal: the donor's identity must not survive the copy.
func TestAddPalTakesTheRequestedShapeNotTheDonors(t *testing.T) {
	w := loadLevelWorld(t)
	owner, box := palboxOf(t, w)

	id, err := w.AddPal(NewPalSpec{
		SpeciesID: "SheepBall",
		Level:     42,
		Rank:      4,
		Talents:   map[string]int{"Talent_HP": 90},
		Owner:     owner,
		Container: box.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range w.Chars() {
		if c.InstanceID != id {
			continue
		}
		p := c.Pal
		if got := p.CharacterID(); got != "SheepBall" {
			t.Errorf("species is %q, want SheepBall", got)
		}
		if got := p.Level(); got != 42 {
			t.Errorf("level is %d, want 42", got)
		}
		if got := p.Rank(); got != 4 {
			t.Errorf("rank is %d, want 4", got)
		}
		if got := p.Talent("Talent_HP"); got != 90 {
			t.Errorf("Talent_HP is %d, want 90", got)
		}
		if o, ok := p.OwnerPlayerUID(); !ok || o != owner {
			t.Errorf("owner is %v, want %v", o, owner)
		}
		// The donor's moves are kept, so the pal is not left moveless — a pal
		// with no EquipWaza is treated as a broken record in-game (it will not
		// work, cannot be picked up, and is dropped on reload).
		if !p.Params().Has("EquipWaza") {
			t.Error("the new pal has no EquipWaza; a moveless pal is culled in-game")
		}
		if p.Params().Has("NickName") {
			t.Error("the donor's nickname survived the copy")
		}
		return
	}
	t.Fatal("the new pal is not in the character map")
}

// An added pal must be keyed like every other pal: a zero PlayerUId, with
// ownership carried in SaveParameter and the guild roster. A nonzero owner here
// made the game read the pal as out-of-guild — no base work, no pick-up, culled
// on reload — so this guards the exact regression.
func TestAddPalGivesTheRecordAZeroMapKey(t *testing.T) {
	w := loadLevelWorld(t)
	owner, box := palboxOf(t, w)

	id, err := w.AddPal(NewPalSpec{SpeciesID: "SheepBall", Level: 1, Rank: 1, Owner: owner, Container: box.ID})
	if err != nil {
		t.Fatal(err)
	}

	// Read the key back from re-encoded bytes, not the in-memory struct.
	w2 := reloadWorld(t, w)
	for _, c := range w2.chars {
		if c.InstanceID != id {
			continue
		}
		if !c.PlayerUID.IsZero() {
			t.Errorf("added pal map-key PlayerUId = %s, want zero", c.PlayerUID)
		}
		if o, ok := c.Pal.OwnerPlayerUID(); !ok || o != owner {
			t.Errorf("owner in SaveParameter = %v, want %v", o, owner)
		}
		return
	}
	t.Fatal("added pal not found after reload")
}

// An added pal must name only its owner in every player-reference field. The
// donor belonged to someone else, and a leftover cross-guild reference makes
// the game cull the pal on the owner's next login — the "made pals vanish"
// report. This guards that the donor's ids are scrubbed.
func TestAddPalNamesOnlyTheOwner(t *testing.T) {
	w := loadLevelWorld(t)
	owner, box := palboxOf(t, w)

	id, err := w.AddPal(NewPalSpec{SpeciesID: "SheepBall", Level: 1, Rank: 1, Owner: owner, Container: box.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range w.Chars() {
		if c.InstanceID != id {
			continue
		}
		p := c.Pal.Params()

		if v, ok := p.Get("LastNickNameModifierPlayerUid"); ok {
			if sp, ok := v.(*gvas.StructProperty); ok {
				if g, ok := sp.Value.(*gvas.GUIDValue); ok && gvas.GUID(*g) != owner {
					t.Errorf("LastNickNameModifierPlayerUid = %v, want owner %v", gvas.GUID(*g), owner)
				}
			}
		}
		if v, ok := p.Get("OldOwnerPlayerUIds"); ok {
			if a, ok := v.(*gvas.ArrayProperty); ok && a.Structs != nil {
				for _, sv := range a.Structs.Values {
					if g, ok := sv.(*gvas.GUIDValue); ok && gvas.GUID(*g) != owner {
						t.Errorf("OldOwnerPlayerUIds has a foreign ref %v, want owner %v", gvas.GUID(*g), owner)
					}
				}
			}
		}
		return
	}
	t.Fatal("added pal not found")
}

// Bad input must be refused rather than written.
func TestAddPalRejectsBadInput(t *testing.T) {
	w := loadLevelWorld(t)
	owner, box := palboxOf(t, w)

	cases := []struct {
		name string
		spec NewPalSpec
	}{
		{"no species", NewPalSpec{Rank: 1, Owner: owner, Container: box.ID}},
		{"rank 0", NewPalSpec{SpeciesID: "SheepBall", Rank: 0, Owner: owner, Container: box.ID}},
		{"rank 6", NewPalSpec{SpeciesID: "SheepBall", Rank: 6, Owner: owner, Container: box.ID}},
		{"unknown container", NewPalSpec{SpeciesID: "SheepBall", Rank: 1, Owner: owner}},
		{"too many passives", NewPalSpec{
			SpeciesID: "SheepBall", Rank: 1, Owner: owner, Container: box.ID,
			Passives: []string{"a", "b", "c", "d", "e"},
		}},
	}
	for _, tc := range cases {
		if _, err := w.AddPal(tc.spec); err == nil {
			t.Errorf("%s: expected an error", tc.name)
		}
	}
}

// Gender round-trips through the encoder, and only the two the game defines
// are accepted.
func TestSetGender(t *testing.T) {
	w := loadLevelWorld(t)
	var p *Pal
	for _, c := range w.Chars() {
		if !c.Pal.IsPlayer() && c.Pal.Gender() != "" {
			p = c.Pal
			break
		}
	}
	if p == nil {
		t.Skip("no pal with a gender in the fixture")
	}

	for _, g := range []string{GenderMale, GenderFemale} {
		if err := p.SetGender(g); err != nil {
			t.Fatalf("SetGender(%s): %v", g, err)
		}
		if got := p.Gender(); got != g {
			t.Errorf("Gender() = %q after setting %q", got, g)
		}
	}
	// The qualified form the save itself uses must also be accepted.
	if err := p.SetGender("EPalGenderType::Male"); err != nil {
		t.Errorf("qualified form rejected: %v", err)
	}
	for _, bad := range []string{"", "male", "Other", "EPalGenderType::Nope"} {
		if err := p.SetGender(bad); err == nil {
			t.Errorf("SetGender(%q) should have failed", bad)
		}
	}
}

// Alpha is the BOSS_ prefix and nothing else, so toggling it must not disturb
// the species the reference tables key on.
func TestSetAlphaIsJustThePrefix(t *testing.T) {
	w := loadLevelWorld(t)
	var p *Pal
	for _, c := range w.Chars() {
		if !c.Pal.IsPlayer() && !c.Pal.IsBoss() && c.Pal.CharacterID() != "" {
			p = c.Pal
			break
		}
	}
	if p == nil {
		t.Skip("no ordinary pal in the fixture")
	}
	species := p.Species()

	if err := p.SetAlpha(true); err != nil {
		t.Fatal(err)
	}
	if !p.IsBoss() {
		t.Error("IsBoss() is false after SetAlpha(true)")
	}
	if got := p.CharacterID(); got != "BOSS_"+species {
		t.Errorf("CharacterID = %q, want BOSS_%s", got, species)
	}
	if got := p.Species(); got != species {
		t.Errorf("Species changed to %q; the prefix must not alter it", got)
	}

	if err := p.SetAlpha(false); err != nil {
		t.Fatal(err)
	}
	if p.IsBoss() {
		t.Error("still a boss after SetAlpha(false)")
	}
	if got := p.CharacterID(); got != species {
		t.Errorf("CharacterID = %q, want %s", got, species)
	}
	// Toggling to the state it is already in is a no-op, not an error.
	if err := p.SetAlpha(false); err != nil {
		t.Errorf("no-op SetAlpha: %v", err)
	}
}

func TestDeletePalRemovesRecordAndSlot(t *testing.T) {
	w := loadLevelWorld(t)
	var inst gvas.GUID
	for _, c := range w.Chars() {
		if !c.Pal.IsPlayer() && c.InstanceID != (gvas.GUID{}) {
			inst = c.InstanceID
			break
		}
	}
	if inst == (gvas.GUID{}) {
		t.Skip("no pal in fixture")
	}

	before := len(w.Chars())
	if err := w.DeletePal(inst); err != nil {
		t.Fatal(err)
	}
	if got := len(w.Chars()); got != before-1 {
		t.Errorf("Chars count %d, want %d", got, before-1)
	}
	for _, c := range w.Chars() {
		if c.InstanceID == inst {
			t.Error("pal still in Chars() after delete")
		}
	}
	for _, cont := range w.PalContainers() {
		for _, s := range cont.Slots {
			if s.Slot != nil && s.Slot.InstanceID == inst {
				t.Error("slot still present after delete")
			}
		}
	}
	if err := w.DeletePal(inst); err == nil {
		t.Error("deleting an already-deleted pal should fail")
	}
}

func TestDeletePalRefusesPlayer(t *testing.T) {
	w := loadLevelWorld(t)
	var player gvas.GUID
	for _, c := range w.Chars() {
		if c.Pal.IsPlayer() {
			player = c.InstanceID
			break
		}
	}
	if player == (gvas.GUID{}) {
		t.Skip("no player in fixture")
	}
	if err := w.DeletePal(player); err == nil {
		t.Error("DeletePal must refuse a player character")
	}
}

// The embedded donor is what lets a pal be added to a save that holds none of
// its own. It ships in a public binary, so it must decode and carry no owner id.
func TestTemplateDonorDecodesAndCarriesNoOwner(t *testing.T) {
	w := loadLevelWorld(t)
	p, err := w.templateDonor()
	if err != nil {
		t.Fatal(err)
	}
	if p.CharacterID() == "" {
		t.Error("template donor has no CharacterID")
	}
	// The template must carry moves, or a pal added to an empty save would be
	// left moveless — the very state the game culls.
	if !p.Params().Has("EquipWaza") {
		t.Error("template donor has no EquipWaza; empty-save adds would be moveless")
	}
	if id, ok := p.OwnerPlayerUID(); ok && id != (gvas.GUID{}) {
		t.Errorf("template donor carries a non-zero owner %v; it ships publicly", id)
	}
	if _, err := p.Raw.Encode(); err != nil {
		t.Fatalf("template donor does not round-trip: %v", err)
	}
}

func TestSetAwakenedTogglesTheFlagAndRemovesItWhenOff(t *testing.T) {
	w := loadLevelWorld(t)
	var p *Pal
	for _, c := range w.Chars() {
		if !c.Pal.IsPlayer() && c.Pal.CharacterID() != "" {
			p = c.Pal
			break
		}
	}
	if p == nil {
		t.Skip("no pal in the fixture")
	}

	if err := p.SetAwakened(true); err != nil {
		t.Fatal(err)
	}
	if !p.IsAwakened() {
		t.Error("IsAwakened() is false after SetAwakened(true)")
	}

	// Off must remove the field, not write false: the game omits it on an
	// un-awakened pal, so a lingering bIsAwakening=false would not match a
	// record the game produces.
	if err := p.SetAwakened(false); err != nil {
		t.Fatal(err)
	}
	if p.IsAwakened() {
		t.Error("still awakened after SetAwakened(false)")
	}
	if p.params.Has(awakeningField) {
		t.Error("bIsAwakening left in the record after SetAwakened(false); it should be removed")
	}
}
