package dialog

import (
	"strings"
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
	level   permission.Level
	purpose string
}

func (w *approvalWorkspace) PermissionLevel() permission.Level { return w.level }
func (w *approvalWorkspace) PermissionSetLevel(level permission.Level) {
	w.level = level
}
func (w *approvalWorkspace) AgentMainID() string { return w.purpose }
func (w *approvalWorkspace) AgentMainCandidates() []string {
	return []string{config.AgentCoder, config.AgentPlan}
}

// Config answers the palette's own read of the configuration; a bare config is
// all the rows this test cares about need.
func (w *approvalWorkspace) Config() *config.Config { return &config.Config{} }

func newApprovalPalette(t *testing.T, level permission.Level) (*Commands, *approvalWorkspace) {
	t.Helper()

	sty := styles.CharmtonePantera()
	ws := &approvalWorkspace{level: level, purpose: config.AgentCoder}
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

// modeRow returns the palette's single mode item.
func modeRow(t *testing.T, dia *Commands) *CommandItem {
	t.Helper()

	var found *CommandItem
	for _, item := range dia.list.FilteredItems() {
		cmd, ok := item.(*CommandItem)
		if !ok || cmd.id != purposeCommandID {
			continue
		}
		require.Nil(t, found, "the palette must offer exactly one mode row")
		found = cmd
	}
	require.NotNil(t, found, "the palette must offer a mode row")
	return found
}

// TestModeRowShowsEveryModeWithTheActiveOneMarked: the mode axis gets the same
// treatment as the approval axis, so both read as a setting with alternatives
// rather than as a list of unrelated commands.
func TestModeRowShowsEveryModeWithTheActiveOneMarked(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		purpose string
		marked  string
	}{
		{config.AgentCoder, "[standard]"},
		{config.AgentPlan, "[planning]"},
	} {
		sty := styles.CharmtonePantera()
		item := NewCommandItem(&sty, purposeCommandID, "Mode:", "shift+tab", nil).
			WithSegments(PurposeSegments(tt.purpose, []string{config.AgentCoder, config.AgentPlan})...)

		out := ansi.Strip(item.Render(60))

		require.Contains(t, out, "Mode:", "the row still names the axis it controls")
		require.Contains(t, out, "standard")
		require.Contains(t, out, "planning")
		require.Contains(t, out, tt.marked)
		require.Equal(t, 1, countBracketed(out), "exactly one mode is marked")
	}
}

// TestModeRowRedrawsWhenTheSwitchLands: the switch is asynchronous, so the row
// is redrawn from the message that reports it rather than from the key press.
func TestModeRowRedrawsWhenTheSwitchLands(t *testing.T) {
	t.Parallel()

	dia, ws := newApprovalPalette(t, permission.LevelPrompt)
	row := modeRow(t, dia)
	require.Contains(t, ansi.Strip(row.Render(60)), "[standard]")

	ws.purpose = config.AgentPlan
	dia.RefreshPurposeMode(config.AgentPlan)

	require.Contains(t, ansi.Strip(row.Render(60)), "[planning]")
	require.NotContains(t, ansi.Strip(row.Render(60)), "[standard]")
}

// TestPaletteNoLongerOffersOneRowPerMode guards the regression: one row per
// mode, each suffixed "(current)", which never showed what the alternatives
// were.
func TestPaletteNoLongerOffersOneRowPerMode(t *testing.T) {
	t.Parallel()

	dia, _ := newApprovalPalette(t, permission.LevelAuto)

	ids := make([]string, 0, len(dia.list.FilteredItems()))
	for _, item := range dia.list.FilteredItems() {
		if cmd, ok := item.(*CommandItem); ok {
			ids = append(ids, cmd.id)
		}
	}
	require.Contains(t, ids, purposeCommandID)
	require.NotContains(t, ids, "set_mode_"+config.AgentCoder)
	require.NotContains(t, ids, "set_mode_"+config.AgentPlan)
}

// TestAutoModeModelIsNotInThePalette: the classifier model is chosen from the
// model chooser, which already offers it, so the palette does not need a second
// door to the same room.
func TestAutoModeModelIsNotInThePalette(t *testing.T) {
	t.Parallel()

	dia, _ := newApprovalPalette(t, permission.LevelAuto)

	for _, item := range dia.list.FilteredItems() {
		if cmd, ok := item.(*CommandItem); ok {
			require.NotEqual(t, "auto_mode_model", cmd.id,
				"the auto-mode model is selected from the model chooser")
		}
	}
}

// TestModeRowIsFilterable pins that both modes and the axis name find the row.
func TestModeRowIsFilterable(t *testing.T) {
	t.Parallel()

	for _, query := range []string{"standard", "planning", "mode", "purpose"} {
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
			require.Contains(t, ids, purposeCommandID, "typing %q must find the mode row", query)
		})
	}
}

// TestAxisRowsCarryNoSubtext: the two axis rows are single-line settings like
// every other system command. They were the only entries in the list with a
// line of prose under them, which made them read as a different kind of thing.
//
// This is deliberately narrow. A rule that no system command may carry a
// description would be the stronger statement, but it is not true of the
// merged build, where the compaction row brings one of its own.
func TestAxisRowsCarryNoSubtext(t *testing.T) {
	t.Parallel()

	dia, _ := newApprovalPalette(t, permission.LevelAuto)

	for _, id := range []string{approvalCommandID, purposeCommandID} {
		var found bool
		for _, item := range dia.list.FilteredItems() {
			cmd, ok := item.(*CommandItem)
			if !ok || cmd.id != id {
				continue
			}
			found = true
			require.Empty(t, cmd.description,
				"%s must stay a single-line row", id)
			require.Zero(t, strings.Count(cmd.Render(62), "\n"),
				"%s must render on one line", id)
		}
		require.True(t, found, "%s must be in the palette", id)
	}
}
