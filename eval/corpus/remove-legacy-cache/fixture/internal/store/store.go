// Package store is the supported in-memory key/value store. It
// exposes the same Get/Set/Delete API the legacy cache package had,
// plus hit/miss counters for observability.
package store

import "sync"

// Store is a string-keyed in-memory store with metrics.
type Store struct {
	mu     sync.Mutex
	items  map[string]string
	hits   int
	misses int
}

// New returns an empty Store.
func New() *Store {
	return &Store{items: make(map[string]string)}
}

// Get returns the value for key, or "" and false when absent.
func (s *Store) Get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.items[key]
	if ok {
		s.hits++
	} else {
		s.misses++
	}
	return v, ok
}

// Set stores value under key.
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = value
}

// Delete removes key. Missing keys are not an error.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
}

// Len reports how many entries are held.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// Stats returns the hit/miss counters.
func (s *Store) Stats() (hits, misses int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits, s.misses
}
