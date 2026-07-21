package paldata

import (
	"strings"
	"testing"
)

// The ids below were all read out of the live server save, so they are the
// exact spellings the editor has to cope with, not ids invented from the table.

func TestPalNameRealSaveIDs(t *testing.T) {
	cases := []struct{ id, want string }{
		{"IceHorse_Dark", "흑천마"},
		{"IceNarwhal_Fire", "홍등고래"},
		{"BlackGriffon", "제노그리프"},
	}
	for _, c := range cases {
		got, ok := PalName(c.id)
		if !ok {
			t.Errorf("PalName(%q): not found", c.id)
			continue
		}
		if got != c.want {
			t.Errorf("PalName(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

func TestItemNameRealSaveIDs(t *testing.T) {
	cases := []struct{ id, want string }{
		{"Money", "금화"},
		{"DogCoin", "도그 코인"},
		{"PalSummon_NightLady_Dark", "벨라루주의 석판"},
		{"PalSummon_NightLady_Dark_2", "벨라루주[궁극]의 석판"},
	}
	for _, c := range cases {
		got, ok := ItemName(c.id)
		if !ok {
			t.Errorf("ItemName(%q): not found", c.id)
			continue
		}
		if got != c.want {
			t.Errorf("ItemName(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

func TestPassiveNameRealSaveIDs(t *testing.T) {
	// Every one of these is on a pal in the live server save. The last two are
	// the reason the build script does not filter on add_pal: upstream marks
	// neither of them as pal-obtainable, yet there they are.
	cases := []struct{ id, want string }{
		{"CraftSpeed_up2", "장인 기질"},
		{"CraftSpeed_up3", "초절기교"},
		{"PAL_CorporateSlave", "일 노예"},
		{"Vampire", "흡혈귀"},
		{"PAL_ALLAttack_up3", "귀신"},
		{"CoolTimeReduction_Up_1", "냉철함"},
		{"WorldTree_CraftSpeed", "악마의 손"},
		{"Legend", "전설"},
	}
	for _, c := range cases {
		got, ok := PassiveName(c.id)
		if !ok {
			t.Errorf("PassiveName(%q): not found", c.id)
			continue
		}
		if got != c.want {
			t.Errorf("PassiveName(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

// TestPassivesExcludeGearOnly pins the filter down at both ends: a gear-only
// passive must be absent, and a pal one present. Without the first half a
// regenerated table could silently readmit 147 unusable entries.
func TestPassivesExcludeGearOnly(t *testing.T) {
	for _, id := range []string{"AirDash_1", "BossDefeatReward_Anubis"} {
		if _, ok := LookupPassive(id); ok {
			t.Errorf("LookupPassive(%q): gear-only passive is in the table", id)
		}
	}
	if _, ok := LookupPassive("Legend"); !ok {
		t.Error("LookupPassive(Legend): missing")
	}
}

// TestPassivesSortedBestFirst guards the picker's browse order: beneficial
// tiers descend, and detrimental traits sit below every beneficial one rather
// than sorting with the untiered entries.
func TestPassivesSortedBestFirst(t *testing.T) {
	all := Passives()
	if len(all) == 0 {
		t.Fatal("Passives() is empty")
	}
	seenNegative := false
	prev := all[0]
	for _, p := range all[1:] {
		if p.IsNegative() {
			seenNegative = true
		} else if seenNegative {
			t.Fatalf("beneficial %q (rank %d) sorts after a detrimental one", p.ID, p.Rank)
		}
		if p.IsNegative() == prev.IsNegative() && p.Rank > prev.Rank {
			t.Fatalf("%q (rank %d) sorts after %q (rank %d)", p.ID, p.Rank, prev.ID, prev.Rank)
		}
		prev = p
	}
	if !seenNegative {
		t.Error("no detrimental passives in the table; the rank sign is not being read")
	}
}

// TestNoPalGetsTheGenericHumanIcon is the regression guard for the bug that
// drew a hooded silhouette over most of the pal grid.
//
// Upstream fills the icon field of 567 of its 809 entries with the catch-all
// human portrait. 169 of those are real pals, and because the file genuinely
// exists on disk every "does the artwork resolve?" check passed while the
// picture was wrong. Only the pal's own full-body art is correct for them.
func TestNoPalGetsTheGenericHumanIcon(t *testing.T) {
	generic := genericHumanIcon + iconExt
	offenders := 0
	for _, p := range Pals() {
		cands := PalIconCandidates(p.ID)
		if len(cands) == 0 {
			t.Errorf("PalIconCandidates(%q) is empty", p.ID)
			continue
		}
		if !strings.EqualFold(cands[0], generic) {
			continue
		}
		// Nobody may be offered the generic portrait first — not even a human
		// NPC, who may still have full-body art worth preferring.
		if offenders < 5 {
			t.Errorf("PalIconCandidates(%q)[0] = %q, the generic portrait", p.ID, cands[0])
		}
		offenders++
	}
	if offenders > 5 {
		t.Errorf("... and %d more entries offered the generic portrait first", offenders-5)
	}
}

// TestGenericIconPalsPreferFullBody names three of the affected pals and pins
// the behaviour: the generic is not offered at all, full-body art leads.
func TestGenericIconPalsPreferFullBody(t *testing.T) {
	for _, c := range []struct{ id, want string }{
		{"Sekhmet", "sekhmet.webp"},
		{"Anubis", "anubis.webp"},
		{"DarkFlameFox", "darkflamefox.webp"},
	} {
		cands := PalIconCandidates(c.id)
		if len(cands) == 0 || cands[0] != c.want {
			t.Errorf("PalIconCandidates(%q) = %v, want %q first", c.id, cands, c.want)
		}
		for _, got := range cands {
			if strings.EqualFold(got, genericHumanIcon+iconExt) {
				t.Errorf("PalIconCandidates(%q) offers the generic portrait: %v", c.id, cands)
			}
		}
	}
}

// TestGenericIconIsTheLastResort is the other half of the rule. A nameless
// merchant has no artwork of its own, and for it the human portrait is the
// right picture — it just has to come after everything specific has missed.
func TestGenericIconIsTheLastResort(t *testing.T) {
	generic := genericHumanIcon + iconExt
	var checked int
	for _, p := range Pals() {
		// Pals are excluded by design: for them the silhouette is worse than
		// the UI's text badge, so it is not offered at all.
		if p.IsPal || !strings.EqualFold(p.Icon, genericHumanIcon) {
			continue
		}
		cands := PalIconCandidates(p.ID)
		if len(cands) < 2 {
			t.Fatalf("PalIconCandidates(%q) = %v, want specific names before the generic", p.ID, cands)
		}
		if last := cands[len(cands)-1]; !strings.EqualFold(last, generic) {
			t.Fatalf("PalIconCandidates(%q) ends with %q, want the generic portrait last", p.ID, last)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no entry uses the generic portrait; the fixture assumption is stale")
	}
}

// TestNamedNPCsGetTheirOwnArtwork covers what the generic portrait was hiding:
// 조이 and the wandering merchants do have pictures, reached either by the
// full-body name or the t_human_ one.
func TestNamedNPCsGetTheirOwnArtwork(t *testing.T) {
	for _, c := range []struct{ id, want string }{
		{"GrassBoss", "t_human_grassboss_icon_normal.webp"}, // 조이
		{"SalesPerson", "salesperson.webp"},                 // 방랑 상인
		{"Male_DarkTrader01", "male_darktrader01.webp"},     // 암시장 상인
	} {
		cands := PalIconCandidates(c.id)
		var found bool
		for _, got := range cands {
			if got == c.want {
				found = true
			}
		}
		if !found {
			t.Errorf("PalIconCandidates(%q) = %v, want it to offer %q", c.id, cands, c.want)
		}
	}
}

// TestSaveSpellingsResolve covers the ids a live save writes that the
// reference table spells differently. Every one of these was showing as an
// unknown pal with no name and no artwork.
//
// The table's capitalisation is not authoritative and never was — the save is
// what the editor has to read — so lookup folds case. The lowercase keys are
// unique across the whole table, so nothing becomes ambiguous by doing it.
func TestSaveSpellingsResolve(t *testing.T) {
	cases := []struct {
		saveID    string
		wantKey   string
		wantKO    string
		wantAlpha bool
	}{
		{"KingWhale_otomo", "KingWhale", "그랜웰", false},
		{"SheepBall", "Sheepball", "도로롱", false},
		{"GhostAnglerFish_Fire", "GhostAnglerfish_Fire", "불초롱", false},
		{"Boss_LazyCatFish", "LazyCatfish", "도도롱", true},
		{"Boss_IceFox", "IceFox", "빙호", true},
	}
	for _, c := range cases {
		key, alpha := ResolvePalID(c.saveID)
		if key != c.wantKey {
			t.Errorf("ResolvePalID(%q) key = %q, want %q", c.saveID, key, c.wantKey)
			continue
		}
		if alpha != c.wantAlpha {
			t.Errorf("ResolvePalID(%q) alpha = %v, want %v", c.saveID, alpha, c.wantAlpha)
		}
		if got, ok := PalName(c.saveID); !ok || got != c.wantKO {
			t.Errorf("PalName(%q) = (%q, %v), want (%q, true)", c.saveID, got, ok, c.wantKO)
		}
	}
}

// TestExactKeyBeatsFolding pins the rule that protects the table's own oddly
// shaped keys: the human bounty NPCs really are named BOSS_*, and one entry
// really does end in _Otomo. Stripping either would resolve them to the wrong
// pal, or to nothing.
func TestExactKeyBeatsFolding(t *testing.T) {
	for _, id := range []string{"BOSS_DarkTrader", "GYM_ElecPanda_Otomo"} {
		if _, ok := pals[id]; !ok {
			t.Fatalf("fixture assumption stale: %q is no longer a table key", id)
		}
		key, alpha := ResolvePalID(id)
		if key != id {
			t.Errorf("ResolvePalID(%q) = %q, want it to resolve to itself", id, key)
		}
		if alpha {
			t.Errorf("ResolvePalID(%q) reported an alpha; it is a table key in its own right", id)
		}
	}
}

func TestBossPrefixResolves(t *testing.T) {
	base, alpha := ResolvePalID("BOSS_IceHorse_Dark")
	if base != "IceHorse_Dark" || !alpha {
		t.Errorf("ResolvePalID(BOSS_IceHorse_Dark) = (%q, %v), want (IceHorse_Dark, true)", base, alpha)
	}

	name, ok := PalName("BOSS_IceHorse_Dark")
	if !ok || name != "흑천마" {
		t.Errorf("PalName(BOSS_IceHorse_Dark) = (%q, %v), want (흑천마, true)", name, ok)
	}

	// The plain id must not be reported as an alpha.
	if _, alpha := ResolvePalID("IceHorse_Dark"); alpha {
		t.Error("ResolvePalID(IceHorse_Dark) reported alpha")
	}
}

// pals.json holds genuine BOSS_-prefixed keys for the human raid NPCs. Those
// must resolve to themselves, not get their prefix eaten.
func TestBossPrefixedRealKeyWins(t *testing.T) {
	const id = "BOSS_DarkTrader"
	if _, ok := LookupPal(id); !ok {
		t.Fatalf("LookupPal(%q): not found; the table no longer has this key", id)
	}
	base, alpha := ResolvePalID(id)
	if base != id || alpha {
		t.Errorf("ResolvePalID(%q) = (%q, %v), want (%q, false)", id, base, alpha, id)
	}
}

func TestIcons(t *testing.T) {
	cases := []struct {
		fn   func(string) (string, bool)
		name string
		id   string
		want string
	}{
		{ItemIcon, "ItemIcon", "Money", "t_itemicon_material_money.webp"},
		{PalIcon, "PalIcon", "IceHorse_Dark", "icehorse_dark.webp"},
		{PalIcon, "PalIcon", "BOSS_IceHorse_Dark", "icehorse_dark.webp"},
		{PalIcon, "PalIcon", "BlackGriffon", "blackgriffon.webp"},
		{PalMenuIcon, "PalMenuIcon", "IceHorse_Dark", "t_icehorse_dark_icon_normal.webp"},
	}
	for _, c := range cases {
		got, ok := c.fn(c.id)
		if !ok {
			t.Errorf("%s(%q): not found", c.name, c.id)
			continue
		}
		if got != c.want {
			t.Errorf("%s(%q) = %q, want %q", c.name, c.id, got, c.want)
		}
	}
}

// A miss must be reported as a miss. An empty string with ok=true would look
// like a pal legitimately named "" and end up rendered as a blank row.
func TestUnknownIDsFailCleanly(t *testing.T) {
	const junk = "NotAPal_Totally_Made_Up"

	if s, ok := PalName(junk); ok || s != "" {
		t.Errorf("PalName(%q) = (%q, %v), want (\"\", false)", junk, s, ok)
	}
	if s, ok := ItemName(junk); ok || s != "" {
		t.Errorf("ItemName(%q) = (%q, %v), want (\"\", false)", junk, s, ok)
	}
	if s, ok := PalIcon(junk); ok || s != "" {
		t.Errorf("PalIcon(%q) = (%q, %v), want (\"\", false)", junk, s, ok)
	}
	if s, ok := ItemIcon(junk); ok || s != "" {
		t.Errorf("ItemIcon(%q) = (%q, %v), want (\"\", false)", junk, s, ok)
	}
	if p, ok := LookupPal(junk); ok || p != nil {
		t.Errorf("LookupPal(%q) = (%v, %v), want (nil, false)", junk, p, ok)
	}
	if it, ok := LookupItem(junk); ok || it != nil {
		t.Errorf("LookupItem(%q) = (%v, %v), want (nil, false)", junk, it, ok)
	}

	// BOSS_ on a nonsense id must not manufacture a hit either.
	if _, ok := PalName(alphaPrefix + junk); ok {
		t.Errorf("PalName(%q): unexpected hit", alphaPrefix+junk)
	}
}

func TestPalFields(t *testing.T) {
	p, ok := LookupPal("BOSS_IceHorse_Dark")
	if !ok {
		t.Fatal("LookupPal(BOSS_IceHorse_Dark): not found")
	}
	if !p.IsPal {
		t.Error("IceHorse_Dark: IsPal = false")
	}
	if len(p.Elements) != 1 || p.Elements[0] != "Dark" {
		t.Errorf("IceHorse_Dark elements = %v, want [Dark]", p.Elements)
	}
	if p.NameEN != "Frostallion Noct" {
		t.Errorf("IceHorse_Dark NameEN = %q, want Frostallion Noct", p.NameEN)
	}
	if p.DescKO == "" {
		t.Error("IceHorse_Dark: no Korean description")
	}
	// ID is filled in from the map key, and holds the resolved base id.
	if p.ID != "IceHorse_Dark" {
		t.Errorf("ID = %q, want IceHorse_Dark", p.ID)
	}
}

func TestSearch(t *testing.T) {
	if got := SearchPals("흑천"); len(got) != 1 || got[0].ID != "IceHorse_Dark" {
		t.Errorf("SearchPals(흑천) = %v, want [IceHorse_Dark]", ids(got))
	}
	if got := SearchPals("frostallion noct"); len(got) != 1 || got[0].ID != "IceHorse_Dark" {
		t.Errorf("SearchPals(frostallion noct) = %v, want [IceHorse_Dark]", ids(got))
	}
	// Case-insensitive on the id itself.
	if got := SearchPals("blackgriffon"); len(got) == 0 {
		t.Error("SearchPals(blackgriffon) found nothing")
	}
	if got := SearchItems("도그 코인"); len(got) != 1 || got[0].ID != "DogCoin" {
		t.Errorf("SearchItems(도그 코인) = %v, want [DogCoin]", itemIDs(got))
	}
	if got := SearchItems("no such item anywhere"); len(got) != 0 {
		t.Errorf("SearchItems(nonsense) = %v, want empty", itemIDs(got))
	}
	if got := SearchItems(""); len(got) != len(Items()) {
		t.Errorf("SearchItems(\"\") returned %d, want all %d", len(got), len(Items()))
	}
}

func TestTablesLoaded(t *testing.T) {
	if n := len(Items()); n < 2000 {
		t.Errorf("item table has %d entries, expected the full table", n)
	}
	if n := len(Pals()); n < 800 {
		t.Errorf("pal table has %d entries, expected the full table", n)
	}
	if _, ok := LookupElement("Dark"); !ok {
		t.Error("LookupElement(Dark): not found")
	}
}

// Every entry must carry an id and, for anything the GUI will actually show, a
// Korean name. A regression in the extraction step would show up here first.
func TestKoreanCoverage(t *testing.T) {
	var missing []string
	for _, p := range Pals() {
		if p.ID == "" {
			t.Fatal("pal entry with empty ID")
		}
		if p.NameKO == "" {
			missing = append(missing, p.ID)
		}
	}
	if len(missing) != 0 {
		t.Errorf("%d pals without a Korean name: %v", len(missing), missing[:min(5, len(missing))])
	}

	missing = missing[:0]
	for _, it := range Items() {
		if it.NameKO == "" {
			missing = append(missing, it.ID)
		}
	}
	// The known gap is 15 unlocalised debug/blueprint items; keep it from growing.
	if len(missing) > 15 {
		t.Errorf("%d items without a Korean name, expected at most 15: %v", len(missing), missing[:5])
	}
	for _, id := range missing {
		if !strings.HasPrefix(id, "Blueprint_") && !strings.HasPrefix(id, "TEST_") &&
			!strings.HasPrefix(id, "Otomo_") && !strings.HasPrefix(id, "Yakushima") &&
			!strings.HasPrefix(id, "PV_") && !strings.HasPrefix(id, "MonsterEquipWeapon_") {
			t.Errorf("unexpected item without a Korean name: %q", id)
		}
	}
}

func ids(ps []*Pal) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.ID
	}
	return out
}

func itemIDs(is []*Item) []string {
	out := make([]string, len(is))
	for i, it := range is {
		out[i] = it.ID
	}
	return out
}
