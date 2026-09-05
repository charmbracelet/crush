package list

import (
	"testing"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// renderMarkdownForCopy renders src through glamour with the given width,
// the same pipeline chat message items use, ready for HighlightContent.
func renderMarkdownForCopy(t *testing.T, src string, width int) string {
	t.Helper()
	sty := styles.CharmtonePantera()
	r, err := glamour.NewTermRenderer(glamour.WithStyles(sty.Markdown), glamour.WithWordWrap(width))
	require.NoError(t, err)
	rendered, err := r.Render(src)
	require.NoError(t, err)
	return rendered
}

// copyMarkdown renders src at the given width and extracts a full-content
// selection copy with markdown marker reconstruction.
func copyMarkdown(t *testing.T, src string, width int) string {
	t.Helper()
	rendered := renderMarkdownForCopy(t, src, width)
	return HighlightContentMarkdown(rendered, uv.Rect(0, 0, width, lipgloss.Height(rendered)), 0, 0, -1, -1)
}

// TestHighlightContentMarkdownRestoresEmphasis is the core copy-side test
// for markdown reconstruction: bold, italic, bold-italic, and
// strikethrough render by dropping their markers and styling the text, so
// a selection copy must put the markers back for the clipboard to hold
// valid markdown.
func TestHighlightContentMarkdownRestoresEmphasis(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"hello **bold** world":   "hello **bold** world\n",
		"hello *italic* world":   "hello *italic* world\n",
		"a ***bi*** b":           "a ***bi*** b\n",
		"a ~~gone~~ b":           "a ~~gone~~ b\n",
		"**Note:** something":    "**Note:** something\n",
		"*foo bar* baz":          "*foo bar* baz\n",
		"> quoted *text* here":   "│ quoted *text* here\n",
		"- item with **bold** x": "• item with **bold** x\n",
	}
	for src, want := range cases {
		require.Equal(t, want, copyMarkdown(t, src, 80), "source: %q", src)
	}
}

// TestHighlightContentLeavesEmphasisRaw pins the non-markdown path: without
// markdown mode the same rendered content copies as displayed text, with no
// reconstructed markers.
func TestHighlightContentLeavesEmphasisRaw(t *testing.T) {
	t.Parallel()

	rendered := renderMarkdownForCopy(t, "hello **bold** world", 80)
	result := HighlightContent(rendered, uv.Rect(0, 0, 80, lipgloss.Height(rendered)), 0, 0, -1, -1)
	require.Equal(t, "hello bold world\n", result)
}

// TestHighlightContentMarkdownWrappedSpan covers a bold span word-wrapped
// across rows: each row closes and reopens its markers, and the wrap join
// keeps the copy a single logical line of valid markdown.
func TestHighlightContentMarkdownWrappedSpan(t *testing.T) {
	t.Parallel()

	// The bold span is longer than the wrap width, so it must split
	// mid-span: each screen row closes and reopens its markers.
	src := "word word **boldspan continues here and keeps going** word word"
	result := copyMarkdown(t, src, 30)
	require.Equal(t, "word word **boldspan continues** **here and keeps going** word word\n", result)
}

// TestHighlightContentMarkdownSkipsHeadings guards the heading exclusion:
// headings render their whole row in bold (H1 additionally paints a
// background), and naive attribute detection would wrap the title in bogus
// ** markers on top of the ## prefix.
func TestHighlightContentMarkdownSkipsHeadings(t *testing.T) {
	t.Parallel()

	require.Equal(t, "## Title Two\n", copyMarkdown(t, "## Title Two", 80))
	require.Equal(t, "Title One\n", copyMarkdown(t, "# Title One", 80))
}

// TestHighlightContentMarkdownSkipsLinks guards the hyperlink exclusion:
// link text renders bold but carries an OSC8 hyperlink, so it must not
// gain ** markers.
func TestHighlightContentMarkdownSkipsLinks(t *testing.T) {
	t.Parallel()

	result := copyMarkdown(t, "see [the docs](https://example.com) now", 80)
	require.Equal(t, "see the docs https://example.com now\n", result)
}

// TestHighlightContentMarkdownSkipsCodeBlocks guards the code fence
// exclusion: syntax highlighting styles some tokens bold or underlined
// (e.g. class names), and wrapping those in markers would corrupt code the
// user copies to run.
func TestHighlightContentMarkdownSkipsCodeBlocks(t *testing.T) {
	t.Parallel()

	src := "before\n\n```python\nclass Foo:\n    pass\n```\n\nafter"
	result := copyMarkdown(t, src, 80)
	require.NotContains(t, result, "**", "code block must not gain markers, got:\n%s", result)
	require.Contains(t, result, "class Foo:")
}

// TestHighlightContentMarkdownNestedCodespan covers emphasis around an
// inline codespan: the codespan pill paints a background, which suspends
// the emphasis run while the sentinel padding still restores the
// backticks.
func TestHighlightContentMarkdownNestedCodespan(t *testing.T) {
	t.Parallel()

	result := copyMarkdown(t, "**bold and `code` mix**", 80)
	require.Equal(t, "**bold and** `code`** mix**\n", result)
}

// TestHighlightContentMarkdownPartialSelection selects only the bold word
// out of a longer line: markers must open and close at the selection
// boundaries so even a mid-span copy stays balanced.
func TestHighlightContentMarkdownPartialSelection(t *testing.T) {
	t.Parallel()

	rendered := renderMarkdownForCopy(t, "hello **bold** world", 80)
	// Columns 6..11 cover "bold".
	result := HighlightContentMarkdown(rendered, uv.Rect(0, 0, 80, lipgloss.Height(rendered)), 0, 6, 0, 11)
	require.Equal(t, "**bold**\n", result)
}

// TestHighlightContentMarkdownTrailingSpace pins the closing-marker
// placement: a space at the end of an emphasis run must land outside the
// closing marker, because "**bold **" is not valid emphasis.
func TestHighlightContentMarkdownTrailingSpace(t *testing.T) {
	t.Parallel()

	content := lipgloss.NewStyle().Bold(true).Render("bold ") + "plain"
	result := HighlightContentMarkdown(content, uv.Rect(0, 0, 40, 1), 0, 0, -1, -1)
	require.Equal(t, "**bold** plain\n", result)
}

// TestHighlightContentMarkdownStyledPlainText documents the contract for
// non-glamour styled text: in markdown mode any bold/italic run is treated
// as emphasis, which is why only items that render markdown may opt in
// (see list.MarkdownCopyable).
func TestHighlightContentMarkdownStyledPlainText(t *testing.T) {
	t.Parallel()

	content := "a " + lipgloss.NewStyle().Italic(true).Render("styled") + " b"
	result := HighlightContentMarkdown(content, uv.Rect(0, 0, 40, 1), 0, 0, -1, -1)
	require.Equal(t, "a *styled* b\n", result)
}
