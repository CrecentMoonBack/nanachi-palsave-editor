package palsave

import (
	"fmt"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// Relics — the effigies offered at the Statue of Power — live in the player's
// own .sav under RecordData, not in the world save.
//
// Two things about the layout matter.
//
// RelicPossessNum duplicates the CapturePower entry. Every player in the
// observed save that has one has them equal, so it is a denormalised copy.
// Which one the game reads is unknown, so both are written together; leaving
// them to disagree is the kind of half-edit that looks fine and is not.
//
// The map holds only the relic types a player has met. Counts of 2, 9, 11 and
// 12 were all present in one save, and two players had no map at all — so an
// absent type means "none found yet", and setting one may have to create the
// map itself.

// RelicCapturePower is the type the Statue of Power raises.
const RelicCapturePower = "EPalRelicType::CapturePower"

// relicPrefix is what the save puts before the bare relic name.
const relicPrefix = "EPalRelicType::"

// MaxRelicCount bounds one relic entry.
//
// **Not the game's cap — that is unknown.** 38 was observed on MoveSpeed. This
// is a guard against nonsense, deliberately loose, for the same reason the
// player status points are: refusing a real value is how the pal soul cap
// destroyed data.
const MaxRelicCount = 999

// recordData returns the player's RecordData block.
func (p *PlayerSave) recordData() (*gvas.Properties, bool) {
	v, ok := p.data.Get("RecordData")
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
	return inner.Props, true
}

// Relics reports how many of each relic type the player has offered, keyed by
// the save's own enum member.
func (p *PlayerSave) Relics() map[string]int {
	rec, ok := p.recordData()
	if !ok {
		return nil
	}
	v, ok := rec.Get("RelicPossessNumMap")
	if !ok {
		return nil
	}
	m, ok := v.(*gvas.MapProperty)
	if !ok {
		return nil
	}
	out := make(map[string]int, len(m.Entries))
	for _, e := range m.Entries {
		out[e.Key.Scalar.Str.Value] = int(e.Value.Scalar.I32)
	}
	return out
}

// RelicOrder returns the relic types in the order the save stores them.
func (p *PlayerSave) RelicOrder() []string {
	rec, ok := p.recordData()
	if !ok {
		return nil
	}
	v, ok := rec.Get("RelicPossessNumMap")
	if !ok {
		return nil
	}
	m, ok := v.(*gvas.MapProperty)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(m.Entries))
	for _, e := range m.Entries {
		out = append(out, e.Key.Scalar.Str.Value)
	}
	return out
}

// SetRelic sets one relic count, adding the entry when the player has none.
//
// Setting CapturePower also updates RelicPossessNum, which mirrors it.
func (p *PlayerSave) SetRelic(relicType string, count int) error {
	if count < 0 || count > MaxRelicCount {
		return fmt.Errorf("palsave: relic count %d out of range 0..%d", count, MaxRelicCount)
	}
	qualified := relicType
	if len(qualified) < len(relicPrefix) || qualified[:len(relicPrefix)] != relicPrefix {
		qualified = relicPrefix + qualified
	}

	rec, ok := p.recordData()
	if !ok {
		return fmt.Errorf("palsave: player save has no RecordData")
	}

	v, ok := rec.Get("RelicPossessNumMap")
	if !ok {
		m := newRelicMap()
		rec.Set("RelicPossessNumMap", m)
		v = m
	}
	m, ok := v.(*gvas.MapProperty)
	if !ok {
		return fmt.Errorf("palsave: RelicPossessNumMap is %T, want a map", v)
	}

	found := false
	for i := range m.Entries {
		if m.Entries[i].Key.Scalar.Str.Value != qualified {
			continue
		}
		m.Entries[i].Value.Scalar.I32 = int32(count)
		found = true
		break
	}
	if !found {
		m.Entries = append(m.Entries, gvas.MapEntry{
			Key:   gvas.MapValue{Scalar: gvas.ScalarValue{Str: gvas.Str(qualified)}},
			Value: gvas.MapValue{Scalar: gvas.ScalarValue{I32: int32(count)}},
		})
	}

	if qualified == RelicCapturePower {
		return p.setRelicPossessNum(rec, count)
	}
	return nil
}

// setRelicPossessNum keeps the standalone copy in step with the map.
func (p *PlayerSave) setRelicPossessNum(rec *gvas.Properties, count int) error {
	v, ok := rec.Get("RelicPossessNum")
	if !ok {
		rec.Set("RelicPossessNum", &gvas.IntProperty{Value: int32(count)})
		return nil
	}
	ip, ok := v.(*gvas.IntProperty)
	if !ok {
		return fmt.Errorf("palsave: RelicPossessNum is %T, want *gvas.IntProperty", v)
	}
	ip.Value = int32(count)
	return nil
}

// RelicPossessNum reports the standalone copy of the CapturePower count.
func (p *PlayerSave) RelicPossessNum() int {
	rec, ok := p.recordData()
	if !ok {
		return 0
	}
	v, ok := rec.Get("RelicPossessNum")
	if !ok {
		return 0
	}
	if ip, ok := v.(*gvas.IntProperty); ok {
		return int(ip.Value)
	}
	return 0
}

// Encode serialises the player save back to GVAS bytes.
func (p *PlayerSave) Encode() ([]byte, error) {
	return gvas.Encode(p.File)
}

func newRelicMap() *gvas.MapProperty {
	return &gvas.MapProperty{
		KeyType:   gvas.Str("EnumProperty"),
		ValueType: gvas.Str("IntProperty"),
	}
}
