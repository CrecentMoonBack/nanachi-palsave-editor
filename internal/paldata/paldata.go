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

//go:embed data/items.json data/pals.json data/elements.json
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

	// Insertion-ordered views, built once so listing and search are stable.
	itemList []*Item
	palList  []*Pal
)

// load parses the embedded tables. A failure here means the embedded JSON is
// malformed, which is a build-time mistake and not something a caller can act
// on, so it panics rather than threading an error through every lookup.
func load() {
	once.Do(func() {
		mustJSON("data/items.json", &items)
		mustJSON("data/pals.json", &pals)
		mustJSON("data/elements.json", &elements)

		itemList = make([]*Item, 0, len(items))
		for id, it := range items {
			it.ID = id
			itemList = append(itemList, it)
		}
		palList = make([]*Pal, 0, len(pals))
		for id, p := range pals {
			p.ID = id
			palList = append(palList, p)
		}
		for id, e := range elements {
			e.ID = id
		}
		sortItems(itemList)
		sortPals(palList)
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
	if _, ok := pals[id]; ok {
		return id, false
	}
	if stripped, ok := strings.CutPrefix(id, alphaPrefix); ok {
		if _, ok := pals[stripped]; ok {
			return stripped, true
		}
	}
	return id, strings.HasPrefix(id, alphaPrefix)
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

// PalMenuIcon returns the paldeck icon filename for a save id, e.g.
// "t_icehorse_dark_icon_normal.webp". Every pal has one; the human NPCs
// mostly do not.
func PalMenuIcon(id string) (string, bool) {
	p, ok := LookupPal(id)
	if !ok || p.Icon == "" {
		return "", false
	}
	return strings.ToLower(p.Icon) + iconExt, true
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
