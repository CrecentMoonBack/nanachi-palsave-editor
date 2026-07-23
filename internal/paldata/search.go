package paldata

import (
	"sort"
	"strings"
)

// Items returns every item in the table, ordered by the game's own sort id.
// The slice is shared; callers must not mutate it.
func Items() []*Item {
	load()
	return itemList
}

// Pals returns every entry in the pal table, paldeck order first, ordered by
// the game's own paldeck index. Human NPCs have no index and sort last.
// The slice is shared; callers must not mutate it.
func Pals() []*Pal {
	load()
	return palList
}

// SearchSkills returns the active skills whose id, Korean or English name
// contains q. An empty query returns everything, sorted by Korean name so the
// picker reads naturally.
func SearchSkills(q string) []*Skill {
	load()
	q = strings.ToLower(strings.TrimSpace(q))
	out := make([]*Skill, 0, len(skillList))
	for _, s := range skillList {
		if q == "" || matches(q, s.ID, s.NameKO, s.NameEN) {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Unique != out[j].Unique {
			return !out[i].Unique // ordinary skills before signature moves
		}
		return out[i].NameKO < out[j].NameKO
	})
	return out
}

// ItemsOfType returns every item whose type_b matches, in sort order. Used to
// find a same-category donor when creating a per-instance item the save has
// never held — a weapon's state record has the shape of its category, not its
// specific id.
func ItemsOfType(typeB string) []*Item {
	load()
	var out []*Item
	for _, it := range itemList {
		if it.TypeB == typeB {
			out = append(out, it)
		}
	}
	return out
}

// SearchItems returns the items whose id, Korean name or English name contains
// q, case-insensitively. An empty query returns everything.
func SearchItems(q string) []*Item {
	load()
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return itemList
	}
	out := make([]*Item, 0, 16)
	for _, it := range itemList {
		if matches(q, it.ID, it.NameKO, it.NameEN) {
			out = append(out, it)
		}
	}
	return out
}

// SearchPals returns the pals whose id, Korean name or English name contains
// q, case-insensitively. An empty query returns everything.
//
// The query is matched against the plain id, so searching "BOSS_" finds only
// the entries genuinely keyed that way, not every alpha spawn.
func SearchPals(q string) []*Pal {
	load()
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return palList
	}
	out := make([]*Pal, 0, 16)
	for _, p := range palList {
		if matches(q, p.ID, p.NameKO, p.NameEN) {
			out = append(out, p)
		}
	}
	return out
}

// Passives returns every passive a pal can carry, best tier first.
// The slice is shared; callers must not mutate it.
func Passives() []*Passive {
	load()
	return passiveList
}

// SearchPassives returns the passives whose id, Korean name or English name
// contains q, case-insensitively. An empty query returns everything.
func SearchPassives(q string) []*Passive {
	load()
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return passiveList
	}
	out := make([]*Passive, 0, 16)
	for _, p := range passiveList {
		if matches(q, p.ID, p.NameKO, p.NameEN) {
			out = append(out, p)
		}
	}
	return out
}

func matches(lowerQuery string, fields ...string) bool {
	for _, f := range fields {
		if f != "" && strings.Contains(strings.ToLower(f), lowerQuery) {
			return true
		}
	}
	return false
}

// sortItems orders by the game's sort id, then by id so ties are stable across
// runs — map iteration order is not.
func sortItems(s []*Item) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].SortID != s[j].SortID {
			return s[i].SortID < s[j].SortID
		}
		return s[i].ID < s[j].ID
	})
}

// sortPassives orders by tier, best first, so browsing the picker starts at
// the traits anyone actually wants. Detrimental traits (negative rank) sort
// last, below the untiered ones. Ties break on the Korean name, which is what
// the list is labelled with, then on id so the order is stable across runs.
func sortPassives(s []*Passive) {
	sort.Slice(s, func(i, j int) bool {
		a, b := s[i], s[j]
		if (a.Rank < 0) != (b.Rank < 0) {
			return b.Rank < 0
		}
		if a.Rank != b.Rank {
			return a.Rank > b.Rank
		}
		if a.NameKO != b.NameKO {
			return a.NameKO < b.NameKO
		}
		return a.ID < b.ID
	})
}

// sortPals orders by paldeck index. Entries without one (human NPCs, quest
// variants) carry index 0, so they are pushed to the end rather than to the
// front where they would crowd out the first real pal.
func sortPals(s []*Pal) {
	sort.Slice(s, func(i, j int) bool {
		a, b := s[i].DeckIndex, s[j].DeckIndex
		switch {
		case a == 0 && b != 0:
			return false
		case a != 0 && b == 0:
			return true
		case a != b:
			return a < b
		}
		return s[i].ID < s[j].ID
	})
}
