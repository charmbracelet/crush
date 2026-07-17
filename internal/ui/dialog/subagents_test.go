package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/subagents"
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
	disabledCalls []disabledCall
}

type disabledCall struct {
	name     string
	disabled bool
}

func (w *subagentsWorkspace) SetSubagentDisabled(name string, disabled bool) error {
	w.disabledCalls = append(w.disabledCalls, disabledCall{name: name, disabled: disabled})
	return nil
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

// TestSubagentsDialog_ToggleLibraryItem verifies that pressing space on a
// library item toggles its disabled state, calling SetSubagentDisabled with
// alternating values (disable then re-enable).
func TestSubagentsDialog_ToggleLibraryItem(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		defs: []workspace.SubagentDefInfo{
			{Name: "lib-agent", Description: "does stuff", Scope: "user", Disabled: false},
		},
	}
	d := newTestSubagentsDialog(t, ws)

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, SubagentsTabLibrary, d.ActiveTab())

	runCmd := func(action Action) {
		if ac, ok := action.(ActionCmd); ok && ac.Cmd != nil {
			ac.Cmd()
		}
	}

	runCmd(d.HandleMsg(keyMsg(' ')))
	require.Len(t, ws.disabledCalls, 1)
	require.Equal(t, "lib-agent", ws.disabledCalls[0].name)
	require.True(t, ws.disabledCalls[0].disabled, "first toggle must disable")

	runCmd(d.HandleMsg(keyMsg(' ')))
	require.Len(t, ws.disabledCalls, 2)
	require.False(t, ws.disabledCalls[1].disabled, "second toggle must re-enable")
}

// TestSubagentsDialog_RuntimeEventRefreshesRunningTab verifies that a
// RuntimeEvent for the dialog's own parent session causes the running tab to
// be rebuilt from a fresh call to com.Workspace.RunningSubagents, reflecting
// entries added after the dialog was constructed.
func TestSubagentsDialog_RuntimeEventRefreshesRunningTab(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		running: []workspace.RunningSubagentInfo{
			{ChildSessionID: "child-1", Name: "agent-one", Color: "blue", Model: "claude-opus-4-7"},
		},
	}
	d := newTestSubagentsDialog(t, ws)

	ws.running = []workspace.RunningSubagentInfo{
		{ChildSessionID: "child-1", Name: "agent-one", Color: "blue", Model: "claude-opus-4-7"},
		{ChildSessionID: "child-2", Name: "agent-two", Color: "red", Model: "claude-sonnet"},
	}

	d.HandleMsg(pubsub.Event[subagents.RuntimeEvent]{
		Type: pubsub.UpdatedEvent,
		Payload: subagents.RuntimeEvent{
			ParentSessionID: "parent-session-id",
			Entries:         nil,
		},
	})

	require.Len(t, d.runningItems, 2, "running tab should be rebuilt from the workspace after a matching RuntimeEvent")

	var ids []string
	for _, item := range d.runningItems {
		ids = append(ids, item.ID())
	}
	require.Contains(t, ids, "child-1")
	require.Contains(t, ids, "child-2")
}

// TestSubagentsDialog_RuntimeEventIgnoresOtherParentSession verifies that a
// RuntimeEvent for a different parent session does not affect the dialog's
// running tab.
func TestSubagentsDialog_RuntimeEventIgnoresOtherParentSession(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		running: []workspace.RunningSubagentInfo{
			{ChildSessionID: "child-1", Name: "agent-one", Color: "blue", Model: "claude-opus-4-7"},
		},
	}
	d := newTestSubagentsDialog(t, ws)

	ws.running = []workspace.RunningSubagentInfo{}

	d.HandleMsg(pubsub.Event[subagents.RuntimeEvent]{
		Type: pubsub.UpdatedEvent,
		Payload: subagents.RuntimeEvent{
			ParentSessionID: "some-other-session",
			Entries:         nil,
		},
	})

	require.Len(t, d.runningItems, 1, "running tab must not change for a RuntimeEvent belonging to a different parent session")
	require.Equal(t, "child-1", d.runningItems[0].ID())
}

// TestSubagentsDialog_RuntimeEventPreservesSelection verifies that the
// selected running item is tracked by ID across a refresh, not by index.
func TestSubagentsDialog_RuntimeEventPreservesSelection(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		running: []workspace.RunningSubagentInfo{
			{ChildSessionID: "child-1", Name: "agent-one", Color: "blue", Model: "claude-opus-4-7"},
			{ChildSessionID: "child-2", Name: "agent-two", Color: "red", Model: "claude-sonnet"},
		},
	}
	d := newTestSubagentsDialog(t, ws)
	d.runningList.SetSelected(1)

	ws.running = []workspace.RunningSubagentInfo{
		{ChildSessionID: "child-2", Name: "agent-two", Color: "red", Model: "claude-sonnet"},
		{ChildSessionID: "child-1", Name: "agent-one", Color: "blue", Model: "claude-opus-4-7"},
	}

	d.HandleMsg(pubsub.Event[subagents.RuntimeEvent]{
		Type: pubsub.UpdatedEvent,
		Payload: subagents.RuntimeEvent{
			ParentSessionID: "parent-session-id",
			Entries:         nil,
		},
	})

	selected, ok := d.runningList.SelectedItem().(ListItem)
	require.True(t, ok, "an item should remain selected after refresh")
	require.Equal(t, "child-2", selected.ID(), "selection should follow the same logical item across a reorder")
}

// TestSubagentsDialog_LibraryEventRefreshesLibraryTab verifies that a
// subagents.Event causes the library tab to be rebuilt from a fresh call to
// com.Workspace.AllSubagents, reflecting entries added after the dialog was
// constructed.
func TestSubagentsDialog_LibraryEventRefreshesLibraryTab(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		defs: []workspace.SubagentDefInfo{
			{Name: "agent-a", Scope: "user"},
		},
	}
	d := newTestSubagentsDialog(t, ws)

	ws.defs = []workspace.SubagentDefInfo{
		{Name: "agent-a", Scope: "user"},
		{Name: "agent-b", Scope: "project"},
	}

	d.HandleMsg(pubsub.Event[subagents.Event]{
		Type:    pubsub.UpdatedEvent,
		Payload: subagents.Event{},
	})

	require.Len(t, d.libraryItems, 2, "library tab should be rebuilt from the workspace after a subagents.Event")

	var ids []string
	for _, item := range d.libraryItems {
		ids = append(ids, item.ID())
	}
	require.Contains(t, ids, "agent-a")
	require.Contains(t, ids, "agent-b")
}

// TestSubagentsDialog_LibraryEventPreservesSelection verifies that the
// selected library item is tracked by ID across a refresh, not by index.
func TestSubagentsDialog_LibraryEventPreservesSelection(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		defs: []workspace.SubagentDefInfo{
			{Name: "agent-a", Scope: "user"},
			{Name: "agent-b", Scope: "user"},
		},
	}
	d := newTestSubagentsDialog(t, ws)
	d.libraryList.SetSelected(1)

	ws.defs = []workspace.SubagentDefInfo{
		{Name: "agent-b", Scope: "user"},
		{Name: "agent-a", Scope: "user"},
	}

	d.HandleMsg(pubsub.Event[subagents.Event]{
		Type:    pubsub.UpdatedEvent,
		Payload: subagents.Event{},
	})

	selected, ok := d.libraryList.SelectedItem().(ListItem)
	require.True(t, ok, "an item should remain selected after refresh")
	require.Equal(t, "agent-b", selected.ID(), "selection should follow the same logical item across a reorder")
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
