package completions

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func TestSetSkillItemsOpensWithSkills(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	require.False(t, c.IsOpen())

	c.SetSkillItems([]SkillCompletionValue{
		{Name: "code-review", Path: "/skills/code-review/SKILL.md"},
		{Name: "commit", Path: "/skills/commit/SKILL.md"},
	})

	require.True(t, c.IsOpen())
	require.Len(t, c.filtered, 2)
	first, ok := c.filtered[0].(*CompletionItem)
	require.True(t, ok)
	require.Equal(t, "code-review", first.Text())
}

func TestSkillCompletionFilter(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetSkillItems([]SkillCompletionValue{
		{Name: "code-review"},
		{Name: "commit"},
		{Name: "coder"},
	})

	c.Filter("cod")

	require.NotEmpty(t, c.filtered)
	first, ok := c.filtered[0].(*CompletionItem)
	require.True(t, ok)
	// "coder" is an exact-stem/prefix tier winner over "code-review".
	require.Equal(t, "coder", first.Text())
}

func TestSelectCurrentReturnsSkillSelectionMsg(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetSkillItems([]SkillCompletionValue{
		{Name: "commit", Path: "/skills/commit/SKILL.md"},
	})

	msg := c.selectCurrent(false)
	sel, ok := msg.(SelectionMsg[SkillCompletionValue])
	require.True(t, ok, "expected SelectionMsg[SkillCompletionValue], got %T", msg)
	require.Equal(t, "commit", sel.Value.Name)
	require.Equal(t, "/skills/commit/SKILL.md", sel.Value.Path)
	require.False(t, sel.KeepOpen)
	require.False(t, c.IsOpen(), "popup should close on non-keep-open select")
}

func TestSelectCurrentSkillKeepOpen(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetSkillItems([]SkillCompletionValue{{Name: "commit"}})

	msg := c.selectCurrent(true)
	sel, ok := msg.(SelectionMsg[SkillCompletionValue])
	require.True(t, ok)
	require.True(t, sel.KeepOpen)
	require.True(t, c.IsOpen(), "popup should stay open on keep-open select")
}

func TestSetSkillItemsEmpty(t *testing.T) {
	t.Parallel()

	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetSkillItems(nil)

	require.True(t, c.IsOpen())
	require.False(t, c.HasItems())
	require.Nil(t, c.selectCurrent(false))
}

func TestSkillItemsDoNotAffectFileSelectionDispatch(t *testing.T) {
	t.Parallel()

	// Regression: file/resource values must still dispatch their own
	// SelectionMsg types after the skill case was added.
	c := New(lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	c.SetItems([]FileCompletionValue{{Path: "foo.go"}}, nil)

	msg := c.selectCurrent(false)
	sel, ok := msg.(SelectionMsg[FileCompletionValue])
	require.True(t, ok, "expected SelectionMsg[FileCompletionValue], got %T", msg)
	require.Equal(t, "foo.go", sel.Value.Path)
}
