package textarea

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// tokenHighlighter is a test LineHighlighter that marks every "@"-prefixed
// word with a fixed style.
type tokenHighlighter struct {
	style lipgloss.Style
}

func (h tokenHighlighter) Highlight(_ int, line []rune) []lipgloss.Range {
	var ranges []lipgloss.Range
	i := 0
	for i < len(line) {
		if line[i] == '@' {
			end := i + 1
			for end < len(line) && line[end] != ' ' {
				end++
			}
			ranges = append(ranges, lipgloss.NewRange(i, end, h.style))
			i = end
			continue
		}
		i++
	}
	return ranges
}

func TestStyleSegmentAppliesRanges(t *testing.T) {
	t.Parallel()

	base := lipgloss.NewStyle()
	tokenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	seg := []rune("see @foo.go here")
	ranges := []lipgloss.Range{lipgloss.NewRange(4, 11, tokenStyle)}

	got := styleSegment(base, seg, 0, ranges)

	// The rendered segment must differ from the plain render and the token
	// text must survive styling.
	require.NotEqual(t, base.Render(string(seg)), got)
	require.Contains(t, ansi.Strip(got), "see @foo.go here")
}

// TestStyleSegmentStylesExactlyTheToken pins which cells the token style
// lands on. Regression: the rune-to-cell conversion used to add one, so
// every token rendered shifted a cell to the right — the trigger character
// stayed unstyled and the following space picked the style up instead.
func TestStyleSegmentStylesExactlyTheToken(t *testing.T) {
	t.Parallel()

	base := lipgloss.NewStyle()
	tokenStyle := lipgloss.NewStyle().Bold(true)

	seg := []rune("ab @foo cd")
	got := styleSegment(base, seg, 0, []lipgloss.Range{lipgloss.NewRange(3, 7, tokenStyle)})

	require.Equal(t, "ab \x1b[1m@foo\x1b[m cd", got)
}

// TestStyleSegmentStylesExactlyTheTokenWideRunes is the same pin with
// double-width runes ahead of the token, where a rune offset and a cell
// offset diverge.
func TestStyleSegmentStylesExactlyTheTokenWideRunes(t *testing.T) {
	t.Parallel()

	base := lipgloss.NewStyle()
	tokenStyle := lipgloss.NewStyle().Bold(true)

	// "世界" is two runes but four cells.
	seg := []rune("世界 @foo")
	got := styleSegment(base, seg, 0, []lipgloss.Range{lipgloss.NewRange(3, 7, tokenStyle)})

	require.Equal(t, "世界 \x1b[1m@foo\x1b[m", got)
}

func TestStyleSegmentOffsetsBySegStart(t *testing.T) {
	t.Parallel()
	base := lipgloss.NewStyle()
	tokenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	// Logical line: "see @foo.go here", wrapped segment is "foo.go here"
	// starting at rune 7. The token range (4,11) intersects as (0,4).
	seg := []rune("foo.go here")
	ranges := []lipgloss.Range{lipgloss.NewRange(4, 11, tokenStyle)}

	got := styleSegment(base, seg, 7, ranges)
	require.Contains(t, ansi.Strip(got), "foo.go here")
	require.NotEqual(t, base.Render(string(seg)), got)
}

func TestStyleSegmentNoRangesIsPlainRender(t *testing.T) {
	t.Parallel()

	base := lipgloss.NewStyle()
	seg := []rune("plain text")

	require.Equal(t, base.Render(string(seg)), styleSegment(base, seg, 0, nil))
	require.Equal(t, base.Render(string(seg)), styleSegment(base, seg, 0, []lipgloss.Range{
		lipgloss.NewRange(50, 60, lipgloss.NewStyle().Bold(true)), // outside segment
	}))
}

func TestViewWithHighlighterStylesTokens(t *testing.T) {
	t.Parallel()
	m := New()
	m.SetValue("see @foo.go and @bar.md")
	m.SetWidth(80)
	m.SetHighlighter(tokenHighlighter{style: lipgloss.NewStyle().Foreground(lipgloss.Color("2"))})

	plain := New()
	plain.SetValue("see @foo.go and @bar.md")
	plain.SetWidth(80)

	highlighted := m.View()
	require.NotEqual(t, plain.View(), highlighted, "highlighter should change the rendered view")
	// Content must survive styling: stripping ANSI yields the same text.
	require.Equal(t, ansi.Strip(plain.View()), ansi.Strip(highlighted))
}

func TestViewWithoutHighlighterUnchanged(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetValue("see @foo.go")
	m.SetWidth(80)

	before := m.View()
	m.SetHighlighter(nil)
	require.Equal(t, before, m.View())
}

func TestViewHighlighterOnCursorLine(t *testing.T) {
	t.Parallel()
	m := New()
	m.Focus()
	m.SetValue("go @foo.go now")
	m.SetWidth(80)
	m.SetHighlighter(tokenHighlighter{style: lipgloss.NewStyle().Foreground(lipgloss.Color("2"))})

	// Move cursor into the middle of the token to exercise the
	// pre/post-cursor split paths.
	m.SetCursorColumn(5)

	highlighted := m.View()
	// Cursor-line rendering must not corrupt content.
	require.Contains(t, ansi.Strip(highlighted), "go @foo.go now")
}

func TestViewHighlighterTokenSurvivesWrap(t *testing.T) {
	t.Parallel()
	m := New()
	// Force a wrap inside the second word region: width 10 wraps
	// "ab @longtoken cd" into multiple segments.
	m.SetValue("ab @longtoken cd")
	m.SetWidth(10)
	m.SetHeight(6)
	m.SetHighlighter(tokenHighlighter{style: lipgloss.NewStyle().Foreground(lipgloss.Color("2"))})

	highlighted := m.View()
	stripped := ansi.Strip(highlighted)
	// All token characters still render after wrapping + styling (the wrap
	// splits the token across segments; assert on the wrap chunks).
	for _, part := range []string{"ab", "@lon", "gtok", "en", "cd"} {
		require.Contains(t, stripped, part)
	}
	require.True(t, strings.Contains(highlighted, "\x1b["), "expected ANSI styling in output")
}
