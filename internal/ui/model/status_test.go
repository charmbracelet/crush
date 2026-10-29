package model

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// leadingSpaces counts the spaces at the start of a stripped line.
func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// drawStatusLines draws the status bar into a screen buffer and returns the
// visible content of each row.
func drawStatusLines(t *testing.T, st *Status, w, h int) []string {
	t.Helper()
	scr := uv.NewScreenBuffer(w, h)
	st.Draw(scr, uv.Rect(0, 0, w, h))
	lines := strings.Split(ansi.Strip(scr.Render()), "\n")
	return lines
}

func TestStatusDrawExpandedHelpRowsAlignWithBadge(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	st := u.status
	st.helpKm = u
	st.SetWidth(100)
	st.ToggleHelp()
	st.SetMode(config.AgentPlan, permission.LevelPrompt)

	lines := drawStatusLines(t, st, 100, 6)
	require.True(t, strings.HasPrefix(lines[0], strings.Repeat(" ", badgeLeftInset)+" "+"PLAN MODE"),
		"the badge row must start with the badge inset and its padding: %q", lines[0])

	// Every subsequent help row must start at the same column as the hints
	// on the badge row: badge inset + badge width + separator + help padding.
	badge := st.modeBadge()
	wantHintsCol := badgeLeftInset + lipgloss.Width(badge) + 1 + u.com.Styles.Status.Help.GetPaddingLeft()
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		require.Equal(t, wantHintsCol, leadingSpaces(line),
			"expanded help row %d must align with the badge row hints", i+1)
	}
}

func TestStatusDrawExpandedHelpRowsAlignWithoutBadge(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	u.attachments = attachments.New(nil, attachments.Keymap{})
	st := u.status
	st.helpKm = u
	st.SetWidth(100)
	st.ToggleHelp()
	st.SetMode(config.AgentCoder, permission.LevelPrompt)

	lines := drawStatusLines(t, st, 100, 6)
	wantCol := u.com.Styles.Status.Help.GetPaddingLeft()
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			break
		}
		require.Equal(t, wantCol, leadingSpaces(line),
			"expanded help row %d must align with the first row", i)
	}
}

// TestStatusModeBadgesShowBothAxes is the point of splitting purpose from
// permission: a user in planning mode at the classifier level needs to see
// both, not one badge chosen by precedence.
func TestStatusModeBadgesShowBothAxes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		purpose  string
		level    permission.Level
		contains []string
		empty    bool
	}{
		{
			name:    "standard purpose at the default level shows nothing",
			purpose: config.AgentCoder,
			level:   permission.LevelPrompt,
			empty:   true,
		},
		{
			name:     "planning alone",
			purpose:  config.AgentPlan,
			level:    permission.LevelPrompt,
			contains: []string{"PLAN MODE"},
		},
		{
			name:     "classifier alone",
			purpose:  config.AgentCoder,
			level:    permission.LevelAuto,
			contains: []string{"AUTO MODE"},
		},
		{
			name:     "planning and classifier render side by side",
			purpose:  config.AgentPlan,
			level:    permission.LevelAuto,
			contains: []string{"PLAN MODE", "AUTO MODE"},
		},
		{
			name:     "planning and bypass render side by side",
			purpose:  config.AgentPlan,
			level:    permission.LevelBypass,
			contains: []string{"PLAN MODE", "YOLO"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			u := newPrismTestUI()
			st := u.status
			st.SetMode(tc.purpose, tc.level)

			badge := st.modeBadge()
			for _, want := range tc.contains {
				require.Contains(t, badge, want)
			}
			if tc.empty {
				require.Empty(t, badge)
			}
		})
	}
}
