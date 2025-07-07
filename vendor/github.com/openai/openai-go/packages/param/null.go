package param

import "github.com/openai/openai-go/internal/encoding/json/sentinel"

// NullMap returns a non-nil map with a length of 0.
// When used with [MarshalObject] or [MarshalUnion], it will be marshaled as null.
//
// It is unspecified behavior to mutate the map returned by [NullMap].
func NullMap[MapT ~map[string]T, T any]() MapT {
	return sentinel.NewNullSentinel(func() MapT { return make(MapT, 1) })
}

// NullSlice returns a non-nil slice with a length of 0.
// When used with [MarshalObject] or [MarshalUnion], it will be marshaled as null.
//
// It is unspecified behavior to mutate the slice returned by [NullSlice].
func NullSlice[SliceT ~[]T, T any]() SliceT {
	return sentinel.NewNullSentinel(func() SliceT { return make(SliceT, 0, 1) })
}
