package editor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEditedRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		old   string
		new   string
		wantS int
		wantE int
	}{
		{name: "identical", old: "a\nb\nc\n", new: "a\nb\nc\n", wantS: 0, wantE: 0},
		{name: "empty -> content (new file)", old: "", new: "a\nb\n", wantS: 0, wantE: 2},
		{name: "content -> empty (delete file)", old: "a\nb\n", new: "", wantS: 0, wantE: 2},
		{name: "single line replaced", old: "a\nb\nc\n", new: "a\nB\nc\n", wantS: 1, wantE: 2},
		{name: "block replaced", old: "a\nb\nc\nd\n", new: "a\nX\nY\nd\n", wantS: 1, wantE: 3},
		{name: "appended at end", old: "a\nb\n", new: "a\nb\nc\n", wantS: 2, wantE: 3},
		{name: "prepended at start", old: "b\nc\n", new: "a\nb\nc\n", wantS: 0, wantE: 1},
		{
			name:  "pure deletion in middle",
			old:   "a\nb\nc\nd\n",
			new:   "a\nd\n",
			wantS: 1,
			wantE: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s, e := EditedRange(tt.old, tt.new)
			require.Equal(t, tt.wantS, s, "start line")
			require.Equal(t, tt.wantE, e, "end line")
		})
	}
}

func TestLineCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"a\n", 1},
		{"a\nb", 2},
		{"a\nb\n", 2},
		{"\n", 1},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, lineCount(tt.in), "input=%q", tt.in)
	}
}
