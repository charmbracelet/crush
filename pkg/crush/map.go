package crush

import "github.com/charmbracelet/crush/internal/csync"

// Map is a concurrent map implementation that provides thread-safe access.
type Map[K comparable, V any] = csync.Map[K, V]

// NewMap creates a new thread-safe map with the specified key and value types.
func NewMap[K comparable, V any]() *Map[K, V] {
	return csync.NewMap[K, V]()
}

// NewMapFrom creates a new thread-safe map from an existing map.
func NewMapFrom[K comparable, V any](m map[K]V) *Map[K, V] {
	return csync.NewMapFrom(m)
}
