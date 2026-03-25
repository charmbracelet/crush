package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestCollaborationModePrompt(t *testing.T) {
	t.Parallel()

	autoPrompt := collaborationModePrompt(session.CollaborationModeAuto)
	require.Contains(t, autoPrompt, "Auto Mode")
	require.Contains(t, autoPrompt, "Minimize interruptions")

	prompt := collaborationModePrompt(session.CollaborationModePlan)
	require.Contains(t, prompt, "Plan Mode")
	require.Contains(t, prompt, "request_user_input")
	require.Contains(t, prompt, "plan_exit")
	require.Contains(t, prompt, "<proposed_plan>")
	require.Contains(t, prompt, "Do not write files")

	defaultPrompt := collaborationModePrompt(session.CollaborationModeDefault)
	require.Contains(t, defaultPrompt, "Auto Mode is not active")
}

func TestBuildSystemPromptForCollaborationMode(t *testing.T) {
	t.Parallel()

	base := "Base system prompt."

	defaultPrompt := buildSystemPromptForCollaborationMode(base, session.CollaborationModeDefault)
	require.Contains(t, defaultPrompt, base)
	require.Contains(t, defaultPrompt, "Auto Mode is not active")

	autoPrompt := buildSystemPromptForCollaborationMode(base, session.CollaborationModeAuto)
	require.Contains(t, autoPrompt, base)
	require.Contains(t, autoPrompt, "You are in Auto Mode.")

	planPrompt := buildSystemPromptForCollaborationMode(base, session.CollaborationModePlan)
	require.Contains(t, planPrompt, base)
	require.Contains(t, planPrompt, "You are in Plan Mode.")
}

func TestFilterToolsForCollaborationMode(t *testing.T) {
	t.Parallel()

	baseTools := []string{
		AgentToolName,
		"bash",
		"grep",
		"ls",
		"view",
		tools.GlobToolName,
		tools.FetchToolName,
		tools.EditToolName,
		tools.MultiEditToolName,
		tools.WriteToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
		tools.DiagnosticsToolName,
		tools.ReferencesToolName,
		tools.ListMCPResourcesToolName,
		tools.ReadMCPResourceToolName,
		tools.SourcegraphToolName,
	}

	require.Equal(t, baseTools, filterToolsForCollaborationMode(baseTools, session.CollaborationModeDefault))
	require.Equal(t, []string{
		"bash",
		"grep",
		"ls",
		"view",
		tools.GlobToolName,
		tools.FetchToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
		tools.DiagnosticsToolName,
		tools.ReferencesToolName,
		tools.ListMCPResourcesToolName,
		tools.ReadMCPResourceToolName,
		tools.SourcegraphToolName,
	}, filterToolsForCollaborationMode(baseTools, session.CollaborationModePlan))
}
