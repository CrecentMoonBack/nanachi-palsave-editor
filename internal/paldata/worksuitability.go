package paldata

import "sort"

// WorkSuitability is one of the thirteen jobs a pal can be suited to.
//
// The save writes these as enum members like
// "EPalWorkSuitability::Handcraft"; ID here is the bare name.
type WorkSuitability struct {
	ID     string
	NameKO string
}

// workSuitabilityOrder is the order the game itself lists jobs in, which the
// UI follows so the rows do not shuffle between pals.
var workSuitabilityOrder = []WorkSuitability{
	{"EmitFlame", "불 붙이기"},
	{"Watering", "물 주기"},
	{"Seeding", "파종"},
	{"GenerateElectricity", "발전"},
	{"Handcraft", "손재주"},
	{"Collection", "채집"},
	{"Deforest", "벌목"},
	{"Mining", "채굴"},
	{"OilExtraction", "석유 채굴"},
	{"ProductMedicine", "약품 제작"},
	{"Cool", "냉각"},
	{"Transport", "운반"},
	{"MonsterFarm", "목장"},
}

// enumPrefix is what the save puts before the bare job name.
const enumPrefix = "EPalWorkSuitability::"

// MaxWorkSuitability is the highest rank the game accepts.
const MaxWorkSuitability = 10

// WorkSuitabilities lists every job in the game's own order.
func WorkSuitabilities() []WorkSuitability {
	out := make([]WorkSuitability, len(workSuitabilityOrder))
	copy(out, workSuitabilityOrder)
	return out
}

// WorkSuitabilityName returns the Korean label for a job.
//
// It accepts both the bare name and the full "EPalWorkSuitability::" form, so
// callers can pass whatever the save handed them.
func WorkSuitabilityName(id string) (string, bool) {
	bare := TrimWorkSuitabilityPrefix(id)
	for _, w := range workSuitabilityOrder {
		if w.ID == bare {
			return w.NameKO, true
		}
	}
	return "", false
}

// TrimWorkSuitabilityPrefix reduces a save enum member to its bare job name.
func TrimWorkSuitabilityPrefix(id string) string {
	if len(id) > len(enumPrefix) && id[:len(enumPrefix)] == enumPrefix {
		return id[len(enumPrefix):]
	}
	return id
}

// QualifyWorkSuitability is the inverse: the form the save stores.
func QualifyWorkSuitability(bare string) string {
	if len(bare) > len(enumPrefix) && bare[:len(enumPrefix)] == enumPrefix {
		return bare
	}
	return enumPrefix + bare
}

// BaseWorkSuitability returns a species' innate job ranks, keyed by bare job
// name. Jobs the species cannot do are absent rather than zero.
//
// This is *not* what a save stores. A pal's GotWorkSuitabilityAddRankList
// holds the ranks added by work suitability books, and the relationship
// between that increment and the level the game displays has not been
// established — see docs/HISTORY.md. Treat this as context to show beside the
// stored value, not as a term in a sum.
func BaseWorkSuitability(id string) (map[string]int, bool) {
	load()
	p, ok := LookupPal(id)
	if !ok {
		return nil, false
	}
	base, ok := workBase[p.ID]
	if !ok {
		return nil, false
	}
	out := make(map[string]int, len(base))
	for k, v := range base {
		out[k] = v
	}
	return out, true
}

// BaseWorkSuitabilityList returns the same data in the game's display order,
// omitting jobs the species cannot do.
func BaseWorkSuitabilityList(id string) []WorkRank {
	base, ok := BaseWorkSuitability(id)
	if !ok {
		return nil
	}
	var out []WorkRank
	for _, w := range workSuitabilityOrder {
		if r, ok := base[w.ID]; ok && r > 0 {
			out = append(out, WorkRank{ID: w.ID, NameKO: w.NameKO, Rank: r})
		}
	}
	return out
}

// WorkRank pairs a job with a rank.
type WorkRank struct {
	ID     string `json:"id"`
	NameKO string `json:"nameKo"`
	Rank   int    `json:"rank"`
}

// sortWorkRanks puts ranks into the game's display order.
func sortWorkRanks(in []WorkRank) {
	pos := make(map[string]int, len(workSuitabilityOrder))
	for i, w := range workSuitabilityOrder {
		pos[w.ID] = i
	}
	sort.SliceStable(in, func(a, b int) bool { return pos[in[a].ID] < pos[in[b].ID] })
}
