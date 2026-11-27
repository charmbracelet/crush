package dialog

import (
	"image"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func newTestArguments(t *testing.T, description string) *Arguments {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	return NewArguments(com, "Review", description, []commands.Argument{
		{ID: "pr", Title: "pr_number", Description: "Pull request number", Required: true},
		{ID: "focus", Title: "focus"},
	}, nil)
}

// TestArgumentsHelpShowsEsc verifies the help line keeps the esc hint
// on wide and narrow screens instead of truncating it away.
func TestArgumentsHelpShowsEsc(t *testing.T) {
	t.Parallel()
	for _, w := range []int{100, 40} {
		a := newTestArguments(t, "")
		scr := uv.NewScreenBuffer(w, 30)
		a.Draw(scr, image.Rect(0, 0, w, 30))
		require.Containsf(t, scr.String(), "esc", "esc hint missing at %d cols:\n%s", w, scr.String())
	}
}

func drawArguments(a *Arguments, w, h int) []string {
	scr := uv.NewScreenBuffer(w, h)
	a.Draw(scr, image.Rect(0, 0, w, h))
	var rows []string
	for _, l := range strings.Split(scr.String(), "\n") {
		if strings.Contains(l, "│") {
			rows = append(rows, strings.Trim(strings.TrimSpace(l), "│ "))
		}
	}
	return rows
}

// TestArgumentsSpacing verifies a long description wraps in full with a
// gap before the first field, a single blank row sits above the help,
// a field without a description does not repeat its label as the
// placeholder, and the loading text reads cleanly.
func TestArgumentsSpacing(t *testing.T) {
	t.Parallel()
	desc := "Review the given pull request and leave comments on anything that looks off. " +
		"This description is intentionally long so that it needs several lines to wrap."
	for _, w := range []int{100, 40} {
		a := newTestArguments(t, desc)
		rows := drawArguments(a, w, 30)
		text := strings.Join(rows, "\n")
		require.Containsf(t, strings.Join(strings.Fields(text), " "), "several lines to wrap.", "description cut at %d:\n%s", w, text)

		first := -1
		for i, r := range rows {
			if strings.HasPrefix(r, "Pr Number") {
				first = i
			}
		}
		require.Positive(t, first)
		require.Emptyf(t, rows[first-1], "no gap before the first field at %d:\n%s", w, text)
		for i := 1; i < len(rows); i++ {
			require.Falsef(t, rows[i] == "" && rows[i-1] == "", "double blank row at %d:\n%s", w, text)
		}
		require.NotContainsf(t, text, "> focus", "label repeated as placeholder:\n%s", text)
	}

	a := newTestArguments(t, "")
	a.loading = true
	text := strings.Join(drawArguments(a, 100, 30), "\n")
	require.Contains(t, text, "Generating prompt...")
	require.NotContains(t, text, "  Generating")
}

// TestArgumentsLongDescriptionFitsShortScreen verifies an overlong
// description gives way on a short screen so the dialog still fits and
// the first field stays visible.
func TestArgumentsLongDescriptionFitsShortScreen(t *testing.T) {
	t.Parallel()
	a := newTestArguments(t, strings.Repeat("word ", 120))
	text := strings.Join(drawFits(t, a, 40, 16), "\n")
	require.Contains(t, text, "…")
	require.Contains(t, text, "Pr Number")
}

// TestArgumentsDescriptionBreaksLongWords verifies a URL or path longer
// than the dialog is broken across lines instead of overflowing it.
func TestArgumentsDescriptionBreaksLongWords(t *testing.T) {
	t.Parallel()
	d := newTestArguments(t, "See https://example.com/"+strings.Repeat("a", 120)+" for details.")
	drawFits(t, d, 60, 30)
}
