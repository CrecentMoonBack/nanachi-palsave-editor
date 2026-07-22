package palsave

import "github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"

// Storage boxes, furnaces, feed bins — anything a base camp builds that holds
// items — live in MapObjectSaveData, not in the base camp record itself.
//
// Two fields tie one together, both inside blobs the property tree only sees
// as opaque bytes:
//
//   - Model.RawData at offset 32 is the base camp the object belongs to.
//     World objects (the ~2,600 loot chests scattered around the map) have a
//     zero GUID here, which is what separates "my camp's box" from "a chest in
//     a dungeon".
//   - ConcreteModel.ModuleMap["…::ItemContainer"].RawData at offset 0 is the
//     item container id, the same key ItemContainerSaveData is indexed by.
//
// Verified against the real save: 2,229 map objects carry a nonzero GUID at
// Model.RawData+32 and *every one* of them is a base camp this save knows —
// no near misses, which is what rules out a coincidental GUID-shaped field.
const (
	// offCampID is where the owning base camp sits in Model.RawData.
	offCampID = 32
	// moduleItemContainer keys the ModuleMap entry holding the container id.
	moduleItemContainer = "EPalMapObjectConcreteModelModuleType::ItemContainer"
)

// CampStorage is one item-holding structure inside a base camp.
type CampStorage struct {
	CampID      gvas.GUID
	ContainerID gvas.GUID
	// Kind is the game's own object name, e.g. "ItemChest_04". There is no
	// Korean name table for buildings yet, so callers show this as-is rather
	// than guess a translation.
	Kind string
}

// CampStorages lists every base camp structure that owns an item container.
//
// Objects with no camp, or with no container module, are skipped — that is the
// whole world map filtered down to what a player actually built.
func (w *World) CampStorages() []CampStorage {
	arr, ok := w.mapObjects()
	if !ok {
		return nil
	}

	var out []CampStorage
	var zero gvas.GUID
	for _, sv := range arr.Structs.Values {
		props, ok := sv.(*gvas.StructProperties)
		if !ok {
			continue
		}
		camp, ok := mapObjectCamp(props.Props)
		if !ok || camp == zero {
			continue
		}
		container, ok := mapObjectContainer(props.Props)
		if !ok || container == zero {
			continue
		}
		// An "Expedition" structure in the real save names a container that
		// ItemContainerSaveData does not hold. Whatever the game does with
		// that, there are no slots to read or write, so it is not storage.
		if _, ok := w.Container(container); !ok {
			continue
		}
		out = append(out, CampStorage{
			CampID:      camp,
			ContainerID: container,
			Kind:        mapObjectKind(props.Props),
		})
	}
	return out
}

// mapObjects fetches the MapObjectSaveData array.
func (w *World) mapObjects() (*gvas.ArrayProperty, bool) {
	v, ok := w.world.Get("MapObjectSaveData")
	if !ok {
		return nil, false
	}
	arr, ok := v.(*gvas.ArrayProperty)
	if !ok || arr.Structs == nil {
		return nil, false
	}
	return arr, true
}

// mapObjectKind reads the object's name, e.g. "ItemChest_04".
func mapObjectKind(props *gvas.Properties) string {
	v, ok := props.Get("MapObjectId")
	if !ok {
		return ""
	}
	n, ok := v.(*gvas.NameProperty)
	if !ok {
		return ""
	}
	return n.Value.Value
}

// mapObjectCamp reads the owning base camp out of Model.RawData.
func mapObjectCamp(props *gvas.Properties) (gvas.GUID, bool) {
	b, ok := nestedRawData(props, "Model")
	if !ok || len(b) < offCampID+16 {
		return gvas.GUID{}, false
	}
	var g gvas.GUID
	copy(g[:], b[offCampID:offCampID+16])
	return g, true
}

// mapObjectContainer reads the item container id out of the concrete model's
// container module.
func mapObjectContainer(props *gvas.Properties) (gvas.GUID, bool) {
	v, ok := props.Get("ConcreteModel")
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
	mv, ok := inner.Props.Get("ModuleMap")
	if !ok {
		return gvas.GUID{}, false
	}
	m, ok := mv.(*gvas.MapProperty)
	if !ok {
		return gvas.GUID{}, false
	}
	for _, e := range m.Entries {
		if e.Key.Scalar.Str.Value != moduleItemContainer {
			continue
		}
		ep, ok := e.Value.Struct.(*gvas.StructProperties)
		if !ok {
			continue
		}
		raw, ok := rawDataArray(ep.Props)
		if !ok || len(raw.Values.Bytes) < 16 {
			continue
		}
		var g gvas.GUID
		copy(g[:], raw.Values.Bytes[:16])
		return g, true
	}
	return gvas.GUID{}, false
}

// nestedRawData pulls the RawData bytes out of a named child struct.
func nestedRawData(props *gvas.Properties, child string) ([]byte, bool) {
	v, ok := props.Get(child)
	if !ok {
		return nil, false
	}
	sp, ok := v.(*gvas.StructProperty)
	if !ok {
		return nil, false
	}
	inner, ok := sp.Value.(*gvas.StructProperties)
	if !ok {
		return nil, false
	}
	raw, ok := rawDataArray(inner.Props)
	if !ok {
		return nil, false
	}
	return raw.Values.Bytes, true
}
