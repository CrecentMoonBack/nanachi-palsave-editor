package palsave

import (
	"fmt"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// A player's own record lives in CharacterSaveParameterMap alongside the pals,
// so it is reached through the same Pal wrapper — IsPlayer tells them apart.
// Level and Exp already work; what a player has on top is status points.

// Status point list names.
const (
	// StatusPointList is what levelling up lets you spend.
	StatusPointList = "GotStatusPointList"
	// ExStatusPointList is a second, separate track. Observed values sit far
	// above the ordinary list's, so the two are not interchangeable.
	ExStatusPointList = "GotExStatusPointList"
)

// MaxStatusPoint bounds a single status entry.
//
// **This is not the game's cap — we do not know it.** Observed values run to
// 20 in the ordinary list and 40 in the Ex list, and the field is an int32, so
// this is a guard against nonsense rather than a claim about the game. It is
// deliberately loose: refusing a legitimate value is how the pal soul cap
// silently destroyed data, and the editor shows no "max" button for these for
// the same reason.
const MaxStatusPoint = 255

// UnusedStatusPoint reports points earned but not yet spent.
func (p *Pal) UnusedStatusPoint() int {
	v, ok := p.params.Get("UnusedStatusPoint")
	if !ok {
		return 0
	}
	if u, ok := v.(*gvas.UInt16Property); ok {
		return int(u.Value)
	}
	return 0
}

// SetUnusedStatusPoint sets the unspent pool.
func (p *Pal) SetUnusedStatusPoint(n int) error {
	if n < 0 || n > 65535 {
		return fmt.Errorf("palsave: unused status point %d out of range 0..65535", n)
	}
	v, ok := p.params.Get("UnusedStatusPoint")
	if !ok {
		p.params.Set("UnusedStatusPoint", &gvas.UInt16Property{Value: uint16(n)})
		return nil
	}
	u, ok := v.(*gvas.UInt16Property)
	if !ok {
		return fmt.Errorf("palsave: UnusedStatusPoint is %T, want *gvas.UInt16Property", v)
	}
	u.Value = uint16(n)
	return nil
}

// StatusPoints reads one of the two lists, keyed by the game's own status name.
//
// The names are Japanese — 最大HP, 攻撃力 and so on. They are internal keys,
// not display text, and are stored verbatim; internal/paldata maps them to
// Korean for the UI.
func (p *Pal) StatusPoints(list string) map[string]int {
	v, ok := p.params.Get(list)
	if !ok {
		return nil
	}
	a, ok := v.(*gvas.ArrayProperty)
	if !ok || a.Structs == nil {
		return nil
	}

	out := map[string]int{}
	for _, sv := range a.Structs.Values {
		sp, ok := sv.(*gvas.StructProperties)
		if !ok {
			continue
		}
		name, ok := statusEntryName(sp)
		if !ok {
			continue
		}
		if pv, ok := sp.Props.Get("StatusPoint"); ok {
			if ip, ok := pv.(*gvas.IntProperty); ok {
				out[name] = int(ip.Value)
			}
		}
	}
	return out
}

// StatusPointOrder returns the status names in the order the save stores them,
// so the UI can list them the way the game does rather than alphabetically.
func (p *Pal) StatusPointOrder(list string) []string {
	v, ok := p.params.Get(list)
	if !ok {
		return nil
	}
	a, ok := v.(*gvas.ArrayProperty)
	if !ok || a.Structs == nil {
		return nil
	}
	var out []string
	for _, sv := range a.Structs.Values {
		if sp, ok := sv.(*gvas.StructProperties); ok {
			if name, ok := statusEntryName(sp); ok {
				out = append(out, name)
			}
		}
	}
	return out
}

// SetStatusPoint sets one entry, adding it when the player has none.
//
// Unlike work suitability, a zero is kept rather than removed: the save lists
// 作業速度 at 0 for a player who has spent nothing on it, so absence and zero
// are not the same thing here.
func (p *Pal) SetStatusPoint(list, status string, value int) error {
	if value < 0 || value > MaxStatusPoint {
		return fmt.Errorf("palsave: status point %d out of range 0..%d", value, MaxStatusPoint)
	}

	v, ok := p.params.Get(list)
	if !ok {
		arr := newStatusPointList(list)
		p.params.Set(list, arr)
		v = arr
	}
	a, ok := v.(*gvas.ArrayProperty)
	if !ok || a.Structs == nil {
		return fmt.Errorf("palsave: %s is not a struct array", list)
	}

	for _, sv := range a.Structs.Values {
		sp, ok := sv.(*gvas.StructProperties)
		if !ok {
			continue
		}
		name, ok := statusEntryName(sp)
		if !ok || name != status {
			continue
		}
		pv, ok := sp.Props.Get("StatusPoint")
		if !ok {
			sp.Props.Set("StatusPoint", &gvas.IntProperty{Value: int32(value)})
			return nil
		}
		ip, ok := pv.(*gvas.IntProperty)
		if !ok {
			return fmt.Errorf("palsave: StatusPoint is %T, want *gvas.IntProperty", pv)
		}
		ip.Value = int32(value)
		return nil
	}

	a.Structs.Values = append(a.Structs.Values, newStatusPointEntry(status, value))
	return nil
}

func statusEntryName(sp *gvas.StructProperties) (string, bool) {
	v, ok := sp.Props.Get("StatusName")
	if !ok {
		return "", false
	}
	n, ok := v.(*gvas.NameProperty)
	if !ok {
		return "", false
	}
	return n.Value.Value, true
}

func newStatusPointEntry(status string, value int) gvas.StructValue {
	props := gvas.NewProperties()
	props.Set("StatusName", &gvas.NameProperty{Value: gvas.Str(status)})
	props.Set("StatusPoint", &gvas.IntProperty{Value: int32(value)})
	return &gvas.StructProperties{Props: props}
}

func newStatusPointList(list string) *gvas.ArrayProperty {
	return &gvas.ArrayProperty{
		ArrayType: gvas.Str("StructProperty"),
		Structs: &gvas.StructArray{
			PropName: gvas.Str(list),
			PropType: gvas.Str("StructProperty"),
			TypeName: gvas.Str("PalPlayerDataStatusPoint"),
		},
	}
}
