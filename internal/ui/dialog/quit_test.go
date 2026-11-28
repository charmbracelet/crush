package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestQuitKeyboardUnchanged(t *testing.T) {
	t.Parallel()

	q := newTestQuit()

	// Space confirms the default selection: Nope closes the dialog.
	require.IsType(t, ActionClose{}, q.HandleMsg(tea.KeyPressMsg{Code: ' ', Text: " "}))

	// Left/right switches the selection; enter confirms it.
	require.Nil(t, q.HandleMsg(tea.KeyPressMsg{Code: tea.KeyRight}))
	require.False(t, q.selectedNo)
	require.IsType(t, ActionQuit{}, q.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
}

// TestQuitWrapsTextToSmallTerminal verifies that the question, subagent
// warning and hints wrap to a small terminal so the whole dialog, buttons
// included, stays on screen. Previously the lines rendered at their natural
// width (54 columns with a subagent warning), pushing "Nope" off-screen.
func TestQuitWrapsTextToSmallTerminal(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	for _, sz := range [][2]int{{30, 12}, {24, 20}} {
		q := NewQuit(&common.Common{Styles: &sty}, 1)
		lines := drawFits(t, q, sz[0], sz[1])
		text := strings.Join(lines, "\n")
		require.Contains(t, text, "Yep!")
		require.Contains(t, text, "Nope")
		require.NotNil(t, q.compositor, "buttons should be clickable at %dx%d", sz[0], sz[1])
	}
}

// TestQuitNarrowKeepsPaddingAndBalance verifies the quit dialog keeps its
// text off the border on narrow terminals and splits a wrapped warning
// into even lines instead of leaving a word on its own.
func TestQuitNarrowKeepsPaddingAndBalance(t *testing.T) {
	t.Parallel()

	for _, w := range []int{24, 30, 40, 50, 80} {
		lines := drawFits(t, NewQuit(newPickerTestCommon(), 2), w, 16)
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "│") && strings.Trim(l, "│ ") != "" {
				require.True(t, strings.HasPrefix(l, "│ ") && strings.HasSuffix(l, " │"), "text touches the border at %d cols: %q", w, l)
			}
		}
	}
}

// TestBalancedWrap verifies balanced wrapping never costs a line over a
// plain wrap, never runs wider than it, and uses the narrowest width that
// keeps the line count, so lines come out of similar length.
func TestBalancedWrap(t *testing.T) {
	t.Parallel()

	texts := []string{
		quitQuestion,
		quitHintLineOne,
		quitHintLineTwo,
		"2 subagents are still running and will be cut off.",
	}
	for _, s := range texts {
		for width := 8; width <= ansi.StringWidth(s)+2; width++ {
			got := balancedWrap(s, width)
			plain := ansi.Wrap(s, width, "")
			lines := strings.Count(plain, "\n") + 1
			require.Equalf(t, lines, strings.Count(got, "\n")+1, "%q at %d", s, width)
			longest := widest(got)
			require.LessOrEqualf(t, longest, widest(plain), "%q at %d: %q", s, width, got)
			// Without hard-broken words, one column less must cost a line.
			if lines > 1 && longestWord(s) <= width {
				narrower := ansi.Wrap(s, longest-1, "")
				require.Greaterf(t, strings.Count(narrower, "\n")+1, lines,
					"%q at %d: %q is not the narrowest split", s, width, got)
			}
		}
	}
}

// longestWord is the width of the widest space-separated word in s.
func longestWord(s string) int {
	n := 0
	for _, w := range strings.Fields(s) {
		n = max(n, ansi.StringWidth(w))
	}
	return n
}

// widest is the width of the widest line in s.
func widest(s string) int {
	n := 0
	for _, l := range strings.Split(s, "\n") {
		n = max(n, ansi.StringWidth(l))
	}
	return n
}
