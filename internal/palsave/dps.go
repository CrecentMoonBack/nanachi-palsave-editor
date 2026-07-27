package palsave

import (
	"fmt"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// A Dimensional Pal Storage file — Players/<uid>_dps.sav, shown in-game as the
// "팰 복원 스토리지" with its public/private toggle — is a separate save from
// Level.sav. Its root is one array, SaveParameterArray, of a fixed 9,600 slots.
// Each slot is a struct with two fields:
//
//   - SaveParameter: the pal's parameters, byte-for-byte the same
//     PalIndividualCharacterSaveParameter block a pal carries in Level.sav.
//   - InstanceId: a PalInstanceID struct { PlayerUId, InstanceId } — the pal's
//     owner and its unique id.
//
// An empty slot is not absent: every one of the 9,600 is a full struct, and an
// empty one is marked by CharacterID "None". A pal placed in storage is *moved*
// out of Level.sav — its record leaves CharacterSaveParameterMap and only its
// instance id stays behind, in worldSaveData.InLockerCharacterInstanceIDArray,
// as the world's note that "this id lives in the storage file". So restoring a
// pal is a two-file operation, and the two files must be kept in step:
// InLocker + a filled DPS slot on one side, a live character record on the
// other, never both and never neither.
type DPSStore struct {
	File *gvas.File
	arr  *gvas.ArrayProperty
	opts *gvas.Options
}

// NewDPSStore wraps a decoded <uid>_dps.sav.
func NewDPSStore(f *gvas.File, opts *gvas.Options) (*DPSStore, error) {
	v, ok := f.Root.Get("SaveParameterArray")
	if !ok {
		return nil, fmt.Errorf("palsave: DPS 세이브가 아닙니다 (SaveParameterArray 없음)")
	}
	arr, ok := v.(*gvas.ArrayProperty)
	if !ok || arr.Structs == nil {
		return nil, fmt.Errorf("palsave: SaveParameterArray 가 %T 입니다", v)
	}
	return &DPSStore{File: f, arr: arr, opts: opts}, nil
}

// Encode serialises the store back to raw GVAS bytes.
func (d *DPSStore) Encode() ([]byte, error) { return gvas.Encode(d.File) }

// DPSPal is one stored pal, viewed through the same Pal accessors a Level.sav
// pal uses.
type DPSPal struct {
	Index      int
	Pal        *Pal
	Owner      gvas.GUID
	InstanceID gvas.GUID

	elem *gvas.StructProperties
}

// dpsElem reads a slot as a property block.
func dpsElem(sv gvas.StructValue) (*gvas.StructProperties, bool) {
	sp, ok := sv.(*gvas.StructProperties)
	return sp, ok
}

// dpsPalView wraps a slot's properties as a Pal. A slot always has a
// SaveParameter, so this is the same view NewPal builds for a Level.sav pal.
func dpsPalView(elem *gvas.StructProperties) (*Pal, bool) {
	p, err := NewPal(&CharacterRaw{Props: elem.Props})
	if err != nil {
		return nil, false
	}
	return p, true
}

// dpsInstance reads a slot's owner and instance id out of its InstanceId field.
func dpsInstance(elem *gvas.StructProperties) (owner, instance gvas.GUID) {
	v, ok := elem.Props.Get("InstanceId")
	if !ok {
		return
	}
	sp, ok := v.(*gvas.StructProperty)
	if !ok {
		return
	}
	inner, ok := sp.Value.(*gvas.StructProperties)
	if !ok {
		return
	}
	owner, _ = guidProp(inner.Props, "PlayerUId")
	instance, _ = guidProp(inner.Props, "InstanceId")
	return
}

// isEmptyPal reports whether a slot holds no pal — CharacterID "None" or absent.
func isEmptyPal(p *Pal) bool {
	id := p.CharacterID()
	return id == "" || id == "None"
}

// Pals lists the occupied slots.
func (d *DPSStore) Pals() []*DPSPal {
	var out []*DPSPal
	for i, sv := range d.arr.Structs.Values {
		elem, ok := dpsElem(sv)
		if !ok {
			continue
		}
		p, ok := dpsPalView(elem)
		if !ok || isEmptyPal(p) {
			continue
		}
		owner, instance := dpsInstance(elem)
		out = append(out, &DPSPal{Index: i, Pal: p, Owner: owner, InstanceID: instance, elem: elem})
	}
	return out
}

// find locates a stored pal by instance id.
func (d *DPSStore) find(instance gvas.GUID) (*DPSPal, bool) {
	for _, dp := range d.Pals() {
		if dp.InstanceID == instance {
			return dp, true
		}
	}
	return nil, false
}

// freeSlotIndex returns the first empty slot's array index.
func (d *DPSStore) freeSlotIndex() (int, bool) {
	for i, sv := range d.arr.Structs.Values {
		elem, ok := dpsElem(sv)
		if !ok {
			continue
		}
		p, ok := dpsPalView(elem)
		if !ok {
			continue
		}
		if isEmptyPal(p) {
			return i, true
		}
	}
	return 0, false
}

// emptyTemplate deep-copies an existing empty slot, so clearing a slot writes
// exactly the shape the game uses for "empty" rather than a hand-built guess.
func (d *DPSStore) emptyTemplate() (*gvas.StructProperties, error) {
	for _, sv := range d.arr.Structs.Values {
		elem, ok := dpsElem(sv)
		if !ok {
			continue
		}
		p, ok := dpsPalView(elem)
		if !ok || !isEmptyPal(p) {
			continue
		}
		cp, err := cloneProps(elem.Props, d.opts)
		if err != nil {
			return nil, err
		}
		return &gvas.StructProperties{Props: cp}, nil
	}
	return nil, fmt.Errorf("palsave: DPS에 빈 슬롯 견본이 없습니다")
}

// cloneProps deep-copies a property block by round-tripping it through the
// encoder — the copy shares no pointers with the original.
func cloneProps(p *gvas.Properties, opts *gvas.Options) (*gvas.Properties, error) {
	w := gvas.NewWriter()
	if err := gvas.WriteProperties(w, p); err != nil {
		return nil, err
	}
	return gvas.ReadProperties(gvas.NewReader(w.Bytes()), "", opts)
}

// palInstanceID builds a PalInstanceID struct as a DPS slot stores it.
func palInstanceID(owner, instance gvas.GUID) *gvas.StructProperty {
	inner := gvas.NewProperties()
	setGUIDProp(inner, "PlayerUId", owner)
	setGUIDProp(inner, "InstanceId", instance)
	return &gvas.StructProperty{
		StructType: gvas.Str("PalInstanceID"),
		Value:      &gvas.StructProperties{Props: inner},
	}
}

// MoveFromDPS restores a stored pal into a palbox container: it rebuilds the
// character record in Level.sav, gives it a slot and a guild handle, then
// empties its DPS slot and drops its InLocker note. The pal keeps its original
// instance id throughout, so nothing else in the save that referred to it
// breaks.
func (w *World) MoveFromDPS(d *DPSStore, instance, container gvas.GUID) error {
	dp, ok := d.find(instance)
	if !ok {
		return fmt.Errorf("palsave: 복원 스토리지에 %v 팰이 없습니다", instance)
	}
	for _, c := range w.chars {
		if c.InstanceID == instance {
			return fmt.Errorf("palsave: %v 팰이 이미 세이브에 있습니다", instance)
		}
	}
	owner := dp.Owner

	cont, ok := w.PalContainerByID(container)
	if !ok {
		return fmt.Errorf("palsave: 팰 컨테이너 %s 를 찾을 수 없습니다", container)
	}
	slotIndex, ok := cont.FreeSlotIndex()
	if !ok {
		return fmt.Errorf("palsave: 팰박스가 가득 찼습니다 (%d칸)", cont.Capacity)
	}

	guild, ok := w.guildOf(owner)
	if !ok {
		return fmt.Errorf("palsave: 소유자 %s 가 길드에 없습니다", owner)
	}
	stored, ok := dp.elem.Props.Get("SaveParameter")
	if !ok {
		return fmt.Errorf("palsave: 저장된 팰에 SaveParameter가 없습니다")
	}

	// Build the character record from a real pal's shell — its unknown bytes
	// and trailer are the exact ones the game writes — then drop the stored
	// SaveParameter in and stamp the owner's guild.
	donorRec, err := w.donorRecord()
	if err != nil {
		return err
	}
	blob, err := donorRec.Raw.Encode()
	if err != nil {
		return fmt.Errorf("palsave: 견본 인코드: %w", err)
	}
	raw, err := DecodeCharacter(blob, w.opts)
	if err != nil {
		return fmt.Errorf("palsave: 견본 디코드: %w", err)
	}
	raw.Props.Set("SaveParameter", stored)
	raw.GroupID = guild
	pal, err := NewPal(raw)
	if err != nil {
		return fmt.Errorf("palsave: 복원 팰 읽기: %w", err)
	}
	setGUIDProp(pal.Params(), "OwnerPlayerUId", owner)
	setSlotID(pal.Params(), container, slotIndex)

	if err := w.appendCharacter(pal, owner, instance); err != nil {
		return err
	}
	if err := cont.appendSlot(slotIndex, owner, instance); err != nil {
		return err
	}
	if err := w.registerInGuild(owner, instance); err != nil {
		return err
	}

	// Only after the world side is committed do we vacate the storage: an empty
	// template in the slot and the InLocker note removed.
	tmpl, err := d.emptyTemplate()
	if err != nil {
		return err
	}
	d.arr.Structs.Values[dp.Index] = tmpl
	w.removeFromLocker(instance)
	return nil
}

// MoveToDPS is the reverse: it takes a pal out of Level.sav — record, slot and
// guild handle — writes it into a free storage slot with an InLocker note, and
// keeps its instance id. A player character is refused, as it is for deletion.
func (w *World) MoveToDPS(d *DPSStore, instance gvas.GUID) error {
	var target *CharEntry
	for _, c := range w.chars {
		if c.InstanceID == instance {
			target = c
			break
		}
	}
	if target == nil {
		return fmt.Errorf("palsave: %v 팰을 찾을 수 없습니다", instance)
	}
	if target.Pal.IsPlayer() {
		return fmt.Errorf("palsave: 플레이어 캐릭터는 보관할 수 없습니다")
	}
	owner, _ := target.Pal.OwnerPlayerUID()

	slotIdx, ok := d.freeSlotIndex()
	if !ok {
		return fmt.Errorf("palsave: 복원 스토리지가 가득 찼습니다")
	}
	stored, ok := target.Pal.Raw.Props.Get("SaveParameter")
	if !ok {
		return fmt.Errorf("palsave: 팰에 SaveParameter가 없습니다")
	}

	// Note the storage side needs adding before the world side is torn down, so
	// a mid-operation failure never loses the pal from both.
	if err := w.addToLocker(owner, instance); err != nil {
		return err
	}
	elem := gvas.NewProperties()
	elem.Set("SaveParameter", stored)
	elem.Set("InstanceId", palInstanceID(owner, instance))
	d.arr.Structs.Values[slotIdx] = &gvas.StructProperties{Props: elem}

	if err := w.removeCharacterEntry(instance); err != nil {
		return err
	}
	w.removePalSlot(instance)
	w.unregisterFromGuild(instance)
	for i, c := range w.chars {
		if c.InstanceID == instance {
			w.chars = append(w.chars[:i], w.chars[i+1:]...)
			break
		}
	}
	return nil
}

// guildOf returns the group id of the guild the owner belongs to, read from the
// head of the guild's roster blob.
func (w *World) guildOf(owner gvas.GUID) (gvas.GUID, bool) {
	m, err := w.mapProp("GroupSaveDataMap")
	if err != nil {
		return gvas.GUID{}, false
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
		r := &blobReader{b: raw.Values.Bytes}
		gid := r.guid()
		r.fstring()
		count := int(r.u32())
		for j := 0; j < count && !r.bad; j++ {
			if r.guid() == owner {
				return gid, true
			}
			r.skip(sizeCharacterHandle - 16)
		}
	}
	return gvas.GUID{}, false
}

// lockerSet returns the worldSaveData set that lists which instance ids are held
// in a storage file.
func (w *World) lockerSet() (*gvas.SetProperty, bool) {
	v, ok := w.world.Get("InLockerCharacterInstanceIDArray")
	if !ok {
		return nil, false
	}
	s, ok := v.(*gvas.SetProperty)
	return s, ok
}

// addToLocker records that an instance id now lives in a storage file.
func (w *World) addToLocker(owner, instance gvas.GUID) error {
	set, ok := w.lockerSet()
	if !ok {
		return fmt.Errorf("palsave: InLockerCharacterInstanceIDArray 가 없습니다")
	}
	e := gvas.NewProperties()
	setGUIDProp(e, "PlayerUId", owner)
	setGUIDProp(e, "InstanceId", instance)
	e.Set("DebugName", &gvas.StrProperty{Value: gvas.Str("")})
	set.Structs = append(set.Structs, &gvas.StructProperties{Props: e})
	return nil
}

// removeFromLocker drops an instance id's storage note. A note that is not there
// is not an error — the pal may predate the feature.
func (w *World) removeFromLocker(instance gvas.GUID) {
	set, ok := w.lockerSet()
	if !ok {
		return
	}
	for i, sv := range set.Structs {
		sp, ok := sv.(*gvas.StructProperties)
		if !ok {
			continue
		}
		if id, ok := guidProp(sp.Props, "InstanceId"); ok && id == instance {
			set.Structs = append(set.Structs[:i], set.Structs[i+1:]...)
			return
		}
	}
}
