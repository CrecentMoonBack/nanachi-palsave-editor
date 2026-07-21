package palsave

import (
	"fmt"
	"strings"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// Pal is a typed view over one character's save parameters.
//
// The accessors exist because reaching into the tree by hand is where edits go
// wrong. Level is a ByteProperty whose value nests one level deeper than Exp's
// Int64Property, so `level.Value = 80` compiles, corrupts the node, and only
// fails later at encode time. Nothing here lets a caller make that mistake.
type Pal struct {
	// Raw is the decoded RawData blob this pal lives in.
	Raw *CharacterRaw
	// params is the SaveParameter property block: the pal's actual fields.
	params *gvas.Properties
}

// NewPal wraps a decoded character blob.
func NewPal(raw *CharacterRaw) (*Pal, error) {
	obj, ok := raw.Props.Get("SaveParameter")
	if !ok {
		return nil, fmt.Errorf("palsave: character has no SaveParameter")
	}
	sp, ok := obj.(*gvas.StructProperty)
	if !ok {
		return nil, fmt.Errorf("palsave: SaveParameter is %T, want *gvas.StructProperty", obj)
	}
	inner, ok := sp.Value.(*gvas.StructProperties)
	if !ok {
		return nil, fmt.Errorf("palsave: SaveParameter value is %T, want nested properties", sp.Value)
	}
	return &Pal{Raw: raw, params: inner.Props}, nil
}

// Params exposes the underlying property block for fields without a typed
// accessor yet.
func (p *Pal) Params() *gvas.Properties { return p.params }

// IsPlayer reports whether this record is a player character rather than a pal.
func (p *Pal) IsPlayer() bool {
	v, ok := p.params.Get("IsPlayer")
	if !ok {
		return false
	}
	b, ok := v.(*gvas.BoolProperty)
	return ok && b.Value
}

// CharacterID is the internal species id, for example "BOSS_IceHorse_Dark".
func (p *Pal) CharacterID() string {
	v, ok := p.params.Get("CharacterID")
	if !ok {
		return ""
	}
	if n, ok := v.(*gvas.NameProperty); ok {
		return n.Value.Value
	}
	return ""
}

// Species strips the alpha/boss prefix from CharacterID, so lookups against
// the reference tables succeed for both variants.
func (p *Pal) Species() string {
	return strings.TrimPrefix(p.CharacterID(), "BOSS_")
}

// IsBoss reports whether this is an alpha/boss variant.
func (p *Pal) IsBoss() bool {
	return strings.HasPrefix(p.CharacterID(), "BOSS_")
}

// OwnerPlayerUID is the player this pal belongs to. Wild and base-camp pals
// may have a zero owner.
func (p *Pal) OwnerPlayerUID() (gvas.GUID, bool) {
	v, ok := p.params.Get("OwnerPlayerUId")
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

// Nickname is the name the owner gave this pal, if any.
func (p *Pal) Nickname() string {
	v, ok := p.params.Get("NickName")
	if !ok {
		return ""
	}
	if s, ok := v.(*gvas.StrProperty); ok {
		return s.Value.Value
	}
	return ""
}

// Level reports the pal's level.
//
// An absent Level property means level 1: the game omits the field at its
// default rather than storing a 1.
func (p *Pal) Level() int {
	v, ok := p.params.Get("Level")
	if !ok {
		return 1
	}
	b, ok := v.(*gvas.ByteProperty)
	if !ok || !b.IsRawByte() {
		return 1
	}
	return int(b.Byte)
}

// SetLevel sets the level, creating the property when the pal is still at the
// default. Levels above the game's cap are allowed — saves in the wild contain
// them — but values outside a byte are refused.
func (p *Pal) SetLevel(level int) error {
	if level < 1 || level > 255 {
		return fmt.Errorf("palsave: level %d out of range 1..255", level)
	}
	v, ok := p.params.Get("Level")
	if !ok {
		p.params.Set("Level", &gvas.ByteProperty{
			EnumType: gvas.Str("None"),
			Byte:     uint8(level),
		})
		return nil
	}
	b, ok := v.(*gvas.ByteProperty)
	if !ok {
		return fmt.Errorf("palsave: Level is %T, want *gvas.ByteProperty", v)
	}
	return b.SetByte(uint8(level))
}

// Exp reports accumulated experience.
func (p *Pal) Exp() int64 {
	v, ok := p.params.Get("Exp")
	if !ok {
		return 0
	}
	if e, ok := v.(*gvas.Int64Property); ok {
		return e.Value
	}
	return 0
}

// SetExp sets accumulated experience, creating the property if absent.
//
// Level and Exp must agree or the game will re-derive one from the other; use
// SetLevelWithExp unless you specifically want them apart.
func (p *Pal) SetExp(exp int64) error {
	v, ok := p.params.Get("Exp")
	if !ok {
		p.params.Set("Exp", &gvas.Int64Property{Value: exp})
		return nil
	}
	e, ok := v.(*gvas.Int64Property)
	if !ok {
		return fmt.Errorf("palsave: Exp is %T, want *gvas.Int64Property", v)
	}
	e.Value = exp
	return nil
}

// SetLevelWithExp sets both together, which is what the game expects.
func (p *Pal) SetLevelWithExp(level int, exp int64) error {
	if err := p.SetLevel(level); err != nil {
		return err
	}
	return p.SetExp(exp)
}

// Rank is the condense rank, 1 through 5. Absent means rank 1.
func (p *Pal) Rank() int {
	v, ok := p.params.Get("Rank")
	if !ok {
		return 1
	}
	b, ok := v.(*gvas.ByteProperty)
	if !ok || !b.IsRawByte() {
		return 1
	}
	return int(b.Byte)
}

// SetRank sets the condense rank.
func (p *Pal) SetRank(rank int) error {
	if rank < 1 || rank > MaxRank {
		return fmt.Errorf("palsave: condense rank %d out of range 1..%d", rank, MaxRank)
	}
	return p.setRawByte("Rank", uint8(rank))
}

// MaxRank is a fully condensed pal; MaxTalent is a perfect IV. Both are the
// game's own ceilings, named so the validator and the UI cannot drift apart.
const (
	MaxRank   = 5
	MaxTalent = 100
)

// Talent reports one of the pal's IVs by property name.
func (p *Pal) Talent(name string) int { return p.rawByte(name) }

// rawByte reads a numeric ByteProperty, or 0 when the property is absent.
//
// Absent genuinely means zero for these: an un-upgraded pal has no
// Rank_Attack property at all, and a pal with no Talent_HP rolled 0.
func (p *Pal) rawByte(name string) int {
	v, ok := p.params.Get(name)
	if !ok {
		return 0
	}
	b, ok := v.(*gvas.ByteProperty)
	if !ok || !b.IsRawByte() {
		return 0
	}
	return int(b.Byte)
}

// Talent property names, as they appear in the save.
const (
	TalentHP      = "Talent_HP"
	TalentMelee   = "Talent_Melee"
	TalentShot    = "Talent_Shot"
	TalentDefense = "Talent_Defense"
)

// SetTalent sets an IV. The game's own range is 0..100.
func (p *Pal) SetTalent(name string, value int) error {
	if value < 0 || value > MaxTalent {
		return fmt.Errorf("palsave: talent %s value %d out of range 0..%d", name, value, MaxTalent)
	}
	return p.setRawByte(name, uint8(value))
}

// Rank-up counts for the four condense-boostable stats.
const (
	RankAttack     = "Rank_Attack"
	RankDefence    = "Rank_Defence"
	RankCraftSpeed = "Rank_CraftSpeed"
	RankHP         = "Rank_HP"
)

// MaxRankBonus is how far one stat can be raised with pal souls. Ten is the
// game's cap, worth +3% each.
const MaxRankBonus = 10

// RankBonus reports how many souls have been spent on one stat.
func (p *Pal) RankBonus(name string) int { return p.rawByte(name) }

// SetRankBonus sets one of the souls-upgrade counters.
//
// The bound is the game's 0..10, not the byte's 0..255: a higher value is not
// a stronger pal, it is a number the game will clamp or reject, and writing it
// only makes the save disagree with what the UI showed.
func (p *Pal) SetRankBonus(name string, value int) error {
	if value < 0 || value > MaxRankBonus {
		return fmt.Errorf("palsave: %s value %d out of range 0..%d", name, value, MaxRankBonus)
	}
	return p.setRawByte(name, uint8(value))
}

// Passives lists the pal's passive skill ids.
func (p *Pal) Passives() []string {
	v, ok := p.params.Get("PassiveSkillList")
	if !ok {
		return nil
	}
	a, ok := v.(*gvas.ArrayProperty)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(a.Values.Strings))
	for _, s := range a.Values.Strings {
		out = append(out, s.Value)
	}
	return out
}

// SetPassives replaces the passive skill list.
func (p *Pal) SetPassives(ids []string) error {
	vals := make([]gvas.String, 0, len(ids))
	for _, id := range ids {
		vals = append(vals, gvas.Str(id))
	}

	v, ok := p.params.Get("PassiveSkillList")
	if !ok {
		p.params.Set("PassiveSkillList", &gvas.ArrayProperty{
			ArrayType: gvas.Str("NameProperty"),
			Values:    gvas.ArrayValues{Strings: vals},
		})
		return nil
	}
	a, ok := v.(*gvas.ArrayProperty)
	if !ok {
		return fmt.Errorf("palsave: PassiveSkillList is %T, want *gvas.ArrayProperty", v)
	}
	a.Values.Strings = vals
	return nil
}

// WorkSuitabilityBonuses reports the per-suitability levels granted by books,
// keyed by the suitability enum name.
func (p *Pal) WorkSuitabilityBonuses() map[string]int {
	v, ok := p.params.Get("GotWorkSuitabilityAddRankList")
	if !ok {
		return nil
	}
	a, ok := v.(*gvas.ArrayProperty)
	if !ok || a.Structs == nil {
		return nil
	}

	out := map[string]int{}
	for _, sv := range a.Structs.Values {
		props, ok := sv.(*gvas.StructProperties)
		if !ok {
			continue
		}
		var kind string
		if kv, ok := props.Props.Get("WorkSuitability"); ok {
			if e, ok := kv.(*gvas.EnumProperty); ok {
				kind = e.Value.Value
			}
		}
		var rank int
		if rv, ok := props.Props.Get("Rank"); ok {
			if ip, ok := rv.(*gvas.IntProperty); ok {
				rank = int(ip.Value)
			}
		}
		if kind != "" {
			out[kind] = rank
		}
	}
	return out
}

// setRawByte assigns a numeric ByteProperty, creating it when absent.
func (p *Pal) setRawByte(name string, value uint8) error {
	v, ok := p.params.Get(name)
	if !ok {
		p.params.Set(name, &gvas.ByteProperty{EnumType: gvas.Str("None"), Byte: value})
		return nil
	}
	b, ok := v.(*gvas.ByteProperty)
	if !ok {
		return fmt.Errorf("palsave: %s is %T, want *gvas.ByteProperty", name, v)
	}
	return b.SetByte(value)
}
