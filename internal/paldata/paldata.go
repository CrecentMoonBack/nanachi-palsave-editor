// Package paldata is the offline reference table for Palworld's items and
// pals: Korean names for display, English names for search, and the icon
// filenames the GUI uses to find artwork.
//
// The tables are embedded, so lookups never touch the network or the game
// install. No artwork is embedded — only the id-to-filename mapping, which is
// the game's own naming. See docs/THIRD_PARTY.md.
//
// Parsing is deferred until first use rather than done in init, because the
// CLI decodes saves without ever asking for a display name and there is no
// reason to make it pay ~1.2 MB of JSON for that.
package paldata

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed data/items.json data/pals.json data/elements.json data/passives.json
var files embed.FS

// iconExt is the extension of the artwork the GUI looks for in assets/icons/.
// The reference UI ships .webp and nothing else, so the mapping commits to it.
const iconExt = ".webp"

// alphaPrefix marks an alpha (boss) spawn in a live save. Saves contain ids
// like "BOSS_IceHorse_Dark" while the reference table keys on "IceHorse_Dark".
//
// The prefix is not purely decorative: pals.json also holds genuine BOSS_ keys
// for the human raid NPCs ("BOSS_DarkTrader" and friends), so an exact match
// must always be tried before the prefix is stripped.
const alphaPrefix = "BOSS_"

// companionSuffix marks the follower form of a pal — "KingWhale_otomo" is the
// same 그랜웰 as "KingWhale". The table keys only the base form, except for one
// genuine GYM_*_Otomo entry that an exact match protects.
const companionSuffix = "_Otomo"

// Item is one entry of the item table.
type Item struct {
	ID     string `json:"-"`
	NameKO string `json:"name_ko"`
	NameEN string `json:"name_en"`
	DescKO string `json:"desc_ko"`

	// Icon is the base filename with no extension, e.g.
	// "t_itemicon_material_money". Use ItemIcon for the name on disk.
	Icon string `json:"icon"`

	TypeA         string  `json:"type_a"`
	TypeB         string  `json:"type_b"`
	Group         string  `json:"group"`
	Rank          int     `json:"rank"`
	Rarity        int     `json:"rarity"`
	MaxStackCount int     `json:"max_stack_count"`
	Weight        float64 `json:"weight"`
	Price         float64 `json:"price"`
	SortID        int     `json:"sort_id"`
	Disabled      bool    `json:"disabled"`
}

// Pal is one entry of the pal table. It also covers the human NPCs, which the
// game stores in the same table; IsPal separates them.
type Pal struct {
	ID     string `json:"-"`
	NameKO string `json:"name_ko"`
	NameEN string `json:"name_en"`
	DescKO string `json:"desc_ko"`

	// Icon is the menu icon base name, e.g. "t_icehorse_dark_icon_normal".
	// The full-body artwork is named after the id instead — see PalIcon.
	Icon string `json:"icon"`

	Tribe     string   `json:"tribe"`
	Size      string   `json:"size"`
	Genus     string   `json:"genus_category"`
	DeckIndex int      `json:"pal_deck_index"`
	Rarity    int      `json:"rarity"`
	Elements  []string `json:"element_types"`

	IsPal       bool `json:"is_pal"`
	IsBoss      bool `json:"is_boss"`
	IsTowerBoss bool `json:"is_tower_boss"`
	IsRaidBoss  bool `json:"is_raid_boss"`
	Nocturnal   bool `json:"nocturnal"`
	Predator    bool `json:"predator"`
	Disabled    bool `json:"disabled"`
}

// Passive is one entry of the passive skill table — the traits a pal carries
// in its PassiveSkillList.
//
// Gear-only passives are not in the table at all: the build script drops them,
// because a pal cannot hold them and they would only pad the picker. See
// scripts/build-passives.py for why the filter is not simply add_pal.
type Passive struct {
	ID     string `json:"-"`
	NameKO string `json:"name_ko"`
	NameEN string `json:"name_en"`
	DescKO string `json:"desc_ko"`

	// Rank is the game's own tier: 1..5 for beneficial traits, negative for
	// detrimental ones. It is not a count of anything, so 0 means the upstream
	// table had no rank rather than "neutral".
	Rank int `json:"rank"`
}

// IsNegative reports whether this is a detrimental trait, which the UI marks
// differently from a beneficial one.
func (p *Passive) IsNegative() bool { return p.Rank < 0 }

// Element is one of the nine damage types, with its Korean label and colour.
type Element struct {
	ID        string `json:"-"`
	NameKO    string `json:"name_ko"`
	Color     string `json:"color"`
	Icon      string `json:"icon"`
	BadgeIcon string `json:"badge_icon"`
}

var (
	once     sync.Once
	items    map[string]*Item
	pals     map[string]*Pal
	elements map[string]*Element
	passives map[string]*Passive

	// palsFold maps a lowercased id to its real table key, so a save's
	// capitalisation never has to match the table's.
	palsFold map[string]string

	// Insertion-ordered views, built once so listing and search are stable.
	itemList    []*Item
	palList     []*Pal
	passiveList []*Passive
)

// load parses the embedded tables. A failure here means the embedded JSON is
// malformed, which is a build-time mistake and not something a caller can act
// on, so it panics rather than threading an error through every lookup.
func load() {
	once.Do(func() {
		mustJSON("data/items.json", &items)
		mustJSON("data/pals.json", &pals)
		mustJSON("data/elements.json", &elements)
		mustJSON("data/passives.json", &passives)

		itemList = make([]*Item, 0, len(items))
		for id, it := range items {
			it.ID = id
			itemList = append(itemList, it)
		}
		palList = make([]*Pal, 0, len(pals))
		palsFold = make(map[string]string, len(pals))
		for id, p := range pals {
			p.ID = id
			palList = append(palList, p)
			palsFold[strings.ToLower(id)] = id
		}
		passiveList = make([]*Passive, 0, len(passives))
		for id, p := range passives {
			p.ID = id
			passiveList = append(passiveList, p)
		}
		for id, e := range elements {
			e.ID = id
		}
		sortItems(itemList)
		sortPals(palList)
		sortPassives(passiveList)
	})
}

func mustJSON(name string, dst any) {
	b, err := files.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("paldata: embedded %s missing: %v", name, err))
	}
	if err := json.Unmarshal(b, dst); err != nil {
		panic(fmt.Sprintf("paldata: embedded %s is malformed: %v", name, err))
	}
}

// ResolvePalID maps an id as it appears in a save onto the key used by the
// reference table, and reports whether the id named an alpha (boss) variant.
//
// The exact id wins when the table has it, so the human raid NPCs whose real
// ids start with BOSS_ resolve to themselves rather than to a stripped name
// that does not exist.
func ResolvePalID(id string) (base string, alpha bool) {
	load()
	// An exact hit wins outright. The table holds 34 genuine BOSS_ keys — the
	// human bounty NPCs — and one GYM_*_Otomo entry, and every one of them
	// must resolve to itself rather than be stripped down to something else.
	if _, ok := pals[id]; ok {
		return id, false
	}

	// Everything below is case-insensitive because the save and the table
	// disagree on capitalisation more often than they agree: the save writes
	// Boss_IceFox, SheepBall, LazyCatFish and GhostAnglerFish_Fire where the
	// table has BOSS_/Sheepball/LazyCatfish/GhostAnglerfish. Folding is safe
	// here — no two table keys collide once lowercased.
	rest, alpha := cutPrefixFold(id, alphaPrefix)
	for _, cand := range []string{id, rest} {
		if cand == "" {
			continue
		}
		if k, ok := foldLookup(cand); ok {
			return k, alpha && cand == rest
		}
		if trimmed, cut := cutSuffixFold(cand, companionSuffix); cut {
			if k, ok := foldLookup(trimmed); ok {
				return k, alpha && cand == rest
			}
		}
	}
	return id, alpha
}

// foldLookup finds a table key ignoring case.
func foldLookup(s string) (string, bool) {
	k, ok := palsFold[strings.ToLower(s)]
	return k, ok
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):], true
	}
	return s, false
}

func cutSuffixFold(s, suffix string) (string, bool) {
	if len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix) {
		return s[:len(s)-len(suffix)], true
	}
	return s, false
}

// LookupPal returns the table entry for a save id, resolving the BOSS_ prefix.
// Use ResolvePalID when the caller needs to know it was an alpha.
func LookupPal(id string) (*Pal, bool) {
	base, _ := ResolvePalID(id)
	p, ok := pals[base]
	return p, ok
}

// LookupItem returns the table entry for an item id.
func LookupItem(id string) (*Item, bool) {
	load()
	it, ok := items[id]
	return it, ok
}

// LookupPassive returns the table entry for a passive skill id.
func LookupPassive(id string) (*Passive, bool) {
	load()
	p, ok := passives[id]
	return p, ok
}

// LookupElement returns the entry for an element name such as "Dark".
func LookupElement(id string) (*Element, bool) {
	load()
	e, ok := elements[id]
	return e, ok
}

// PalName returns the Korean name for a save id, resolving the BOSS_ prefix.
// ok is false for an unknown id and for a known id with no Korean name, so a
// caller never mistakes an empty string for a successful lookup.
func PalName(id string) (string, bool) {
	p, ok := LookupPal(id)
	if !ok || p.NameKO == "" {
		return "", false
	}
	return p.NameKO, true
}

// ItemName returns the Korean name for an item id. See PalName on ok.
func ItemName(id string) (string, bool) {
	it, ok := LookupItem(id)
	if !ok || it.NameKO == "" {
		return "", false
	}
	return it.NameKO, true
}

// PassiveName returns the Korean name for a passive skill id. See PalName on ok.
func PassiveName(id string) (string, bool) {
	p, ok := LookupPassive(id)
	if !ok || p.NameKO == "" {
		return "", false
	}
	return p.NameKO, true
}

// ItemIcon returns the artwork filename for an item id, e.g.
// "t_itemicon_material_money.webp". The file itself is not shipped.
func ItemIcon(id string) (string, bool) {
	it, ok := LookupItem(id)
	if !ok || it.Icon == "" {
		return "", false
	}
	return strings.ToLower(it.Icon) + iconExt, true
}

// PalIcon returns the full-body artwork filename for a save id, e.g.
// "icehorse_dark.webp".
//
// The name is derived from the id rather than read from a field, because the
// artwork is shared across spawn variants: alpha, raid, predator and oil rig
// spawns all reuse the base pal's picture. PalMenuIcon is the fallback when
// the full-body file is absent.
func PalIcon(id string) (string, bool) {
	if _, ok := LookupPal(id); !ok {
		return "", false
	}
	return iconStem(id) + iconExt, true
}

// genericHumanIcon is upstream's catch-all NPC portrait — a hooded silhouette.
// 567 of the 809 table entries carry it, so it is a filled-in blank rather
// than artwork anyone chose.
const genericHumanIcon = "t_commonhuman_icon_normal"

// PalMenuIcon returns the paldeck icon filename a save id's table entry names,
// e.g. "t_icehorse_dark_icon_normal.webp".
//
// This is the raw field. For 567 of the 809 entries it is the generic human
// portrait, so prefer PalIconCandidates unless you specifically want what the
// table says.
func PalMenuIcon(id string) (string, bool) {
	p, ok := LookupPal(id)
	if !ok || p.Icon == "" {
		return "", false
	}
	return strings.ToLower(p.Icon) + iconExt, true
}

// PalIconCandidates returns the artwork filenames to try for a save id, best
// first; the caller keeps the first that exists on disk. Empty for an id the
// table does not know.
//
// The ordering exists because the table's icon field is unreliable: 567 of 809
// entries carry the generic human portrait (genericHumanIcon), a real file
// that renders as a hooded silhouette. Taking it at face value drew that
// silhouette for 169 genuine pals, and every "does the artwork resolve?" check
// passed while it did, because the file was really there.
//
// So the rule is: anything specific beats the generic, and the generic is a
// last resort rather than a first answer.
//
//  1. the table's icon, when it is not the generic one
//  2. the full-body art, named after the id
//  3. the human portrait some named NPCs have — only three exist, but one of
//     them is 조이 (GrassBoss), whose table entry points at the generic
//  4. the generic portrait, but only for a human NPC — for a nameless merchant
//     it is the right picture and the only one there is
//
// A real pal is never offered the generic, even as a last resort. The
// silhouette is a human and is identical for every entry that uses it, so on a
// pal it is worse than nothing: the UI's text badge at least says which pal it
// is.
func PalIconCandidates(id string) []string {
	p, ok := LookupPal(id)
	if !ok {
		return nil
	}
	generic := strings.EqualFold(p.Icon, genericHumanIcon)

	var out []string
	if p.Icon != "" && !generic {
		out = append(out, strings.ToLower(p.Icon)+iconExt)
	}
	stem := iconStem(id)
	out = append(out, stem+iconExt, "t_human_"+stem+"_icon_normal"+iconExt)
	if generic && !p.IsPal {
		out = append(out, strings.ToLower(p.Icon)+iconExt)
	}
	return out
}

// iconStem reduces a save id to the artwork base name. The rules mirror
// palworld-save-pal's cleanseCharacterId: each pattern is dropped once, in
// this order, so that e.g. "BOSS_IceHorse_Dark" and "IceHorse_Dark" land on
// the same file.
func iconStem(id string) string {
	s := strings.ToLower(id)
	for _, cut := range []string{
		"predator_", "_oilrig", "raid_", "summon_", "_max",
	} {
		s = strings.Replace(s, cut, "", 1)
	}
	s = trimNumericSuffix(s)
	for _, cut := range []string{"boss_", "quest_farmer03_", "_otomo"} {
		s = strings.Replace(s, cut, "", 1)
	}
	return s
}

// trimNumericSuffix drops a trailing "_2"-style variant marker.
func trimNumericSuffix(s string) string {
	i := strings.LastIndexByte(s, '_')
	if i < 0 || i == len(s)-1 {
		return s
	}
	for _, c := range s[i+1:] {
		if c < '0' || c > '9' {
			return s
		}
	}
	return s[:i]
}
