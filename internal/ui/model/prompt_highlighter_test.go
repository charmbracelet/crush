package model

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

// newTestHighlighter builds a highlighter and primes the known-skill set it
// validates "/" tokens against. That set is process-wide (see
// common.SetPromptSkillNames), so these tests do not run in parallel.
func newTestHighlighter(t *testing.T, skillNames ...string) *promptHighlighter {
	t.Helper()
	common.SetPromptSkillNames(skillNames)
	t.Cleanup(func() { common.SetPromptSkillNames(nil) })
	return newPromptHighlighter(
		lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
	)
}

func TestPromptHighlighterFileTokens(t *testing.T) {
	h := newTestHighlighter(t)
	h.Rescan("see @foo.go and @bar.md end")

	line := h.Highlight(0, nil)
	require.Len(t, line, 2)
	require.Equal(t, 4, line[0].Start)
	require.Equal(t, 11, line[0].End) // "@foo.go"
	require.Equal(t, 16, line[1].Start)
	require.Equal(t, 23, line[1].End) // "@bar.md"
}

func TestPromptHighlighterSkillTokens(t *testing.T) {
	h := newTestHighlighter(t, "code-review", "commit")

	h.Rescan("please /code-review this")
	line := h.Highlight(0, nil)
	require.Len(t, line, 1)
	require.Equal(t, 7, line[0].Start)
	require.Equal(t, 19, line[0].End) // "/code-review"

	// Unknown slash-words are not styled.
	h.Rescan("please /bogus this")
	require.Empty(t, h.Highlight(0, nil))
}

// TestPromptHighlighterUsesDistinctStyles pins that the two token kinds do
// not collapse onto one colour.
func TestPromptHighlighterUsesDistinctStyles(t *testing.T) {
	h := newTestHighlighter(t, "commit")
	h.Rescan("@foo.go /commit")

	line := h.Highlight(0, nil)
	require.Len(t, line, 2)
	require.Equal(t, h.fileStyle, line[0].Style)
	require.Equal(t, h.skillStyle, line[1].Style)
}

func TestPromptHighlighterSkillPrefixWhileTyping(t *testing.T) {
	h := newTestHighlighter(t, "code-review")

	// Mid-typing prefix of a known skill highlights (live feedback).
	h.Rescan("/cod")
	require.Len(t, h.Highlight(0, nil), 1)

	// Diverged from every known skill: no highlight.
	h.Rescan("/codeX")
	require.Empty(t, h.Highlight(0, nil))
}

func TestPromptHighlighterWordBoundary(t *testing.T) {
	h := newTestHighlighter(t, "commit")

	// Triggers must start a word: email-like and path-like runs don't count.
	h.Rescan("mail me@foo.go or a/b")
	require.Empty(t, h.Highlight(0, nil))

	// Tab counts as a boundary too.
	h.Rescan("x\t@foo.go")
	require.Len(t, h.Highlight(0, nil), 1)
}

func TestPromptHighlighterBareTriggerNotStyled(t *testing.T) {
	h := newTestHighlighter(t)
	h.Rescan("@ / @")
	require.Empty(t, h.Highlight(0, nil))
}

func TestPromptHighlighterMultiline(t *testing.T) {
	h := newTestHighlighter(t)
	h.Rescan("first @one.go\nsecond @two.go\nthird plain")

	require.Len(t, h.Highlight(0, nil), 1)
	require.Len(t, h.Highlight(1, nil), 1)
	require.Empty(t, h.Highlight(2, nil))
	// Out of range is safe.
	require.Nil(t, h.Highlight(3, nil))
}

func TestPromptHighlighterSkillNamesUpdate(t *testing.T) {
	h := newTestHighlighter(t)
	h.Rescan("/commit")
	require.Empty(t, h.Highlight(0, nil))

	common.SetPromptSkillNames([]string{"commit"})
	h.Rescan("/commit")
	require.Len(t, h.Highlight(0, nil), 1)
}

// TestSkillNamesHelper verifies the UI helper filters to healthy skills.
func TestSkillNamesHelper(t *testing.T) {
	t.Parallel()

	m := &UI{
		skillStates: []*skills.SkillState{
			{Name: "good", State: skills.StateNormal},
			{Name: "bad", State: skills.StateError},
			nil,
		},
	}
	require.Equal(t, []string{"good"}, m.skillNames())
}

// TestSkillNamesFromCatalog verifies the highlighter validates "/" tokens
// against the same effective skill set the completions popup offers, so a
// disabled or overridden skill cannot highlight in one place and be missing
// from the other.
func TestSkillNamesFromCatalog(t *testing.T) {
	t.Parallel()

	m := &UI{
		skillStates: []*skills.SkillState{
			{Name: "stale-state-only", State: skills.StateNormal},
		},
		skillCatalog: []skills.CatalogEntry{
			{Name: "commit"},
			{Name: "alpha"},
		},
	}
	require.Equal(t, []string{"alpha", "commit"}, m.skillNames())
}
