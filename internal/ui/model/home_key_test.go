package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

var altW = tea.KeyPressMsg{Code: 'w', Mod: tea.ModAlt}

// Alt+W leaves the workspace for Home: the TUI quits and flags it, without
// cancelling the run or typing into the editor.
func TestAltWQuitsToHomeWithoutCancel(t *testing.T) {
	t.Parallel()

	for _, focus := range []uiFocusState{uiFocusEditor, uiFocusMain} {
		ws := &countingWorkspace{ready: true, agentBusy: true, sessions: makeSessions(2)}
		m := newPanelUI(ws)
		m.focus = focus
		m.textarea.SetValue("draft")

		cmd := m.handleKeyPressMsg(altW)
		require.NotNil(t, cmd, "focus %d", focus)
		require.IsType(t, tea.QuitMsg{}, cmd(), "focus %d", focus)
		require.True(t, m.BackToHome(), "focus %d", focus)
		require.Zero(t, ws.cancelCalls, "the run keeps going")
		require.Equal(t, "draft", m.textarea.Value())
	}

	// A plain quit is not a way back to Home.
	m := newPanelUI(&countingWorkspace{ready: true})
	require.False(t, m.BackToHome())
}

// Roadmap P1.3a retires the index keys: the jump action stays (bindable in
// keybindings.json) but has no default keys, and the alt-tab style
// switcher is gone.
func TestRetiredNavigationKeysUnbound(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	require.Equal(t, []string{"alt+w"}, km.Workspaces.Keys())
	require.False(t, km.Chat.JumpSession.Enabled(), "chat.jump_session")

	m, ws := newJumpUI()
	m.keyMap = DefaultKeyMap()
	runLeaves(m.handleKeyPressMsg(altKey(2)))
	require.Empty(t, ws.loaded, "alt+2 does nothing by default")
	for _, k := range []tea.KeyPressMsg{
		{Code: tea.KeyTab, Mod: tea.ModCtrl},
		{Code: tea.KeyTab, Mod: tea.ModAlt},
	} {
		m.handleKeyPressMsg(k)
		require.False(t, m.dialog.HasDialogs(), k.String())
	}
}
