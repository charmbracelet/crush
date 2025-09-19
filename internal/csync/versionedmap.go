package csync

import (
	"iter"
	"sync/atomic"
)

// NewVersionedMap creates a new versioned, thread-safe map.
func NewVersionedMap[K comparable, V any]() *VersionedMap[K, V] {
	return &VersionedMap[K, V]{
		m: NewMap[K, V](),
	}
}

// VersionedMap is a thread-safe map that keeps track of its version.
type VersionedMap[K comparable, V any] struct {
	m *Map[K, V]
	v atomic.Uint64
}

// Get gets the value for the specified key from the map.
func (m *VersionedMap[K, V]) Get(key K) (V, bool) {
	return m.m.Get(key)
}

// Set sets the value for the specified key in the map and increments the version.
func (m *VersionedMap[K, V]) Set(key K, value V) {
	m.m.Set(key, value)
	m.v.Add(1)
}

// Del deletes the specified key from the map and increments the version.
func (m *VersionedMap[K, V]) Del(key K) {
	m.m.Del(key)
	m.v.Add(1)
}

// Seq2 returns an iter.Seq2 that yields key-value pairs from the map.
func (m *VersionedMap[K, V]) Seq2() iter.Seq2[K, V] {
	return m.m.Seq2()
}

// Len returns the number of items in the map.
func (m *VersionedMap[K, V]) Len() int {
	return m.m.Len()
}

// Version returns the current version of the map.
func (m *VersionedMap[K, V]) Version() uint64 {
	return m.v.Load()
}
