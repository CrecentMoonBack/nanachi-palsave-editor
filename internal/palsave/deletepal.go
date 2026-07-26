package palsave

import (
	"fmt"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// DeletePal removes a pal from the save completely: its record in
// CharacterSaveParameterMap, its slot in the owning container, and its handle in
// the guild roster. The three must go together — a record with no slot, or a
// roster handle with no record, is the sort of dangling reference the game drops
// or chokes on.
//
// A player character is refused: a player's record lives in the same map, and
// removing it would orphan their save.
func (w *World) DeletePal(instance gvas.GUID) error {
	var target *CharEntry
	for _, c := range w.chars {
		if c.InstanceID == instance {
			target = c
			break
		}
	}
	if target == nil {
		return fmt.Errorf("palsave: 삭제할 팰을 찾을 수 없습니다: %v", instance)
	}
	if target.Pal.IsPlayer() {
		return fmt.Errorf("palsave: 플레이어 캐릭터는 삭제할 수 없습니다")
	}

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

// removeCharacterEntry drops the pal's entry from CharacterSaveParameterMap.
func (w *World) removeCharacterEntry(instance gvas.GUID) error {
	m, err := w.mapProp("CharacterSaveParameterMap")
	if err != nil {
		return err
	}
	for i := range m.Entries {
		ks, ok := m.Entries[i].Key.Struct.(*gvas.StructProperties)
		if !ok {
			continue
		}
		if id, ok := guidProp(ks.Props, "InstanceId"); ok && id == instance {
			m.Entries = append(m.Entries[:i], m.Entries[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("palsave: 팰 %v 가 캐릭터 맵에 없습니다", instance)
}

// removePalSlot clears the pal's slot from whichever container holds it — the
// palbox, the party, or a base. A pal with no slot simply is not found, which is
// harmless.
func (w *World) removePalSlot(instance gvas.GUID) {
	for _, c := range w.PalContainers() {
		for i, s := range c.Slots {
			if s.Slot != nil && s.Slot.InstanceID == instance {
				c.arr.Structs.Values = append(c.arr.Structs.Values[:i], c.arr.Structs.Values[i+1:]...)
				return
			}
		}
	}
}

// unregisterFromGuild removes the pal's handle from its guild roster. The
// instance id is unique across the save, so matching it in the second half of a
// handle is enough; the owner half is not needed. A handle that is not there is
// no error — the pal may predate the roster — but one that is must go, or the
// guild points at a pal that no longer exists.
func (w *World) unregisterFromGuild(instance gvas.GUID) {
	m, err := w.mapProp("GroupSaveDataMap")
	if err != nil {
		return
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

		removeAt := -1
		for j := 0; j < count && !r.bad; j++ {
			start := r.off
			r.guid()         // owner half
			h := r.guid()    // character instance half
			if h == instance {
				removeAt = start
				break
			}
		}
		if r.bad || removeAt < 0 {
			continue
		}
		out := make([]byte, 0, len(b)-sizeCharacterHandle)
		out = append(out, b[:countAt]...)
		out = appendU32(out, uint32(count-1))
		out = append(out, b[countAt+4:removeAt]...)
		out = append(out, b[removeAt+sizeCharacterHandle:]...)
		raw.Values.Bytes = out
		return
	}
}
