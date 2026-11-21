package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateDiffCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		before    string
		after     string
		additions int
		removals  int
	}{
		{
			name:      "plain lines",
			before:    "a\nb\nc\n",
			after:     "a\nB\nc\n",
			additions: 1,
			removals:  1,
		},
		{
			name:      "removed sql comment",
			before:    "-- drop the index\nSELECT 1;\n",
			after:     "SELECT 1;\n",
			additions: 0,
			removals:  1,
		},
		{
			name:      "removed yaml front matter",
			before:    "---\ntitle: x\n---\nbody\n",
			after:     "body\n",
			additions: 0,
			removals:  3,
		},
		{
			name:      "added pre-increment",
			before:    "int i = 0;\n",
			after:     "int i = 0;\n++i;\n",
			additions: 1,
			removals:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, additions, removals := GenerateDiff(tt.before, tt.after, "/file.txt")
			require.Equal(t, tt.additions, additions, "additions")
			require.Equal(t, tt.removals, removals, "removals")
		})
	}
}
