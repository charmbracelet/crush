package model

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

// pressKey sends a key through Update, as a running program would.
func pressKey(p *promptEditor, code rune) {
	msg := tea.KeyPressMsg{Code: code}
	if code >= 0x20 && code != 0x7f {
		msg.Text = string(code)
	}
	*p, _ = p.Update(msg)
}

func TestPromptEditorHeightBounds(t *testing.T) {
	t.Parallel()

	p := newPromptEditor()
	p.SetWidth(40)
	require.Equal(t, TextareaMinHeight, p.Height())

	p.SetValue(strings.Repeat("line\n", 30))
	require.Equal(t, TextareaMaxHeight, p.Height())

	p.Reset()
	require.Equal(t, TextareaMinHeight, p.Height())
}

func TestPromptEditorPlaceholder(t *testing.T) {
	t.Parallel()

	p := newPromptEditor()
	p.SetPlaceholder("Ask anything")
	require.Equal(t, "Ask anything", p.Placeholder())
	require.Contains(t, p.View(), "Ask anything")
}

func TestPromptEditorVimMode(t *testing.T) {
	t.Parallel()

	p := newPromptEditor()
	p.SetMode("vim")
	require.True(t, p.IsVimMode())
	require.False(t, p.Inserting(), "normal mode does not insert")

	pressKey(&p, 'i')
	require.True(t, p.Inserting(), "i enters insert mode")

	for _, r := range "hello" {
		pressKey(&p, r)
	}
	require.Equal(t, "hello", p.Value())

	pressKey(&p, tea.KeyEscape)
	require.False(t, p.Inserting())

	// Enter in normal mode is a vim motion, not a send: the text stays.
	pressKey(&p, tea.KeyEnter)
	require.Equal(t, "hello", p.Value())
}

func TestPromptEditorHelixMode(t *testing.T) {
	t.Parallel()

	p := newPromptEditor()
	p.SetMode("helix")
	require.True(t, p.HelixEnabled())
	require.False(t, p.Inserting(), "select mode does not insert")

	pressKey(&p, 'i')
	require.True(t, p.Inserting())

	for _, r := range "hello" {
		pressKey(&p, r)
	}
	require.Equal(t, "hello", p.Value())

	pressKey(&p, tea.KeyEscape)
	require.False(t, p.Inserting())
	pressKey(&p, tea.KeyEnter)
	require.Equal(t, "hello", p.Value())
}

func TestPromptEditorEmacsModeInserts(t *testing.T) {
	t.Parallel()

	p := newPromptEditor()
	require.True(t, p.Inserting())
	for _, r := range "hello" {
		pressKey(&p, r)
	}
	require.Equal(t, "hello", p.Value())
}
