package sentinel

import (
	"github.com/openai/openai-go/internal/encoding/json/shims"
	"reflect"
	"sync"
)

var nullPtrsCache sync.Map // map[reflect.Type]*T

func NullPtr[T any]() *T {
	t := shims.TypeFor[T]()
	ptr, loaded := nullPtrsCache.Load(t) // avoid premature allocation
	if !loaded {
		ptr, _ = nullPtrsCache.LoadOrStore(t, new(T))
	}
	return (ptr.(*T))
}

var nullSlicesCache sync.Map // map[reflect.Type][]T

func NullSlice[T any]() []T {
	t := shims.TypeFor[T]()
	slice, loaded := nullSlicesCache.Load(t) // avoid premature allocation
	if !loaded {
		slice, _ = nullSlicesCache.LoadOrStore(t, []T{})
	}
	return slice.([]T)
}

func IsNullPtr[T any](ptr *T) bool {
	nullptr, ok := nullPtrsCache.Load(shims.TypeFor[T]())
	return ok && ptr == nullptr.(*T)
}

func IsNullSlice[T any](slice []T) bool {
	nullSlice, ok := nullSlicesCache.Load(shims.TypeFor[T]())
	return ok && reflect.ValueOf(slice).Pointer() == reflect.ValueOf(nullSlice).Pointer()
}

// internal only
func IsValueNullPtr(v reflect.Value) bool {
	if v.Kind() != reflect.Ptr {
		return false
	}
	nullptr, ok := nullPtrsCache.Load(v.Type().Elem())
	return ok && v.Pointer() == reflect.ValueOf(nullptr).Pointer()
}

// internal only
func IsValueNullSlice(v reflect.Value) bool {
	if v.Kind() != reflect.Slice {
		return false
	}
	nullSlice, ok := nullSlicesCache.Load(v.Type().Elem())
	return ok && v.Pointer() == reflect.ValueOf(nullSlice).Pointer()
}
