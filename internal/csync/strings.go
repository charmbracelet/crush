package csync

import "sync"

// NewString returns a new AtomicString.
func NewString() *String {
	return &String{}
}

// String is a thread-safe string.
type String struct {
	s  string
	mu sync.RWMutex
}

// String implements Stringer.
func (a *String) String() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.s
}

// Store stores the given string atomically.
func (a *String) Store(s string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.s = s
}
