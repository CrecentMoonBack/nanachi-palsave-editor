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
	if count <= 0 {
		return 0, fmt.Errorf("palsave: count must be positive, got %d", count)
	}
	c, ok := w.Container(container)
	if !ok {
		return 0, fmt.Errorf("palsave: no container %s", container)
	}

	// Stack onto an existing slot when possible.
	for _, s := range c.Slots {
		if s.Slot != nil && s.Slot.StaticID.Value == itemID {
			s.Slot.Count += count
			return s.Slot.Index, nil
		}
	}

	// Reuse a materialised but empty slot.
	for _, s := range c.Slots {
		if s.Slot != nil && s.Slot.IsEmpty() {
			s.Slot.StaticID = gvas.Str(itemID)
			s.Slot.Count = count
			return s.Slot.Index, nil
		}
	}

	// Otherwise materialise one.
	idx, ok := c.FreeIndex()
	if !ok {
		return 0, fmt.Errorf("palsave: container %s is full (%d slots, capacity %d)",
			container, len(c.Slots), c.Capacity)
	}
	slot := &ItemSlot{
		Index:    idx,
		Count:    count,
		StaticID: gvas.Str(itemID),
		Trailing: templateTrailing(c.Slots),
	}
	if _, err := w.appendSlot(c, slot); err != nil {
		return 0, err
	}
	return idx, nil
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
