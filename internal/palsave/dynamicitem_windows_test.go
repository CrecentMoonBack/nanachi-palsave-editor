//go:build windows

package palsave

import (
	"testing"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// findWeaponItem returns the id of a non-stackable item the fixture already
// has a dynamic record for, and a container to put a copy in.
func findWeaponItem(t *testing.T, w *World) (string, gvas.GUID) {
	t.Helper()
	d, ok := w.loadDynamicItems()
	if !ok {
		t.Skip("no DynamicItemSaveData in fixture")
	}
	var item string
	for _, want := range []string{"Bat2", "Pickaxe_Tier_01", "BowGun_4", "Shield_01"} {
		if _, ok := d.byItem[want]; ok {
			item = want
			break
		}
	}
	if item == "" {
		t.Skip("fixture has no known weapon dynamic record")
	}
	// A container with room to spare, so appends do not hit capacity.
	counts := map[gvas.GUID]int{}
	for _, s := range w.itemSlots {
		counts[s.ContainerID]++
	}
	for id := range counts {
		if c, ok := w.Container(id); ok && c.Capacity-int32(len(c.Slots)) >= 3 {
			return item, id
		}
	}
	t.Skip("no container with free room")
	return "", gvas.GUID{}
}

// A weapon given by the editor must come with a per-instance record, or the
// game cannot reload it. This checks the slot gets a nonzero LocalID and a
// matching DynamicItemSaveData entry appears — and that both survive a full
// encode/decode of the world.
func TestGiveInstancedItemCreatesDynamicRecord(t *testing.T) {
	w := loadLevelWorld(t)
	item, cid := findWeaponItem(t, w)

	dBefore, _ := w.loadDynamicItems()
	countBefore := len(dBefore.arr.Structs.Values)

	idx, err := w.GiveInstancedItem(cid, item, item)
	if err != nil {
		t.Fatalf("GiveInstancedItem: %v", err)
	}

	// The new slot must carry a nonzero LocalID.
	var localID gvas.GUID
	for _, s := range w.SlotsInContainer(cid) {
		if s.Slot != nil && s.Slot.Index == idx {
			localID = s.Slot.LocalID
		}
	}
	var zero gvas.GUID
	if localID == zero {
		t.Fatal("new weapon slot has a zero LocalID")
	}

	// A dynamic record with that LocalID must now exist.
	dAfter, _ := w.loadDynamicItems()
	if got := len(dAfter.arr.Structs.Values); got != countBefore+1 {
		t.Errorf("dynamic entries went %d -> %d, want +1", countBefore, got)
	}
	if !dynamicRecordExists(dAfter, localID, item) {
		t.Errorf("no dynamic record for the new weapon's LocalID")
	}

	// Now the real test: encode the whole world and read it back. The dynamic
	// entry and the slot's LocalID both have to survive.
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	b, err := gvas.Encode(w.File)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	f2, err := gvas.Decode(b, gvas.PalworldOptions())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	w2, err := NewWorld(f2, gvas.PalworldOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := w2.Load(); err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, s := range w2.SlotsInContainer(cid) {
		if s.Slot != nil && s.Slot.LocalID == localID {
			found = true
		}
	}
	if !found {
		t.Error("after reopen, no slot carries the weapon's LocalID")
	}
	d2, _ := w2.loadDynamicItems()
	if !dynamicRecordExists(d2, localID, item) {
		t.Error("after reopen, the weapon's dynamic record is gone")
	}
}

func dynamicRecordExists(d *dynamicItems, localID gvas.GUID, item string) bool {
	for _, sv := range d.arr.Structs.Values {
		props, ok := sv.(*gvas.StructProperties)
		if !ok {
			continue
		}
		raw, ok := rawDataArray(props.Props)
		if !ok {
			continue
		}
		lid, ok := dynamicLocalID(raw.Values.Bytes)
		if !ok || lid != localID {
			continue
		}
		id, _ := dynamicItemID(raw.Values.Bytes)
		return id == item
	}
	return false
}

// Giving an item the save has never held cannot fabricate a state record, so
// it must fail loudly rather than write a weapon the game will choke on.
func TestGiveInstancedItemRefusesUnknownItem(t *testing.T) {
	w := loadLevelWorld(t)
	_, cid := findWeaponItem(t, w)
	if _, err := w.GiveInstancedItem(cid, "Totally_Made_Up_Weapon_XYZ", "Totally_Made_Up_Weapon_XYZ"); err == nil {
		t.Error("expected an error giving an item with no dynamic record to clone")
	}
}

// Two weapons of the same kind are two objects: distinct slots, distinct
// LocalIDs, distinct dynamic records. They must not merge.
func TestInstancedItemsDoNotStack(t *testing.T) {
	w := loadLevelWorld(t)
	item, cid := findWeaponItem(t, w)

	i1, err := w.GiveInstancedItem(cid, item, item)
	if err != nil {
		t.Fatal(err)
	}
	i2, err := w.GiveInstancedItem(cid, item, item)
	if err != nil {
		t.Fatal(err)
	}
	if i1 == i2 {
		t.Fatalf("both weapons landed in slot %d; they must not stack", i1)
	}
	ids := map[gvas.GUID]bool{}
	for _, s := range w.SlotsInContainer(cid) {
		if s.Slot != nil && (s.Slot.Index == i1 || s.Slot.Index == i2) {
			ids[s.Slot.LocalID] = true
		}
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 distinct LocalIDs, got %d", len(ids))
	}
}

// The point of the donor parameter: a weapon the save has never held can still
// be created, modelled on a different item of the same kind. Here a made-up id
// is built from a real donor, and the result must carry the made-up id with the
// donor's tail — proving the tail travels and the id is rewritten.
func TestMaterialiseFromADifferentDonor(t *testing.T) {
	w := loadLevelWorld(t)
	item, cid := findWeaponItem(t, w) // a real item the save has
	const target = "Some_Grade5_Weapon_Nobody_Owns"

	idx, err := w.GiveInstancedItem(cid, target, item)
	if err != nil {
		t.Fatalf("GiveInstancedItem(%s, donor=%s): %v", target, item, err)
	}

	var localID gvas.GUID
	for _, s := range w.SlotsInContainer(cid) {
		if s.Slot != nil && s.Slot.Index == idx {
			localID = s.Slot.LocalID
		}
	}
	if localID == (gvas.GUID{}) {
		t.Fatal("new slot has a zero LocalID")
	}
	d, _ := w.loadDynamicItems()
	if !dynamicRecordExists(d, localID, target) {
		t.Errorf("no dynamic record carrying the made-up id %q", target)
	}

	// And it survives a round trip.
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	b, err := gvas.Encode(w.File)
	if err != nil {
		t.Fatal(err)
	}
	f2, _ := gvas.Decode(b, gvas.PalworldOptions())
	w2, _ := NewWorld(f2, gvas.PalworldOptions())
	if err := w2.Load(); err != nil {
		t.Fatal(err)
	}
	d2, _ := w2.loadDynamicItems()
	if !dynamicRecordExists(d2, localID, target) {
		t.Error("the made-up weapon's record did not survive reopen")
	}
}

// A stackable item given past a full stack must open a new slot rather than
// grow one past its cap — the overflow the user hit with 9,999 ammo.
func TestStackableItemSpillsIntoNewSlots(t *testing.T) {
	w := loadLevelWorld(t)
	// A container with plenty of room.
	var cid gvas.GUID
	counts := map[gvas.GUID]int{}
	for _, s := range w.itemSlots {
		counts[s.ContainerID]++
	}
	for id := range counts {
		if c, ok := w.Container(id); ok && c.Capacity-int32(len(c.Slots)) >= 5 {
			cid = id
			break
		}
	}
	if cid == (gvas.GUID{}) {
		t.Skip("no roomy container")
	}

	const item = "Wood"
	const maxStack = 9999
	before := 0
	for _, s := range w.ContainerContents(cid) {
		if s.ItemID == item {
			before++
		}
	}

	// Give 25,000: should land as 9999 + 9999 + 5002 across three new stacks.
	if _, err := w.GiveStackableItem(cid, item, 25000, maxStack); err != nil {
		t.Fatalf("GiveStackableItem: %v", err)
	}

	var total, over int32
	stacks := 0
	for _, s := range w.ContainerContents(cid) {
		if s.ItemID != item {
			continue
		}
		stacks++
		total += s.Count
		if s.Count > maxStack {
			over = s.Count
		}
	}
	if over > 0 {
		t.Errorf("a stack holds %d, over the %d cap", over, maxStack)
	}
	if total != 25000 {
		t.Errorf("total %s is %d, want 25000", item, total)
	}
	if stacks-before < 3 {
		t.Errorf("25000 at cap 9999 should need 3 stacks, got %d new", stacks-before)
	}
}

// Topping up a partial stack fills it to the cap first, then spills.
func TestStackableItemTopsUpBeforeSpilling(t *testing.T) {
	w := loadLevelWorld(t)
	var cid gvas.GUID
	for _, s := range w.itemSlots {
		if c, ok := w.Container(s.ContainerID); ok && c.Capacity-int32(len(c.Slots)) >= 3 {
			cid = s.ContainerID
			break
		}
	}
	if cid == (gvas.GUID{}) {
		t.Skip("no roomy container")
	}
	const item = "PalSphere"
	const maxStack = 9999

	// Seed a partial stack of 9000, then add 2000: 9000->9999 (+999) and a new
	// stack of 1001.
	if _, err := w.GiveStackableItem(cid, item, 9000, maxStack); err != nil {
		t.Fatal(err)
	}
	if _, err := w.GiveStackableItem(cid, item, 2000, maxStack); err != nil {
		t.Fatal(err)
	}
	var got []int32
	for _, s := range w.ContainerContents(cid) {
		if s.ItemID == item {
			got = append(got, s.Count)
		}
	}
	var total int32
	for _, g := range got {
		total += g
		if g > maxStack {
			t.Errorf("stack %d over cap", g)
		}
	}
	if total != 11000 {
		t.Errorf("total %d, want 11000; stacks=%v", total, got)
	}
}
