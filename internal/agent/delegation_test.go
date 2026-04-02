package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDelegationPromptPrefixSkipsSubagentsAndMissingAgentTool(t *testing.T) {
	t.Parallel()

	base := "provider-prefix"

	noAgentTool := buildDelegationPromptPrefix(base, nil, false)
	assert.Equal(t, base, noAgentTool)

	withSubagent := buildDelegationPromptPrefix(base, []fantasy.AgentTool{testAgentTool()}, true)
	assert.Equal(t, base, withSubagent)
}

func TestBuildDelegationPromptPrefixAddsCostAwareDelegationPolicyForPrimaryAgent(t *testing.T) {
	t.Parallel()

	prefix := buildDelegationPromptPrefix("provider-prefix", []fantasy.AgentTool{testAgentTool()}, false)

	assert.Contains(t, prefix, "provider-prefix")
	assert.Contains(t, prefix, "Operate as an orchestrator first")
	assert.Contains(t, prefix, "prefer batching direct tool calls in parallel instead of spawning subagents")
	assert.Contains(t, prefix, "Do delegate review or change-inspection tasks")
	assert.Contains(t, prefix, "explore subagent")
	assert.Contains(t, prefix, "Do not spawn subagents for tiny file-local edits")
	assert.Contains(t, prefix, "For broad implementation requests, do the minimum shared setup")
}

func TestPromptForAgentUsesWorkerPromptForWritableSubagents(t *testing.T) {
	t.Parallel()

	promptBuilder, err := promptForAgent(config.Agent{ID: config.AgentCoder}, false)
	require.NoError(t, err)
	assert.Equal(t, "coder", promptBuilder.Name())

	promptBuilder, err = promptForAgent(config.Agent{ID: config.AgentGeneral, Role: "executor"}, true)
	require.NoError(t, err)
	assert.Equal(t, "general", promptBuilder.Name())

	promptBuilder, err = promptForAgent(config.Agent{ID: config.AgentExplore, Role: "reviewer"}, true)
	require.NoError(t, err)
	assert.Equal(t, "explore", promptBuilder.Name())

	promptBuilder, err = promptForAgent(config.Agent{
		ID:           "reviewer",
		Role:         "planner",
		Mode:         config.AgentModeSubagent,
		AllowedTools: []string{"bash", "view"},
	}, true)
	require.NoError(t, err)
	assert.Equal(t, "general", promptBuilder.Name())
}

func testAgentTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		AgentToolName,
		"delegates work",
		func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)
}
