package util

type Pointer[T any] struct{ Value T }

func (p Pointer[T]) unwrap() any { return p.Value }

type PointerUnwrap interface{ unwrap() any }

func UnwrapPointer(p PointerUnwrap) any {
	return p.unwrap()
}
