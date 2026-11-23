// Package tips remembers which one-time hints Crush has already shown.
//
// Each tip is identified by a short name and is shown at most once per
// machine. The record lives in a small JSON file next to the global data
// config so it survives across projects and sessions.
package tips

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
)

// Names of the tips Crush shows.
const (
	// Permissions explains the permission prompt on the first edit.
	Permissions = "permissions"
	// Folding explains folded tool output on the first long result.
	Folding = "folding"
)

// Store records which tips have been shown.
type Store struct {
	mu   sync.Mutex
	path string
	seen map[string]bool
}

// Open loads the store kept at path. A missing or unreadable file is treated
// as an empty record: tips are a convenience and must never block the UI.
func Open(path string) *Store {
	s := &Store{path: path, seen: map[string]bool{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return s
	}
	for _, n := range names {
		s.seen[n] = true
	}
	return s
}

// DefaultPath is where the store lives by default.
func DefaultPath() string {
	return filepath.Join(filepath.Dir(config.GlobalConfigData()), "tips.json")
}

// Seen reports whether the tip has already been shown.
func (s *Store) Seen(name string) bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen[name]
}

// Claim marks the tip as shown and reports whether the caller is the first
// to do so. Callers show the tip only when Claim returns true. A failure to
// persist the record is returned, but the tip is still claimed for this run
// so it is not repeated.
func (s *Store) Claim(name string) (bool, error) {
	if s == nil {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[name] {
		return false, nil
	}
	s.seen[name] = true
	return true, s.saveLocked()
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	data, err := json.Marshal(slices.Sorted(maps.Keys(s.seen)))
	if err != nil {
		return fmt.Errorf("encode tips: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("create tips directory: %w", err)
	}
	if err := config.AtomicWriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("write tips: %w", err)
	}
	return nil
}
