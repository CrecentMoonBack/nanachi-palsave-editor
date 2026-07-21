// Package palsave interprets the Palworld-specific blobs inside a decoded
// GVAS archive.
//
// The generic codec in internal/gvas parses those blobs as opaque byte arrays,
// which is enough to round-trip a save untouched. To *edit* one, the bytes have
// to be understood, and that is what lives here.
//
// Layouts follow palsav (deafdudecomputers/PalworldSaveTools), the reference
// implementation known to produce saves the game accepts.
package palsave

import (
	"fmt"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// Paths of the RawData blobs this package understands.
const (
	PathCharacter          = ".worldSaveData.CharacterSaveParameterMap.Value.RawData"
	PathItemContainer      = ".worldSaveData.ItemContainerSaveData.Value.RawData"
	PathItemContainerSlot  = ".worldSaveData.ItemContainerSaveData.Value.Slots.Slots.RawData"
	PathCharacterContainer = ".worldSaveData.CharacterContainerSaveData.Value.Slots.Slots.RawData"
)

// CharacterRaw is the payload of a character's RawData array.
//
// It opens with an ordinary GVAS property block — the pal's actual save
// parameters — followed by a short fixed tail.
type CharacterRaw struct {
	Props   *gvas.Properties
	Unknown [4]byte
	GroupID gvas.GUID
	// Trailing is four bytes that follow the group id.
	Trailing [4]byte
	// Rest is anything left over. Normally empty; preserved so an unfamiliar
	// save still round-trips.
	Rest []byte
}

// DecodeCharacter parses a character RawData blob.
func DecodeCharacter(b []byte, opts *gvas.Options) (*CharacterRaw, error) {
	r := gvas.NewReader(b)

	props, err := gvas.ReadProperties(r, "", opts)
	if err != nil {
		return nil, fmt.Errorf("palsave: character properties: %w", err)
	}
	c := &CharacterRaw{Props: props}

	unknown, err := r.Bytes(4)
	if err != nil {
		return nil, fmt.Errorf("palsave: character unknown bytes: %w", err)
	}
	copy(c.Unknown[:], unknown)

	if c.GroupID, err = r.GUID(); err != nil {
		return nil, fmt.Errorf("palsave: character group id: %w", err)
	}

	trailing, err := r.Bytes(4)
	if err != nil {
		return nil, fmt.Errorf("palsave: character trailing bytes: %w", err)
	}
	copy(c.Trailing[:], trailing)

	c.Rest = r.Rest()
	return c, nil
}

// Encode serialises a character RawData blob.
func (c *CharacterRaw) Encode() ([]byte, error) {
	w := gvas.NewWriter()
	if err := gvas.WriteProperties(w, c.Props); err != nil {
		return nil, fmt.Errorf("palsave: character properties: %w", err)
	}
	w.Raw(c.Unknown[:])
	w.GUID(c.GroupID)
	w.Raw(c.Trailing[:])
	w.Raw(c.Rest)
	return w.Bytes(), nil
}

// ItemSlot is one occupied slot of an item container.
//
// An empty blob means an empty slot, which is distinct from a slot holding
// zero of something — hence DecodeItemSlot returning nil for it.
type ItemSlot struct {
	Index    int32
	Count    int32
	StaticID gvas.String
	// CreatedWorldID and LocalID identify a per-instance item, such as a
	// weapon with its own durability. Stackable items leave both zero.
	CreatedWorldID gvas.GUID
	LocalID        gvas.GUID
	Trailing       []byte
}

// DecodeItemSlot parses an item slot blob. A zero-length blob yields nil.
func DecodeItemSlot(b []byte) (*ItemSlot, error) {
	if len(b) == 0 {
		return nil, nil
	}
	r := gvas.NewReader(b)
	s := &ItemSlot{}
	var err error

	if s.Index, err = r.I32(); err != nil {
		return nil, fmt.Errorf("palsave: slot index: %w", err)
	}
	if s.Count, err = r.I32(); err != nil {
		return nil, fmt.Errorf("palsave: slot count: %w", err)
	}
	if s.StaticID, err = r.String(); err != nil {
		return nil, fmt.Errorf("palsave: slot static id: %w", err)
	}
	if s.CreatedWorldID, err = r.GUID(); err != nil {
		return nil, fmt.Errorf("palsave: slot created world id: %w", err)
	}
	if s.LocalID, err = r.GUID(); err != nil {
		return nil, fmt.Errorf("palsave: slot local id: %w", err)
	}
	s.Trailing = r.Rest()
	return s, nil
}

// Encode serialises an item slot. A nil slot encodes to an empty blob.
func (s *ItemSlot) Encode() []byte {
	if s == nil {
		return nil
	}
	w := gvas.NewWriter()
	w.I32(s.Index)
	w.I32(s.Count)
	w.String(s.StaticID)
	w.GUID(s.CreatedWorldID)
	w.GUID(s.LocalID)
	w.Raw(s.Trailing)
	return w.Bytes()
}

// IsEmpty reports whether the slot holds nothing.
func (s *ItemSlot) IsEmpty() bool {
	return s == nil || s.StaticID.Value == "" || s.StaticID.Value == "None" || s.Count <= 0
}

// ItemContainerRaw is the container-level blob, which carries access
// permissions rather than contents.
type ItemContainerRaw struct {
	PermissionTypeA []byte
	PermissionTypeB []byte
	ItemStaticIDs   []gvas.String
	Trailing        []byte
}

// DecodeItemContainer parses a container blob. A zero-length blob yields nil.
func DecodeItemContainer(b []byte) (*ItemContainerRaw, error) {
	if len(b) == 0 {
		return nil, nil
	}
	r := gvas.NewReader(b)
	c := &ItemContainerRaw{}

	var err error
	if c.PermissionTypeA, err = readByteArray(r); err != nil {
		return nil, fmt.Errorf("palsave: container permission type a: %w", err)
	}
	if c.PermissionTypeB, err = readByteArray(r); err != nil {
		return nil, fmt.Errorf("palsave: container permission type b: %w", err)
	}

	n, err := r.U32()
	if err != nil {
		return nil, fmt.Errorf("palsave: container item id count: %w", err)
	}
	c.ItemStaticIDs = make([]gvas.String, 0, n)
	for i := uint32(0); i < n; i++ {
		s, err := r.String()
		if err != nil {
			return nil, fmt.Errorf("palsave: container item id %d: %w", i, err)
		}
		c.ItemStaticIDs = append(c.ItemStaticIDs, s)
	}

	c.Trailing = r.Rest()
	return c, nil
}

// Encode serialises a container blob.
func (c *ItemContainerRaw) Encode() []byte {
	if c == nil {
		return nil
	}
	w := gvas.NewWriter()
	writeByteArray(w, c.PermissionTypeA)
	writeByteArray(w, c.PermissionTypeB)
	w.U32(uint32(len(c.ItemStaticIDs)))
	for _, s := range c.ItemStaticIDs {
		w.String(s)
	}
	w.Raw(c.Trailing)
	return w.Bytes()
}

// CharacterContainerSlot is one slot of a pal container: the palbox, a party,
// or a base camp's roster.
type CharacterContainerSlot struct {
	PlayerUID         gvas.GUID
	InstanceID        gvas.GUID
	PermissionTribeID uint8
	Unknown           []byte
}

// DecodeCharacterContainerSlot parses a pal container slot. Empty yields nil.
func DecodeCharacterContainerSlot(b []byte) (*CharacterContainerSlot, error) {
	if len(b) == 0 {
		return nil, nil
	}
	r := gvas.NewReader(b)
	s := &CharacterContainerSlot{}
	var err error

	if s.PlayerUID, err = r.GUID(); err != nil {
		return nil, fmt.Errorf("palsave: container slot player uid: %w", err)
	}
	if s.InstanceID, err = r.GUID(); err != nil {
		return nil, fmt.Errorf("palsave: container slot instance id: %w", err)
	}
	if s.PermissionTribeID, err = r.U8(); err != nil {
		return nil, fmt.Errorf("palsave: container slot tribe id: %w", err)
	}
	s.Unknown = r.Rest()
	return s, nil
}

// Encode serialises a pal container slot.
func (s *CharacterContainerSlot) Encode() []byte {
	if s == nil {
		return nil
	}
	w := gvas.NewWriter()
	w.GUID(s.PlayerUID)
	w.GUID(s.InstanceID)
	w.U8(s.PermissionTribeID)
	w.Raw(s.Unknown)
	return w.Bytes()
}

func readByteArray(r *gvas.Reader) ([]byte, error) {
	n, err := r.U32()
	if err != nil {
		return nil, err
	}
	return r.Bytes(int(n))
}

func writeByteArray(w *gvas.Writer, b []byte) {
	w.U32(uint32(len(b)))
	w.Raw(b)
}
