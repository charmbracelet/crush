package csync

import (
	"reflect"
	"sync"
)

// Value is a generic thread-safe wrapper for any value type.
//
// For slices, use [Slice]. For maps, use [Map]. Pointers are not supported.
type Value[T any] struct {
	v  T
	mu sync.RWMutex
}

// NewValue creates a new Value with the given initial value.
//
// Panics if t is a pointer, slice, or map. Use the dedicated types for those.
func NewValue[T any](t T) *Value[T] {
	v := reflect.ValueOf(t)
	switch v.Kind() {
	case reflect.Pointer:
		panic("csync.Value does not support pointer types")
	case reflect.Slice:
		panic("csync.Value does not support slice types; use csync.Slice")
	case reflect.Map:
		panic("csync.Value does not support map types; use csync.Map")
	}
	return &Value[T]{v: t}
}

// Get returns the current value.
func (v *Value[T]) Get() T {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.v
}

// Set updates the value.
func (v *Value[T]) Set(t T) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.v = t
}
