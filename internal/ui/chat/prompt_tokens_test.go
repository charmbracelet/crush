package chat

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// tokenTestStyles returns styles with unmistakable token colours so a test
// can assert which span picked which one up.
func tokenTestStyles() *styles.Styles {
	sty := styles.CharmtonePantera()
	sty.Editor.TokenFile = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	sty.Editor.TokenSkill = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	return &sty
}

// styledSpans returns the substrings of an ANSI string rendered under the
// given SGR foreground parameter.
func styledSpans(s, sgr string) []string {
	var out []string
	for _, part := range strings.Split(s, "\x1b["+sgr+"m")[1:] {
		if idx := strings.Index(part, "\x1b["); idx >= 0 {
			part = part[:idx]
		}
		out = append(out, part)
	}
	return out
}

func withKnownSkills(t *testing.T, names ...string) {
	t.Helper()
	common.SetPromptSkillNames(names)
	t.Cleanup(func() { common.SetPromptSkillNames(nil) })
}

func TestHighlightPromptTokensStylesFileAndSkill(t *testing.T) {
	withKnownSkills(t, "code-review")
	sty := tokenTestStyles()

	got := highlightPromptTokens("please /code-review @main.go now", sty)

	require.Equal(t, "please /code-review @main.go now", ansi.Strip(got))
	require.Equal(t, []string{"@main.go"}, styledSpans(got, "31"))
	require.Equal(t, []string{"/code-review"}, styledSpans(got, "32"))
}

func TestHighlightPromptTokensLeavesProseAlone(t *testing.T) {
	withKnownSkills(t, "commit")
	sty := tokenTestStyles()

	// No word-boundary trigger anywhere: an email-like run, an inline
	// slash, and an unknown slash-word.
	const in = "mail me@foo.com or a/b or /bogus"
	require.Equal(t, in, highlightPromptTokens(in, sty))
}

func TestHighlightPromptTokensPreservesSurroundingAnsi(t *testing.T) {
	withKnownSkills(t)
	sty := tokenTestStyles()

	// Glamour hands us pre-styled text; restyling a token must not eat the
	// styling around it or change the visible characters.
	in := lipgloss.NewStyle().Bold(true).Render("see @main.go here")
	got := highlightPromptTokens(in, sty)

	require.Equal(t, "see @main.go here", ansi.Strip(got))
	require.Equal(t, []string{"@main.go"}, styledSpans(got, "31"))
}

func TestHighlightPromptTokensMultiline(t *testing.T) {
	withKnownSkills(t, "commit")
	sty := tokenTestStyles()

	got := highlightPromptTokens("@one.go\nplain\n/commit", sty)

	lines := strings.Split(got, "\n")
	require.Len(t, lines, 3)
	require.Equal(t, []string{"@one.go"}, styledSpans(lines[0], "31"))
	require.Equal(t, "plain", lines[1], "untouched lines are returned verbatim")
	require.Equal(t, []string{"/commit"}, styledSpans(lines[2], "32"))
}

// TestUserMessageRendersPromptTokens is the end-to-end check: the same
// colours the prompt editor uses survive into the posted message.
func TestUserMessageRendersPromptTokens(t *testing.T) {
	withKnownSkills(t, "code-review")

	item := newTestUserItem("please /code-review @main.go", 0)
	item.sty = tokenTestStyles()

	out := item.RawRender(80)

	require.Contains(t, ansi.Strip(out), "/code-review")
	require.Contains(t, styledSpans(out, "31"), "@main.go")
	require.Contains(t, styledSpans(out, "32"), "/code-review")
}
