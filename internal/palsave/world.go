package palsave

import (
	"fmt"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// World is an editable view over a decoded Level.sav.
//
// Compression is deliberately not this package's concern: callers hand it a
// decoded archive and take one back, which keeps palsave buildable and
// testable on any platform while the Oodle codec stays Windows-only.
type World struct {
	File *gvas.File

	world *gvas.Properties
	opts  *gvas.Options

	chars      []*CharEntry
	itemSlots  []*ItemSlotEntry
	containers map[gvas.GUID]*Container
	loadedOnce bool
}

// Container is an item container and the array its slots live in.
//
// Slots are sparse: Capacity is what the container declares, but only occupied
// slots exist in the array. Adding an item to a full-looking container means
// appending a new element at an unused index, not writing to a free one.
type Container struct {
	ID       gvas.GUID
	Capacity int32
	Slots    []*ItemSlotEntry

	arr *gvas.ArrayProperty
}

// UsedIndices reports which slot indices are materialised.
func (c *Container) UsedIndices() map[int32]bool {
	used := map[int32]bool{}
	for _, s := range c.Slots {
		if s.Slot != nil {
			used[s.Slot.Index] = true
		}
	}
	return used
}

// FreeIndex returns the lowest index below Capacity with no slot on it.
func (c *Container) FreeIndex() (int32, bool) {
	used := c.UsedIndices()
	for i := int32(0); i < c.Capacity; i++ {
		if !used[i] {
			return i, true
		}
	}
	return 0, false
}

// CharEntry is one character in the save, along with where its blob lives so
// edits can be written back.
type CharEntry struct {
	// Index is the position in CharacterSaveParameterMap.
	Index int
	// Pal is the typed view. Players are included; check Pal.IsPlayer.
	Pal *Pal

	// PlayerUID comes from the map key. For a player character this is their
	// own identifier — a player record has no OwnerPlayerUId, so this is the
	// only place their id appears.
	PlayerUID gvas.GUID
	// InstanceID uniquely identifies this character.
	InstanceID gvas.GUID

	arr *gvas.ArrayProperty
}

// ItemSlotEntry is one item slot along with the container that holds it.
type ItemSlotEntry struct {
	// ContainerID identifies the container this slot belongs to.
	ContainerID gvas.GUID
	Slot        *ItemSlot

	arr *gvas.ArrayProperty
}

// NewWorld wraps a decoded Level.sav.
func NewWorld(f *gvas.File, opts *gvas.Options) (*World, error) {
	v, ok := f.Root.Get("worldSaveData")
	if !ok {
		return nil, fmt.Errorf("palsave: not a Level.sav (no worldSaveData)")
	}
	sp, ok := v.(*gvas.StructProperty)
	if !ok {
		return nil, fmt.Errorf("palsave: worldSaveData is %T", v)
	}
	inner, ok := sp.Value.(*gvas.StructProperties)
	if !ok {
		return nil, fmt.Errorf("palsave: worldSaveData value is %T", sp.Value)
	}
	return &World{File: f, world: inner.Props, opts: opts}, nil
}

// Load decodes every character and item slot blob. Call before using the
// accessors; repeated calls are no-ops.
func (w *World) Load() error {
	if w.loadedOnce {
		return nil
	}
	if err := w.loadChars(); err != nil {
		return err
	}
	if err := w.loadItemSlots(); err != nil {
		return err
	}
	w.loadedOnce = true
	return nil
}

func (w *World) loadChars() error {
	m, err := w.mapProp("CharacterSaveParameterMap")
	if err != nil {
		return err
	}
	for i, e := range m.Entries {
		sv, ok := e.Value.Struct.(*gvas.StructProperties)
		if !ok {
			return fmt.Errorf("palsave: character %d value is %T", i, e.Value.Struct)
		}
		arr, ok := rawDataArray(sv.Props)
		if !ok {
			return fmt.Errorf("palsave: character %d has no RawData", i)
		}
		raw, err := DecodeCharacter(arr.Values.Bytes, w.opts)
		if err != nil {
			return fmt.Errorf("palsave: character %d: %w", i, err)
		}
		p, err := NewPal(raw)
		if err != nil {
			return fmt.Errorf("palsave: character %d: %w", i, err)
		}

		ce := &CharEntry{Index: i, Pal: p, arr: arr}
		if ks, ok := e.Key.Struct.(*gvas.StructProperties); ok {
			ce.PlayerUID, _ = guidProp(ks.Props, "PlayerUId")
			ce.InstanceID, _ = guidProp(ks.Props, "InstanceId")
		}
		w.chars = append(w.chars, ce)
	}
	return nil
}

// guidProp reads a StructProperty holding a bare Guid.
func guidProp(props *gvas.Properties, name string) (gvas.GUID, bool) {
	v, ok := props.Get(name)
	if !ok {
		return gvas.GUID{}, false
	}
	sp, ok := v.(*gvas.StructProperty)
	if !ok {
		return gvas.GUID{}, false
	}
	g, ok := sp.Value.(*gvas.GUIDValue)
	if !ok {
		return gvas.GUID{}, false
	}
	return gvas.GUID(*g), true
}

func (w *World) loadItemSlots() error {
	m, err := w.mapProp("ItemContainerSaveData")
	if err != nil {
		return err
	}
	w.containers = map[gvas.GUID]*Container{}

	for i, e := range m.Entries {
		sv, ok := e.Value.Struct.(*gvas.StructProperties)
		if !ok {
			continue
		}
		// The container's own id is the map key.
		var cid gvas.GUID
		if ks, ok := e.Key.Struct.(*gvas.StructProperties); ok {
			cid, _ = guidProp(ks.Props, "ID")
		}

		sv2, ok := sv.Props.Get("Slots")
		if !ok {
			continue
		}
		arr, ok := sv2.(*gvas.ArrayProperty)
		if !ok || arr.Structs == nil {
			continue
		}

		c := &Container{ID: cid, arr: arr}
		if nv, ok := sv.Props.Get("SlotNum"); ok {
			if ip, ok := nv.(*gvas.IntProperty); ok {
				c.Capacity = ip.Value
			}
		}

		for j, elem := range arr.Structs.Values {
			ep, ok := elem.(*gvas.StructProperties)
			if !ok {
				continue
			}
			sarr, ok := rawDataArray(ep.Props)
			if !ok {
				continue
			}
			slot, err := DecodeItemSlot(sarr.Values.Bytes)
			if err != nil {
				return fmt.Errorf("palsave: container %d slot %d: %w", i, j, err)
			}
			entry := &ItemSlotEntry{ContainerID: cid, Slot: slot, arr: sarr}
			c.Slots = append(c.Slots, entry)
			w.itemSlots = append(w.itemSlots, entry)
		}

		// Capacity is occasionally absent; fall back to what exists so
		// FreeIndex cannot hand out an index the game would reject.
		if c.Capacity < int32(len(c.Slots)) {
			c.Capacity = int32(len(c.Slots))
		}
		w.containers[cid] = c
	}
	return nil
}

// Container returns a container by id.
func (w *World) Container(id gvas.GUID) (*Container, bool) {
	c, ok := w.containers[id]
	return c, ok
}

// appendSlot materialises a new slot in a container.
//
// The element is built to match the ones already present — RawData plus
// CustomVersionData, in that order — because the array's element layout is
// fixed and a differently shaped element would not encode.
func (w *World) appendSlot(c *Container, slot *ItemSlot) (*ItemSlotEntry, error) {
	if len(c.Slots) == 0 {
		return nil, fmt.Errorf("palsave: container %s has no slot to copy a layout from", c.ID)
	}
	template, ok := c.arr.Structs.Values[len(c.arr.Structs.Values)-1].(*gvas.StructProperties)
	if !ok {
		return nil, fmt.Errorf("palsave: container %s slot element is %T",
			c.ID, c.arr.Structs.Values[len(c.arr.Structs.Values)-1])
	}

	rawArr := &gvas.ArrayProperty{
		ArrayType: gvas.Str("ByteProperty"),
		Values:    gvas.ArrayValues{Bytes: slot.Encode()},
	}

	props := gvas.NewProperties()
	props.Set("RawData", rawArr)
	// Copy the version blob verbatim from the template; it is engine
	// bookkeeping, identical across the container's slots.
	if v, ok := template.Props.Get("CustomVersionData"); ok {
		if a, ok := v.(*gvas.ArrayProperty); ok {
			b := make([]byte, len(a.Values.Bytes))
			copy(b, a.Values.Bytes)
			props.Set("CustomVersionData", &gvas.ArrayProperty{
				ArrayType: gvas.Str("ByteProperty"),
				Values:    gvas.ArrayValues{Bytes: b},
			})
		}
	}

	c.arr.Structs.Values = append(c.arr.Structs.Values, &gvas.StructProperties{Props: props})

	entry := &ItemSlotEntry{ContainerID: c.ID, Slot: slot, arr: rawArr}
	c.Slots = append(c.Slots, entry)
	w.itemSlots = append(w.itemSlots, entry)
	return entry, nil
}

// Flush writes every decoded blob back into the archive.
//
// All entries are re-encoded, not just modified ones. Untouched blobs are
// byte-identical after a round trip — the tests assert exactly that — so
// tracking which ones changed would buy nothing but a way to get it wrong.
func (w *World) Flush() error {
	for _, c := range w.chars {
		b, err := c.Pal.Raw.Encode()
		if err != nil {
			return fmt.Errorf("palsave: re-encoding character %d: %w", c.Index, err)
		}
		c.arr.Values.Bytes = b
	}
	for _, s := range w.itemSlots {
		s.arr.Values.Bytes = s.Slot.Encode()
	}
	return nil
}

// Chars returns every character record, players included.
func (w *World) Chars() []*CharEntry { return w.chars }

// ItemSlots returns every item slot in the save.
func (w *World) ItemSlots() []*ItemSlotEntry { return w.itemSlots }

// Players returns only the player characters.
func (w *World) Players() []*CharEntry {
	var out []*CharEntry
	for _, c := range w.chars {
		if c.Pal.IsPlayer() {
			out = append(out, c)
		}
	}
	return out
}

// PalsOwnedBy returns the pals belonging to a player, excluding the player's
// own character record.
func (w *World) PalsOwnedBy(owner gvas.GUID) []*CharEntry {
	var out []*CharEntry
	for _, c := range w.chars {
		if c.Pal.IsPlayer() {
			continue
		}
		got, ok := c.Pal.OwnerPlayerUID()
		if ok && got == owner {
			out = append(out, c)
		}
	}
	return out
}

// SlotsInContainer returns the slots of one container.
func (w *World) SlotsInContainer(id gvas.GUID) []*ItemSlotEntry {
	var out []*ItemSlotEntry
	for _, s := range w.itemSlots {
		if s.ContainerID == id {
			out = append(out, s)
		}
	}
	return out
}

func (w *World) mapProp(name string) (*gvas.MapProperty, error) {
	v, ok := w.world.Get(name)
	if !ok {
		return nil, fmt.Errorf("palsave: worldSaveData has no %s", name)
	}
	m, ok := v.(*gvas.MapProperty)
	if !ok {
		return nil, fmt.Errorf("palsave: %s is %T, want a map", name, v)
	}
	return m, nil
}

// rawDataArray finds the "RawData" ByteProperty array inside a property block.
func rawDataArray(props *gvas.Properties) (*gvas.ArrayProperty, bool) {
	v, ok := props.Get("RawData")
	if !ok {
		return nil, false
	}
	a, ok := v.(*gvas.ArrayProperty)
	if !ok {
		return nil, false
	}
	return a, true
}
