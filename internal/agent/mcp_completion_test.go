package agent

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestMCPCompletionEvidenceRejectsFileOnlySuccessClaim(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("write", `{"file_path":"C:\\Users\\admin\\.config\\crush\\crush.json"}`, "wrote file"),
		makeToolStep("mcp_refresh", `{"all":true}`, "other-server: connected"),
	}
	call := SessionAgentCall{Prompt: "Add and configure the requested-server MCP server"}
	require.True(t, needsMCPCompletionEvidence(call, steps, "The requested-server MCP was added and now appears in the MCP list."))
}

func TestMCPCompletionEvidenceDoesNotOverrideNegativeUserIntent(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_refresh", `{"name":"unrequested-server"}`, "unrequested-server: connected"),
	}
	call := SessionAgentCall{Prompt: "Do not install or add any MCP server"}
	require.False(t, needsMCPCompletionEvidence(call, steps, "The unrequested-server MCP installed and connected."))
}

func TestMCPCompletionEvidencePreservesPositiveIntentWithExclusion(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_refresh", `{"all":true}`, "requested-server: connected"),
	}
	call := SessionAgentCall{Prompt: "Install the requested-server MCP, but do not add any other MCP"}
	require.True(t, needsMCPCompletionEvidence(call, steps, "The requested-server MCP installed and connected."))
}

func TestMCPCompletionEvidenceAcceptsNamedConnectedResult(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_refresh", `{"name":"requested-server"}`, "requested-server: connected"),
	}
	call := SessionAgentCall{Prompt: "Install the requested-server MCP server"}
	require.False(t, needsMCPCompletionEvidence(call, steps, "The requested-server MCP installed and connected."))
}

func TestMCPCompletionEvidenceAcceptsEnsureConnectedResult(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_add", `{"name":"requested-server","stdio":{"command":"server"}}`, "requested-server: connected; config=reused"),
	}
	call := SessionAgentCall{Prompt: "Install the requested-server MCP server"}
	require.False(t, needsMCPCompletionEvidence(call, steps, "The requested-server MCP installed and connected."))
}

func TestMCPCompletionEvidenceRejectsUnrelatedNamedRefresh(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_refresh", `{"name":"other-server"}`, "other-server: connected"),
	}
	call := SessionAgentCall{Prompt: "Install the requested-server MCP server"}
	require.True(t, needsMCPCompletionEvidence(call, steps, "The requested-server MCP installed and connected."))
}

func TestMCPCompletionEvidenceRejectsPartialNameMatch(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_add", `{"name":"git","stdio":{"command":"server"}}`, "git: connected; config=reused"),
	}
	call := SessionAgentCall{Prompt: "Install the github MCP server"}
	require.True(t, needsMCPCompletionEvidence(call, steps, "The github MCP installed and connected."))
}

func TestMCPCompletionEvidenceAllowsHonestFailure(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_refresh", `{"name":"requested-server"}`, "requested-server: error: executable not found"),
	}
	call := SessionAgentCall{Prompt: "Enable the requested-server MCP server"}
	require.False(t, needsMCPCompletionEvidence(call, steps, "The requested-server MCP could not be initialized because its executable was unavailable."))
}

func TestMCPCompletionEvidenceAllowsExplicitNotConnectedReport(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_refresh", `{"name":"requested-server"}`, "requested-server: error: executable not found"),
	}
	call := SessionAgentCall{Prompt: "Enable the requested-server MCP server"}
	require.False(t, needsMCPCompletionEvidence(call, steps, "The requested-server MCP is not connected."))
}

func TestMCPCompletionEvidenceRecognizesRecoveredSuccess(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_refresh", `{"all":true}`, "requested-server: connected"),
	}
	call := SessionAgentCall{Prompt: "Enable the requested-server MCP server"}
	require.True(t, needsMCPCompletionEvidence(call, steps, "The first MCP refresh failed, but the requested-server MCP is now connected."))
}

func TestMCPCompletionEvidenceIsBounded(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{makeToolStep("mcp_refresh", `{"all":true}`, "other-server: connected")}
	call := SessionAgentCall{Prompt: "Add the requested-server MCP server", mcpCompletionCheck: true}
	require.False(t, needsMCPCompletionEvidence(call, steps, "The requested-server MCP was added successfully."))
}
