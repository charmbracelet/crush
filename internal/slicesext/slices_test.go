package slicesext

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSubset(t *testing.T) {
	tests := []struct {
		name   string
		a      []string
		b      []string
		expect bool
	}{
		// Basic subset cases
		{
			name:   "empty subset of empty",
			a:      []string{},
			b:      []string{},
			expect: true,
		},
		{
			name:   "empty subset of non-empty",
			a:      []string{},
			b:      []string{"a", "b", "c"},
			expect: true,
		},
		{
			name:   "non-empty not subset of empty",
			a:      []string{"a"},
			b:      []string{},
			expect: false,
		},
		{
			name:   "single element subset",
			a:      []string{"b"},
			b:      []string{"a", "b", "c"},
			expect: true,
		},
		{
			name:   "single element not subset",
			a:      []string{"d"},
			b:      []string{"a", "b", "c"},
			expect: false,
		},
		{
			name:   "multiple elements subset",
			a:      []string{"a", "c"},
			b:      []string{"a", "b", "c", "d"},
			expect: true,
		},
		{
			name:   "multiple elements not subset",
			a:      []string{"a", "e"},
			b:      []string{"a", "b", "c", "d"},
			expect: false,
		},
		{
			name:   "equal sets are subsets",
			a:      []string{"a", "b", "c"},
			b:      []string{"a", "b", "c"},
			expect: true,
		},
		{
			name:   "larger set not subset of smaller",
			a:      []string{"a", "b", "c", "d"},
			b:      []string{"a", "b"},
			expect: false,
		},

		// Order independence
		{
			name:   "subset with different order",
			a:      []string{"c", "a"},
			b:      []string{"b", "a", "d", "c"},
			expect: true,
		},

		// Duplicate handling
		{
			name:   "duplicates in subset",
			a:      []string{"a", "a", "b"},
			b:      []string{"a", "b", "c"},
			expect: true,
		},
		{
			name:   "duplicates in superset",
			a:      []string{"a", "b"},
			b:      []string{"a", "a", "b", "b", "c"},
			expect: true,
		},
		{
			name:   "duplicates in both",
			a:      []string{"a", "a", "b"},
			b:      []string{"a", "a", "b", "b", "c"},
			expect: true,
		},

		// Real-world examples
		{
			name:   "npm flags subset",
			a:      []string{"-g"},
			b:      []string{"-g", "--verbose", "--save-dev"},
			expect: true,
		},
		{
			name:   "npm flags not subset",
			a:      []string{"--global"},
			b:      []string{"-g", "--verbose", "--save-dev"},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSubset(tt.a, tt.b)
			require.Equal(t, tt.expect, result,
				"IsSubset(%v, %v) should be %v", tt.a, tt.b, tt.expect)
		})
	}
}

func TestIsSubsetWithInts(t *testing.T) {
	tests := []struct {
		name   string
		a      []int
		b      []int
		expect bool
	}{
		{
			name:   "int subset",
			a:      []int{1, 3},
			b:      []int{1, 2, 3, 4},
			expect: true,
		},
		{
			name:   "int not subset",
			a:      []int{1, 5},
			b:      []int{1, 2, 3, 4},
			expect: false,
		},
		{
			name:   "empty int subset",
			a:      []int{},
			b:      []int{1, 2, 3},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSubset(tt.a, tt.b)
			require.Equal(t, tt.expect, result,
				"IsSubset(%v, %v) should be %v", tt.a, tt.b, tt.expect)
		})
	}
}
