package palsave

import (
	"fmt"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// PlayerSave is a decoded Players/<uid>.sav.
//
// It holds almost no contents of its own: what matters are the container ids,
// which point into Level.sav. The inventory a player sees is stored in the
// world, not in their own file.
type PlayerSave struct {
	File *gvas.File

	data *gvas.Properties
}

// Container ids exposed by a player save.
const (
	ContainerCommon    = "CommonContainerId"
	ContainerDropSlot  = "DropSlotContainerId"
	ContainerEssential = "EssentialContainerId"
	ContainerWeapon    = "WeaponLoadOutContainerId"
	ContainerArmor     = "PlayerEquipArmorContainerId"
	ContainerFood      = "FoodEquipContainerId"
)

// NewPlayerSave wraps a decoded player archive.
func NewPlayerSave(f *gvas.File) (*PlayerSave, error) {
	v, ok := f.Root.Get("SaveData")
	if !ok {
		return nil, fmt.Errorf("palsave: not a player save (no SaveData)")
	}
	sp, ok := v.(*gvas.StructProperty)
	if !ok {
		return nil, fmt.Errorf("palsave: SaveData is %T", v)
	}
	inner, ok := sp.Value.(*gvas.StructProperties)
	if !ok {
		return nil, fmt.Errorf("palsave: SaveData value is %T", sp.Value)
	}
	return &PlayerSave{File: f, data: inner.Props}, nil
}

// PlayerUID is the player's own identifier.
func (p *PlayerSave) PlayerUID() (gvas.GUID, bool) {
	return p.guidField(p.data, "PlayerUId")
}

// InventoryContainer returns one of the player's item container ids. Use the
// Container* constants for the name.
func (p *PlayerSave) InventoryContainer(name string) (gvas.GUID, bool) {
	v, ok := p.data.Get("InventoryInfo")
	if !ok {
		return gvas.GUID{}, false
	}
	sp, ok := v.(*gvas.StructProperty)
	if !ok {
		return gvas.GUID{}, false
	}
	inner, ok := sp.Value.(*gvas.StructProperties)
	if !ok {
		return gvas.GUID{}, false
	}

	cv, ok := inner.Props.Get(name)
	if !ok {
		return gvas.GUID{}, false
	}
	csp, ok := cv.(*gvas.StructProperty)
	if !ok {
		return gvas.GUID{}, false
	}
	cinner, ok := csp.Value.(*gvas.StructProperties)
	if !ok {
		return gvas.GUID{}, false
	}
	return p.guidField(cinner.Props, "ID")
}

// PalStorageContainer is the palbox.
func (p *PlayerSave) PalStorageContainer() (gvas.GUID, bool) {
	return p.containerField("PalStorageContainerId")
}

// PartyContainer is the active party.
func (p *PlayerSave) PartyContainer() (gvas.GUID, bool) {
	return p.containerField("OtomoCharacterContainerId")
}

func (p *PlayerSave) containerField(name string) (gvas.GUID, bool) {
	v, ok := p.data.Get(name)
	if !ok {
		return gvas.GUID{}, false
	}
	sp, ok := v.(*gvas.StructProperty)
	if !ok {
		return gvas.GUID{}, false
	}
	inner, ok := sp.Value.(*gvas.StructProperties)
	if !ok {
		return gvas.GUID{}, false
	}
	return p.guidField(inner.Props, "ID")
}

func (p *PlayerSave) guidField(props *gvas.Properties, name string) (gvas.GUID, bool) {
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
