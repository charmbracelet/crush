package dialog

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/filehistory"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRewindRequiresSelectionAndConfirmation(t *testing.T) {
	picker := NewRewind(nil, "session", []filehistory.Choice{{Turn: "new", Label: "newer prompt"}, {Turn: "old", Label: "older prompt"}})
	press := func(code rune) Action { return picker.HandleMsg(tea.KeyPressMsg{Code: code}) }
	require.Nil(t, press(tea.KeyDown))
	require.Nil(t, press(tea.KeyEnter), "selecting a prompt must not immediately restore")
	require.Equal(t, ActionRewind{SessionID: "session", Action: "rewind", Target: "old"}, press(tea.KeyEnter))
	picker = NewRewind(nil, "session", []filehistory.Choice{{Turn: "new"}})
	require.Nil(t, press(tea.KeyEnter))
	require.Nil(t, press(tea.KeyEscape), "Esc returns from confirmation to the list")
	require.Equal(t, ActionClose{}, press(tea.KeyEscape))
}
