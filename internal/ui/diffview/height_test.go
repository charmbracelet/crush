package diffview

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A height-limited diff stops producing rows once it is full, even when a
// hunk header lands on the last row and pushes the row count past the
// height.
func TestRenderStopsWhenHunkHeaderFillsHeight(t *testing.T) {
	t.Parallel()

	// Each hunk is a header, 3 context lines, 3 inserted lines and 3 more
	// context lines: 10 rows in both layouts. The second hunk header is
	// therefore row 10, the last row of an 11-row view.
	var before, after strings.Builder
	for i := range 400 {
		line := fmt.Sprintf("line %d\n", i)
		before.WriteString(line)
		after.WriteString(line)
		if i%20 == 10 {
			for j := range 3 {
				fmt.Fprintf(&after, "added %d.%d\n", i, j)
			}
		}
	}

	const height = 11
	for _, split := range []bool{false, true} {
		dv := New().Before("", before.String()).After("", after.String()).Height(height)
		if split {
			dv = dv.Split()
		}
		out := dv.String()
		require.Equal(t, height, strings.Count(out, "\n")+1, "split %v", split)
		require.Contains(t, strings.Split(out, "\n")[height-1], "@@", "split %v", split)

		var raw string
		if split {
			raw = dv.renderSplit()
		} else {
			raw = dv.renderUnified()
		}
		rows := strings.Count(strings.TrimSuffix(raw, "\n"), "\n") + 1
		require.LessOrEqual(t, rows, height+1, "split %v", split)
	}
}
