package list

import (
	"strings"
	"testing"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// renderAndCopy renders markdown the same way the chat view does and
// runs a full-selection copy over it, the path a mouse selection takes.
func renderAndCopy(t *testing.T, sty *styles.Styles, md string, width int) string {
	t.Helper()
	r, err := glamour.NewTermRenderer(glamour.WithStyles(sty.Markdown), glamour.WithWordWrap(width))
	require.NoError(t, err)
	rendered, err := r.Render(md)
	require.NoError(t, err)
	return HighlightContent(rendered, uv.Rect(0, 0, width, lipgloss.Height(rendered)), 0, 0, -1, -1)
}

func TestRawCopyEmphasis(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	require.Equal(t, "Some *emph* text.\n", renderAndCopy(t, &sty, "Some *emph* text.", 80))
	require.Equal(t, "Some **strong** text.\n", renderAndCopy(t, &sty, "Some **strong** text.", 80))
	require.Equal(t, "Some ~~struck~~ text.\n", renderAndCopy(t, &sty, "Some ~~struck~~ text.", 80))
	require.Equal(
		t,
		"A *bit* of **everything** and ~~nothing~~.\n",
		renderAndCopy(t, &sty, "A *bit* of **everything** and ~~nothing~~.", 80),
	)
}

func TestRawCopyNestedEmphasis(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	// Nested markers compose; the raw form normalizes to canonical
	// asterisk syntax. The sentinel cell of a nested emphasis carries
	// both bold and italic attributes, so *** is restored exactly.
	require.Equal(t, "before ***both*** after\n", renderAndCopy(t, &sty, "before ***both*** after", 80))
	require.Equal(t, "**bold *ital* bold**\n", renderAndCopy(t, &sty, "**bold *ital* bold**", 80))
}

func TestRawCopyCodeSpan(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	require.Equal(t, "use `crush copy` now\n", renderAndCopy(t, &sty, "use `crush copy` now", 80))
}

func TestRawCopyLink(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	require.Equal(
		t,
		"See [the docs](https://example.com/x) for more.\n",
		renderAndCopy(t, &sty, "See [the docs](https://example.com/x) for more.", 80),
	)
}

func TestRawCopyLinkSelectionCoversURLOnly(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	r, err := glamour.NewTermRenderer(glamour.WithStyles(sty.Markdown), glamour.WithWordWrap(80))
	require.NoError(t, err)
	rendered, err := r.Render("See [the docs](https://example.com/x) for more.")
	require.NoError(t, err)

	// The link text renders as "the docs" followed by the URL. Selecting
	// from the URL onwards must still produce the full raw link.
	urlCol, urlRow := linkURLStart(rendered)
	require.GreaterOrEqual(t, urlCol, 0, "expected the rendered URL cells in the rendered link")
	copied := HighlightContent(rendered, uv.Rect(0, 0, 80, lipgloss.Height(rendered)), urlRow, urlCol, urlRow, -1)
	// The selection runs from the URL to the end of the row, so the
	// trailing text after the link is included.
	require.Equal(t, "https://example.com/x for more.\n", copied)
}

// linkURLStart finds the first cell of the rendered URL portion of a
// link: the start of the second tagged run in the row (the first is the
// link text, separated from the URL by the untagged blank cell glamour
// writes between them).
func linkURLStart(rendered string) (int, int) {
	buf := renderBuffer(stringext.NormalizeSpace(rendered), uv.Rect(0, 0, 80, lipgloss.Height(rendered)), 80, lipgloss.Height(rendered))
	for y := 0; y < buf.Height(); y++ {
		line := buf.Line(y)
		var runs []int
		inRun := false
		for x := range 80 {
			c := line.At(x)
			tagged := c != nil && c.Link.URL != ""
			switch {
			case tagged && !inRun:
				runs = append(runs, x)
				inRun = true
			case !tagged:
				inRun = false
			}
		}
		if len(runs) >= 2 {
			return runs[1], y
		}
	}
	return -1, -1
}

func TestRawCopyImage(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	require.Equal(
		t,
		"Logo: ![logo](https://example.com/logo.png)\n",
		renderAndCopy(t, &sty, "Logo: ![logo](https://example.com/logo.png)", 80),
	)
}

func TestRawCopyList(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	require.Equal(t, "- one\n- two\n", renderAndCopy(t, &sty, "- one\n- two", 80))
	require.Equal(t, "1. one\n2. two\n", renderAndCopy(t, &sty, "1. one\n2. two", 80))
}

func TestRawCopyTasks(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	require.Equal(
		t,
		"- [x] done task\n- [ ] open task\n",
		renderAndCopy(t, &sty, "- [x] done task\n- [ ] open task", 80),
	)
}

func TestRawCopyPlainTextChecklistNotMistakenForTask(t *testing.T) {
	t.Parallel()

	// A plain-text line that happens to start with "[ ] " is not a
	// rendered task (no sentinel) and must copy verbatim.
	require.Equal(t, "[ ] fix bug\n", HighlightContent("[ ] fix bug", uv.Rect(0, 0, 40, 1), 0, 0, -1, -1))
}

func TestRawCopyBlockquote(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	require.Equal(t, "> quoted line\n", renderAndCopy(t, &sty, "> quoted line", 80))
}

func TestRawCopyCodeFence(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	copied := renderAndCopy(t, &sty, "```go\nfunc main() {}\n```\n", 80)
	require.Contains(t, copied, "```\nfunc main() {}\n```", "code must be wrapped in restored fences, got:\n%s", copied)
	require.NotContains(t, copied, "go\n", "the fence language is not recoverable and must not appear")
}

func TestRawCopyHorizontalRule(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	copied := renderAndCopy(t, &sty, "above\n\n---\n\nbelow\n", 80)
	require.Contains(t, copied, "\n---\n", "the rule must copy as raw dashes, got:\n%s", copied)
}

func TestRawCopyDashRowInsideCodeBlockNotARule(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	copied := renderAndCopy(t, &sty, "```\n--------\n```\n", 80)
	require.Contains(t, copied, "```\n--------\n```", "a dash run inside code must copy verbatim, got:\n%s", copied)
	require.NotContains(t, copied, "\n---\n")
}

func TestRawCopyTable(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	copied := renderAndCopy(t, &sty, "| a | b |\n|---|---|\n| 1 | 2 |\n", 80)
	require.Contains(t, copied, "| a | b |\n", "header row must rebuild as pipe syntax, got:\n%s", copied)
	require.Contains(t, copied, "|---|---|\n", "separator row must be synthesized, got:\n%s", copied)
	require.Contains(t, copied, "| 1 | 2 |", "data row must rebuild as pipe syntax, got:\n%s", copied)
}

func TestRawCopyPipeInTextNotATable(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	// A lone line containing the │ box rune with no neighboring table
	// structure must not be rebuilt as a table.
	copied := renderAndCopy(t, &sty, "the │ rune is not a table\n", 80)
	require.Contains(t, copied, "the │ rune is not a table", got(copied))
	require.NotContains(t, copied, "| the")
}

func TestRawCopyTableSelectionStartsMidTable(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	r, err := glamour.NewTermRenderer(glamour.WithStyles(sty.Markdown), glamour.WithWordWrap(80))
	require.NoError(t, err)
	rendered, err := r.Render("| a | b |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |\n")
	require.NoError(t, err)

	// Select from the first data row only: the header and border rows
	// are outside the selection, but the data rows chain together and
	// must still rebuild as pipe syntax.
	tableRow := -1
	for y, line := range strings.Split(ansi.Strip(rendered), "\n") {
		if strings.Contains(line, " 1 ") {
			tableRow = y
			break
		}
	}
	require.GreaterOrEqual(t, tableRow, 0, "expected a data row containing 1, got:\n%s", ansi.Strip(rendered))

	copied := HighlightContent(rendered, uv.Rect(0, 0, 80, lipgloss.Height(rendered)), tableRow, 0, -1, -1)
	require.Contains(t, copied, "| 1 | 2 |", "mid-table selection must rebuild pipe rows, got:\n%s", copied)
	require.Contains(t, copied, "| 3 | 4 |", got(copied))
}

func TestRawCopyTableManyColumns(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	copied := renderAndCopy(t, &sty, "| a | b | c |\n|---|---|---|\n| 1 | 2 | 3 |\n", 80)
	require.Contains(t, copied, "| a | b | c |\n", got(copied))
	require.Contains(t, copied, "|---|---|---|\n", got(copied))
	require.Contains(t, copied, "| 1 | 2 | 3 |", got(copied))
}

func TestRawCopyTableWrappedCell(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	// A cell too wide for its column wraps onto continuation rows;
	// the continuation text is appended to the cell it belongs to so
	// the copy keeps every word.
	copied := renderAndCopy(t, &sty, "| key | value |\n|---|---|\n| short | a fairly long value that will wrap inside its narrow column |\n", 60)
	require.Contains(t, copied, "| key | value |", got(copied))
	require.Contains(t, copied, "a fairly long value that will wrap", "wrapped cell text must survive the copy, got:\n%s", copied)
	require.Contains(t, copied, "inside its narrow column", got(copied))
}

func TestRawCopyTableQuietStyle(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	r, err := glamour.NewTermRenderer(glamour.WithStyles(sty.QuietMarkdown), glamour.WithWordWrap(80))
	require.NoError(t, err)
	rendered, err := r.Render("| a | b |\n|---|---|\n| 1 | 2 |\n")
	require.NoError(t, err)
	copied := HighlightContent(rendered, uv.Rect(0, 0, 80, lipgloss.Height(rendered)), 0, 0, -1, -1)
	require.Contains(t, copied, "| a | b |\n", got(copied))
	require.Contains(t, copied, "| 1 | 2 |", got(copied))
}

func TestRawCopyFullDocument(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	md := "# Title\n\nIntro with *emph* and `code`.\n\n- bullet one\n- [x] task\n\n> note\n\n```go\nrun()\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"
	copied := renderAndCopy(t, &sty, md, 80)
	for _, want := range []string{
		"Title",
		"Intro with *emph* and `code`.",
		"- bullet one",
		"- [x] task",
		"> note",
		"```\nrun()\n```",
		"| a | b |",
		"|---|---|",
		"| 1 | 2 |",
	} {
		require.Contains(t, copied, want, "full-document copy must contain %q, got:\n%s", want, copied)
	}
}

func TestRawCopyQuietStyle(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	r, err := glamour.NewTermRenderer(glamour.WithStyles(sty.QuietMarkdown), glamour.WithWordWrap(80))
	require.NoError(t, err)
	rendered, err := r.Render("Some *emph* and **strong** and ~~struck~~ text.")
	require.NoError(t, err)
	copied := HighlightContent(rendered, uv.Rect(0, 0, 80, lipgloss.Height(rendered)), 0, 0, -1, -1)
	require.Equal(t, "Some *emph* and **strong** and ~~struck~~ text.\n", copied)
}

func TestRawCopyWrappedEmphasis(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	width := 20
	r, err := glamour.NewTermRenderer(glamour.WithStyles(sty.Markdown), glamour.WithWordWrap(width))
	require.NoError(t, err)
	rendered, err := r.Render("some words *emphasized term* and more words to force wrapping")
	require.NoError(t, err)
	copied := HighlightContent(rendered, uv.Rect(0, 0, width, lipgloss.Height(rendered)), 0, 0, -1, -1)
	require.Contains(t, copied, "*emphasized term*", "wrapped emphasis must reassemble in copy, got:\n%s", copied)
}

func TestRawCopyRealTextPreserved(t *testing.T) {
	t.Parallel()

	// Plain (unstyled) content must pass through untouched, including
	// characters that resemble sentinels' bases.
	result := HighlightContent("a ⠀ b ﾠ c", uv.Rect(0, 0, 40, 1), 0, 0, -1, -1)
	require.Equal(t, "a ⠀ b ﾠ c\n", result)
}

func got(copied string) string { return "got:\n" + copied }
