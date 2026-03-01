package util

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

func TestRangesOverlap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		r1   protocol.Range
		r2   protocol.Range
		want bool
	}{
		{
			name: "adjacent ranges do not overlap",
			r1: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 5},
			},
			r2: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 5},
				End:   protocol.Position{Line: 0, Character: 10},
			},
			want: false,
		},
		{
			name: "overlapping ranges",
			r1: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 8},
			},
			r2: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 5},
				End:   protocol.Position{Line: 0, Character: 10},
			},
			want: true,
		},
		{
			name: "non-overlapping with gap",
			r1: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 3},
			},
			r2: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 7},
				End:   protocol.Position{Line: 0, Character: 10},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := rangesOverlap(tt.r1, tt.r2)
			require.Equal(t, tt.want, got, "rangesOverlap(r1, r2)")
			// Overlap should be symmetric
			got2 := rangesOverlap(tt.r2, tt.r1)
			require.Equal(t, tt.want, got2, "rangesOverlap(r2, r1) symmetry")
		})
	}
}
