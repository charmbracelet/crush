package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// The edit tool reports the file as it read it (LF) next to the file as it
// wrote it (CRLF) for a file that is entirely CRLF, so the inline diff is
// handed two versions in different line-ending conventions. Both layouts have
// to show the single edited line and leave the rest of the file alone instead
// of marking every line as rewritten.
func TestEditDiffIgnoresLineEndingConvention(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		width int
	}{
		{name: "unified", width: 80},
		{name: "split", width: maxTextWidth + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sty := styles.CharmtonePantera()
			rendered := stripANSI(toolOutputDiffContent(
				&sty,
				"test.txt",
				"alpha\nbeta\ngamma\ndelta\n",
				"alpha\r\nBETA\r\ngamma\r\ndelta\r\n",
				tt.width,
				true,
			))

			require.Equal(t, 1, markedLines(rendered, "beta"), "the old form of the edited line should be marked as changed")
			require.Equal(t, 1, markedLines(rendered, "BETA"), "the new form of the edited line should be marked as changed")
			for _, untouched := range []string{"alpha", "gamma", "delta"} {
				require.Zero(t, markedLines(rendered, untouched), "%q was not edited and should not be marked as changed", untouched)
			}
		})
	}
}

// markedLines counts the rendered diff lines that show content as changed, for
// one piece of content. The hunk header is skipped: it carries a -/+ pair of
// its own and says nothing about the content.
func markedLines(rendered, content string) int {
	marked := 0
	for line := range strings.SplitSeq(rendered, "\n") {
		if strings.Contains(line, "@@") || !strings.Contains(line, content) {
			continue
		}
		if strings.Contains(line, "-") || strings.Contains(line, "+") {
			marked++
		}
	}
	return marked
}
