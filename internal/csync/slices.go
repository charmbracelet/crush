package csync

import (
	"iter"
	"sync"
)

// LazySlice is a thread-safe lazy-loaded slice.
type LazySlice[K any] struct {
	inner []K
	mu    sync.Mutex
}

// NewLazySlice creates a new slice and runs the [load] function in a goroutine
// to populate it.
func NewLazySlice[K any](load func() []K) *LazySlice[K] {
	s := &LazySlice[K]{}
	s.mu.Lock()
	go func() {
		s.inner = load()
		s.mu.Unlock()
	}()
	return s
}

// Seq returns an iterator that yields elements from the slice.
func (s *LazySlice[K]) Seq() iter.Seq[K] {
	s.mu.Lock()
	inner := s.inner
	s.mu.Unlock()
	return func(yield func(K) bool) {
		for _, v := range inner {
			if !yield(v) {
				return
			}
		}
	}
}
