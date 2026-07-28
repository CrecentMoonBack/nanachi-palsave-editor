package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Passive presets are the user's own shortcuts, not save data, so they live
// beside the app's other settings rather than in the .sav — they have to
// survive opening a different save, or a different server entirely.

// Preset is a named set of passive skill ids.
type Preset struct {
	Name string   `json:"name"`
	IDs  []string `json:"ids"`
	// Updated is when the preset was last written, used only to order the list.
	Updated time.Time `json:"updated"`
}

// MaxPresets caps how many can be stored.
//
// A bound exists so a stuck loop cannot grow the file without limit; it is
// far above what anyone would hand-make.
const MaxPresets = 100

type presetStore struct {
	mu   sync.Mutex
	path string
	list []Preset
	// loaded guards against re-reading the file on every call.
	loaded bool
}

// presetPath is where a preset file lives: the per-user config directory,
// falling back to beside the executable when the OS will not name one. The file
// name is passed in so passive and skill presets get their own file.
func presetPath(file string) string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "NanachiPalSaveEditor", file)
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), file)
	}
	return file
}

func newPresetStore(file string) *presetStore {
	return &presetStore{path: presetPath(file)}
}

// load reads the file once. A missing file is not an error — it just means no
// preset has been saved yet.
func (s *presetStore) load() error {
	if s.loaded {
		return nil
	}
	s.loaded = true

	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("프리셋을 읽지 못했습니다: %w", err)
	}
	if err := json.Unmarshal(b, &s.list); err != nil {
		// A corrupt file must not make the editor unusable. Keep it aside so
		// the user can recover it by hand rather than silently deleting it.
		bad := s.path + ".broken"
		_ = os.Rename(s.path, bad)
		s.list = nil
		return fmt.Errorf("프리셋 파일이 손상되어 %s 로 옮겼습니다: %w", filepath.Base(bad), err)
	}
	return nil
}

// save writes atomically, so an interrupted write cannot leave a half file
// where a good one used to be.
func (s *presetStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// all returns the presets, most recently updated first.
func (s *presetStore) all() ([]Preset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return nil, err
	}
	out := make([]Preset, len(s.list))
	copy(out, s.list)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, nil
}

// put adds or replaces a preset by name.
func (s *presetStore) put(name string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}

	for i := range s.list {
		if strings.EqualFold(s.list[i].Name, name) {
			s.list[i].IDs = ids
			s.list[i].Updated = time.Now()
			return s.save()
		}
	}
	if len(s.list) >= MaxPresets {
		return fmt.Errorf("프리셋은 최대 %d개까지입니다", MaxPresets)
	}
	s.list = append(s.list, Preset{Name: name, IDs: ids, Updated: time.Now()})
	return s.save()
}

// remove deletes a preset by name.
func (s *presetStore) remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	for i := range s.list {
		if strings.EqualFold(s.list[i].Name, name) {
			s.list = append(s.list[:i], s.list[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("그런 이름의 프리셋이 없습니다: %s", name)
}
