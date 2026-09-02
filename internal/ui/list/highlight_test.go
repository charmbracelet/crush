package list

import (
	"strings"
	"testing"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestHighlightContentWrappedLines(t *testing.T) {
	t.Parallel()

	// A long line that will wrap at width 20.
	content := "This is a long line that should wrap around"
	width := 20

	// When selecting the entire content, wrapped portions should be joined
	// with spaces, not newlines.
	result := HighlightContent(content, uv.Rect(0, 0, width, lipgloss.Height(content)), 0, 0, -1, -1)

	// The result should contain only one trailing newline, no internal ones
	// from wrapping.
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	require.Len(t, lines, 1, "wrapped content should be a single logical line")
}

func TestHighlightContentRealNewlinesPreserved(t *testing.T) {
	t.Parallel()

	// Short lines that don't wrap should preserve real newlines.
	content := "first\nsecond"
	width := 40

	result := HighlightContent(content, uv.Rect(0, 0, width, lipgloss.Height(content)), 0, 0, -1, -1)

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	require.Len(t, lines, 2, "real newlines should be preserved")
	require.Contains(t, lines[0], "first")
	require.Contains(t, lines[1], "second")
}

func TestHighlightContentParagraphBreak(t *testing.T) {
	t.Parallel()

	content := "first paragraph\n\nsecond paragraph"
	width := 40

	result := HighlightContent(content, uv.Rect(0, 0, width, lipgloss.Height(content)), 0, 0, -1, -1)

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	require.GreaterOrEqual(t, len(lines), 3, "paragraph break should produce empty line")
}

func TestHighlightContentHardWrap(t *testing.T) {
	t.Parallel()

	// A word longer than the width is cut mid-word by the screen buffer;
	// the pieces must be joined without inserting a space.
	content := strings.Repeat("a", 79) + "b"
	width := 80

	result := HighlightContent(content, uv.Rect(0, 0, width, lipgloss.Height(content)), 0, 0, -1, -1)

	require.Equal(t, strings.Repeat("a", 79)+"b\n", result)
}

func TestHighlightContentMarkdownList(t *testing.T) {
	t.Parallel()

	// Render a markdown list through glamour so the output matches what
	// the chat view actually produces: a long item word-wrapped onto a
	// continuation row, followed by another item.
	md := "- If the current row's content extends past sixty percent of the buffer width emit a space (space, wrap continuation)\n- Otherwise emit a newline (real newline, short lines like headings, list items, code)"
	sty := styles.CharmtonePantera()
	width := 100
	r, err := glamour.NewTermRenderer(glamour.WithStyles(sty.Markdown), glamour.WithWordWrap(width))
	require.NoError(t, err)
	content, err := r.Render(md)
	require.NoError(t, err)

	result := HighlightContent(content, uv.Rect(0, 0, width, lipgloss.Height(content)), 0, 0, -1, -1)

	// The wrapped item must join with its continuation, and the next item
	// must start on its own line.
	require.Contains(t, result, "(space, wrap continuation)\n", "wrapped continuation must join with a space, got:\n%s", result)
	require.Contains(t, result, "continuation)\n• Otherwise", "next list item must start on its own line, got:\n%s", result)
}
