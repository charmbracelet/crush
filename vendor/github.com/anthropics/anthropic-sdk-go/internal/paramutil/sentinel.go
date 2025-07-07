package paramutil

import (
	"github.com/anthropics/anthropic-sdk-go/internal/encoding/json/sentinel"
)

// NullPtr returns a pointer to the zero value of the type T.
// When used with [MarshalObject] or [MarshalUnion], it will be marshaled as null.
//
// It is unspecified behavior to mutate the value pointed to by the returned pointer.
func NullPtr[T any]() *T {
	return sentinel.NullPtr[T]()
}

// IsNullPtr returns true if the pointer was created by [NullPtr].
func IsNullPtr[T any](ptr *T) bool {
	return sentinel.IsNullPtr(ptr)
}

// NullSlice returns a non-nil slice with a length of 0.
// When used with [MarshalObject] or [MarshalUnion], it will be marshaled as null.
//
// It is undefined behavior to mutate the slice returned by [NullSlice].
func NullSlice[T any]() []T {
	return sentinel.NullSlice[T]()
}

// IsNullSlice returns true if the slice was created by [NullSlice].
func IsNullSlice[T any](slice []T) bool {
	return sentinel.IsNullSlice(slice)
}
