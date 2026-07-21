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
