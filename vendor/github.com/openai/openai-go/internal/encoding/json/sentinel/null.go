package sentinel

import (
	"github.com/openai/openai-go/internal/encoding/json/shims"
	"reflect"
	"sync"
)

type cacheEntry struct {
	x    any
	ptr  uintptr
	kind reflect.Kind
}

var nullCache sync.Map // map[reflect.Type]cacheEntry

func NewNullSentinel[T any](mk func() T) T {
	t := shims.TypeFor[T]()
	entry, loaded := nullCache.Load(t) // avoid premature allocation
	if !loaded {
		x := mk()
		ptr := reflect.ValueOf(x).Pointer()
		entry, _ = nullCache.LoadOrStore(t, cacheEntry{x, ptr, t.Kind()})
	}
	return entry.(cacheEntry).x.(T)
}

// for internal use only
func IsValueNull(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Map, reflect.Slice:
		null, ok := nullCache.Load(v.Type())
		return ok && v.Pointer() == null.(cacheEntry).ptr
	}
	return false
}

func IsNull[T any](v T) bool {
	t := shims.TypeFor[T]()
	switch t.Kind() {
	case reflect.Map, reflect.Slice:
		null, ok := nullCache.Load(t)
		return ok && reflect.ValueOf(v).Pointer() == null.(cacheEntry).ptr
	}
	return false
}
