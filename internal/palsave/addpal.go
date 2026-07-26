package palsave

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// Adding a pal means writing three places, and the save will not tell you if
// you miss one:
//
//  1. CharacterSaveParameterMap gains the pal's own record.
//  2. The owning container (the palbox) gains a slot entry pointing at it.
//  3. The guild roster gains a handle, or the game does not consider the pal
//     to belong to anybody.
//
// The slot entry is where this goes wrong. Its SlotIndex lives *beside*
// RawData rather than inside it, so cloning a slot and only replacing the
// instance id leaves the original's SlotIndex in place. Every pal added that
// way claims the same slot; the game resolves the conflict on load by keeping
// one and deleting the rest — including whatever was in that slot already.
// The symptom is unmistakable once seen: exactly one pal survives, always at
// the same slot number.
//
// Two rules follow, and this file exists to make them impossible to get wrong:
//
//   - SlotIndex is written in both places, the slot entry and the pal's own
//     SlotId, from one value.
//   - The free index is taken from the slot entries, never from the pals'
//     SlotId. A pal that has left the box keeps a stale SlotId, so deriving
//     free slots from the pals reports occupied slots that are not.

// PalSlot is one slot of a pal container.
type PalSlot struct {
	// Index is the slot's own SlotIndex, which is authoritative. The position
	// in the array is not: this save has a container with 340 entries whose
	// indices run past 344, and 339 of them do not match their position.
	Index int32
	Slot  *CharacterContainerSlot

	props *gvas.Properties
}

// PalContainer is one entry of CharacterContainerSaveData: a palbox, a party,
// or a base camp roster.
type PalContainer struct {
	ID       gvas.GUID
	Capacity int32
	Slots    []*PalSlot

	arr *gvas.ArrayProperty
}

// UsedSlotIndices reports which slot numbers are taken, from the slot entries.
func (c *PalContainer) UsedSlotIndices() map[int32]bool {
	used := make(map[int32]bool, len(c.Slots))
	for _, s := range c.Slots {
		used[s.Index] = true
	}
	return used
}

// FreeSlotIndex returns the lowest unused slot number below Capacity.
func (c *PalContainer) FreeSlotIndex() (int32, bool) {
	used := c.UsedSlotIndices()
	for i := int32(0); i < c.Capacity; i++ {
		if !used[i] {
			return i, true
		}
	}
	return 0, false
}

// PalContainers reads every pal container in the save.
func (w *World) PalContainers() []*PalContainer {
	m, err := w.mapProp("CharacterContainerSaveData")
	if err != nil {
		return nil
	}
	var out []*PalContainer
	for i := range m.Entries {
		sv, ok := m.Entries[i].Value.Struct.(*gvas.StructProperties)
		if !ok {
			continue
		}
		c := &PalContainer{}
		if ks, ok := m.Entries[i].Key.Struct.(*gvas.StructProperties); ok {
			c.ID, _ = guidProp(ks.Props, "ID")
		}
		if nv, ok := sv.Props.Get("SlotNum"); ok {
			if ip, ok := nv.(*gvas.IntProperty); ok {
				c.Capacity = ip.Value
			}
		}
		v, ok := sv.Props.Get("Slots")
		if !ok {
			continue
		}
		arr, ok := v.(*gvas.ArrayProperty)
		if !ok || arr.Structs == nil {
			continue
		}
		c.arr = arr
		for _, elem := range arr.Structs.Values {
			ep, ok := elem.(*gvas.StructProperties)
			if !ok {
				continue
			}
			ps := &PalSlot{props: ep.Props}
			if iv, ok := ep.Props.Get("SlotIndex"); ok {
				if ip, ok := iv.(*gvas.IntProperty); ok {
					ps.Index = ip.Value
				}
			}
			if raw, ok := rawDataArray(ep.Props); ok {
				ps.Slot, _ = DecodeCharacterContainerSlot(raw.Values.Bytes)
			}
			c.Slots = append(c.Slots, ps)
		}
		out = append(out, c)
	}
	return out
}

// PalContainerByID finds one container.
func (w *World) PalContainerByID(id gvas.GUID) (*PalContainer, bool) {
	for _, c := range w.PalContainers() {
		if c.ID == id {
			return c, true
		}
	}
	return nil, false
}

// NewPalSpec describes a pal to create.
type NewPalSpec struct {
	// SpeciesID is the save's own name for the species, e.g. "SheepBall".
	SpeciesID string
	Level     int
	Rank      int
	// Talents are the four individual values, keyed as Pal.SetTalent expects.
	Talents map[string]int
	// Passives are passive skill ids; at most MaxPassives.
	Passives []string
	// Alpha makes it the BOSS_ variant, which is the whole of what an alpha is.
	Alpha bool
	// Gender is "Male" or "Female"; empty leaves whatever the donor had.
	Gender string
	// Owner is the player who will own it, and Container their palbox.
	Owner     gvas.GUID
	Container gvas.GUID
}

// MaxPassives is how many passive skills the game allows.
const MaxPassives = 4

// AddPal creates a pal and puts it in a container, returning its instance id.
//
// The record is cloned from a pal already in the save rather than built from
// nothing: a pal record carries a large number of fields whose meaning is not
// established, and copying a working one and replacing what is understood is
// the approach that produces saves the game accepts. Moves are cleared,
// because they would otherwise arrive from the donor species.
func (w *World) AddPal(spec NewPalSpec) (gvas.GUID, error) {
	var zero gvas.GUID

	if spec.SpeciesID == "" {
		return zero, fmt.Errorf("palsave: no species given")
	}
	if spec.Rank < 1 || spec.Rank > MaxRank {
		return zero, fmt.Errorf("palsave: rank %d out of range 1..%d", spec.Rank, MaxRank)
	}
	if len(spec.Passives) > MaxPassives {
		return zero, fmt.Errorf("palsave: %d passives, at most %d", len(spec.Passives), MaxPassives)
	}

	container, ok := w.PalContainerByID(spec.Container)
	if !ok {
		return zero, fmt.Errorf("palsave: no pal container %s", spec.Container)
	}
	slotIndex, ok := container.FreeSlotIndex()
	if !ok {
		return zero, fmt.Errorf("palsave: container %s is full (%d slots)",
			spec.Container, container.Capacity)
	}

	donorRec, err := w.donorRecord()
	if err != nil {
		return zero, err
	}

	instance, err := newGUID()
	if err != nil {
		return zero, err
	}

	// Deep copy: encoding and decoding produces a tree sharing nothing with
	// the donor, which hand-copying the struct would not.
	blob, err := donorRec.Raw.Encode()
	if err != nil {
		return zero, fmt.Errorf("palsave: encoding the donor: %w", err)
	}
	raw, err := DecodeCharacter(blob, w.opts)
	if err != nil {
		return zero, fmt.Errorf("palsave: decoding the copy: %w", err)
	}
	pal, err := NewPal(raw)
	if err != nil {
		return zero, fmt.Errorf("palsave: reading the copy: %w", err)
	}

	if err := preparePal(pal, spec, instance, slotIndex); err != nil {
		return zero, err
	}

	if err := w.appendCharacter(pal, spec.Owner, instance); err != nil {
		return zero, err
	}
	if err := container.appendSlot(slotIndex, spec.Owner, instance); err != nil {
		return zero, err
	}
	if err := w.registerInGuild(spec.Owner, instance); err != nil {
		return zero, err
	}
	return instance, nil
}

// preparePal overwrites everything about the donor that identifies it.
func preparePal(p *Pal, spec NewPalSpec, instance gvas.GUID, slotIndex int32) error {
	params := p.Params()

	species := spec.SpeciesID
	if spec.Alpha {
		species = alphaIDPrefix + strings.TrimPrefix(species, alphaIDPrefix)
	}
	params.Set("CharacterID", &gvas.NameProperty{Value: gvas.Str(species)})
	setGUIDProp(params, "OwnerPlayerUId", spec.Owner)
	setGUIDProp(params, "OldOwnerPlayerUId", spec.Owner)

	// A nickname belonging to the donor would follow the copy over.
	params.Delete("NickName")
	// Moves are per-species. Left alone, a Sheepball arrives knowing whatever
	// the donor had learned.
	params.Delete("EquipWaza")
	params.Delete("MasteredWaza")
	// Anything tying the copy to the donor's history.
	params.Delete("PassiveSkillList")
	params.Delete("CharacterMakeInfo")

	if err := p.SetLevel(spec.Level); err != nil {
		return err
	}
	if err := p.SetRank(spec.Rank); err != nil {
		return err
	}
	for name, v := range spec.Talents {
		if err := p.SetTalent(name, v); err != nil {
			return err
		}
	}
	if spec.Gender != "" {
		if err := p.SetGender(spec.Gender); err != nil {
			return err
		}
	}
	if len(spec.Passives) > 0 {
		if err := p.SetPassives(spec.Passives); err != nil {
			return err
		}
	}
	setSlotID(params, spec.Container, slotIndex)
	_ = instance // the instance id lives in the map key, not the record
	return nil
}

// setSlotID writes the pal's own view of where it lives. This must agree with
// the slot entry's SlotIndex; see the note at the top of this file.
func setSlotID(params *gvas.Properties, container gvas.GUID, index int32) {
	inner := gvas.NewProperties()
	cid := gvas.NewProperties()
	g := gvas.GUIDValue(container)
	cid.Set("ID", &gvas.StructProperty{
		StructType: gvas.Str("Guid"),
		Value:      &g,
	})
	inner.Set("ContainerId", &gvas.StructProperty{
		StructType: gvas.Str("PalContainerId"),
		Value:      &gvas.StructProperties{Props: cid},
	})
	inner.Set("SlotIndex", &gvas.IntProperty{Value: index})

	params.Set("SlotId", &gvas.StructProperty{
		StructType: gvas.Str("PalCharacterSlotId"),
		Value:      &gvas.StructProperties{Props: inner},
	})
}

func setGUIDProp(props *gvas.Properties, name string, id gvas.GUID) {
	g := gvas.GUIDValue(id)
	props.Set(name, &gvas.StructProperty{
		StructType: gvas.Str("Guid"),
		Value:      &g,
	})
}

// donorPal picks a record to copy. Any ordinary pal will do; a player record
// is a different shape and a boss carries fields an ordinary pal does not.
func (w *World) donorPal() (*CharEntry, bool) {
	for _, c := range w.chars {
		if c.Pal.IsPlayer() || c.Pal.IsBoss() {
			continue
		}
		if _, ok := c.Pal.Params().Get("SlotId"); !ok {
			continue
		}
		return c, true
	}
	return nil, false
}

// appendCharacter adds the record to CharacterSaveParameterMap and registers
// it with the in-memory world, so the new pal is editable straight away.
func (w *World) appendCharacter(p *Pal, owner, instance gvas.GUID) error {
	m, err := w.mapProp("CharacterSaveParameterMap")
	if err != nil {
		return err
	}
	if len(m.Entries) == 0 {
		return fmt.Errorf("palsave: CharacterSaveParameterMap is empty")
	}

	key := gvas.NewProperties()
	setGUIDProp(key, "PlayerUId", owner)
	setGUIDProp(key, "InstanceId", instance)
	key.Set("DebugName", &gvas.StrProperty{Value: gvas.Str("")})

	blob, err := p.Raw.Encode()
	if err != nil {
		return fmt.Errorf("palsave: encoding the new pal: %w", err)
	}
	val := gvas.NewProperties()
	arr := &gvas.ArrayProperty{
		ArrayType: gvas.Str("ByteProperty"),
		Values:    gvas.ArrayValues{Bytes: blob},
	}
	val.Set("RawData", arr)

	// The version stamp is the same for every record; copy it rather than
	// invent one.
	if src, ok := m.Entries[0].Value.Struct.(*gvas.StructProperties); ok {
		if cv, ok := src.Props.Get("CustomVersionData"); ok {
			if ca, ok := cv.(*gvas.ArrayProperty); ok {
				val.Set("CustomVersionData", &gvas.ArrayProperty{
					ArrayType: ca.ArrayType,
					Values:    gvas.ArrayValues{Bytes: append([]byte(nil), ca.Values.Bytes...)},
				})
			}
		}
	}

	m.Entries = append(m.Entries, gvas.MapEntry{
		Key:   gvas.MapValue{Struct: &gvas.StructProperties{Props: key}},
		Value: gvas.MapValue{Struct: &gvas.StructProperties{Props: val}},
	})

	w.chars = append(w.chars, &CharEntry{
		Index:      len(m.Entries) - 1,
		Pal:        p,
		PlayerUID:  owner,
		InstanceID: instance,
		arr:        arr,
	})
	return nil
}

// appendSlot adds the container's pointer at the pal.
func (c *PalContainer) appendSlot(index int32, owner, instance gvas.GUID) error {
	if c.arr == nil || c.arr.Structs == nil {
		return fmt.Errorf("palsave: container %s has no slot array", c.ID)
	}

	slot := &CharacterContainerSlot{PlayerUID: owner, InstanceID: instance}
	// The trailing bytes and permission id are copied from a populated slot:
	// their meaning is not established, and an occupied slot is a working
	// example of what the game accepts. A brand-new box has no occupied slot,
	// so fall back to the observed default — a zero permission and a zero
	// trailer of the length every slot carries.
	for _, s := range c.Slots {
		if s.Slot != nil && s.Slot.InstanceID != (gvas.GUID{}) {
			slot.PermissionTribeID = s.Slot.PermissionTribeID
			slot.Unknown = append([]byte(nil), s.Slot.Unknown...)
			break
		}
	}
	if slot.Unknown == nil {
		slot.Unknown = make([]byte, slotTrailerLen)
	}

	props := gvas.NewProperties()
	// SlotIndex first: it is what the whole operation turns on.
	props.Set("SlotIndex", &gvas.IntProperty{Value: index})
	props.Set("RawData", &gvas.ArrayProperty{
		ArrayType: gvas.Str("ByteProperty"),
		Values:    gvas.ArrayValues{Bytes: slot.Encode()},
	})
	cvd := slotVersionData
	if len(c.arr.Structs.Values) > 0 {
		if src, ok := c.arr.Structs.Values[0].(*gvas.StructProperties); ok {
			if cv, ok := src.Props.Get("CustomVersionData"); ok {
				if ca, ok := cv.(*gvas.ArrayProperty); ok {
					cvd = ca.Values.Bytes
				}
			}
		}
	}
	props.Set("CustomVersionData", &gvas.ArrayProperty{
		ArrayType: gvas.Str("ByteProperty"),
		Values:    gvas.ArrayValues{Bytes: append([]byte(nil), cvd...)},
	})

	c.arr.Structs.Values = append(c.arr.Structs.Values, &gvas.StructProperties{Props: props})
	c.Slots = append(c.Slots, &PalSlot{Index: index, Slot: slot, props: props})
	return nil
}

// registerInGuild appends the pal to its owner's roster.
//
// The roster is a length-prefixed list at the head of the guild blob, ahead of
// a tail this code deliberately does not parse — the tail changed shape in a
// game update, and rewriting only the head keeps that irrelevant.
func (w *World) registerInGuild(owner, instance gvas.GUID) error {
	m, err := w.mapProp("GroupSaveDataMap")
	if err != nil {
		return err
	}
	for i := range m.Entries {
		props, ok := m.Entries[i].Value.Struct.(*gvas.StructProperties)
		if !ok {
			continue
		}
		raw, ok := rawDataArray(props.Props)
		if !ok {
			continue
		}
		b := raw.Values.Bytes
		r := &blobReader{b: b}
		r.guid()
		r.fstring()
		countAt := r.off
		count := int(r.u32())
		found := false
		for j := 0; j < count && !r.bad; j++ {
			if r.guid() == owner {
				found = true
			}
			r.skip(sizeCharacterHandle - 16)
		}
		if r.bad || !found {
			continue
		}
		endOfList := r.off

		// head, then the count raised by one, then the existing handles, the
		// new handle, and whatever followed.
		out := make([]byte, 0, len(b)+sizeCharacterHandle)
		out = append(out, b[:countAt]...)
		out = appendU32(out, uint32(count+1))
		out = append(out, b[countAt+4:endOfList]...)
		out = append(out, owner[:]...)
		out = append(out, instance[:]...)
		out = append(out, b[endOfList:]...)
		raw.Values.Bytes = out
		return nil
	}
	return fmt.Errorf("palsave: player %s is in no guild", owner)
}

func appendU32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

// newGUID makes an instance id. Palworld's own ids are version 4 and nothing
// reads the version bits here, but they are set so the value is a well-formed
// UUID rather than an arbitrary 16 bytes.
func newGUID() (gvas.GUID, error) {
	var g gvas.GUID
	if _, err := rand.Read(g[:]); err != nil {
		return g, fmt.Errorf("palsave: generating an instance id: %w", err)
	}
	g[7] = (g[7] & 0x0f) | 0x40
	g[8] = (g[8] & 0x3f) | 0x80
	return g, nil
}
