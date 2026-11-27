package dialog

import (
	"image"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func newTestPermissions(t *testing.T) *Permissions {
	t.Helper()
	return newTestPermissionsWithDanger(t, "")
}

// newTestPermissionsWithDanger builds a dialog for a request flagged with
// the given danger reason. An empty reason means an ordinary request.
func newTestPermissionsWithDanger(t *testing.T, danger string) *Permissions {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	perm := permission.PermissionRequest{
		ID:         "perm-test",
		ToolCallID: "tool-call-test",
		ToolName:   "bash",
		Danger:     danger,
	}
	return NewPermissions(com, perm)
}

// TestPermissions_ActionKeysResolve verifies that action keys produce the
// correct permission response.
func TestPermissions_ActionKeysResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key    tea.KeyPressMsg
		action PermissionAction
	}{
		{keyMsg('a'), PermissionAllow},
		{keyMsg('A'), PermissionAllow},
		{keyMsg('d'), PermissionDeny},
		{keyMsg('D'), PermissionDeny},
		{keyMsg('s'), PermissionAllowForSession},
		{keyMsg('S'), PermissionAllowForSession},
	}

	for _, tc := range tests {
		p := newTestPermissions(t)
		action := p.HandleMsg(tc.key)
		resp, ok := action.(ActionPermissionResponse)
		require.Truef(t, ok, "key %q should produce ActionPermissionResponse", tc.key.Text)
		require.Equal(t, tc.action, resp.Action)
	}
}

// TestPermissions_NavigationCyclesOptions verifies that tab and arrow keys
// cycle through the three permission options.
func TestPermissions_NavigationCyclesOptions(t *testing.T) {
	t.Parallel()

	p := newTestPermissions(t)
	require.Equal(t, 0, p.selectedOption)

	// Tab cycles forward.
	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, 1, p.selectedOption)

	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, 2, p.selectedOption)

	// Wrap around.
	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, 0, p.selectedOption)

	// Left cycles backward.
	p.HandleMsg(keyMsg('h'))
	require.Equal(t, 2, p.selectedOption)
}

// TestPermissions_EnterConfirmsSelection verifies that enter confirms the
// currently selected option.
func TestPermissions_EnterConfirmsSelection(t *testing.T) {
	t.Parallel()

	p := newTestPermissions(t)
	p.selectedOption = 1 // Allow for session.

	action := p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	resp, ok := action.(ActionPermissionResponse)
	require.True(t, ok)
	require.Equal(t, PermissionAllowForSession, resp.Action)
}

// TestPermissions_DefaultSelectionAvoidsApprovingDangerousRequests verifies
// that an ordinary request starts on Allow while a dangerous one starts on
// Deny, so a reflexive enter refuses instead of approving.
func TestPermissions_DefaultSelectionAvoidsApprovingDangerousRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		danger string
		action PermissionAction
	}{
		{"ordinary request", "", PermissionAllow},
		{"dangerous request", "sudo", PermissionDeny},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := newTestPermissionsWithDanger(t, tc.danger)
			require.Equal(t, optionIndex(tc.action), p.selectedOption)

			// The selection is only meaningful if confirming it lands on
			// the matching action.
			action := p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
			resp, ok := action.(ActionPermissionResponse)
			require.True(t, ok)
			require.Equal(t, tc.action, resp.Action)
		})
	}
}

// TestPermissions_CtrlYDoesNotRespond verifies that ctrl+y, the global
// yolo-mode toggle, does not resolve the dialog. Dialogs see keys before
// the global handler, so accepting it here would silently approve whatever
// is on screen.
func TestPermissions_CtrlYDoesNotRespond(t *testing.T) {
	t.Parallel()

	for _, danger := range []string{"", "sudo"} {
		p := newTestPermissionsWithDanger(t, danger)
		action := p.HandleMsg(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
		require.Nil(t, action, "ctrl+y should not resolve the permission dialog")
	}
}

// TestPermissions_DangerousButtonsLookDifferent verifies that the approving
// buttons are styled distinctly on a dangerous request, so the prompt does
// not look identical to a routine one.
func TestPermissions_DangerousButtonsLookDifferent(t *testing.T) {
	t.Parallel()

	ordinary := newTestPermissionsWithDanger(t, "")
	dangerous := newTestPermissionsWithDanger(t, "sudo")

	// Compare the same selection so only the danger styling differs.
	ordinary.selectedOption = optionIndex(PermissionAllow)
	dangerous.selectedOption = optionIndex(PermissionAllow)

	require.NotEqual(t, ordinary.renderButtonGroup("  "), dangerous.renderButtonGroup("  "))
}

// TestPermissions_EscapeDenies verifies that escape denies the request.
func TestPermissions_EscapeDenies(t *testing.T) {
	t.Parallel()

	p := newTestPermissions(t)
	action := p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	resp, ok := action.(ActionPermissionResponse)
	require.True(t, ok)
	require.Equal(t, PermissionDeny, resp.Action)
}

// TestPermissions_TallerViewportClampsOffset verifies that growing the
// dialog's height while scrolled to the bottom does not leave the viewport
// scrolled past the end with blank rows under the content.
func TestPermissions_TallerViewportClampsOffset(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "line " + strconv.Itoa(i)
	}
	p := NewPermissions(com, permission.PermissionRequest{
		ID:          "perm-test",
		ToolName:    "custom_tool",
		Description: strings.Join(lines, "\n"),
	})

	p.Draw(uv.NewScreenBuffer(250, 25), image.Rect(0, 0, 250, 25))
	p.viewport.GotoBottom()
	p.Draw(uv.NewScreenBuffer(250, 60), image.Rect(0, 0, 250, 60))

	maxOffset := max(0, p.viewport.TotalLineCount()-p.viewport.Height())
	require.LessOrEqual(t, p.viewport.YOffset(), maxOffset)
}

// TestPermissions_BashSessionButtonNamesProgram verifies that the session
// button names the program when its grant covers every call of it, and keeps
// the plain label when the grant is keyed to the exact command.
func TestPermissions_BashSessionButtonNamesProgram(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s}

	buttons := func(program string) string {
		p := NewPermissions(com, permission.PermissionRequest{
			ID:       "perm-test",
			ToolName: tools.BashToolName,
			Params: tools.BashPermissionsParams{
				Command:        "git commit -m x",
				SessionProgram: program,
			},
		})
		return ansi.Strip(p.renderButtonGroup("  "))
	}

	require.Contains(t, buttons("git"), "Allow git for Session")
	require.Contains(t, buttons(""), "Allow for Session")
	require.NotContains(t, buttons(""), "git")
}

type permissionsTestWorkspace struct{ workspace.Workspace }

func (permissionsTestWorkspace) WorkingDir() string { return "/tmp" }

// TestPermissions_SmallWindowShowsContent verifies that in a window too
// small for the normal layout the request content (the command or diff the
// user is approving) stays visible and the whole dialog fits the area.
// Previously the fixed chrome left a zero or negative viewport, so the user
// approved blind, and at the smallest sizes the buttons and border were
// clipped.
func TestPermissions_SmallWindowShowsContent(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s, Workspace: permissionsTestWorkspace{}}

	reqs := map[string]struct {
		req  permission.PermissionRequest
		want string
	}{
		"bash": {permission.PermissionRequest{
			ID: "p", ToolName: tools.BashToolName, Path: "/tmp",
			Params: tools.BashPermissionsParams{Command: "rm -rf /tmp/foo", Description: "Remove foo"},
		}, "rm -rf /tmp/foo"},
		"edit": {permission.PermissionRequest{
			ID: "p", ToolName: tools.EditToolName, Path: "/tmp",
			Params: tools.EditPermissionsParams{FilePath: "/tmp/a.go", OldContent: "a\nb\n", NewContent: "a\nc\n"},
		}, "@@"},
	}
	for name, tc := range reqs {
		for _, sz := range [][2]int{{40, 14}, {30, 10}} {
			p := NewPermissions(com, tc.req)
			lines := drawFits(t, p, sz[0], sz[1])
			text := strings.Join(lines, "\n")
			require.Containsf(t, text, tc.want, "%s at %dx%d hides the content:\n%s", name, sz[0], sz[1], text)
			require.Contains(t, text, "Deny")
			require.GreaterOrEqual(t, p.viewport.Height(), 3, "%s at %dx%d", name, sz[0], sz[1])
		}
	}
}

// TestPermissions_EscapeHelpSaysDeny verifies the help bar labels esc by
// what it does: it refuses the request rather than just closing a view.
func TestPermissions_EscapeHelpSaysDeny(t *testing.T) {
	t.Parallel()

	p := newTestPermissions(t)
	for _, b := range p.ShortHelp() {
		if b.Help().Key == "esc" {
			require.Equal(t, "deny", b.Help().Desc)
			return
		}
	}
	t.Fatal("esc missing from help")
}

// TestPermissions_TypicalContentFitsWithoutScrolling verifies that on a
// 30-row screen a short MCP params blob is shown whole. Previously the
// dialog was capped at half the screen, leaving a three-row viewport that
// showed only the opening brace.
func TestPermissions_TypicalContentFitsWithoutScrolling(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s, Workspace: permissionsTestWorkspace{}}
	p := NewPermissions(com, permission.PermissionRequest{
		ID:       "p",
		ToolName: "mcp_github_create_issue",
		Path:     "/tmp",
		Params:   `{"owner":"charmbracelet","repo":"crush","title":"Bug","body":"text"}`,
	})
	text := strings.Join(drawFits(t, p, 100, 30), "\n")
	require.Contains(t, text, `"title": "Bug"`)
	require.Contains(t, text, "}")
	require.False(t, p.canScroll(), "content should fit without scrolling:\n%s", text)
}
