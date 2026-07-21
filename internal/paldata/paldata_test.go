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
