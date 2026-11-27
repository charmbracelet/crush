package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/sshaskpass"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type sshTouchWorkspace struct {
	workspace.Workspace
	responded *bool
}

func (w sshTouchWorkspace) SSHRespond(string, string) bool {
	*w.responded = true
	return true
}

func (sshTouchWorkspace) Config() *config.Config { return nil }

// TestSSHTouchWarningLastsUntilTouchLands verifies the security key touch
// warning is not answered or timed out early, and clears only when the
// prompt server reports that same touch resolved.
func TestSSHTouchWarningLastsUntilTouchLands(t *testing.T) {
	t.Parallel()
	ui := newTestUI()
	ui.dialog = dialog.NewOverlay()
	var responded bool
	ui.com.Workspace = sshTouchWorkspace{responded: &responded}

	ui.handleSSHPrompt(sshaskpass.PromptRequest{ID: "t", Prompt: "Confirm user presence for key ED25519-SK"})
	require.False(t, responded, "touch notices are never answered")
	require.NotEmpty(t, ui.status.msg.Msg)
	require.Greater(t, ui.status.msg.TTL, DefaultStatusTTL)

	ui.handleSSHNotification(sshaskpass.Notification{RequestID: "other"})
	require.NotEmpty(t, ui.status.msg.Msg, "another prompt resolving keeps the warning")

	ui.handleSSHNotification(sshaskpass.Notification{RequestID: "t"})
	require.Empty(t, ui.status.msg.Msg, "the touch landing clears the warning")
}
