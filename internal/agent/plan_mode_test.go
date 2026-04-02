package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestCollaborationModePrompt(t *testing.T) {
	t.Parallel()

	autoPrompt := permissionModePrompt(session.PermissionModeAuto)
	require.Contains(t, autoPrompt, "Auto Mode")
	require.Contains(t, autoPrompt, "Minimize interruptions")

	prompt := collaborationModePrompt(session.CollaborationModePlan)
	require.Contains(t, prompt, "Plan Mode")
	require.Contains(t, prompt, "request_user_input")
	require.Contains(t, prompt, "plan_exit")
	require.Contains(t, prompt, "<proposed_plan>")
	require.Contains(t, prompt, "Do not write files")

	defaultPrompt := collaborationModePrompt(session.CollaborationModeDefault)
	require.Empty(t, defaultPrompt)
}

func TestBuildSystemPromptForCollaborationMode(t *testing.T) {
	t.Parallel()

	base := "Base system prompt."

	defaultPrompt := buildSystemPromptForModes(base, session.CollaborationModeDefault, session.PermissionModeDefault)
	require.Contains(t, defaultPrompt, base)
	require.Contains(t, defaultPrompt, "Auto Mode is not active")

	autoPrompt := buildSystemPromptForModes(base, session.CollaborationModeDefault, session.PermissionModeAuto)
	require.Contains(t, autoPrompt, base)
	require.Contains(t, autoPrompt, "You are in Auto Mode.")

	planPrompt := buildSystemPromptForModes(base, session.CollaborationModePlan, session.PermissionModeDefault)
	require.Contains(t, planPrompt, base)
	require.Contains(t, planPrompt, "You are in Plan Mode.")
}

func TestRiskLevelForTool(t *testing.T) {
	t.Parallel()

	require.Equal(t, toolRiskDelegation, riskLevelForTool(AgentToolName))
	require.Equal(t, toolRiskNetwork, riskLevelForTool(tools.AgenticFetchToolName))
	require.Equal(t, toolRiskRead, riskLevelForTool(tools.ViewToolName))
	require.Equal(t, toolRiskWrite, riskLevelForTool(tools.WriteToolName))
	require.Equal(t, toolRiskWrite, riskLevelForTool(tools.LongTermMemoryToolName))
	require.Equal(t, toolRiskExecute, riskLevelForTool(tools.BashToolName))
	require.Equal(t, toolRiskExecute, riskLevelForTool("unknown_tool"))
}

func TestFilterToolsForRiskPolicy(t *testing.T) {
	t.Parallel()

	baseTools := []string{
		tools.ViewToolName,
		tools.BashToolName,
		tools.LongTermMemoryToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
		tools.ViewToolName,
	}

	require.Equal(t, []string{
		tools.ViewToolName,
		tools.BashToolName,
		tools.LongTermMemoryToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
	}, filterToolsForRiskPolicy(baseTools, session.CollaborationModeDefault, nil))

	require.Equal(t, []string{
		tools.ViewToolName,
		tools.LongTermMemoryToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
	}, filterToolsForRiskPolicy(baseTools, session.CollaborationModeDefault, []string{tools.BashToolName}))

	require.Equal(t, []string{
		tools.ViewToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
	}, filterToolsForRiskPolicy(baseTools, session.CollaborationModePlan, []string{tools.ViewToolName}))

	require.Equal(t, []string{
		tools.ViewToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
	}, filterToolsForRiskPolicy([]string{AgentToolName, tools.AgenticFetchToolName, tools.ViewToolName}, session.CollaborationModePlan, nil))

	require.Equal(t, []string{
		tools.LSPDefinitionToolName,
		tools.LSPHoverToolName,
		tools.LSPDocumentSymbolsToolName,
		tools.LSPWorkspaceSymbolsToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
	}, filterToolsForRiskPolicy([]string{
		tools.LSPDefinitionToolName,
		tools.LSPHoverToolName,
		tools.LSPDocumentSymbolsToolName,
		tools.LSPWorkspaceSymbolsToolName,
	}, session.CollaborationModePlan, nil))
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
		tools.AgenticFetchToolName,
		tools.EditToolName,
		tools.MultiEditToolName,
		tools.WriteToolName,
		tools.HistorySearchToolName,
		tools.LongTermMemoryToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
		tools.DiagnosticsToolName,
		tools.ReferencesToolName,
		tools.LSPDefinitionToolName,
		tools.LSPHoverToolName,
		tools.LSPDocumentSymbolsToolName,
		tools.LSPWorkspaceSymbolsToolName,
		tools.ListMCPResourcesToolName,
		tools.ReadMCPResourceToolName,
		tools.SourcegraphToolName,
	}

	require.Equal(t, []string{
		AgentToolName,
		"bash",
		"grep",
		"ls",
		"view",
		tools.GlobToolName,
		tools.FetchToolName,
		tools.AgenticFetchToolName,
		tools.EditToolName,
		tools.MultiEditToolName,
		tools.WriteToolName,
		tools.HistorySearchToolName,
		tools.LongTermMemoryToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
		tools.DiagnosticsToolName,
		tools.ReferencesToolName,
		tools.LSPDefinitionToolName,
		tools.LSPHoverToolName,
		tools.LSPDocumentSymbolsToolName,
		tools.LSPWorkspaceSymbolsToolName,
		tools.ListMCPResourcesToolName,
		tools.ReadMCPResourceToolName,
		tools.SourcegraphToolName,
	}, filterToolsForCollaborationMode(baseTools, session.CollaborationModeDefault))

	require.Equal(t, []string{
		"grep",
		"ls",
		"view",
		tools.GlobToolName,
		tools.HistorySearchToolName,
		tools.RequestUserInputToolName,
		tools.PlanExitToolName,
		tools.DiagnosticsToolName,
		tools.ReferencesToolName,
		tools.LSPDefinitionToolName,
		tools.LSPHoverToolName,
		tools.LSPDocumentSymbolsToolName,
		tools.LSPWorkspaceSymbolsToolName,
	}, filterToolsForCollaborationMode(baseTools, session.CollaborationModePlan))
}
