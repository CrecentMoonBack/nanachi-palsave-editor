package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// tempStore builds a store rooted in a temp dir, so tests never touch the
// user's real presets.
func tempStore(t *testing.T) *presetStore {
	t.Helper()
	return &presetStore{path: filepath.Join(t.TempDir(), "passive-presets.json")}
}

func TestPresetStoreRoundTrip(t *testing.T) {
	s := tempStore(t)

	if got, err := s.all(); err != nil || len(got) != 0 {
		t.Fatalf("empty store: got %d presets, err %v", len(got), err)
	}

	if err := s.put("작업용", []string{"Workaholic", "Artisan"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := s.put("전투용", []string{"Legend"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	// A fresh store over the same file must see both.
	s2 := &presetStore{path: s.path}
	got, err := s2.all()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d presets after reload, want 2", len(got))
	}

	byName := map[string][]string{}
	for _, p := range got {
		byName[p.Name] = p.IDs
	}
	if len(byName["작업용"]) != 2 || byName["작업용"][0] != "Workaholic" {
		t.Errorf("작업용 = %v", byName["작업용"])
	}
}

func TestPresetPutReplacesByName(t *testing.T) {
	s := tempStore(t)
	if err := s.put("세트", []string{"Legend"}); err != nil {
		t.Fatal(err)
	}
	if err := s.put("세트", []string{"Artisan", "Workaholic"}); err != nil {
		t.Fatal(err)
	}

	got, err := s.all()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d presets, want 1 after replacing by name", len(got))
	}
	if len(got[0].IDs) != 2 {
		t.Errorf("ids = %v, want the replacement", got[0].IDs)
	}
}

// Names are matched case-insensitively, so a user does not end up with two
// presets that look identical in the list.
func TestPresetNameIsCaseInsensitive(t *testing.T) {
	s := tempStore(t)
	if err := s.put("Work", []string{"Legend"}); err != nil {
		t.Fatal(err)
	}
	if err := s.put("WORK", []string{"Artisan"}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.all()
	if len(got) != 1 {
		t.Fatalf("got %d presets, want 1", len(got))
	}
}

func TestPresetRemove(t *testing.T) {
	s := tempStore(t)
	if err := s.put("가", []string{"Legend"}); err != nil {
		t.Fatal(err)
	}
	if err := s.remove("가"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	got, _ := s.all()
	if len(got) != 0 {
		t.Errorf("got %d presets after remove, want 0", len(got))
	}
	if err := s.remove("없는것"); err == nil {
		t.Error("removing an absent preset should fail")
	}
}

func TestPresetCap(t *testing.T) {
	s := tempStore(t)
	for i := 0; i < MaxPresets; i++ {
		if err := s.put(string(rune('a'+i%26))+string(rune('0'+i/26)), []string{"Legend"}); err != nil {
			t.Fatalf("put %d: %v", i, err)
		}
	}
	if err := s.put("one too many", []string{"Legend"}); err == nil {
		t.Errorf("expected the %dth preset to be refused", MaxPresets+1)
	}
}

// A corrupt file must not make the editor unusable, and must not be deleted
// out from under the user either.
func TestPresetCorruptFileIsSetAside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "passive-presets.json")
	if err := os.WriteFile(path, []byte("{ this is not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &presetStore{path: path}
	_, err := s.all()
	if err == nil {
		t.Fatal("expected an error for a corrupt file")
	}

	if _, err := os.Stat(path + ".broken"); err != nil {
		t.Errorf("corrupt file should have been kept as .broken: %v", err)
	}

	// And the store still works afterwards.
	s2 := &presetStore{path: path}
	if err := s2.put("새거", []string{"Legend"}); err != nil {
		t.Fatalf("store unusable after corruption: %v", err)
	}
	got, err := s2.all()
	if err != nil || len(got) != 1 {
		t.Errorf("got %d presets, err %v", len(got), err)
	}
}

// The file is written atomically, so a reader never sees a partial one.
func TestPresetWriteLeavesNoTempFile(t *testing.T) {
	s := tempStore(t)
	if err := s.put("가", []string{"Legend"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file should not survive a successful write")
	}

	b, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	var list []Preset
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatalf("written file is not valid json: %v", err)
	}
}

func TestValidatePassivesGuardsPresets(t *testing.T) {
	// Presets go through the same validation as a direct edit, so the rules
	// only exist once. These are the cases that matter for a preset.
	if err := validatePassives([]string{"Legend", "Legend"}); err == nil {
		t.Error("duplicate passives should be refused")
	}
	if err := validatePassives([]string{"NoSuchPassiveExists"}); err == nil {
		t.Error("unknown passive should be refused")
	}
	tooMany := make([]string, MaxPassives+1)
	for i := range tooMany {
		tooMany[i] = "Legend"
	}
	if err := validatePassives(tooMany); err == nil {
		t.Errorf("more than %d passives should be refused", MaxPassives)
	}
}
