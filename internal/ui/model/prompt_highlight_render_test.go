package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestRenderEditorViewStylesTokens is the end-to-end check: a prompt
// containing @file and /skill tokens renders with ANSI styling once the
// highlighter is wired, and the plain text is unchanged underneath.
func TestRenderEditorViewStylesTokens(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.promptHighlighter = newTestHighlighter("code-review")
	m.textarea.SetHighlighter(m.promptHighlighter)
	m.textarea.SetValue("please /code-review @main.go")
	m.textarea.SetWidth(120)

	view := m.renderEditorView(120)

	stripped := ansi.Strip(view)
	require.Contains(t, stripped, "please /code-review @main.go")
	require.True(t, strings.Contains(view, "\x1b["), "expected styled output, got %q", view)
}

// TestRenderEditorViewUnknownSkillNotStyled verifies an arbitrary /word in
// the prompt is left alone.
func TestRenderEditorViewUnknownSkillNotStyled(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.promptHighlighter = newTestHighlighter("code-review")
	m.textarea.SetHighlighter(m.promptHighlighter)
	m.textarea.SetValue("what is /bogus anyway")
	m.textarea.SetWidth(120)

	before := m.textarea.View()
	after := func() string {
		m.promptHighlighter.Rescan(m.textarea.Value())
		return m.textarea.View()
	}()
	require.Equal(t, before, after)
}

// TestRenderEditorViewBangModeSuppressesTokens verifies shell prompts don't
// get token styling.
func TestRenderEditorViewBangModeSuppressesTokens(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.promptHighlighter = newTestHighlighter("ls")
	m.textarea.SetHighlighter(m.promptHighlighter)
	m.bangMode = true
	m.textarea.SetValue("ls /tmp")
	m.textarea.SetWidth(120)

	before := m.textarea.View()
	_ = m.renderEditorView(120)
	after := m.textarea.View()

	require.Equal(t, before, after, "bang mode must not restyle the prompt")
}

// TestPromptHighlighterSeededFromSkillStates covers the construction-time
// seeding: names come from skillStates so a /skill typed before the first
// skills.Event still highlights.
func TestPromptHighlighterSeededFromSkillStates(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.skillStates = []*skills.SkillState{
		{Name: "commit", State: skills.StateNormal},
	}
	h := newTestHighlighter(m.skillNames()...)
	h.Rescan("/commit")
	require.Len(t, h.Highlight(0, nil), 1)
}
