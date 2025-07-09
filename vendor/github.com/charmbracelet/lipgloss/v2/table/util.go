package table

import (
	"sort"
)

// btoi converts a boolean to an integer, 1 if true, 0 if false.
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// bton converts a boolean to a specific integer, n if true, 0 if false.
func bton(b bool, n int) int {
	if b {
		return n
	}
	return 0
}

// sum returns the sum of all integers in a slice.
func sum(n []int) int {
	var sum int
	for _, i := range n {
		sum += i
	}
	return sum
}

// median returns the median of a slice of integers.
func median(n []int) int {
	sort.Ints(n)

	if len(n) <= 0 {
		return 0
	}
	if len(n)%2 == 0 {
		h := len(n) / 2            //nolint:mnd
		return (n[h-1] + n[h]) / 2 //nolint:mnd
	}
	return n[len(n)/2]
}
