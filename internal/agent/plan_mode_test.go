package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestCollaborationModePrompt(t *testing.T) {
	t.Parallel()

	prompt := collaborationModePrompt(session.CollaborationModePlan)
	require.Contains(t, prompt, "Plan Mode")
	require.Contains(t, prompt, "request_user_input")
	require.Contains(t, prompt, "<proposed_plan>")
	require.Contains(t, prompt, "Do not write files")
	require.Empty(t, collaborationModePrompt(session.CollaborationModeDefault))
}
