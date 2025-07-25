package csync

import (
	"iter"
	"sync"
)

// LazySlice is a thread-safe lazy-loaded slice.
type LazySlice[K any] struct {
	inner []K
	wg    sync.WaitGroup
}

// NewLazySlice creates a new slice and runs the [load] function in a goroutine
// to populate it.
func NewLazySlice[K any](load func() []K) *LazySlice[K] {
	s := &LazySlice[K]{}
	s.wg.Add(1)
	go func() {
		s.inner = load()
		s.wg.Done()
	}()
	return s
}

// Seq returns an iterator that yields elements from the slice.
func (s *LazySlice[K]) Seq() iter.Seq[K] {
	s.wg.Wait()
	return func(yield func(K) bool) {
		for _, v := range s.inner {
			if !yield(v) {
				return
			}
		}
	}
}
