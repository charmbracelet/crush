// Package automation implements the persistent state behind the /goal and
// /loop commands. It is UI-agnostic: parsing, scheduling, and persistence
// live here so they can be tested without a terminal.
package automation

import "time"

// State is the automation state stored for one session.
type State struct {
	Goal  *Goal  `json:"goal,omitempty"`
	Loops []Task `json:"loops,omitempty"`
}

// FindLoop returns the index of the loop with the given id, or -1.
func (s *State) FindLoop(id string) int {
	for i := range s.Loops {
		if s.Loops[i].ID == id {
			return i
		}
	}
	return -1
}

// RemoveLoop removes the loop with the given id and reports whether it
// existed.
func (s *State) RemoveLoop(id string) bool {
	i := s.FindLoop(id)
	if i < 0 {
		return false
	}
	s.Loops = append(s.Loops[:i], s.Loops[i+1:]...)
	return true
}

// RemoveExpiredLoops removes loops whose lifetime has elapsed and returns
// them so the caller can report the expiry.
func (s *State) RemoveExpiredLoops(now time.Time) []Task {
	var expired []Task
	kept := s.Loops[:0]
	for _, task := range s.Loops {
		if !task.ExpiresAt.IsZero() && !now.Before(task.ExpiresAt) {
			expired = append(expired, task)
			continue
		}
		kept = append(kept, task)
	}
	s.Loops = kept
	return expired
}
