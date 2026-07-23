package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newTestSnippet(t *testing.T) *Snippet {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	return NewSnippet(com)
}

func ctrlKeyMsg(ch rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: ch, Mod: tea.ModCtrl}
}

// TestSnippet_IDIsSnippet verifies the dialog ID constant.
func TestSnippet_IDIsSnippet(t *testing.T) {
	t.Parallel()

	s := newTestSnippet(t)
	require.Equal(t, SnippetID, s.ID())
}

// TestSnippet_EscClosesDialog verifies that pressing Esc returns ActionClose.
func TestSnippet_EscClosesDialog(t *testing.T) {
	t.Parallel()

	s := newTestSnippet(t)
	action := s.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.IsType(t, ActionClose{}, action)
}

// TestSnippet_EmptySubmitClosesDialog verifies that ctrl+d on empty content
// returns ActionClose rather than an empty snippet.
func TestSnippet_EmptySubmitClosesDialog(t *testing.T) {
	t.Parallel()

	s := newTestSnippet(t)
	action := s.HandleMsg(ctrlKeyMsg('d'))
	require.IsType(t, ActionClose{}, action, "empty snippet should close without inserting")
}

// TestSnippet_SubmitReturnsContent verifies that ctrl+d with typed content
// returns ActionInsertSnippet with the correct text.
func TestSnippet_SubmitReturnsContent(t *testing.T) {
	t.Parallel()

	s := newTestSnippet(t)

	// Type some content character-by-character.
	for _, r := range "hello world" {
		s.HandleMsg(keyMsg(r))
	}

	action := s.HandleMsg(ctrlKeyMsg('d'))
	ins, ok := action.(ActionInsertSnippet)
	require.True(t, ok, "ctrl+d with content should return ActionInsertSnippet")
	require.Equal(t, "hello world", ins.Content)
}

// TestSnippet_PasteIsForwardedToEditor verifies that paste messages reach the
// underlying textarea (returned as an ActionCmd for deferred processing).
func TestSnippet_PasteIsForwardedToEditor(t *testing.T) {
	t.Parallel()

	s := newTestSnippet(t)
	action := s.HandleMsg(tea.PasteMsg{Content: "pasted code"})
	_, ok := action.(ActionCmd)
	require.True(t, ok, "PasteMsg should produce an ActionCmd")
}
