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

	// presets are the user's saved passive sets, stored outside any save file.
	presets *presetStore
}

type playerFile struct {
	path string
	save *palsave.PlayerSave
	// dirty marks a player save the editor has changed. Player saves are read
	// for every session but written only when something in them was edited,
	// so an untouched one is never rewritten — even a byte-identical rewrite
	// would replace a file the game owns for no reason.
	dirty bool
}

func NewApp() *App {
	return &App{
		players: map[string]*playerFile{},
		presets: newPresetStore(),
	}
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
	out := []PlayerInfo{}
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
	// Gender is "Male", "Female", or "" when the save records none — eleven
	// pals in the live save have no Gender at all.
	Gender string `json:"gender"`
	Icon   string `json:"icon"`

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

	// Friendship is accumulated friendship points. The game shows this as a
	// heart gauge; the raw number is what the save stores.
	Friendship int `json:"friendship"`

	// Location is where the pal is kept: "box", "party", "base" or "" when the
	// save records no slot at all.
	Location string `json:"location"`
	// Camp is the 1-based base camp number, 0 for a pal that is not at one.
	Camp int `json:"camp"`

	Passives []PassiveInfo `json:"passives"`

	// Skills is the pal's equipped active skills, at most MaxEquipWaza.
	Skills []SkillInfo `json:"skills"`

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

// SkillInfo is one active (waza) skill for display.
type SkillInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	NameEN  string `json:"nameEn"`
	Element string `json:"element"`
	Type    string `json:"type"`
	Power   int    `json:"power"`
	Unique  bool   `json:"unique"`
	Known   bool   `json:"known"`
}

// describeSkill renders one active-skill id for display, falling back to the
// raw id when the table has no entry — the same stance as passives.
func describeSkill(id string) SkillInfo {
	s, ok := paldata.LookupSkill(id)
	if !ok {
		return SkillInfo{ID: id, Name: id}
	}
	name := s.NameKO
	if name == "" {
		name = id
	}
	return SkillInfo{
		ID: id, Name: name, NameEN: s.NameEN,
		Element: s.Element, Type: s.Type, Power: s.Power,
		Unique: s.Unique, Known: true,
	}
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
		Gender:        p.Gender(),
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

		Friendship: p.Friendship(),

		Name: species,
	}
	if uid == "" {
		info.Location = LocationBase
	} else {
		info.Location = a.palLocation(uid, p)
	}
	// Built empty rather than left nil: a nil slice marshals to JSON null, the
	// generated TypeScript still says PassiveInfo[], and the first `.length`
	// in the UI throws and blanks the whole window. Fifty pals in the live
	// save carry no passives at all, so this is the common case, not an edge.
	info.Passives = make([]PassiveInfo, 0, len(p.Passives()))
	for _, id := range p.Passives() {
		info.Passives = append(info.Passives, describePassive(id))
	}
	info.Skills = make([]SkillInfo, 0, len(p.EquipWaza()))
	for _, id := range p.EquipWaza() {
		info.Skills = append(info.Skills, describeSkill(id))
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

// StorageInfo is one item-holding structure in a base camp.
type StorageInfo struct {
	// Camp is the same 1-based number BaseCamps hands out.
	Camp int `json:"camp"`
	// Kind is the game's own building name, e.g. "ItemChest_04". There is no
	// Korean building table yet, so this is shown raw rather than guessed at.
	Kind string `json:"kind"`
	// ContainerID addresses the box for the item calls below.
	ContainerID string     `json:"containerId"`
	Items       []ItemInfo `json:"items"`
}

// BaseStorages lists the storage in the given player's guild's camps.
//
// Camps are numbered exactly as BaseCamps numbers them, so the two views line
// up. Structures with nothing in them are still listed — an empty chest is a
// place to put things, and hiding it was the bug reported for camps.
func (a *App) BaseStorages(uid string) ([]StorageInfo, error) {
	if a.world == nil {
		return nil, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	camps := a.guildCamps(uid)
	index := make(map[gvas.GUID]int, len(camps))
	for i, c := range camps {
		index[c.ID] = i + 1
	}

	out := []StorageInfo{}
	for _, s := range a.world.CampStorages() {
		n, ok := index[s.CampID]
		if !ok {
			continue // another guild's camp
		}
		out = append(out, StorageInfo{
			Camp:        n,
			Kind:        s.Kind,
			ContainerID: s.ContainerID.String(),
			Items:       a.describeContainer(s.ContainerID),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Camp != out[j].Camp {
			return out[i].Camp < out[j].Camp
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

// AddPal creates a pal in the player's palbox and returns its instance id.
//
// Destination is the palbox rather than the party on purpose: the party has
// five slots and is usually full, and a pal appearing in the box is what a
// user expects from "add".
func (a *App) AddPal(uid, speciesID string, level, rank int, talents map[string]int, passives []string, alpha bool, gender string) (string, error) {
	if a.world == nil {
		return "", fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	pf, ok := a.players[uid]
	if !ok {
		return "", fmt.Errorf("이 플레이어의 세이브 파일이 없습니다 (Players 폴더 확인)")
	}
	box, ok := pf.save.PalStorageContainer()
	if !ok {
		return "", fmt.Errorf("팰박스를 찾을 수 없습니다")
	}
	owner, ok := pf.save.PlayerUID()
	if !ok {
		return "", fmt.Errorf("플레이어 ID를 찾을 수 없습니다")
	}

	if _, ok := paldata.LookupPal(speciesID); !ok {
		return "", fmt.Errorf("모르는 팰입니다: %s", speciesID)
	}
	if err := validatePassives(passives); err != nil {
		return "", err
	}
	for name := range talents {
		if !allowedTalent[name] {
			return "", fmt.Errorf("수정할 수 없는 항목입니다: %s", name)
		}
	}

	id, err := a.world.AddPal(palsave.NewPalSpec{
		SpeciesID: speciesID,
		Level:     level,
		Rank:      rank,
		Talents:   talents,
		Passives:  passives,
		Alpha:     alpha,
		Gender:    gender,
		Owner:     owner,
		Container: box,
	})
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// PalChoice is a searchable species, for the "add pal" picker.
type PalChoice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// SetPalGender sets a pal's gender.
func (a *App) SetPalGender(instanceID, gender string) error {
	p, err := a.findPal(instanceID)
	if err != nil {
		return err
	}
	return p.SetGender(gender)
}

// SetPalAlpha turns the alpha (boss) variant on or off.
//
// Being an alpha is only the BOSS_ prefix on CharacterID; the game derives the
// bigger model and the extra health from it.
func (a *App) SetPalAlpha(instanceID string, alpha bool) error {
	p, err := a.findPal(instanceID)
	if err != nil {
		return err
	}
	return p.SetAlpha(alpha)
}

// SearchPals finds species by id or Korean name.
//
// Real pals rank first, in Paldeck order (tower and raid bosses after them),
// then the human NPCs — wandering and legendary merchants, villagers, bounty
// actors. Browsing still opens on pals, but a search like "상인" now surfaces
// the merchants, which the game will let you keep in a palbox.
//
// ponytail: every NPC is included rather than only the merchants — no name
// heuristic to maintain, and pals fill the browse view first regardless.
func (a *App) SearchPals(q string) []PalChoice {
	found := paldata.SearchPals(q)

	// Unsorted, the pal list opens on a wall of tower boss duos — several of
	// which share a Korean name — which is not what anyone means by "add a pal".
	pals := make([]*paldata.Pal, 0, len(found))
	npcs := make([]*paldata.Pal, 0, len(found))
	for _, p := range found {
		if p.IsPal {
			pals = append(pals, p)
		} else {
			npcs = append(npcs, p)
		}
	}
	sort.SliceStable(pals, func(i, j int) bool {
		bi, bj := palRank(pals[i]), palRank(pals[j])
		if bi != bj {
			return bi < bj
		}
		if pals[i].DeckIndex != pals[j].DeckIndex {
			return pals[i].DeckIndex < pals[j].DeckIndex
		}
		return pals[i].ID < pals[j].ID
	})
	sort.SliceStable(npcs, func(i, j int) bool {
		if npcs[i].NameKO != npcs[j].NameKO {
			return npcs[i].NameKO < npcs[j].NameKO
		}
		return npcs[i].ID < npcs[j].ID
	})
	ranked := append(pals, npcs...)

	out := make([]PalChoice, 0, 50)
	for _, p := range ranked {
		c := PalChoice{ID: p.ID, Name: p.ID}
		if ko, ok := paldata.PalName(p.ID); ok {
			c.Name = ko
		}
		for _, cand := range paldata.PalIconCandidates(p.ID) {
			if icons.Has(cand) {
				c.Icon = cand
				break
			}
		}
		out = append(out, c)
		if len(out) >= 50 {
			break
		}
	}
	return out
}

// PalboxSpace reports how many free slots the player's palbox has, so the UI
// can say so before the user fills in a form that cannot be applied.
func (a *App) PalboxSpace(uid string) (int, error) {
	if a.world == nil {
		return 0, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	pf, ok := a.players[uid]
	if !ok {
		return 0, fmt.Errorf("이 플레이어의 세이브 파일이 없습니다")
	}
	box, ok := pf.save.PalStorageContainer()
	if !ok {
		return 0, fmt.Errorf("팰박스를 찾을 수 없습니다")
	}
	c, ok := a.world.PalContainerByID(box)
	if !ok {
		return 0, fmt.Errorf("팰박스 컨테이너를 찾을 수 없습니다")
	}
	return int(c.Capacity) - len(c.Slots), nil
}

// BasePals lists the pals working at the given player's guild's camps.
func (a *App) BasePals(uid string) ([]PalInfo, error) {
	if a.world == nil {
		return nil, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	camps := a.guildCamps(uid)
	pals := a.campPals(camps)

	out := []PalInfo{}
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

// SetPalFriendship sets a pal's friendship points.
func (a *App) SetPalFriendship(instanceID string, value int) error {
	p, err := a.findPal(instanceID)
	if err != nil {
		return err
	}
	return p.SetFriendship(value)
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

// --- passive presets ------------------------------------------------------

// PresetInfo is a saved passive set as the UI shows it.
//
// Passives carries the resolved entries rather than bare ids so the list can
// render names and tiers without a second round trip per preset.
type PresetInfo struct {
	Name     string        `json:"name"`
	Passives []PassiveInfo `json:"passives"`
	// Stale marks a preset holding an id the reference table no longer knows,
	// which happens after a game update. It is shown rather than hidden, so a
	// preset that silently stopped working is visible instead of mysterious.
	Stale bool `json:"stale"`
}

// Presets lists the saved passive sets, most recently updated first.
func (a *App) Presets() ([]PresetInfo, error) {
	list, err := a.presets.all()
	if err != nil {
		return nil, err
	}
	out := make([]PresetInfo, 0, len(list))
	for _, p := range list {
		info := PresetInfo{Name: p.Name}
		for _, id := range p.IDs {
			d := describePassive(id)
			if !d.Known {
				info.Stale = true
			}
			info.Passives = append(info.Passives, d)
		}
		out = append(out, info)
	}
	return out, nil
}

// SavePreset stores a passive set under a name, replacing one of the same name.
//
// The ids are validated exactly as an edit would be. A preset is only useful
// if it can actually be applied, so refusing a bad one at save time beats
// discovering it later on a pal.
func (a *App) SavePreset(name string, ids []string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("프리셋 이름을 입력하세요")
	}
	if len([]rune(name)) > 40 {
		return fmt.Errorf("프리셋 이름이 너무 깁니다 (최대 40자)")
	}
	if len(ids) == 0 {
		return fmt.Errorf("패시브를 하나 이상 골라야 합니다")
	}
	if err := validatePassives(ids); err != nil {
		return err
	}
	return a.presets.put(name, ids)
}

// DeletePreset removes a saved set.
func (a *App) DeletePreset(name string) error {
	return a.presets.remove(name)
}

// ApplyPreset writes a preset's passives onto one pal, replacing whatever it
// had. It returns the resolved list so the editor can show the new state.
func (a *App) ApplyPreset(instanceID, name string) ([]PassiveInfo, error) {
	list, err := a.presets.all()
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		if !strings.EqualFold(p.Name, name) {
			continue
		}
		// Re-validate: the table can change under a preset saved long ago.
		if err := validatePassives(p.IDs); err != nil {
			return nil, fmt.Errorf("프리셋 %q 을(를) 쓸 수 없습니다: %w", name, err)
		}
		if err := a.SetPalPassives(instanceID, p.IDs); err != nil {
			return nil, err
		}
		out := make([]PassiveInfo, 0, len(p.IDs))
		for _, id := range p.IDs {
			out = append(out, describePassive(id))
		}
		return out, nil
	}
	return nil, fmt.Errorf("그런 이름의 프리셋이 없습니다: %s", name)
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

// SetPalSkills replaces a pal's equipped active skills.
//
// Unknown ids are refused, as with passives: the game drops a skill it does
// not recognise, so a typo would silently cost the pal a slot. At most
// MaxEquipWaza skills; palsave also adds each to MasteredWaza so the game
// keeps it.
func (a *App) SetPalSkills(instanceID string, ids []string) error {
	p, err := a.findPal(instanceID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, ok := paldata.LookupSkill(id); !ok {
			return fmt.Errorf("모르는 액티브 스킬입니다: %s", id)
		}
	}
	return p.SetEquipWaza(ids)
}

// SearchSkills finds active skills by id or name, for the picker.
func (a *App) SearchSkills(q string) []SkillInfo {
	found := paldata.SearchSkills(q)
	out := make([]SkillInfo, 0, len(found))
	for _, s := range found {
		out = append(out, describeSkill(s.ID))
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

// --- player stats ---------------------------------------------------------

// StatInfo is one status point row of the player editor.
//
// The save keys these by Japanese name — 最大HP and so on — which are internal
// identifiers, not display text. Key is what gets written back; Name is ours.
type StatInfo struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Value int    `json:"value"`
	// Known is false for a status the label table does not cover, so the UI
	// can show the raw key rather than pretend it understood it.
	Known bool `json:"known"`
}

// PlayerDetail is everything the player editor shows.
type PlayerDetail struct {
	UID    string `json:"uid"`
	Name   string `json:"name"`
	Level  int    `json:"level"`
	Exp    int64  `json:"exp"`
	Unused int    `json:"unused"`

	// Stats is what levelling lets you spend. Ex is a second, separate track
	// whose values sit far higher; the two are not interchangeable.
	Stats []StatInfo `json:"stats"`
	Ex    []StatInfo `json:"ex"`
}

// PlayerDetail returns one player's editable numbers.
func (a *App) PlayerDetail(uid string) (*PlayerDetail, error) {
	c, err := a.findPlayer(uid)
	if err != nil {
		return nil, err
	}
	p := c.Pal

	d := &PlayerDetail{
		UID:    c.PlayerUID.String(),
		Name:   p.Nickname(),
		Level:  p.Level(),
		Exp:    p.Exp(),
		Unused: p.UnusedStatusPoint(),
		Stats:  describeStats(p, palsave.StatusPointList),
		Ex:     describeStats(p, palsave.ExStatusPointList),
	}
	return d, nil
}

// describeStats lists a status track in the game's order, then anything the
// save holds that the table does not know about.
func describeStats(p *palsave.Pal, list string) []StatInfo {
	have := p.StatusPoints(list)
	if len(have) == 0 {
		return nil
	}

	out := []StatInfo{}
	seen := map[string]bool{}
	for _, sp := range paldata.StatusPoints() {
		v, ok := have[sp.ID]
		if !ok {
			continue
		}
		seen[sp.ID] = true
		out = append(out, StatInfo{Key: sp.ID, Name: sp.NameKO, Value: v, Known: true})
	}
	// Anything the label table missed still gets shown, under its raw key.
	for _, key := range p.StatusPointOrder(list) {
		if seen[key] {
			continue
		}
		seen[key] = true
		name, known := paldata.StatusPointName(key)
		if !known {
			name = key
		}
		out = append(out, StatInfo{Key: key, Name: name, Value: have[key], Known: known})
	}
	return out
}

func (a *App) findPlayer(uid string) (*palsave.CharEntry, error) {
	if a.world == nil {
		return nil, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	id, err := gvas.ParseGUID(uid)
	if err != nil {
		return nil, err
	}
	for _, c := range a.world.Players() {
		if c.PlayerUID == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("플레이어를 찾을 수 없습니다: %s", uid)
}

// SetPlayerLevel sets a player's level, keeping experience consistent.
//
// A player is stored like a pal — same Level and Exp fields — but is *not* on
// the pal experience curve, which this used to assume. The curves diverge
// sharply: level 80 costs a pal 144,829,235 and a player 45,859,908. Writing
// the pal figure onto a player therefore left them far above their own
// threshold, and since the game re-derives level from experience as soon as
// any is earned, a lowered level jumped back the instant the player picked
// anything up. A user hit exactly that after eating one egg.
func (a *App) SetPlayerLevel(uid string, level int) error {
	c, err := a.findPlayer(uid)
	if err != nil {
		return err
	}
	// The curve is defined to 100 but SetLevel accepts only up to the game's
	// cap, so the bound quoted here is the one actually enforced.
	exp, ok := palsave.TotalPlayerExpForLevel(level)
	if !ok || level > palsave.MaxPalLevel {
		return fmt.Errorf("레벨 %d 는 범위를 벗어납니다 (1-%d)", level, palsave.MaxPalLevel)
	}
	return c.Pal.SetLevelWithExp(level, exp)
}

// SetPlayerUnusedPoints sets the unspent status point pool.
func (a *App) SetPlayerUnusedPoints(uid string, n int) error {
	c, err := a.findPlayer(uid)
	if err != nil {
		return err
	}
	return c.Pal.SetUnusedStatusPoint(n)
}

// SetPlayerStat sets one status point entry.
//
// track is "" or "ex". The key is written into the save verbatim, so it is
// checked against what the player already has rather than accepted blind —
// an invented key would add a status the game does not read.
func (a *App) SetPlayerStat(uid, track, key string, value int) error {
	c, err := a.findPlayer(uid)
	if err != nil {
		return err
	}
	list := palsave.StatusPointList
	if track == "ex" {
		list = palsave.ExStatusPointList
	}

	known := false
	for _, k := range c.Pal.StatusPointOrder(list) {
		if k == key {
			known = true
			break
		}
	}
	if !known {
		if _, ok := paldata.StatusPointName(key); !ok {
			return fmt.Errorf("알 수 없는 스테이터스입니다: %s", key)
		}
	}
	return c.Pal.SetStatusPoint(list, key, value)
}

// --- relics (statue of power) ---------------------------------------------

// RelicInfo is one relic row of the player editor.
type RelicInfo struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Count int    `json:"count"`
	Known bool   `json:"known"`
}

// Relics lists a player's relic counts, known types first in a fixed order,
// then anything the save holds that the label table does not cover.
func (a *App) Relics(uid string) ([]RelicInfo, error) {
	pf, ok := a.players[uid]
	if !ok {
		return nil, fmt.Errorf("이 플레이어의 세이브 파일이 없습니다 (Players 폴더 확인)")
	}
	have := pf.save.Relics()

	out := []RelicInfo{}
	seen := map[string]bool{}
	for _, r := range paldata.Relics() {
		key := paldata.QualifyRelic(r.ID)
		seen[key] = true
		out = append(out, RelicInfo{
			Key:   key,
			Name:  r.NameKO,
			Count: have[key],
			Known: true,
		})
	}
	for _, key := range pf.save.RelicOrder() {
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, RelicInfo{Key: key, Name: key, Count: have[key]})
	}
	return out, nil
}

// SetRelic sets one relic count on a player.
//
// Setting capture power also updates the standalone RelicPossessNum, which
// mirrors it — see internal/palsave/relics.go.
func (a *App) SetRelic(uid, relicType string, count int) error {
	pf, ok := a.players[uid]
	if !ok {
		return fmt.Errorf("이 플레이어의 세이브 파일이 없습니다")
	}
	if _, known := paldata.RelicName(relicType); !known {
		// Allow a type the save already holds even if the table lacks a label,
		// but refuse an invented one — the key is written verbatim.
		found := false
		for _, k := range pf.save.RelicOrder() {
			if k == relicType {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("알 수 없는 유물 종류입니다: %s", relicType)
		}
	}
	if err := pf.save.SetRelic(relicType, count); err != nil {
		return err
	}
	pf.dirty = true
	return nil
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

	return a.describeContainer(cid), nil
}

// describeContainer renders one container's stacks for the UI.
func (a *App) describeContainer(cid gvas.GUID) []ItemInfo {
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
	return out
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

// SetPlayerSlotCount sets one stack in the player's own inventory, addressed
// by slot. Same reasoning as SetSlotCount: a player can carry the same item in
// two stacks just as a chest can.
func (a *App) SetPlayerSlotCount(uid string, slot, count int) error {
	cid, err := a.commonContainer(uid)
	if err != nil {
		return err
	}
	return a.world.SetSlotCount(cid, int32(slot), int32(count))
}

// GiveItem adds items, materialising a slot when none is free.
func (a *App) GiveItem(uid, itemID string, count int) (int32, error) {
	cid, err := a.commonContainer(uid)
	if err != nil {
		return 0, err
	}
	return a.giveItem(cid, itemID, count)
}

// SetContainerItemCount sets a stack in any container, addressed by id. This
// is what the base camp storage view edits through; the player-inventory calls
// above are the same operation with the container looked up from a uid.
func (a *App) SetContainerItemCount(containerID, itemID string, count int) error {
	cid, err := a.namedContainer(containerID)
	if err != nil {
		return err
	}
	_, err = a.world.SetItemCount(cid, itemID, int32(count))
	return err
}

// SetSlotCount sets one stack, addressed by the slot it sits in.
//
// This is what clicking a stack in the UI uses. SetContainerItemCount above
// addresses by item id, which rewrites every matching stack — fine for the
// search-and-apply toolbar, wrong when a box holds the same item twice and
// the user pointed at one of them.
func (a *App) SetSlotCount(containerID string, slot, count int) error {
	cid, err := a.namedContainer(containerID)
	if err != nil {
		return err
	}
	return a.world.SetSlotCount(cid, int32(slot), int32(count))
}

// GiveContainerItem adds items to any container, addressed by id.
func (a *App) GiveContainerItem(containerID, itemID string, count int) (int32, error) {
	cid, err := a.namedContainer(containerID)
	if err != nil {
		return 0, err
	}
	return a.giveItem(cid, itemID, count)
}

// giveItem routes to the right giver by whether the item stacks. A
// non-stackable item — a weapon, tool, shield, armour — is a per-instance
// object with durability, ammo and shield state that lives in a separate
// record, so it goes through GiveInstancedItem, one item per slot. Giving it
// as a plain stack is what left a user's weapons unable to reload.
func (a *App) giveItem(cid gvas.GUID, itemID string, count int) (int32, error) {
	if isInstancedItem(itemID) {
		donor, err := a.dynamicDonor(itemID)
		if err != nil {
			return 0, err
		}
		var last int32
		for i := 0; i < count; i++ {
			idx, err := a.world.GiveInstancedItem(cid, itemID, donor)
			if err != nil {
				return last, err
			}
			last = idx
		}
		return last, nil
	}
	// Stackable: pass the item's stack cap so a large count spreads across
	// slots instead of overflowing one past its limit.
	max := int32(0)
	if it, ok := paldata.LookupItem(itemID); ok {
		max = int32(it.MaxStackCount)
	}
	return a.world.GiveStackableItem(cid, itemID, int32(count), max)
}

// isInstancedItem reports whether an item needs a per-instance state record.
// The game gives every non-stackable item one; max_stack_count of 1 is the
// tell. An item the table does not know is treated as stackable, the safe
// default — it just gets no state record, which is exactly today's behaviour.
func isInstancedItem(itemID string) bool {
	it, ok := paldata.LookupItem(itemID)
	if !ok {
		return false
	}
	return it.MaxStackCount == 1
}

// dynamicDonor picks an item already in the save whose per-instance record can
// be the template for itemID's. The record's tail depends on the item category
// (type_b), not the item, so any item of the same category serves — a grade-1
// assault rifle donates for a grade-5 plasma rifle. The exact item is used when
// the save happens to have it.
//
// An error naming the category, rather than a silent failure, is what tells the
// user why a weapon nobody in the save has ever held cannot be added: nothing
// of its kind exists to model it on.
func (a *App) dynamicDonor(itemID string) (string, error) {
	if a.world.HasDynamicItem(itemID) {
		return itemID, nil
	}
	it, ok := paldata.LookupItem(itemID)
	if !ok {
		return "", fmt.Errorf("모르는 아이템입니다: %s", itemID)
	}
	// Any other item of the same category that the save already holds.
	for _, cand := range paldata.ItemsOfType(it.TypeB) {
		if cand.ID != itemID && a.world.HasDynamicItem(cand.ID) {
			return cand.ID, nil
		}
	}
	return "", fmt.Errorf("이 세이브에 %s 계열 아이템이 하나도 없어 %s 를 만들 수 없습니다. "+
		"같은 계열 아이템을 게임에서 하나 얻은 뒤 다시 시도하세요", it.TypeB, itemID)
}

// namedContainer resolves a container id string, refusing one the save does
// not hold rather than letting a typo write into nothing.
func (a *App) namedContainer(containerID string) (gvas.GUID, error) {
	if a.world == nil {
		return gvas.GUID{}, fmt.Errorf("세이브가 열려 있지 않습니다")
	}
	cid, err := gvas.ParseGUID(containerID)
	if err != nil {
		return gvas.GUID{}, fmt.Errorf("보관함 ID 형식이 잘못되었습니다: %w", err)
	}
	if _, ok := a.world.Container(cid); !ok {
		return gvas.GUID{}, fmt.Errorf("그런 보관함이 없습니다")
	}
	return cid, nil
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
		// Weapons and armour come in grades that share one name: a search for
		// "플라즈마 소총" returns five identical-looking rows. The grade is the
		// trailing _2.._5 on the id (bare id is grade 1), so surface it or the
		// list is unusable. The id is shown too, since two grades are otherwise
		// indistinguishable in a dropdown.
		if g := itemGrade(it.ID); g > 1 {
			c.Name = fmt.Sprintf("%s (등급 %d · %s)", c.Name, g, it.ID)
		}
		if icon, ok := paldata.ItemIcon(it.ID); ok && icons.Has(icon) {
			c.Icon = icon
		}
		out = append(out, c)
	}
	return out
}

// itemGrade reads a weapon/armour grade off the id's trailing _2.._5, where a
// bare id (or _Default) is grade 1. Anything else returns 0, meaning "not a
// graded item" — most items have no grade and should show no suffix.
func itemGrade(id string) int {
	if len(id) < 2 || id[len(id)-2] != '_' {
		return 0
	}
	switch id[len(id)-1] {
	case '2', '3', '4', '5':
		return int(id[len(id)-1] - '0')
	}
	return 0
}

// --- saving ---------------------------------------------------------------

// SaveResult reports what was written.
type SaveResult struct {
	BackupPath string `json:"backupPath"`
	SizeBytes  int    `json:"sizeBytes"`
	// PlayerSaves is how many Players/<uid>.sav files were rewritten, which is
	// zero unless something in one of them was edited.
	PlayerSaves int `json:"playerSaves"`
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
	stamp := time.Now().Format("20060102-150405")
	backup := fmt.Sprintf("%s.%s.bak", a.levelPath, stamp)
	if err := os.WriteFile(backup, orig, 0o644); err != nil {
		return nil, fmt.Errorf("백업 실패: %w", err)
	}
	if err := os.WriteFile(a.levelPath, packed, 0o644); err != nil {
		return nil, err
	}

	res := &SaveResult{BackupPath: filepath.Base(backup), SizeBytes: len(packed)}
	written, err := a.savePlayerFiles(stamp)
	if err != nil {
		// The world save is already on disk at this point, so report the
		// failure rather than pretending the whole save succeeded.
		return res, fmt.Errorf("월드 저장은 끝났지만 플레이어 세이브 저장 실패: %w", err)
	}
	res.PlayerSaves = written
	return res, nil
}

// savePlayerFiles writes every player save the editor changed, each with its
// own backup, and returns how many were written.
func (a *App) savePlayerFiles(stamp string) (int, error) {
	n := 0
	for uid, pf := range a.players {
		if !pf.dirty {
			continue
		}
		raw, err := pf.save.Encode()
		if err != nil {
			return n, fmt.Errorf("%s 인코드 실패: %w", uid, err)
		}
		packed, err := oodle.CompressSav(raw, a.saveType)
		if err != nil {
			return n, fmt.Errorf("%s 압축 실패: %w", uid, err)
		}

		orig, err := os.ReadFile(pf.path)
		if err != nil {
			return n, err
		}
		backup := fmt.Sprintf("%s.%s.bak", pf.path, stamp)
		if err := os.WriteFile(backup, orig, 0o644); err != nil {
			return n, fmt.Errorf("%s 백업 실패: %w", uid, err)
		}
		if err := os.WriteFile(pf.path, packed, 0o644); err != nil {
			return n, err
		}
		pf.dirty = false
		n++
	}
	return n, nil
}

// palRank orders the species picker: ordinary pals, then tower bosses, then
// raid bosses. They are all addable; this is only about what comes first.
func palRank(p *paldata.Pal) int {
	switch {
	case p.IsRaidBoss:
		return 2
	case p.IsTowerBoss:
		return 1
	default:
		return 0
	}
}
