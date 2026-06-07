package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// subagentsWorkspace stubs only the workspace methods exercised by the
// Subagents dialog.
type subagentsWorkspace struct {
	workspace.Workspace
	running       []workspace.RunningSubagentInfo
	defs          []workspace.SubagentDefInfo
	cancelledIDs  []string
	deletedNames  []string
	deleteUserErr error
}

func (w *subagentsWorkspace) RunningSubagents(_ string) []workspace.RunningSubagentInfo {
	return w.running
}

func (w *subagentsWorkspace) AllSubagents() []workspace.SubagentDefInfo {
	return w.defs
}

func (w *subagentsWorkspace) CancelSubagent(childSessionID string) {
	w.cancelledIDs = append(w.cancelledIDs, childSessionID)
}

func (w *subagentsWorkspace) DeleteUserSubagent(name string) error {
	w.deletedNames = append(w.deletedNames, name)
	return w.deleteUserErr
}

func newTestSubagentsDialog(t *testing.T, ws *subagentsWorkspace) *Subagents {
	t.Helper()
	st := styles.CharmtonePantera()
	com := &common.Common{Styles: &st, Workspace: ws}
	return NewSubagents(com, "parent-session-id")
}

// TestSubagentsDialog_ImplementsDialogInterface is a compile-time assertion.
var _ Dialog = (*Subagents)(nil)

// TestSubagentsDialog_TabToggle verifies that tab key toggles between
// Running and Library tabs and that a second tab returns to Running.
func TestSubagentsDialog_TabToggle(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		running: []workspace.RunningSubagentInfo{
			{ChildSessionID: "child-1", Name: "agent-one", Color: "blue", Model: "claude-opus-4-7"},
		},
		defs: []workspace.SubagentDefInfo{
			{Name: "lib-agent", Scope: "user"},
		},
	}
	d := newTestSubagentsDialog(t, ws)

	require.Equal(t, SubagentsTabRunning, d.ActiveTab(), "initial tab should be Running")

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, SubagentsTabLibrary, d.ActiveTab(), "after one tab, should be Library")

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, SubagentsTabRunning, d.ActiveTab(), "after two tabs, should return to Running")
}

// TestSubagentsDialog_EnterOnRunningItem verifies that pressing enter on
// a running subagent row returns ActionLoadSubagentSession with the correct
// child session ID.
func TestSubagentsDialog_EnterOnRunningItem(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		running: []workspace.RunningSubagentInfo{
			{ChildSessionID: "child-session-42", Name: "my-agent", Color: "red", Model: "claude-opus-4-7"},
		},
	}
	d := newTestSubagentsDialog(t, ws)

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	loaded, ok := action.(ActionLoadSubagentSession)
	require.True(t, ok, "enter on running item should return ActionLoadSubagentSession, got %T", action)
	require.Equal(t, "child-session-42", loaded.SessionID)
}

// TestSubagentsDialog_XCancelsRunningSubagent verifies that pressing x on a
// running subagent row calls CancelSubagent with the child session ID.
func TestSubagentsDialog_XCancelsRunningSubagent(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		running: []workspace.RunningSubagentInfo{
			{ChildSessionID: "child-cancel-me", Name: "cancellable-agent", Color: "green", Model: "claude-sonnet"},
		},
	}
	d := newTestSubagentsDialog(t, ws)

	d.HandleMsg(keyMsg('x'))

	require.Contains(t, ws.cancelledIDs, "child-cancel-me", "CancelSubagent must be called with child session ID")
}

// TestSubagentsDialog_EscReturnsActionClose verifies that pressing esc
// returns ActionClose{}.
func TestSubagentsDialog_EscReturnsActionClose(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{}
	d := newTestSubagentsDialog(t, ws)

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})

	_, ok := action.(ActionClose)
	require.True(t, ok, "esc should return ActionClose{}, got %T", action)
}

// TestSubagentsDialog_DeleteLibraryItem verifies that pressing d on a
// user-scoped library item enters confirm-delete mode, and pressing y
// calls DeleteUserSubagent with the item name.
func TestSubagentsDialog_DeleteLibraryItem(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		defs: []workspace.SubagentDefInfo{
			{Name: "user-agent", Description: "does stuff", Scope: "user"},
		},
	}
	d := newTestSubagentsDialog(t, ws)

	// Navigate to Library tab first.
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, SubagentsTabLibrary, d.ActiveTab())

	// Press d to enter confirm-delete mode.
	d.HandleMsg(keyMsg('d'))
	require.True(t, d.IsConfirmingDelete(), "pressing d should enter confirm-delete mode")

	// Press y to confirm deletion; execute the returned cmd to drive the IO.
	action := d.HandleMsg(keyMsg('y'))
	if ac, ok := action.(ActionCmd); ok && ac.Cmd != nil {
		ac.Cmd()
	}
	require.Contains(t, ws.deletedNames, "user-agent", "DeleteUserSubagent must be called with agent name")
}

// TestSubagentsDialog_DeleteLibraryItem_Cancel verifies that pressing d
// then n cancels the deletion without calling DeleteUserSubagent.
func TestSubagentsDialog_DeleteLibraryItem_Cancel(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		defs: []workspace.SubagentDefInfo{
			{Name: "user-agent", Description: "does stuff", Scope: "user"},
		},
	}
	d := newTestSubagentsDialog(t, ws)

	// Navigate to Library tab.
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})

	// Enter confirm-delete mode.
	d.HandleMsg(keyMsg('d'))
	require.True(t, d.IsConfirmingDelete())

	// Cancel with n.
	d.HandleMsg(keyMsg('n'))
	require.False(t, d.IsConfirmingDelete(), "pressing n should exit confirm-delete mode")
	require.Empty(t, ws.deletedNames, "DeleteUserSubagent must not be called when deletion is cancelled")
}

// stripANSIDialog strips ANSI escape sequences from a string for plain-text
// assertions in dialog tests.
func stripANSIDialog(s string) string {
	var b strings.Builder
	esc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			esc = true
			continue
		}
		if esc {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
				esc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
