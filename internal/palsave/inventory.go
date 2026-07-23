package palsave

import (
	"fmt"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// SetItemCount changes how many of an item a container holds.
//
// It only touches slots that already hold that item; it will not create one.
// Use GiveItem for that.
func (w *World) SetItemCount(container gvas.GUID, itemID string, count int32) (int, error) {
	if count < 0 {
		return 0, fmt.Errorf("palsave: count %d is negative", count)
	}
	n := 0
	for _, s := range w.SlotsInContainer(container) {
		if s.Slot == nil || s.Slot.StaticID.Value != itemID {
			continue
		}
		s.Slot.Count = count
		n++
	}
	if n == 0 {
		return 0, fmt.Errorf("palsave: container holds no %s", itemID)
	}
	return n, nil
}

// SetSlotCount changes one slot, addressed by its index.
//
// SetItemCount above addresses stacks by item id, which cannot express "this
// one" when a container holds the same item twice — a chest with 9,999 crude
// oil in one slot and 500 in another is ordinary, and asking for 100 there
// sets *both* to 100 and destroys the larger stack. Anything driven by a
// user clicking a specific stack must come through here instead.
//
// A count of zero empties the slot: IsEmpty already treats a non-positive
// count as empty and GiveItem reuses such slots, so this needs no separate
// "remove" call.
func (w *World) SetSlotCount(container gvas.GUID, slot, count int32) error {
	if count < 0 {
		return fmt.Errorf("palsave: count %d is negative", count)
	}
	for _, s := range w.SlotsInContainer(container) {
		if s.Slot == nil || s.Slot.Index != slot {
			continue
		}
		s.Slot.Count = count
		return nil
	}
	return fmt.Errorf("palsave: container %s has no slot %d", container, slot)
}

// AddItemCount adds to an existing stack, returning the new total.
func (w *World) AddItemCount(container gvas.GUID, itemID string, delta int32) (int32, error) {
	for _, s := range w.SlotsInContainer(container) {
		if s.Slot == nil || s.Slot.StaticID.Value != itemID {
			continue
		}
		total := s.Slot.Count + delta
		if total < 0 {
			return 0, fmt.Errorf("palsave: %s would go negative (%d + %d)", itemID, s.Slot.Count, delta)
		}
		s.Slot.Count = total
		return total, nil
	}
	return 0, fmt.Errorf("palsave: container holds no %s", itemID)
}

// GiveItem puts items into a container.
//
// It stacks onto an existing slot when the container already holds the item,
// reuses a materialised empty slot if there is one, and otherwise appends a
// new slot at a free index.
//
// That last case is the common one and easy to miss: a container with 45
// capacity and 28 occupied slots has 17 free indices, but none of them exist
// in the array, because only occupied slots are stored.
func (w *World) GiveItem(container gvas.GUID, itemID string, count int32) (int32, error) {
	return w.giveItem(container, itemID, "", count, false)
}

// GiveStackableItem adds count of a stackable item, spilling into new slots
// once each stack reaches maxStack rather than overflowing one slot past its
// cap — which the game rejects. maxStack is the item's max_stack_count.
func (w *World) GiveStackableItem(container gvas.GUID, itemID string, count, maxStack int32) (int32, error) {
	return w.giveItemStack(container, itemID, "", count, false, maxStack)
}

// GiveInstancedItem gives a non-stackable item — a weapon, tool, shield or
// piece of armour — with the per-instance state record such an item needs.
//
// These do not stack: every one is a distinct instance with its own LocalID
// and its own DynamicItemSaveData entry holding durability, ammo and shield
// charge. Giving one without that entry is what left a user's added weapons
// unable to reload; count is therefore forced to a single item per slot.
//
// donorID names an item already in the save whose dynamic record is copied for
// its tail shape; the caller picks one of the same category. It may equal
// itemID when the save already holds that exact item.
func (w *World) GiveInstancedItem(container gvas.GUID, itemID, donorID string) (int32, error) {
	return w.giveItem(container, itemID, donorID, 1, true)
}

// HasDynamicItem reports whether the save already carries a per-instance record
// for itemID, so the caller can prefer it as its own donor.
func (w *World) HasDynamicItem(itemID string) bool {
	d, ok := w.loadDynamicItems()
	return ok && d.has(itemID)
}

func (w *World) giveItem(container gvas.GUID, itemID, donorID string, count int32, instanced bool) (int32, error) {
	return w.giveItemStack(container, itemID, donorID, count, instanced, 0)
}

// giveItemStack adds count of an item, spilling into new slots once a stack is
// full. maxStack is the game's per-item stack cap (0 means "no cap known", the
// old single-slot behaviour); it matters for stackable items only.
//
// The overflow is the point: a full 9,999 stack of ammo used to just grow past
// its cap when more was added, which the game rejects. Now a second slot opens.
func (w *World) giveItemStack(container gvas.GUID, itemID, donorID string, count int32, instanced bool, maxStack int32) (int32, error) {
	if count <= 0 {
		return 0, fmt.Errorf("palsave: count must be positive, got %d", count)
	}
	c, ok := w.Container(container)
	if !ok {
		return 0, fmt.Errorf("palsave: no container %s", container)
	}

	// A per-instance item is prepared first: if the donor is missing, fail
	// before touching any slot.
	var localID gvas.GUID
	if instanced {
		d, ok := w.loadDynamicItems()
		if !ok {
			return 0, fmt.Errorf("palsave: save has no DynamicItemSaveData")
		}
		id, err := d.materialise(itemID, donorID)
		if err != nil {
			return 0, err
		}
		localID = id
	}

	// Top up existing stacks of this item up to the cap. Per-instance items
	// skip this entirely — two weapons are two objects even when identical.
	remaining := count
	var lastIdx int32
	touched := false
	if !instanced {
		cap := maxStack
		if cap <= 0 {
			cap = remaining // no cap known: behave as before, one big stack
		}
		for _, s := range c.Slots {
			if remaining <= 0 {
				break
			}
			if s.Slot == nil || s.Slot.StaticID.Value != itemID {
				continue
			}
			room := cap - s.Slot.Count
			if room <= 0 {
				continue
			}
			add := room
			if add > remaining {
				add = remaining
			}
			s.Slot.Count += add
			remaining -= add
			lastIdx = s.Slot.Index
			touched = true
		}
	}

	// Whatever did not fit goes into fresh slots, each capped at maxStack.
	for remaining > 0 {
		put := remaining
		if maxStack > 0 && put > maxStack {
			put = maxStack
		}
		if instanced {
			put = 1 // one object per slot
		}

		idx, ok := w.placeInEmptySlot(c, itemID, put, localID)
		if !ok {
			// Out of room. If we already placed some, that is a partial
			// success worth returning; otherwise it is a hard failure.
			if touched {
				return lastIdx, nil
			}
			return 0, fmt.Errorf("palsave: container %s is full (%d slots, capacity %d)",
				container, len(c.Slots), c.Capacity)
		}
		lastIdx = idx
		touched = true
		remaining -= put

		// A per-instance item needs a distinct dynamic record per slot, so a
		// second one has to be materialised for a second weapon.
		if instanced && remaining > 0 {
			d, _ := w.loadDynamicItems()
			id, err := d.materialise(itemID, donorID)
			if err != nil {
				return lastIdx, err
			}
			localID = id
		}
	}
	return lastIdx, nil
}

// placeInEmptySlot puts an item into a reused empty slot or a fresh one,
// returning the slot index and whether it fit.
func (w *World) placeInEmptySlot(c *Container, itemID string, count int32, localID gvas.GUID) (int32, bool) {
	for _, s := range c.Slots {
		if s.Slot != nil && s.Slot.IsEmpty() {
			s.Slot.StaticID = gvas.Str(itemID)
			s.Slot.Count = count
			s.Slot.LocalID = localID
			return s.Slot.Index, true
		}
	}
	idx, ok := c.FreeIndex()
	if !ok {
		return 0, false
	}
	slot := &ItemSlot{
		Index:    idx,
		Count:    count,
		StaticID: gvas.Str(itemID),
		LocalID:  localID,
		Trailing: templateTrailing(c.Slots),
	}
	if _, err := w.appendSlot(c, slot); err != nil {
		return 0, false
	}
	return idx, true
}

// templateTrailing copies the trailing bytes an existing slot uses, so a new
// slot matches the shape the game writes rather than guessing at it.
func templateTrailing(slots []*ItemSlotEntry) []byte {
	for _, s := range slots {
		if s.Slot != nil && len(s.Slot.Trailing) > 0 {
			out := make([]byte, len(s.Slot.Trailing))
			copy(out, s.Slot.Trailing)
			return out
		}
	}
	return nil
}

// ContainerContents summarises what a container holds.
func (w *World) ContainerContents(container gvas.GUID) []ItemStack {
	var out []ItemStack
	for _, s := range w.SlotsInContainer(container) {
		if s.Slot.IsEmpty() {
			continue
		}
		out = append(out, ItemStack{
			Index:  s.Slot.Index,
			ItemID: s.Slot.StaticID.Value,
			Count:  s.Slot.Count,
		})
	}
	return out
}

// ItemStack is one occupied inventory slot.
type ItemStack struct {
	Index  int32
	ItemID string
	Count  int32
}
