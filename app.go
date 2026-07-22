package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/icons"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/oodle"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/paldata"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/palsave"
)

// App is the Wails binding surface.
//
// Every method is safe to call before a save is open — they return an error
// rather than panicking, because a UI calls them in whatever order the user
// clicks, not the order the code expects.
type App struct {
	ctx context.Context

	levelPath string
	saveType  oodle.SaveType
	file      *gvas.File
	world     *palsave.World

	// players maps a player UID to their decoded Players/<uid>.sav.
	players map[string]*playerFile
}

type playerFile struct {
	path string
	save *palsave.PlayerSave
}

func NewApp() *App {
	return &App{players: map[string]*playerFile{}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// --- status ---------------------------------------------------------------

// MaxPassives is how many passive skills one pal may carry.
//
// Four is the game's own limit, and the live server save agrees: of 1663 pals
// not one holds a fifth. The UI reads it from Status rather than hardcoding a
// 4 of its own, so the cap has exactly one definition.
const MaxPassives = 4

// Status describes what the app can currently do, for the opening screen.
type Status struct {
	CodecOK    bool   `json:"codecOk"`
	CodecError string `json:"codecError"`
	IconsOK    bool   `json:"iconsOk"`
	IconCount  int    `json:"iconCount"`
	SaveOpen   bool   `json:"saveOpen"`
	SavePath   string `json:"savePath"`

	// Editing bounds, so the UI clamps to the same numbers the setters
	// enforce instead of keeping its own copy of each limit.
	MaxPassives  int `json:"maxPassives"`
	MaxLevel     int `json:"maxLevel"`
	MaxRank      int `json:"maxRank"`
	MaxTalent    int `json:"maxTalent"`
	MaxRankBonus int `json:"maxRankBonus"`
	MaxWork      int `json:"maxWork"`
}

func (a *App) Status() Status {
	s := Status{
		IconsOK:      icons.Available(),
		IconCount:    icons.Count(),
		SaveOpen:     a.world != nil,
		SavePath:     a.levelPath,
		MaxPassives:  MaxPassives,
		MaxLevel:     palsave.MaxPalLevel,
		MaxRank:      palsave.MaxRank,
		MaxTalent:    palsave.MaxTalent,
		MaxRankBonus: palsave.MaxRankBonus,
		MaxWork:      palsave.MaxWorkSuitabilityRank,
	}
	if err := oodle.Available(); err != nil {
		s.CodecError = err.Error()
	} else {
		s.CodecOK = true
	}
	return s
}

// --- opening --------------------------------------------------------------

// PickSaveFile opens a native file dialog and returns the chosen path.
func (a *App) PickSaveFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Level.sav 선택",
		Filters: []runtime.FileFilter{
			{DisplayName: "팰월드 세이브", Pattern: "*.sav"},
		},
	})
}

// SaveInfo summarises an opened save.
type SaveInfo struct {
	Path        string `json:"path"`
	Format      string `json:"format"`
	Engine      string `json:"engine"`
	SizeBytes   int    `json:"sizeBytes"`
	PlayerCount int    `json:"playerCount"`
	PalCount    int    `json:"palCount"`
	ItemSlots   int    `json:"itemSlots"`
	// PlayerSaves is how many Players/<uid>.sav files were found alongside.
	PlayerSaves int `json:"playerSaves"`
}

// OpenSave loads Level.sav and every player save beside it.
//
// The player saves are not optional extras: a player's inventory container ids
// live there, not in the world save, so without them an inventory cannot be
// located at all.
func (a *App) OpenSave(levelPath string) (*SaveInfo, error) {
	if levelPath == "" {
		return nil, fmt.Errorf("경로가 비어 있습니다")
	}
	data, err := os.ReadFile(levelPath)
	if err != nil {
		return nil, err
	}
	raw, st, err := oodle.DecompressSav(data)
	if err != nil {
		return nil, fmt.Errorf("압축 해제 실패: %w", err)
	}
	opts := gvas.PalworldOptions()
	f, err := gvas.Decode(raw, opts)
	if err != nil {
		return nil, fmt.Errorf("디코드 실패: %w", err)
	}
	w, err := palsave.NewWorld(f, opts)
	if err != nil {
		return nil, err
	}
	if err := w.Load(); err != nil {
		return nil, err
	}

	a.levelPath = levelPath
	a.saveType = st
	a.file = f
	a.world = w
	a.players = map[string]*playerFile{}
	a.loadPlayerSaves()

	info := &SaveInfo{
		Path:        levelPath,
		Format:      st.String(),
		Engine:      f.Header.EngineVersion(),
		SizeBytes:   len(data),
		ItemSlots:   len(w.ItemSlots()),
		PlayerSaves: len(a.players),
	}
	for _, c := range w.Chars() {
		if c.Pal.IsPlayer() {
			info.PlayerCount++
		} else {
			info.PalCount++
		}
	}
	return info, nil
}

// loadPlayerSaves reads every .sav in the sibling Players directory.
//
// Failures are skipped rather than fatal: one unreadable player file should
// not stop the rest of the save from being editable.
func (a *App) loadPlayerSaves() {
	dir := filepath.Join(filepath.Dir(a.levelPath), "Players")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".sav") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		raw, _, err := oodle.DecompressSav(data)
		if err != nil {
			continue
		}
		f, err := gvas.Decode(raw, gvas.PalworldOptions())
		if err != nil {
			continue
		}
		ps, err := palsave.NewPlayerSave(f)
		if err != nil {
			continue
		}
		uid, ok := ps.PlayerUID()
		if !ok {
			continue
		}
		a.players[uid.String()] = &playerFile{path: p, save: ps}
	}
}

// --- players --------------------------------------------------------------

// PlayerInfo is one row of the player picker.
type PlayerInfo struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Level     int    `json:"level"`
	PalCount  int    `json:"palCount"`
	HasSave   bool   `json:"hasSave"`
	ItemCount int    `json:"itemCount"`
}

// Players lists everyone in the save.
func (a *App) Players() ([]PlayerInfo, error) {
	if a.world == nil {
		return nil, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	var out []PlayerInfo
	for _, c := range a.world.Players() {
		uid := c.PlayerUID.String()
		name := c.Pal.Nickname()
		if name == "" {
			name = "(이름 없음)"
		}
		pi := PlayerInfo{
			UID:      uid,
			Name:     name,
			Level:    c.Pal.Level(),
			PalCount: len(a.world.PalsOwnedBy(c.PlayerUID)),
		}
		if pf, ok := a.players[uid]; ok {
			pi.HasSave = true
			if cid, ok := pf.save.InventoryContainer(palsave.ContainerCommon); ok {
				pi.ItemCount = len(a.world.ContainerContents(cid))
			}
		}
		out = append(out, pi)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PalCount > out[j].PalCount })
	return out, nil
}

// --- pals -----------------------------------------------------------------

// PalInfo is one pal as the UI shows it.
type PalInfo struct {
	InstanceID string `json:"instanceId"`
	SpeciesID  string `json:"speciesId"`
	Name       string `json:"name"`
	Nickname   string `json:"nickname"`
	IsBoss     bool   `json:"isBoss"`
	Icon       string `json:"icon"`

	Level int   `json:"level"`
	Exp   int64 `json:"exp"`
	Rank  int   `json:"rank"`

	TalentHP      int `json:"talentHp"`
	TalentMelee   int `json:"talentMelee"`
	TalentShot    int `json:"talentShot"`
	TalentDefense int `json:"talentDefense"`

	// Souls spent per stat, 0..MaxRankBonus. Separate from Rank, which is the
	// condense level: a pal can be fully condensed with no souls spent.
	SoulAttack     int `json:"soulAttack"`
	SoulDefence    int `json:"soulDefence"`
	SoulCraftSpeed int `json:"soulCraftSpeed"`
	SoulHP         int `json:"soulHp"`

	// Location is where the pal is kept: "box", "party", "base" or "" when the
	// save records no slot at all.
	Location string `json:"location"`
	// Camp is the 1-based base camp number, 0 for a pal that is not at one.
	Camp int `json:"camp"`

	Passives []PassiveInfo `json:"passives"`

	// Work is every job in the game's order, with what a book has added and
	// what the species starts with.
	Work []WorkInfo `json:"work"`
}

// WorkInfo is one job row of the pal editor.
//
// Bonus and Base are kept apart rather than summed. Bonus is what the save
// actually stores — the rank books added — while Base is the species' innate
// rank from the reference tables. How the two combine into the level the game
// displays has not been established, so the UI shows both and claims neither
// is the total. See docs/HISTORY.md.
type WorkInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Bonus  int    `json:"bonus"`
	Base   int    `json:"base"`
	HasAny bool   `json:"hasAny"`
}

// Pal locations, as the UI groups them.
const (
	LocationBox   = "box"
	LocationParty = "party"
	LocationBase  = "base"
)

// describeWork builds a row per job, in the game's own order so the list does
// not reshuffle as the user clicks between pals.
//
// Every job is listed, including ones the species cannot do, because a book
// can grant a rank in a job the pal has no innate talent for.
func describeWork(species string, bonuses map[string]int) []WorkInfo {
	base, _ := paldata.BaseWorkSuitability(species)

	jobs := paldata.WorkSuitabilities()
	out := make([]WorkInfo, 0, len(jobs))
	for _, j := range jobs {
		bonus := bonuses[paldata.QualifyWorkSuitability(j.ID)]
		w := WorkInfo{
			ID:     j.ID,
			Name:   j.NameKO,
			Bonus:  bonus,
			Base:   base[j.ID],
			HasAny: bonus > 0 || base[j.ID] > 0,
		}
		out = append(out, w)
	}
	return out
}

// palLocation classifies a pal by the container holding it.
//
// The palbox and party container ids live in the owner's Players/<uid>.sav, so
// a player whose save file is missing gets "" rather than a wrong answer.
// Anything held in some other container is at a base camp: BaseCampSaveData is
// not parsed into typed form yet, so this cannot say which camp, only that it
// is not the two containers the player carries.
func (a *App) palLocation(uid string, p *palsave.Pal) string {
	cid, ok := p.ContainerID()
	if !ok {
		return ""
	}
	pf, ok := a.players[uid]
	if !ok {
		return ""
	}
	if box, ok := pf.save.PalStorageContainer(); ok && box == cid {
		return LocationBox
	}
	if party, ok := pf.save.PartyContainer(); ok && party == cid {
		return LocationParty
	}
	return LocationBase
}

// PassiveInfo is one passive skill as the UI shows it, in the pal list and in
// the picker both.
type PassiveInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NameEN string `json:"nameEn"`
	Desc   string `json:"desc"`
	Rank   int    `json:"rank"`

	// Known is false for an id held by a pal that the reference table has no
	// entry for — a game update adding traits, or a save edited by another
	// tool. The UI shows the raw id rather than hiding the trait, because
	// dropping it silently would lose it on the next write.
	Known bool `json:"known"`
}

// describePassive renders one id for display, falling back to the raw id.
func describePassive(id string) PassiveInfo {
	p, ok := paldata.LookupPassive(id)
	if !ok {
		return PassiveInfo{ID: id, Name: id}
	}
	name := p.NameKO
	if name == "" {
		name = id
	}
	return PassiveInfo{
		ID:     id,
		Name:   name,
		NameEN: p.NameEN,
		Desc:   p.DescKO,
		Rank:   p.Rank,
		Known:  true,
	}
}

// Pals lists the pals belonging to a player.
func (a *App) Pals(uid string) ([]PalInfo, error) {
	if a.world == nil {
		return nil, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	owner, err := gvas.ParseGUID(uid)
	if err != nil {
		return nil, fmt.Errorf("잘못된 UID: %w", err)
	}

	owned := a.world.PalsOwnedBy(owner)
	out := make([]PalInfo, 0, len(owned))
	for _, c := range owned {
		out = append(out, a.describePal(uid, c))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Level > out[j].Level
	})
	return out, nil
}

// describePal renders one character as the UI shows it. uid is the owner whose
// palbox and party containers decide the location, and is empty for a base
// camp pal, which belongs to the guild rather than to anyone.
func (a *App) describePal(uid string, c *palsave.CharEntry) PalInfo {
	p := c.Pal
	species := p.Species()

	info := PalInfo{
		InstanceID:    c.InstanceID.String(),
		SpeciesID:     species,
		Nickname:      p.Nickname(),
		IsBoss:        p.IsBoss(),
		Level:         p.Level(),
		Exp:           p.Exp(),
		Rank:          p.Rank(),
		TalentHP:      p.Talent(palsave.TalentHP),
		TalentMelee:   p.Talent(palsave.TalentMelee),
		TalentShot:    p.Talent(palsave.TalentShot),
		TalentDefense: p.Talent(palsave.TalentDefense),

		SoulAttack:     p.RankBonus(palsave.RankAttack),
		SoulDefence:    p.RankBonus(palsave.RankDefence),
		SoulCraftSpeed: p.RankBonus(palsave.RankCraftSpeed),
		SoulHP:         p.RankBonus(palsave.RankHP),

		Name: species,
	}
	if uid == "" {
		info.Location = LocationBase
	} else {
		info.Location = a.palLocation(uid, p)
	}
	for _, id := range p.Passives() {
		info.Passives = append(info.Passives, describePassive(id))
	}
	info.Work = describeWork(species, p.WorkSuitabilityBonuses())
	if ko, ok := paldata.PalName(species); ok {
		info.Name = ko
	}
	info.Icon = palIcon(species)
	return info
}

// --- base camps -----------------------------------------------------------

// guildCamps returns the camps belonging to a player's guild, in a stable
// order so a camp keeps the same number between runs.
//
// Read from BaseCampSaveData and filtered by the owning guild, which matters
// twice over: a server holds several guilds' camps, and a camp with no workers
// exists just the same. Deriving camps from where pals are — the obvious
// shortcut — gets both wrong, silently.
func (a *App) guildCamps(uid string) []palsave.BaseCamp {
	owner, err := gvas.ParseGUID(uid)
	if err != nil {
		return nil
	}
	guild, ok := a.world.GuildOf(owner)
	if !ok {
		return nil
	}
	var out []palsave.BaseCamp
	for _, c := range a.world.BaseCamps() {
		if c.GuildID == guild.ID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID.String() < out[j].ID.String() })
	return out
}

// campPals maps each of a guild's camp containers to the pals inside it.
func (a *App) campPals(camps []palsave.BaseCamp) map[gvas.GUID][]*palsave.CharEntry {
	want := map[gvas.GUID]bool{}
	for _, c := range camps {
		want[c.ContainerID] = true
	}
	out := map[gvas.GUID][]*palsave.CharEntry{}
	for _, c := range a.world.Chars() {
		if c.Pal.IsPlayer() {
			continue
		}
		cid, ok := c.Pal.ContainerID()
		if !ok || !want[cid] {
			continue
		}
		out[cid] = append(out[cid], c)
	}
	return out
}

// CampInfo is one base camp, for the camp filter.
type CampInfo struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	PalCount int    `json:"palCount"`
}

// BaseCamps summarises the camps of the given player's guild.
//
// Numbered rather than named for display: the save's own names are the game's
// untranslated placeholders ("新規生成拠点テンプレート名4(仮)"), which say less
// than a number does. The real name is carried anyway for anyone who wants it.
func (a *App) BaseCamps(uid string) ([]CampInfo, error) {
	if a.world == nil {
		return nil, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	camps := a.guildCamps(uid)
	pals := a.campPals(camps)
	out := make([]CampInfo, 0, len(camps))
	for i, c := range camps {
		out = append(out, CampInfo{
			Index:    i + 1,
			Name:     c.Name,
			PalCount: len(pals[c.ContainerID]),
		})
	}
	return out, nil
}

// BasePals lists the pals working at the given player's guild's camps.
func (a *App) BasePals(uid string) ([]PalInfo, error) {
	if a.world == nil {
		return nil, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	camps := a.guildCamps(uid)
	pals := a.campPals(camps)

	var out []PalInfo
	for i, c := range camps {
		for _, e := range pals[c.ContainerID] {
			info := a.describePal("", e)
			info.Camp = i + 1
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Camp != out[j].Camp {
			return out[i].Camp < out[j].Camp
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Level > out[j].Level
	})
	return out, nil
}

// SpeciesSummary groups a player's pals by species, which is how bulk edits
// are chosen.
type SpeciesSummary struct {
	SpeciesID string `json:"speciesId"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	Count     int    `json:"count"`
	MinLevel  int    `json:"minLevel"`
	MaxLevel  int    `json:"maxLevel"`
}

// PalSpecies summarises a player's pals per species.
func (a *App) PalSpecies(uid string) ([]SpeciesSummary, error) {
	pals, err := a.Pals(uid)
	if err != nil {
		return nil, err
	}
	return summariseSpecies(pals), nil
}

// summariseSpecies groups pals by species for the card grid.
func summariseSpecies(pals []PalInfo) []SpeciesSummary {
	byID := map[string]*SpeciesSummary{}
	for _, p := range pals {
		s, ok := byID[p.SpeciesID]
		if !ok {
			s = &SpeciesSummary{
				SpeciesID: p.SpeciesID,
				Name:      p.Name,
				Icon:      p.Icon,
				MinLevel:  p.Level,
				MaxLevel:  p.Level,
			}
			byID[p.SpeciesID] = s
		}
		s.Count++
		if p.Level < s.MinLevel {
			s.MinLevel = p.Level
		}
		if p.Level > s.MaxLevel {
			s.MaxLevel = p.Level
		}
	}

	out := make([]SpeciesSummary, 0, len(byID))
	for _, s := range byID {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// SetPalLevel changes one pal's level, keeping experience consistent.
func (a *App) SetPalLevel(instanceID string, level int) error {
	p, err := a.findPal(instanceID)
	if err != nil {
		return err
	}
	exp, ok := palsave.TotalPalExpForLevel(level)
	if !ok {
		return fmt.Errorf("레벨 %d 는 범위를 벗어납니다 (1-%d)", level, palsave.MaxPalLevel)
	}
	return p.SetLevelWithExp(level, exp)
}

// SetPalRank sets a pal's condense rank.
func (a *App) SetPalRank(instanceID string, rank int) error {
	p, err := a.findPal(instanceID)
	if err != nil {
		return err
	}
	return p.SetRank(rank)
}

// SetPalTalent sets one IV. Name must be a Talent_* property name.
func (a *App) SetPalTalent(instanceID, name string, value int) error {
	p, err := a.findPal(instanceID)
	if err != nil {
		return err
	}
	if !allowedTalent[name] {
		return fmt.Errorf("알 수 없는 능력치입니다: %s", name)
	}
	return p.SetTalent(name, value)
}

// SetPalRankBonus spends pal souls on one stat. Name must be a Rank_* name.
func (a *App) SetPalRankBonus(instanceID, name string, value int) error {
	p, err := a.findPal(instanceID)
	if err != nil {
		return err
	}
	if !allowedRankBonus[name] {
		return fmt.Errorf("알 수 없는 강화 항목입니다: %s", name)
	}
	return p.SetRankBonus(name, value)
}

// The property names the UI may write. Both setters take a name straight from
// the frontend and hand it to a generic raw-byte write, so an unchecked name
// would let a typo create a junk property on the pal rather than fail.
var (
	allowedTalent = map[string]bool{
		palsave.TalentHP:      true,
		palsave.TalentMelee:   true,
		palsave.TalentShot:    true,
		palsave.TalentDefense: true,
	}
	allowedRankBonus = map[string]bool{
		palsave.RankAttack:     true,
		palsave.RankDefence:    true,
		palsave.RankCraftSpeed: true,
		palsave.RankHP:         true,
	}
)

// SetPalWorkSuitability sets the rank a book has added for one job.
//
// Rank 0 removes the job from the pal, which is how the save represents "no
// book was ever used", rather than storing an explicit zero.
//
// The job name is checked against the reference table for the same reason the
// talent and soul setters are: this writes a name straight into the save, so
// an unrecognised one would create a junk entry the game silently ignores.
func (a *App) SetPalWorkSuitability(instanceID, job string, rank int) error {
	p, err := a.findPal(instanceID)
	if err != nil {
		return err
	}
	if _, ok := paldata.WorkSuitabilityName(job); !ok {
		return fmt.Errorf("알 수 없는 노동 적성입니다: %s", job)
	}
	return p.SetWorkSuitability(job, rank)
}

// WorkSuitabilityOptions lists every job with its Korean label, for the editor
// to render rows in a fixed order.
func (a *App) WorkSuitabilityOptions() []paldata.WorkSuitability {
	return paldata.WorkSuitabilities()
}

// SetPalPassives replaces a pal's passive skill list.
//
// Validated here rather than in palsave, which is the save codec and has no
// reference table to check against. Unknown ids are refused instead of being
// written through: the game drops a trait it does not recognise, so a typo
// would look like it worked and quietly cost the pal a slot.
func (a *App) SetPalPassives(instanceID string, ids []string) error {
	p, err := a.findPal(instanceID)
	if err != nil {
		return err
	}
	if err := validatePassives(ids); err != nil {
		return err
	}
	return p.SetPassives(ids)
}

// validatePassives is the whole rule set for a passive list, kept separate
// from the binding so it can be exercised without a save on disk.
func validatePassives(ids []string) error {
	if len(ids) > MaxPassives {
		return fmt.Errorf("패시브는 최대 %d개까지입니다 (%d개 요청)", MaxPassives, len(ids))
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return fmt.Errorf("패시브가 중복됩니다: %s", passiveLabel(id))
		}
		seen[id] = true
		if _, ok := paldata.LookupPassive(id); !ok {
			return fmt.Errorf("알 수 없는 패시브입니다: %s", id)
		}
	}
	return nil
}

// passiveLabel names a passive for an error message, Korean when known.
func passiveLabel(id string) string {
	if name, ok := paldata.PassiveName(id); ok {
		return name
	}
	return id
}

// SearchPassives finds passives by id, Korean name or English name, for the
// picker. An empty query returns the whole table, best tier first.
func (a *App) SearchPassives(q string) []PassiveInfo {
	found := paldata.SearchPassives(q)
	out := make([]PassiveInfo, 0, len(found))
	for _, p := range found {
		out = append(out, describePassive(p.ID))
	}
	return out
}

// palIcon picks the best available artwork for a species, or "" for none.
//
// paldata decides the preference order and this only asks which of its
// candidates is actually on disk — the two questions are separate because
// artwork is optional and supplied by the user.
func palIcon(species string) string {
	for _, f := range paldata.PalIconCandidates(species) {
		if icons.Has(f) {
			return f
		}
	}
	return ""
}

func (a *App) findPal(instanceID string) (*palsave.Pal, error) {
	if a.world == nil {
		return nil, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	id, err := gvas.ParseGUID(instanceID)
	if err != nil {
		return nil, err
	}
	for _, c := range a.world.Chars() {
		if c.InstanceID == id {
			return c.Pal, nil
		}
	}
	return nil, fmt.Errorf("팰을 찾을 수 없습니다: %s", instanceID)
}

// --- inventory ------------------------------------------------------------

// ItemInfo is one inventory stack as the UI shows it.
type ItemInfo struct {
	Slot   int32  `json:"slot"`
	ItemID string `json:"itemId"`
	Name   string `json:"name"`
	Count  int32  `json:"count"`
	Icon   string `json:"icon"`
}

// Inventory returns a player's main inventory.
func (a *App) Inventory(uid string) ([]ItemInfo, error) {
	cid, err := a.commonContainer(uid)
	if err != nil {
		return nil, err
	}

	stacks := a.world.ContainerContents(cid)
	out := make([]ItemInfo, 0, len(stacks))
	for _, s := range stacks {
		ii := ItemInfo{Slot: s.Index, ItemID: s.ItemID, Count: s.Count, Name: s.ItemID}
		if ko, ok := paldata.ItemName(s.ItemID); ok {
			ii.Name = ko
		}
		if icon, ok := paldata.ItemIcon(s.ItemID); ok && icons.Has(icon) {
			ii.Icon = icon
		}
		out = append(out, ii)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slot < out[j].Slot })
	return out, nil
}

// SetItemCount sets an existing stack to an exact count.
func (a *App) SetItemCount(uid, itemID string, count int) error {
	cid, err := a.commonContainer(uid)
	if err != nil {
		return err
	}
	_, err = a.world.SetItemCount(cid, itemID, int32(count))
	return err
}

// GiveItem adds items, materialising a slot when none is free.
func (a *App) GiveItem(uid, itemID string, count int) (int32, error) {
	cid, err := a.commonContainer(uid)
	if err != nil {
		return 0, err
	}
	return a.world.GiveItem(cid, itemID, int32(count))
}

func (a *App) commonContainer(uid string) (gvas.GUID, error) {
	if a.world == nil {
		return gvas.GUID{}, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	pf, ok := a.players[uid]
	if !ok {
		return gvas.GUID{}, fmt.Errorf("이 플레이어의 세이브 파일이 없습니다 (Players 폴더 확인)")
	}
	cid, ok := pf.save.InventoryContainer(palsave.ContainerCommon)
	if !ok {
		return gvas.GUID{}, fmt.Errorf("인벤토리 컨테이너를 찾을 수 없습니다")
	}
	return cid, nil
}

// --- search ---------------------------------------------------------------

// ItemChoice is a searchable item, for the "give item" picker.
type ItemChoice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// SearchItems finds items by id or Korean name.
func (a *App) SearchItems(q string) []ItemChoice {
	found := paldata.SearchItems(q)
	if len(found) > 50 {
		found = found[:50]
	}
	out := make([]ItemChoice, 0, len(found))
	for _, it := range found {
		c := ItemChoice{ID: it.ID, Name: it.ID}
		if ko, ok := paldata.ItemName(it.ID); ok {
			c.Name = ko
		}
		if icon, ok := paldata.ItemIcon(it.ID); ok && icons.Has(icon) {
			c.Icon = icon
		}
		out = append(out, c)
	}
	return out
}

// --- saving ---------------------------------------------------------------

// SaveResult reports what was written.
type SaveResult struct {
	BackupPath string `json:"backupPath"`
	SizeBytes  int    `json:"sizeBytes"`
}

// SaveToDisk writes the world back, taking a timestamped backup first.
//
// The backup is not optional and not configurable. A save holds other players'
// characters too, and a bad write costs them their progress as well.
func (a *App) SaveToDisk() (*SaveResult, error) {
	if a.world == nil {
		return nil, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	if err := a.world.Flush(); err != nil {
		return nil, err
	}
	raw, err := gvas.Encode(a.file)
	if err != nil {
		return nil, fmt.Errorf("인코드 실패: %w", err)
	}
	packed, err := oodle.CompressSav(raw, a.saveType)
	if err != nil {
		return nil, fmt.Errorf("압축 실패: %w", err)
	}

	orig, err := os.ReadFile(a.levelPath)
	if err != nil {
		return nil, err
	}
	backup := fmt.Sprintf("%s.%s.bak", a.levelPath, time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backup, orig, 0o644); err != nil {
		return nil, fmt.Errorf("백업 실패: %w", err)
	}
	if err := os.WriteFile(a.levelPath, packed, 0o644); err != nil {
		return nil, err
	}
	return &SaveResult{BackupPath: filepath.Base(backup), SizeBytes: len(packed)}, nil
}
