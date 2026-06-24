package logo

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func textFreeOpts(width int) Opts {
	return Opts{
		FieldColor:  lipgloss.Color("#444444"),
		TitleColorA: lipgloss.Color("#ff0000"),
		TitleColorB: lipgloss.Color("#0000ff"),
		Width:       width,
		TextFree:    true,
	}
}

// onlyDiagonals returns any glyphs left after stripping the diagonal field
// character and layout whitespace. It must be empty for a text-free logo.
func onlyDiagonals(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '╱', '\n', ' ', '\t':
			return -1
		}
		return r
	}, ansi.Strip(s))
}

func TestRenderTextFreeHasNoWordmark(t *testing.T) {
	t.Parallel()

	// The version string would appear in the wordmark meta row; the text-free
	// field must drop it along with the letterforms.
	out := Render(lipgloss.NewStyle(), "v1.2.3", true, textFreeOpts(30))
	require.Contains(t, ansi.Strip(out), "╱")
	require.Empty(t, onlyDiagonals(out),
		"text-free logo should render only the diagonal field")
}

func TestRenderTextFreeMatchesWordmarkHeight(t *testing.T) {
	t.Parallel()

	base := lipgloss.NewStyle()
	mk := func(textFree bool) string {
		o := textFreeOpts(30)
		o.TextFree = textFree
		return Render(base, "v1.2.3", true, o)
	}
	require.Equal(t, lipgloss.Height(mk(false)), lipgloss.Height(mk(true)),
		"text-free logo must match the wordmark height to avoid layout shift")
}

func TestSmallRenderTextFree(t *testing.T) {
	t.Parallel()

	s := styles.ThemeForProvider("")
	out := SmallRender(&s, 20, Opts{TextFree: true})
	require.NotContains(t, ansi.Strip(out), "\n", "small logo is a single line")
	require.Contains(t, ansi.Strip(out), "╱")
	require.Empty(t, onlyDiagonals(out),
		"text-free small logo should render only the diagonal field")
}
