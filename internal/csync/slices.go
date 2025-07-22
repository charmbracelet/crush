package csync

import (
	"iter"
	"sync"
)

type LazySlice[K any] struct {
	inner []K
	mu    sync.Mutex
}

func NewLazySlice[K any](load func() []K) *LazySlice[K] {
	s := &LazySlice[K]{}
	s.mu.Lock()
	go func() {
		s.inner = load()
		s.mu.Unlock()
	}()
	return s
}

func (s *LazySlice[K]) Iter() iter.Seq[K] {
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
