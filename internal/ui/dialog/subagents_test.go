package dialog

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
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
// RuntimeEvent for the dialog's own parent session rebuilds the running tab
// from a fresh call to com.Workspace.RunningSubagents, reflecting entries added
// after the dialog was constructed. The fetch happens in a tea.Cmd rather than
// inline, so the event yields a command whose message carries the new list.
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

	action := d.HandleMsg(pubsub.Event[subagents.RuntimeEvent]{
		Type: pubsub.UpdatedEvent,
		Payload: subagents.RuntimeEvent{
			ParentSessionID: "parent-session-id",
			Entries:         nil,
		},
	})

	ac, ok := action.(ActionCmd)
	require.True(t, ok, "a matching RuntimeEvent must dispatch the fetch as a command, not query inline")
	require.NotNil(t, ac.Cmd)

	fetched, ok := ac.Cmd().(RunningSubagentsFetchedMsg)
	require.True(t, ok, "the command must deliver a RunningSubagentsFetchedMsg")
	require.Len(t, d.runningItems, 1, "the list must not change until the fetch is applied")

	d.HandleMsg(fetched)

	require.Len(t, d.runningItems, 2, "running tab should be rebuilt from the workspace after a matching RuntimeEvent")

	var ids []string
	for _, item := range d.runningItems {
		ids = append(ids, item.ID())
	}
	require.Contains(t, ids, "child-1")
	require.Contains(t, ids, "child-2")
}

// TestSubagentsDialog_StaleRunningFetchDiscarded verifies the fetch reply is
// scoped to the dialog's parent session, mirroring the sidebar's guard.
func TestSubagentsDialog_StaleRunningFetchDiscarded(t *testing.T) {
	t.Parallel()

	ws := &subagentsWorkspace{
		running: []workspace.RunningSubagentInfo{
			{ChildSessionID: "child-1", Name: "agent-one", Color: "blue", Model: "claude-opus-4-7"},
		},
	}
	d := newTestSubagentsDialog(t, ws)

	d.HandleMsg(RunningSubagentsFetchedMsg{
		ParentSessionID: "some-other-session",
		List: []workspace.RunningSubagentInfo{
			{ChildSessionID: "child-9", Name: "wrong-agent"},
			{ChildSessionID: "child-8", Name: "also-wrong"},
		},
	})

	require.Len(t, d.runningItems, 1, "a fetch for another parent session must not replace this dialog's list")
	require.Equal(t, "child-1", d.runningItems[0].ID())
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

// subagentsDialogInnerWidth reproduces the width arithmetic Subagents.Draw
// uses to derive its content width from a screen width.
func subagentsDialogInnerWidth(t *styles.Styles, screenWidth int) int {
	width := max(0, min(defaultDialogMaxWidth, screenWidth-t.Dialog.View.GetHorizontalBorderSize()))
	return width - t.Dialog.View.GetHorizontalFrameSize()
}

// drawSubagentsDialog draws d onto a fresh screen of the given size.
func drawSubagentsDialog(d *Subagents, width, height int) uv.ScreenBuffer {
	scr := uv.NewScreenBuffer(width, height)
	d.Draw(scr, image.Rect(0, 0, width, height))
	return scr
}

// drawItem draws a rendered list item onto a fresh screen of the given size.
func drawItem(rendered string, width, height int) uv.ScreenBuffer {
	scr := uv.NewScreenBuffer(width, height)
	uv.NewStyledString(rendered).Draw(scr, scr.Bounds())
	return scr
}

// screenCell returns the cell where the first occurrence of text starts on
// scr, failing the test when text is not on screen.
func screenCell(t *testing.T, scr uv.ScreenBuffer, text string) *uv.Cell {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(scr.Render()), "\n") {
		if i := strings.Index(line, text); i >= 0 {
			return scr.CellAt(ansi.StringWidth(line[:i]), y)
		}
	}
	require.FailNow(t, "text not on screen", "%q", text)
	return nil
}

// TestSubagentsDialog_HelpFooterMatchesRenderDialogHelp verifies that the help
// footer is the shared renderDialogHelp line, padded like every other
// dialog's, rather than a raw help.Model.View.
func TestSubagentsDialog_HelpFooterMatchesRenderDialogHelp(t *testing.T) {
	t.Parallel()

	d := newTestSubagentsDialog(t, &subagentsWorkspace{})
	scr := drawSubagentsDialog(d, 80, 30)

	st := d.com.Styles
	want := ansi.Strip(renderDialogHelp(st, &d.help, d, subagentsDialogInnerWidth(st, 80)))
	require.Contains(t, ansi.Strip(scr.Render()), want)
}

// TestSubagentsDialog_HelpFooterNoWrapAtNarrowWidth verifies that at a narrow
// width the help footer stays on one line, so the dialog keeps its clamped
// height. help.Model.ShortHelpView keeps an oversized item when even the
// ellipsis doesn't fit, and that line wraps inside the dialog border.
func TestSubagentsDialog_HelpFooterNoWrapAtNarrowWidth(t *testing.T) {
	t.Parallel()

	d := newTestSubagentsDialog(t, &subagentsWorkspace{})
	st := d.com.Styles
	const innerWidth, screenHeight = 12, 30
	screenWidth := innerWidth + st.Dialog.View.GetHorizontalBorderSize() + st.Dialog.View.GetHorizontalFrameSize()
	require.Equal(t, innerWidth, subagentsDialogInnerWidth(st, screenWidth), "test setup must target the intended inner width")

	lines := strings.Split(ansi.Strip(drawSubagentsDialog(d, screenWidth, screenHeight).Render()), "\n")
	top := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "╭") })
	bottom := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "╰") })
	require.NotEqual(t, -1, top)
	require.NotEqual(t, -1, bottom)

	border := st.Dialog.View.GetVerticalBorderSize()
	want := min(defaultDialogHeight, screenHeight-border) + border
	require.Equal(t, want, bottom-top+1, "a wrapped help footer makes the dialog taller than its clamped height")
}

// TestSubagentsDialog_TabIndicatorStylesEverySegment verifies that the
// separator and the inactive tab label carry their own style instead of
// falling back to the terminal default after the active label's reset.
func TestSubagentsDialog_TabIndicatorStylesEverySegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		toggleTab     bool
		inactiveLabel string
	}{
		{name: "running active", toggleTab: false, inactiveLabel: "Library"},
		{name: "library active", toggleTab: true, inactiveLabel: "Running"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := newTestSubagentsDialog(t, &subagentsWorkspace{})
			if tt.toggleTab {
				d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
			}
			scr := drawSubagentsDialog(d, 80, 30)

			want := d.com.Styles.Radio.Label.GetForeground()
			requirePlanHandoffColorEqual(t, want, screenCell(t, scr, "| ").Style.Fg)
			requirePlanHandoffColorEqual(t, want, screenCell(t, scr, tt.inactiveLabel).Style.Fg)
		})
	}
}

// TestSubagentsDialog_LibraryListShowsScrollbarWhenOverflowing verifies that
// an overflowing Library list renders a scrollbar column, as every other list
// dialog does via joinScrollbar.
func TestSubagentsDialog_LibraryListShowsScrollbarWhenOverflowing(t *testing.T) {
	t.Parallel()

	var defs []workspace.SubagentDefInfo
	for i := range 40 {
		defs = append(defs, workspace.SubagentDefInfo{
			Name:  fmt.Sprintf("agent-%02d", i),
			Scope: "user",
		})
	}
	d := newTestSubagentsDialog(t, &subagentsWorkspace{defs: defs})
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, SubagentsTabLibrary, d.ActiveTab())

	raw := drawSubagentsDialog(d, 80, 30).Render()

	// Only the thumb is checked: the track glyph "│" is also the dialog's
	// rounded-border side, so its presence proves nothing.
	st := d.com.Styles
	require.Contains(t, raw, st.Dialog.ScrollbarThumb.Render(styles.ScrollbarThumb),
		"an overflowing library list must render a scrollbar thumb")
}

// TestSubagentsDialog_RunningListUsesFullInnerWidthWhenShort verifies that a
// list short enough to need no scrollbar sizes its rows to the dialog's full
// inner width instead of always reserving a gutter.
func TestSubagentsDialog_RunningListUsesFullInnerWidthWhenShort(t *testing.T) {
	t.Parallel()

	// The name fits only once rows span the full inner width; with the old
	// fixed innerWidth-3 it is truncated.
	name := strings.Repeat("n", 63)
	d := newTestSubagentsDialog(t, &subagentsWorkspace{
		running: []workspace.RunningSubagentInfo{
			{ChildSessionID: "child-1", Name: name, Color: "blue"},
		},
	})

	raw := ansi.Strip(drawSubagentsDialog(d, 80, 30).Render())
	require.Contains(t, raw, name,
		"a short list (no scrollbar needed) must size rows to the full inner width, not innerWidth-3")
}
