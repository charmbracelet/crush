package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type historyWorkspace struct {
	workspace.Workspace
}

func (historyWorkspace) Config() *config.Config {
	return &config.Config{}
}

func (historyWorkspace) PermissionSkipRequests() bool {
	return false
}

func TestHistoryBangCommandStripsPrefixWhileAlreadyInBangMode(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.com.Workspace = historyWorkspace{}
	u.promptHistory.messages = []string{"!echo one", "!echo two"}
	u.promptHistory.index = -1

	require.True(t, u.historyPrev())
	require.True(t, u.bangMode)
	require.Equal(t, "echo one", u.textarea.Value())

	require.True(t, u.historyPrev())
	require.True(t, u.bangMode)
	require.Equal(t, "echo two", u.textarea.Value())
}

func TestSyncBangModeSecondBangEngagesInteractive(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.com.Workspace = historyWorkspace{}

	// First "!" engages bang mode.
	u.textarea.SetValue("!")
	u.syncBangModeFromTextarea()
	require.True(t, u.bangMode)
	require.False(t, u.interactiveBang)
	require.Equal(t, "", u.textarea.Value())

	// A second "!" (e.g. pasted or restored from history) engages the
	// interactive terminal on top of bang mode.
	u.textarea.SetValue("!")
	u.syncBangModeFromTextarea()
	require.True(t, u.bangMode)
	require.True(t, u.interactiveBang)
	require.Equal(t, "", u.textarea.Value())

	// Regular typing does not resync; interactive mode stays armed.
	u.textarea.InsertString("gh auth login")
	require.Equal(t, "gh auth login", u.textarea.Value())
	require.True(t, u.interactiveBang)
}

func TestSyncBangModeInteractiveDisengagesWithBangMode(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.com.Workspace = historyWorkspace{}

	u.bangMode = true
	u.interactiveBang = true

	// A paste or history value without the bang prefix disengages bang
	// mode, and interactive mode goes with it.
	u.textarea.SetValue("plain text")
	u.syncBangModeFromTextarea()
	require.False(t, u.bangMode)
	require.False(t, u.interactiveBang)
}
