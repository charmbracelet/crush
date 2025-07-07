// Package slice provides utility functions for working with slices in Go.
package slice

import (
	"slices"
)

// GroupBy groups a slice of items by a key function.
func GroupBy[T any, K comparable](list []T, key func(T) K) map[K][]T {
	groups := make(map[K][]T)

	for _, item := range list {
		k := key(item)
		groups[k] = append(groups[k], item)
	}

	return groups
}

// Take returns the first n elements of the given slice. If there are not
// enough elements in the slice, the whole slice is returned.
func Take[A any](slice []A, n int) []A {
	if n > len(slice) {
		return slice
	}
	return slice[:n]
}

// Last returns the last element of a slice and true. If the slice is empty, it
// returns the zero value and false.
func Last[T any](list []T) (T, bool) {
	if len(list) == 0 {
		var zero T
		return zero, false
	}
	return list[len(list)-1], true
}

// Uniq returns a new slice with all duplicates removed.
func Uniq[T comparable](list []T) []T {
	seen := make(map[T]struct{}, len(list))
	uniqList := make([]T, 0, len(list))

	for _, item := range list {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			uniqList = append(uniqList, item)
		}
	}

	return uniqList
}

// Intersperse puts an item between each element of a slice, returning a new
// slice.
func Intersperse[T any](slice []T, insert T) []T {
	if len(slice) <= 1 {
		return slice
	}

	// Create a new slice with the required capacity.
	result := make([]T, len(slice)*2-1)

	for i := range slice {
		// Fill the new slice with original elements and the insertion string.
		result[i*2] = slice[i]

		// Add the insertion string between items (except the last one).
		if i < len(slice)-1 {
			result[i*2+1] = insert
		}
	}

	return result
}

// ContainsAny checks if any of the given values present in the list.
func ContainsAny[T comparable](list []T, values ...T) bool {
	return slices.ContainsFunc(list, func(v T) bool {
		return slices.Contains(values, v)
	})
}

// Shift removes and returns the first element of a slice.
// It returns the removed element and the modified slice.
// The third return value (ok) indicates whether an element was removed.
func Shift[T any](slice []T) (element T, newSlice []T, ok bool) {
	if len(slice) == 0 {
		var zero T
		return zero, slice, false
	}
	return slice[0], slice[1:], true
}

// Pop removes and returns the last element of a slice.
// It returns the removed element and the modified slice.
// The third return value (ok) indicates whether an element was removed.
func Pop[T any](slice []T) (element T, newSlice []T, ok bool) {
	if len(slice) == 0 {
		var zero T
		return zero, slice, false
	}
	lastIdx := len(slice) - 1
	return slice[lastIdx], slice[:lastIdx], true
}

// DeleteAt removes and returns the element at the specified index.
// It returns the removed element and the modified slice.
// The third return value (ok) indicates whether an element was removed.
func DeleteAt[T any](slice []T, index int) (element T, newSlice []T, ok bool) {
	if index < 0 || index >= len(slice) {
		var zero T
		return zero, slice, false
	}

	element = slice[index]
	newSlice = slices.Delete(slices.Clone(slice), index, index+1)

	return element, newSlice, true
}
