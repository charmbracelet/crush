package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

// orcaTitleStatus mirrors the part of Orca's title classifier
// (computeAgentStatusFromTitle) that needs no known agent name: those are the
// signals a terminal-side tracker can read from Crush today.
func orcaTitleStatus(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return ""
	}
	r := []rune(trimmed)[0]
	switch {
	case r >= 0x2800 && r <= 0x28FF: // braille
		return "working"
	case r >= 0x25D0 && r <= 0x25D3: // quarter circles
		return "working"
	case r == '✋':
		return "permission"
	case strings.HasPrefix(trimmed, "✳ "):
		return "idle"
	}
	return "unknown"
}

func TestWindowTitleSignalsAgentStatus(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.com.Workspace = &testWorkspace{cfg: &config.Config{}}
	u.dialog = dialog.NewOverlay()

	// Idle: no run in flight.
	require.Equal(t, "idle", orcaTitleStatus(u.windowTitle()), u.windowTitle())
	require.Contains(t, u.windowTitle(), "crush")

	// Working: a run is in flight. The frame advances with each animation
	// tick so the title stays a moving spinner.
	u.agentBusyCache.val = true
	busy := u.windowTitle()
	require.Equal(t, "working", orcaTitleStatus(busy), busy)

	u.titleFrame++
	require.NotEqual(t, busy, u.windowTitle(), "the working title should animate")
	require.Equal(t, "working", orcaTitleStatus(u.windowTitle()))

	// A permission prompt outranks the generic working state.
	u.dialog.OpenDialogWithGrace(dialog.NewPermissions(u.com, permission.PermissionRequest{
		ID:         "perm-title",
		ToolCallID: "tool-call-title",
		ToolName:   "bash",
	}))
	blocked := u.windowTitle()
	require.Equal(t, "permission", orcaTitleStatus(blocked), blocked)
}

func TestWindowTitleAlwaysNamesCrush(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.com.Workspace = &testWorkspace{cfg: &config.Config{}}
	u.dialog = dialog.NewOverlay()

	for _, busy := range []bool{false, true} {
		u.agentBusyCache.val = busy
		require.Contains(t, u.windowTitle(), "crush",
			"every title must stay attributable to crush (busy=%v)", busy)
	}
}
