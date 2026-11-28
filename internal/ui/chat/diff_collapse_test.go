package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// renderToolDiffFull is the collapse renderToolDiff replaced: render the
// whole diff, then keep the rows a collapsed view shows.
func renderToolDiffFull(sty *styles.Styles, file, before, after string, bodyWidth int, split, expanded bool) (string, int) {
	formatter := common.DiffFormatter(sty).Before(file, before).After(file, after).Width(bodyWidth)
	if split {
		formatter = formatter.Split()
	}
	formatted := formatter.String()
	lines := strings.Split(formatted, "\n")
	maxLines := collapsedMaxLines(len(lines))
	if expanded || len(lines) <= maxLines {
		return formatted, 0
	}
	return strings.Join(lines[:maxLines], "\n"), len(lines) - maxLines
}

// A collapsed diff stops rendering past the visible rows, yet shows the
// same rows and hidden-row count as rendering the whole diff.
func TestRenderToolDiffMatchesFullRender(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()

	numbered := func(n int, tag string) string {
		var b strings.Builder
		for i := range n {
			fmt.Fprintf(&b, "func f%d() { return %q } // %s\n", i, tag, strings.Repeat("x", i%30))
		}
		return b.String()
	}
	type pair struct{ before, after string }
	pairs := []pair{
		{"", ""},
		{"a\n", "b\n"},
		{"", numbered(5, "new")},
		{"", numbered(9, "new")},
		{"", numbered(10, "new")},
		{"", numbered(11, "new")},
		{"", numbered(200, "new")},
		{numbered(200, "old"), numbered(200, "new")},
	}
	// Many small hunks, so hunk headers land around the cut.
	var spread strings.Builder
	var spreadAfter strings.Builder
	for i := range 120 {
		line := fmt.Sprintf("line %d\n", i)
		spread.WriteString(line)
		if i%9 == 0 {
			line = fmt.Sprintf("changed %d\n", i)
		}
		spreadAfter.WriteString(line)
	}
	pairs = append(pairs, pair{spread.String(), spreadAfter.String()})

	for _, p := range pairs {
		for _, width := range []int{1, 5, 10, 20, 30, 40, 60, 80, 118, 140, 200} {
			for _, split := range []bool{false, true} {
				for _, expanded := range []bool{false, true} {
					wantOut, wantHidden := renderToolDiffFull(&sty, "main.go", p.before, p.after, width, split, expanded)
					formatter := common.DiffFormatter(&sty).Before("main.go", p.before).After("main.go", p.after).Width(width)
					if split {
						formatter = formatter.Split()
					}
					gotOut, gotHidden := renderToolDiff(formatter, expanded)
					require.Equal(t, wantHidden, gotHidden, "width %d split %v expanded %v", width, split, expanded)
					require.Equal(t, wantOut, gotOut, "width %d split %v expanded %v", width, split, expanded)
				}
			}
		}
	}
}
