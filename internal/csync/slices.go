package csync

import (
	"iter"
	"slices"
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
	s.wg.Go(func() {
		s.inner = load()
	})
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

// Slice is a thread-safe slice implementation that provides concurrent access.
type Slice[T any] struct {
	inner []T
	mu    sync.RWMutex
}

// NewSlice creates a new thread-safe slice.
func NewSlice[T any]() *Slice[T] {
	return &Slice[T]{
		inner: make([]T, 0),
	}
}

// NewSliceFrom creates a new thread-safe slice from an existing slice.
func NewSliceFrom[T any](s []T) *Slice[T] {
	inner := make([]T, len(s))
	copy(inner, s)
	return &Slice[T]{
		inner: inner,
	}
}

// Append adds an element to the end of the slice.
func (s *Slice[T]) Append(items ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inner = append(s.inner, items...)
}

// Prepend adds an element to the beginning of the slice.
func (s *Slice[T]) Prepend(item T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inner = append([]T{item}, s.inner...)
}

// Delete removes the element at the specified index.
func (s *Slice[T]) Delete(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.inner) {
		return false
	}
	s.inner = slices.Delete(s.inner, index, index+1)
	return true
}

// Get returns the element at the specified index.
func (s *Slice[T]) Get(index int) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var zero T
	if index < 0 || index >= len(s.inner) {
		return zero, false
	}
	return s.inner[index], true
}

// Set updates the element at the specified index.
func (s *Slice[T]) Set(index int, item T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.inner) {
		return false
	}
	s.inner[index] = item
	return true
}

// Len returns the number of elements in the slice.
func (s *Slice[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.inner)
}

// SetSlice replaces the entire slice with a new one.
func (s *Slice[T]) SetSlice(items []T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inner = make([]T, len(items))
	copy(s.inner, items)
}

// Seq returns an iterator that yields elements from the slice.
func (s *Slice[T]) Seq() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range s.Seq2() {
			if !yield(v) {
				return
			}
		}
	}
}

// Seq2 returns an iterator that yields index-value pairs from the slice.
func (s *Slice[T]) Seq2() iter.Seq2[int, T] {
	s.mu.RLock()
	items := make([]T, len(s.inner))
	copy(items, s.inner)
	s.mu.RUnlock()
	return func(yield func(int, T) bool) {
		for i, v := range items {
			if !yield(i, v) {
				return
			}
		}
	}
}
