// This package provides shims over Go 1.2{2,3} APIs
// which are missing from Go 1.21, and used by the Go 1.24 encoding/json package.
//
// Inside the vendored package, all shim code has comments that begin look like
// // SHIM(...): ...
package shims

import (
	"encoding/base64"
	"reflect"
	"slices"
)

type OverflowableType struct{ reflect.Type }

func (t OverflowableType) OverflowInt(x int64) bool {
	k := t.Kind()
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		bitSize := t.Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic("reflect: OverflowInt of non-int type " + t.String())
}

func (t OverflowableType) OverflowUint(x uint64) bool {
	k := t.Kind()
	switch k {
	case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		bitSize := t.Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic("reflect: OverflowUint of non-uint type " + t.String())
}

// Original src code from Go 1.23 go/src/reflect/type.go (taken 1/9/25)
/*

func (t *rtype) OverflowInt(x int64) bool {
	k := t.Kind()
	switch k {
	case Int, Int8, Int16, Int32, Int64:
		bitSize := t.Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic("reflect: OverflowInt of non-int type " + t.String())
}

func (t *rtype) OverflowUint(x uint64) bool {
	k := t.Kind()
	switch k {
	case Uint, Uintptr, Uint8, Uint16, Uint32, Uint64:
		bitSize := t.Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic("reflect: OverflowUint of non-uint type " + t.String())
}

*/

// TypeFor returns the [Type] that represents the type argument T.
func TypeFor[T any]() reflect.Type {
	var v T
	if t := reflect.TypeOf(v); t != nil {
		return t // optimize for T being a non-interface kind
	}
	return reflect.TypeOf((*T)(nil)).Elem() // only for an interface kind
}

// Original src code from Go 1.23 go/src/reflect/type.go (taken 1/9/25)
/*

// TypeFor returns the [Type] that represents the type argument T.
func TypeFor[T any]() Type {
	var v T
	if t := TypeOf(v); t != nil {
		return t // optimize for T being a non-interface kind
	}
	return TypeOf((*T)(nil)).Elem() // only for an interface kind
}

*/

type AppendableStdEncoding struct{ *base64.Encoding }

// AppendEncode appends the base64 encoded src to dst
// and returns the extended buffer.
func (enc AppendableStdEncoding) AppendEncode(dst, src []byte) []byte {
	n := enc.EncodedLen(len(src))
	dst = slices.Grow(dst, n)
	enc.Encode(dst[len(dst):][:n], src)
	return dst[:len(dst)+n]
}

// Original src code from Go 1.23.4 go/src/encoding/base64/base64.go (taken 1/9/25)
/*

// AppendEncode appends the base64 encoded src to dst
// and returns the extended buffer.
func (enc *Encoding) AppendEncode(dst, src []byte) []byte {
	n := enc.EncodedLen(len(src))
	dst = slices.Grow(dst, n)
	enc.Encode(dst[len(dst):][:n], src)
	return dst[:len(dst)+n]
}

*/
