package palsave

import (
	"fmt"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// A weapon, tool, shield or piece of armour is not just a slot in a container.
// Its slot carries a LocalID, and the item's real state — durability, ammo,
// how much a shield has charged — lives in a separate DynamicItemSaveData
// entry keyed by that same id.
//
// GiveItem used to write only the slot, leaving LocalID zero and no dynamic
// entry. The game then had a weapon with no state record to find, so it could
// not be reloaded and a shield never charged. A user reported exactly that.
//
// The fix is to give every non-stackable item a fresh LocalID and a dynamic
// entry to match. The entry is cloned from an existing one of the same kind
// rather than built from raw bytes: the tail differs by item type — a weapon
// has durability and ammo fields a shield does not, and an accessory has no
// tail at all — and copying a record the game already accepts is safer than
// reproducing each shape by hand. Only the LocalID and the item id are
// rewritten.

// dynLeadingZero is the 16-byte zero GUID every dynamic record opens with,
// ahead of the LocalID.
const dynLeadingZero = 16

// dynamicItemID reads the item id string a dynamic record is for. The record
// is [16B zero][16B LocalID][FString id][type-specific tail].
func dynamicItemID(b []byte) (string, bool) {
	r := &blobReader{b: b}
	r.skip(dynLeadingZero)
	r.skip(16) // LocalID
	id := r.fstring()
	if r.bad {
		return "", false
	}
	return id, true
}

// dynamicLocalID reads the LocalID a dynamic record is keyed by.
func dynamicLocalID(b []byte) (gvas.GUID, bool) {
	if len(b) < dynLeadingZero+16 {
		return gvas.GUID{}, false
	}
	var g gvas.GUID
	copy(g[:], b[dynLeadingZero:dynLeadingZero+16])
	return g, true
}

// dynamicItems is the DynamicItemSaveData array and an index by item id of the
// records already present, so a new item can be given a clone of its own kind.
type dynamicItems struct {
	arr      *gvas.ArrayProperty
	template *gvas.StructProperties // any element, for the CustomVersionData shape
	byItem   map[string][]byte
}

// loadDynamicItems reads DynamicItemSaveData.
func (w *World) loadDynamicItems() (*dynamicItems, bool) {
	v, ok := w.world.Get("DynamicItemSaveData")
	if !ok {
		return nil, false
	}
	arr, ok := v.(*gvas.ArrayProperty)
	if !ok || arr.Structs == nil {
		return nil, false
	}
	d := &dynamicItems{arr: arr, byItem: map[string][]byte{}}
	for _, sv := range arr.Structs.Values {
		props, ok := sv.(*gvas.StructProperties)
		if !ok {
			continue
		}
		if d.template == nil {
			d.template = props
		}
		raw, ok := rawDataArray(props.Props)
		if !ok {
			continue
		}
		if id, ok := dynamicItemID(raw.Values.Bytes); ok {
			if _, seen := d.byItem[id]; !seen {
				d.byItem[id] = raw.Values.Bytes
			}
		}
	}
	return d, true
}

// has reports whether the save already carries a dynamic record for itemID.
func (d *dynamicItems) has(itemID string) bool {
	_, ok := d.byItem[itemID]
	return ok
}

// materialise adds a dynamic entry for targetID with a fresh LocalID, and
// returns the LocalID.
//
// The record is built by cloning donorID's, which must exist in the save, and
// replacing two things: the LocalID and the item id string. Everything after
// the item id — the tail holding durability, ammo, shield charge — is copied
// byte for byte. That tail's layout depends on the item *category*, not the
// specific item, so any weapon can be created from any other of the same kind:
// a grade-5 plasma rifle nobody owns is built from a grade-1 assault rifle
// that someone does. The caller chooses the donor, since it knows the
// categories; this only needs donorID to be present.
//
// Preserving the tail rather than reproducing it is deliberate: its exact
// shape differs across melee, gun, shield and accessory, and copying one the
// game wrote is safer than reconstructing it.
func (d *dynamicItems) materialise(targetID, donorID string) (gvas.GUID, error) {
	donor, ok := d.byItem[donorID]
	if !ok {
		return gvas.GUID{}, fmt.Errorf("palsave: no %s in the save to model %s on", donorID, targetID)
	}

	// Everything after the donor's item id string is the tail to keep.
	r := &blobReader{b: donor}
	r.skip(dynLeadingZero)
	r.skip(16) // donor LocalID
	r.fstring()
	if r.bad {
		return gvas.GUID{}, fmt.Errorf("palsave: donor %s record is malformed", donorID)
	}
	tail := donor[r.off:]

	localID, err := newGUID()
	if err != nil {
		return gvas.GUID{}, err
	}

	// [16B zero][16B new LocalID][FString targetID][donor tail].
	w := gvas.NewWriter()
	w.Raw(make([]byte, dynLeadingZero))
	w.GUID(localID)
	w.String(gvas.Str(targetID))
	w.Raw(tail)
	blob := w.Bytes()

	props := gvas.NewProperties()
	props.Set("RawData", &gvas.ArrayProperty{
		ArrayType: gvas.Str("ByteProperty"),
		Values:    gvas.ArrayValues{Bytes: blob},
	})
	if d.template != nil {
		if cv, ok := d.template.Props.Get("CustomVersionData"); ok {
			if ca, ok := cv.(*gvas.ArrayProperty); ok {
				props.Set("CustomVersionData", &gvas.ArrayProperty{
					ArrayType: ca.ArrayType,
					Values:    gvas.ArrayValues{Bytes: append([]byte(nil), ca.Values.Bytes...)},
				})
			}
		}
	}
	d.arr.Structs.Values = append(d.arr.Structs.Values, &gvas.StructProperties{Props: props})
	d.byItem[targetID] = blob // a copy of this one can now be a donor too
	return localID, nil
}
