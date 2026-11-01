package automation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store persists automation state per session in a directory.
type Store struct {
	dir string
}

// NewStore creates a store rooted at dir.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Dir returns the directory backing the store.
func (s *Store) Dir() string {
	return s.dir
}

func (s *Store) path(sessionID string) string {
	name := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', 0:
			return '-'
		default:
			return r
		}
	}, sessionID)
	if name == "" || name == "." || name == ".." {
		name = "unknown"
	}
	return filepath.Join(s.dir, name+".json")
}

// Load reads the state for a session. A missing file is an empty state.
func (s *Store) Load(sessionID string) (State, error) {
	data, err := os.ReadFile(s.path(sessionID))
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decoding automation state: %w", err)
	}
	return state, nil
}

// Save writes the state for a session atomically.
func (s *Store) Save(sessionID string, state State) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".automation-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path(sessionID))
}

// Delete removes the state for a session.
func (s *Store) Delete(sessionID string) error {
	err := os.Remove(s.path(sessionID))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
