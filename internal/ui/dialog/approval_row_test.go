package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// approvalWorkspace answers only the questions the palette asks about the
// permission axis. Everything else is the embedded nil interface, so a test
// that drifts into another part of the workspace fails loudly rather than
// quietly reading a zero value.
type approvalWorkspace struct {
	workspace.Workspace
	level permission.Level
}

func (w *approvalWorkspace) PermissionLevel() permission.Level { return w.level }
func (w *approvalWorkspace) PermissionSetLevel(level permission.Level) {
	w.level = level
}
func (w *approvalWorkspace) AgentMainID() string           { return config.AgentCoder }
func (w *approvalWorkspace) AgentMainCandidates() []string { return []string{config.AgentCoder} }

// Config answers the palette's own read of the configuration; a bare config is
// all the rows this test cares about need.
func (w *approvalWorkspace) Config() *config.Config { return &config.Config{} }

func newApprovalPalette(t *testing.T, level permission.Level) (*Commands, *approvalWorkspace) {
	t.Helper()

	sty := styles.CharmtonePantera()
	ws := &approvalWorkspace{level: level}
	com := &common.Common{Styles: &sty, Workspace: ws}
	dia, err := NewCommands(com, "session-1", true, false, false, nil, nil)
	require.NoError(t, err)
	return dia, ws
}

// approvalRow returns the palette's single approval item.
func approvalRow(t *testing.T, dia *Commands) *CommandItem {
	t.Helper()

	var found *CommandItem
	for _, item := range dia.list.FilteredItems() {
		cmd, ok := item.(*CommandItem)
		if !ok || cmd.id != approvalCommandID {
			continue
		}
		require.Nil(t, found, "the palette must offer exactly one approval row")
		found = cmd
	}
	require.NotNil(t, found, "the palette must offer an approval row")
	return found
}

// TestApprovalRowShowsEveryOptionWithTheActiveOneMarked is the point of the
// change: a user should be able to see what the alternatives are and which one
// is in force, rather than cycling blind to find out.
func TestApprovalRowShowsEveryOptionWithTheActiveOneMarked(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		level  permission.Level
		marked string
	}{
		{permission.LevelPrompt, "[ask]"},
		{permission.LevelAuto, "[auto]"},
		{permission.LevelBypass, "[yolo]"},
	} {
		sty := styles.CharmtonePantera()
		item := NewCommandItem(&sty, approvalCommandID, "Approval Mode:", "ctrl+y", nil).
			WithSegments(ApprovalSegments(tt.level)...)

		out := ansi.Strip(item.Render(60))

		require.Contains(t, out, "Approval Mode:",
			"the row still names the axis it controls")
		require.Contains(t, out, "ask", "every option is visible, not just the active one")
		require.Contains(t, out, "auto")
		require.Contains(t, out, "yolo")
		require.Contains(t, out, tt.marked,
			"the level in force is marked in text, so the row still reads on a monochrome terminal")
		require.Equal(t, 1, countBracketed(out), "exactly one option is marked")
	}
}

// TestApprovalRowIsFilterableByEveryOption drives the real filter rather than
// inspecting the search string: the row shows three options, so typing any of
// them has to find it. Fuzzy matching is case-insensitive, which is why
// "approval" finds "Approval Mode".
func TestApprovalRowIsFilterableByEveryOption(t *testing.T) {
	t.Parallel()

	for _, query := range []string{"ask", "auto", "yolo", "bypass", "approval", "permissions", "mode"} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()

			dia, _ := newApprovalPalette(t, permission.LevelAuto)
			dia.list.SetFilter(query)

			ids := make([]string, 0)
			for _, item := range dia.list.FilteredItems() {
				if cmd, ok := item.(*CommandItem); ok {
					ids = append(ids, cmd.id)
				}
			}
			require.Contains(t, ids, approvalCommandID,
				"typing %q must find the approval row", query)
		})
	}
}

// TestApprovalRowRedrawsInPlace is what lets the palette stay open while the
// level changes: the row mutates and bumps its version, which the list cache
// reads as an invalidate, and nothing around it is rebuilt.
func TestApprovalRowRedrawsInPlace(t *testing.T) {
	t.Parallel()

	dia, ws := newApprovalPalette(t, permission.LevelPrompt)
	row := approvalRow(t, dia)
	require.Contains(t, ansi.Strip(row.Render(60)), "[ask]")

	dia.list.SetFilter("approv")
	before := len(dia.list.FilteredItems())

	dia.RefreshApprovalMode(permission.LevelBypass)

	require.Contains(t, ansi.Strip(row.Render(60)), "[yolo]")
	require.NotContains(t, ansi.Strip(row.Render(60)), "[ask]")
	require.Equal(t, before, len(dia.list.FilteredItems()),
		"redrawing must not disturb what the user had filtered to")
	require.Equal(t, permission.LevelPrompt, ws.level,
		"refreshing the display is not what changes the setting")
}

// TestPaletteNoLongerOffersOneRowPerLevel guards the regression this replaced:
// three separate commands that each looked like a different feature.
func TestPaletteNoLongerOffersOneRowPerLevel(t *testing.T) {
	t.Parallel()

	dia, _ := newApprovalPalette(t, permission.LevelAuto)

	ids := make([]string, 0, len(dia.list.FilteredItems()))
	for _, item := range dia.list.FilteredItems() {
		if cmd, ok := item.(*CommandItem); ok {
			ids = append(ids, cmd.id)
		}
	}
	require.Contains(t, ids, approvalCommandID)
	for _, gone := range []string{"permissions_ask", "permissions_auto", "permissions_bypass"} {
		require.NotContains(t, ids, gone, "the per-level rows are replaced by the cycling row")
	}
}

// TestApprovalSegmentsFollowTheAxisOrder pins the row to the same order Ctrl+Y
// walks, so the display and the shortcut cannot disagree about what comes next.
func TestApprovalSegmentsFollowTheAxisOrder(t *testing.T) {
	t.Parallel()

	segments := ApprovalSegments(permission.LevelAuto)
	require.Len(t, segments, len(permissionLevelEntries))

	for i, seg := range segments {
		require.Equal(t, permissionLevelEntries[i].label, seg.Text)
		require.Equal(t, permissionLevelEntries[i].level == permission.LevelAuto, seg.Active)
	}
}

func countBracketed(s string) int {
	count := 0
	for _, r := range s {
		switch r {
		case '[':
			count++
		}
	}
	return count
}
