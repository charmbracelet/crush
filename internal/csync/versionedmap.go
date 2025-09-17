package csync

import (
	"sync/atomic"
)

// NewVersionedMap creates a new versioned, thread-safe map.
func NewVersionedMap[K comparable, V any]() *VersionedMap[K, V] {
	return &VersionedMap[K, V]{
		Map: NewMap[K, V](),
	}
}

// VersionedMap is a thread-safe map that keeps track of its version.
type VersionedMap[K comparable, V any] struct {
	*Map[K, V]
	v atomic.Uint64
}

// Set sets the value for the specified key in the map and increments the version.
func (m *VersionedMap[K, V]) Set(key K, value V) {
	m.Map.Set(key, value)
	m.v.Add(1)
}

// Del deletes the specified key from the map and increments the version.
func (m *VersionedMap[K, V]) Del(key K) {
	m.Map.Del(key)
	m.v.Add(1)
}

// Version returns the current version of the map.
func (m *VersionedMap[K, V]) Version() uint64 {
	return m.v.Load()
}
