package chat

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// A wrapped parameter keeps the colour of the token it was broken in the
// middle of.
//
// There are two ways to get this wrong and they look different on screen.
// Wrapping first and styling after emits the parameter colour at the head of
// every line, so the tail of a string literal comes out drab. Styling first
// and wrapping after emits nothing there, so it comes out in the terminal
// default instead. Both are checked here: the head of a continuation line
// has to carry the colour that was open at the break.
func TestToolParamListKeepsHighlightingAcrossWraps(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	// Long enough to break in the middle of the first quoted path.
	cmd := `for p in ("providers/openrouter/language_model_hooks.go","openai"),("x/y.go","z")`
	highlighted, err := common.SyntaxHighlightLexerName(&sty, cmd, "bash", nil)
	require.NoError(t, err)

	const width = 60
	out := toolParamList(&sty, []string{highlighted}, width, &ToolRenderOpts{ExpandedContent: true})

	lines := strings.Split(out, "\n")
	require.Greater(t, len(lines), 1, "the command must actually wrap for this to test anything")

	for i, line := range lines[1:] {
		require.NotEmpty(t, line, "continuation line %d is empty", i+1)
		require.True(t, strings.HasPrefix(line, "\x1b["),
			"continuation line %d starts with no colour at all, so it renders in the "+
				"terminal default instead of the highlight that was open at the break: %q",
			i+1, line)

		// Past the parameter style, the line has to re-open the colour that
		// was running when the wrap fell, rather than starting a fresh token.
		rest := strings.TrimPrefix(line, sgrPrefix(t, sty.Tool.ParamMain.Render("x")))
		require.True(t, strings.HasPrefix(rest, "\x1b["),
			"continuation line %d carries only the parameter colour, so the rest of the "+
				"token it was broken inside comes out drab: %q", i+1, line)
	}
}

// sgrPrefix returns the leading escape sequence of a rendered string.
func sgrPrefix(t *testing.T, rendered string) string {
	t.Helper()
	if !strings.HasPrefix(rendered, "\x1b[") {
		return ""
	}
	end := strings.Index(rendered, "m")
	require.Greater(t, end, 0, "expected a terminated escape sequence")
	return rendered[:end+1]
}

// Truncation is the other branch through the same function, and a truncated
// parameter still has to carry the parameter style.
func TestToolParamListStylesTruncatedParams(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	long := strings.Repeat("abcdefghij", 20)

	out := toolParamList(&sty, []string{long}, 30, nil)
	require.Contains(t, out, "…", "an over-long parameter must be truncated")
	require.NotEqual(t, long, out, "the parameter must be styled, not passed through bare")
}

// A wrapped header has to stay inside the space it was given.
//
// Word wrapping keeps the space it broke on, so a word ending exactly at the
// edge pushes its line one column past the width. A header that overruns the
// pane by a column is wrapped again by the terminal, which splits it across
// two rows and leaves a stray line in the transcript.
func TestToolHeaderStaysWithinWidth(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	opts := &ToolRenderOpts{ExpandedContent: true}
	commands := []string{
		"go test ./internal/ui/chat -run TestToolParamList -v && echo done",
		"git commit -m aaaa bbbb cccc dddd eeee ffff gggg hhhh iiii jjjj",
	}

	for _, cmd := range commands {
		for width := 30; width <= 100; width++ {
			out := toolHeader(&sty, ToolStatusSuccess, "Bash", width, opts, cmd)
			for i, line := range strings.Split(out, "\n") {
				require.LessOrEqual(t, lipgloss.Width(line), width,
					"line %d of %q at width %d overruns the pane: %q", i, cmd, width, line)
			}
		}
	}
}
