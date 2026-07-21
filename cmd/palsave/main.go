// Command palsave is the CLI face of the editor.
//
// It exists ahead of the GUI so the editing core can be exercised end to end
// without a UI in the way: if a change cannot be made here, no amount of
// frontend work will fix it.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/oodle"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/paldata"
	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/palsave"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`나나치의 팰월드 세이브 에디터 (CLI)

  palsave codec
        report whether the native Oodle codec loaded

  palsave info <Level.sav>
        container header and archive summary

  palsave players <Level.sav>
        list players, with pal and inventory counts

  palsave pals <Level.sav> -owner <uid> [-species <id>]
        list a player's pals

  palsave set-level <Level.sav> -owner <uid> -species <id> -level <n> [-dry-run]
        set the level of every matching pal

  palsave inv <Level.sav> -player <Players/UID.sav>
        list a player's main inventory

  palsave give <Level.sav> -player <Players/UID.sav> -item <id> -count <n> [-set] [-dry-run]
        add items to a player's main inventory, or -set an exact count

Item and pal ids are the game's internal names. The container ids live in the
player save, not the world save, which is why the item commands need both.

Every command that writes takes a backup first.`)
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "codec":
		if err := oodle.Available(); err != nil {
			return err
		}
		fmt.Println("oodle codec: ok")
		return nil
	case "info":
		return cmdInfo(args[1:])
	case "players":
		return cmdPlayers(args[1:])
	case "pals":
		return cmdPals(args[1:])
	case "set-level":
		return cmdSetLevel(args[1:])
	case "inv":
		return cmdInv(args[1:])
	case "give":
		return cmdGive(args[1:])
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (try: palsave help)", args[0])
	}
}

// --- loading and saving ---------------------------------------------------

type session struct {
	path     string
	saveType oodle.SaveType
	file     *gvas.File
	world    *palsave.World
}

func open(path string) (*session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, st, err := oodle.DecompressSav(data)
	if err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}
	opts := gvas.PalworldOptions()
	f, err := gvas.Decode(raw, opts)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	w, err := palsave.NewWorld(f, opts)
	if err != nil {
		return nil, err
	}
	if err := w.Load(); err != nil {
		return nil, err
	}
	return &session{path: path, saveType: st, file: f, world: w}, nil
}

// save writes the archive back, taking a timestamped backup first.
//
// The backup is not optional. A save holds other players' characters too, and
// this tool is the only thing standing between a bad edit and their loss.
func (s *session) save() error {
	if err := s.world.Flush(); err != nil {
		return err
	}
	raw, err := gvas.Encode(s.file)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	packed, err := oodle.CompressSav(raw, s.saveType)
	if err != nil {
		return fmt.Errorf("compress: %w", err)
	}

	backup := fmt.Sprintf("%s.%s.bak", s.path, time.Now().Format("20060102-150405"))
	orig, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(backup, orig, 0o644); err != nil {
		return fmt.Errorf("writing backup: %w", err)
	}
	fmt.Printf("backup: %s\n", filepath.Base(backup))

	if err := os.WriteFile(s.path, packed, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote:  %s (%d bytes)\n", filepath.Base(s.path), len(packed))
	return nil
}

// --- commands -------------------------------------------------------------

func cmdInfo(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("info needs a path to a .sav file")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	h, err := oodle.ParseHeader(data)
	if err != nil {
		return err
	}
	fmt.Printf("file        %s\n", args[0])
	fmt.Printf("size        %d bytes\n", len(data))
	fmt.Printf("format      %s (save_type %d)\n", h.Type, byte(h.Type))
	fmt.Printf("compressed  %d bytes\n", h.CompressedLen)
	fmt.Printf("expanded    %d bytes\n", h.UncompressedLen)

	s, err := open(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("engine      %s\n", s.file.Header.EngineVersion())
	fmt.Printf("class       %s\n", s.file.Header.SaveGameClassName.Value)

	var players, pals int
	for _, c := range s.world.Chars() {
		if c.Pal.IsPlayer() {
			players++
		} else {
			pals++
		}
	}
	fmt.Printf("players     %d\n", players)
	fmt.Printf("pals        %d\n", pals)
	fmt.Printf("item slots  %d\n", len(s.world.ItemSlots()))
	return nil
}

func cmdPlayers(args []string) error {
	path, _, err := splitPathAndFlags(args)
	if err != nil {
		return err
	}
	s, err := open(path)
	if err != nil {
		return err
	}

	fmt.Printf("%-38s %-22s %6s\n", "UID", "NAME", "PALS")
	for _, p := range s.world.Players() {
		name := p.Pal.Nickname()
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("%-38s %-22s %6d\n", p.PlayerUID, name, len(s.world.PalsOwnedBy(p.PlayerUID)))
	}
	return nil
}

// splitPathAndFlags takes the save path from the first argument and leaves the
// rest for the flag package.
//
// Go's flag parser stops at the first positional, so a path written after the
// flags would silently disable every flag beyond it. Fixing the path at
// position one is unambiguous, unlike sniffing which arguments are flag values
// — boolean flags take none, and there is no way to tell from the text alone.
func splitPathAndFlags(args []string) (string, []string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", nil, fmt.Errorf("expected a .sav path as the first argument")
	}
	return args[0], args[1:], nil
}

func cmdPals(args []string) error {
	fs := flag.NewFlagSet("pals", flag.ContinueOnError)
	owner := fs.String("owner", "", "player UID")
	species := fs.String("species", "", "filter by species id, BOSS_ prefix ignored")
	path, rest, err := splitPathAndFlags(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("pals needs a path to a .sav file")
	}
	uid, err := gvas.ParseGUID(*owner)
	if err != nil {
		return fmt.Errorf("-owner: %w", err)
	}

	s, err := open(path)
	if err != nil {
		return err
	}

	type stat struct {
		count    int
		minLevel int
		maxLevel int
	}
	stats := map[string]*stat{}

	for _, c := range s.world.PalsOwnedBy(uid) {
		sp := c.Pal.Species()
		if *species != "" && !strings.EqualFold(sp, *species) {
			continue
		}
		lv := c.Pal.Level()
		st, ok := stats[sp]
		if !ok {
			st = &stat{minLevel: lv, maxLevel: lv}
			stats[sp] = st
		}
		st.count++
		if lv < st.minLevel {
			st.minLevel = lv
		}
		if lv > st.maxLevel {
			st.maxLevel = lv
		}
	}
	if len(stats) == 0 {
		fmt.Println("no matching pals")
		return nil
	}

	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	total := 0
	fmt.Printf("%-30s %6s  %s\n", "SPECIES", "COUNT", "LEVEL")
	for _, k := range keys {
		st := stats[k]
		level := fmt.Sprintf("%d", st.minLevel)
		if st.minLevel != st.maxLevel {
			level = fmt.Sprintf("%d-%d", st.minLevel, st.maxLevel)
		}
		name := k
		if ko, ok := paldata.PalName(k); ok {
			name = fmt.Sprintf("%s (%s)", k, ko)
		}
		fmt.Printf("%-30s %6d  %s\n", name, st.count, level)
		total += st.count
	}
	fmt.Printf("%-30s %6d\n", "total", total)
	return nil
}

func cmdSetLevel(args []string) error {
	fs := flag.NewFlagSet("set-level", flag.ContinueOnError)
	owner := fs.String("owner", "", "player UID")
	species := fs.String("species", "", "species id, BOSS_ prefix ignored; comma-separated for several")
	level := fs.Int("level", 0, "level to set")
	dry := fs.Bool("dry-run", false, "report what would change without writing")
	path, rest, err := splitPathAndFlags(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("set-level needs a path to a .sav file")
	}
	if *level <= 0 {
		return fmt.Errorf("-level is required")
	}
	uid, err := gvas.ParseGUID(*owner)
	if err != nil {
		return fmt.Errorf("-owner: %w", err)
	}
	wanted := map[string]bool{}
	for _, sp := range strings.Split(*species, ",") {
		if sp = strings.TrimSpace(sp); sp != "" {
			wanted[strings.ToLower(sp)] = true
		}
	}
	if len(wanted) == 0 {
		return fmt.Errorf("-species is required")
	}

	s, err := open(path)
	if err != nil {
		return err
	}

	exp, ok := palsave.TotalPalExpForLevel(*level)
	if !ok {
		return fmt.Errorf("no experience value known for level %d", *level)
	}

	changed := map[string]int{}
	for _, c := range s.world.PalsOwnedBy(uid) {
		if !wanted[strings.ToLower(c.Pal.Species())] {
			continue
		}
		if err := c.Pal.SetLevelWithExp(*level, exp); err != nil {
			return fmt.Errorf("pal %d: %w", c.Index, err)
		}
		changed[c.Pal.Species()]++
	}

	total := 0
	for k, v := range changed {
		fmt.Printf("  %-28s %4d -> level %d\n", k, v, *level)
		total += v
	}
	if total == 0 {
		fmt.Println("no matching pals; nothing to do")
		return nil
	}
	fmt.Printf("  %-28s %4d\n", "total", total)

	if *dry {
		fmt.Println("dry run: nothing written")
		return nil
	}
	return s.save()
}

// openPlayer decodes a Players/<uid>.sav to reach its container ids.
func openPlayer(path string) (*palsave.PlayerSave, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, _, err := oodle.DecompressSav(data)
	if err != nil {
		return nil, fmt.Errorf("decompress player save: %w", err)
	}
	f, err := gvas.Decode(raw, gvas.PalworldOptions())
	if err != nil {
		return nil, fmt.Errorf("decode player save: %w", err)
	}
	return palsave.NewPlayerSave(f)
}

func cmdInv(args []string) error {
	fs := flag.NewFlagSet("inv", flag.ContinueOnError)
	player := fs.String("player", "", "path to Players/<uid>.sav")
	path, rest, err := splitPathAndFlags(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if *player == "" {
		return fmt.Errorf("inv needs a Level.sav and -player")
	}

	s, err := open(path)
	if err != nil {
		return err
	}
	ps, err := openPlayer(*player)
	if err != nil {
		return err
	}
	cid, ok := ps.InventoryContainer(palsave.ContainerCommon)
	if !ok {
		return fmt.Errorf("player save has no main inventory container")
	}

	stacks := s.world.ContainerContents(cid)
	fmt.Printf("container %s, %d occupied slots\n\n", cid, len(stacks))
	fmt.Printf("%5s  %-36s %10s\n", "SLOT", "ITEM", "COUNT")
	for _, st := range stacks {
		fmt.Printf("%5d  %-36s %10d\n", st.Index, st.ItemID, st.Count)
	}
	return nil
}

func cmdGive(args []string) error {
	fs := flag.NewFlagSet("give", flag.ContinueOnError)
	player := fs.String("player", "", "path to Players/<uid>.sav")
	item := fs.String("item", "", "item static id")
	count := fs.Int("count", 0, "how many")
	set := fs.Bool("set", false, "set the stack to this exact count instead of adding")
	dry := fs.Bool("dry-run", false, "report what would change without writing")
	path, rest, err := splitPathAndFlags(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if *player == "" {
		return fmt.Errorf("give needs a Level.sav and -player")
	}
	if *item == "" {
		return fmt.Errorf("-item is required")
	}
	if *count <= 0 && !*set {
		return fmt.Errorf("-count must be positive")
	}

	s, err := open(path)
	if err != nil {
		return err
	}
	ps, err := openPlayer(*player)
	if err != nil {
		return err
	}
	cid, ok := ps.InventoryContainer(palsave.ContainerCommon)
	if !ok {
		return fmt.Errorf("player save has no main inventory container")
	}

	if *set {
		n, err := s.world.SetItemCount(cid, *item, int32(*count))
		if err != nil {
			return err
		}
		fmt.Printf("  %s set to %d across %d slot(s)\n", *item, *count, n)
	} else {
		idx, err := s.world.GiveItem(cid, *item, int32(*count))
		if err != nil {
			return err
		}
		fmt.Printf("  %s +%d -> slot %d\n", *item, *count, idx)
	}

	if *dry {
		fmt.Println("dry run: nothing written")
		return nil
	}
	return s.save()
}
