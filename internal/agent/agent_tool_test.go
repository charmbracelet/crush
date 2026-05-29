package agent

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/require"
)

func TestBuildAgentDispatchInfo_NoSubagents(t *testing.T) {
	t.Parallel()

	info := buildAgentDispatchInfo(nil)

	require.Equal(t, "agent", info.Name)
	require.True(t, info.Parallel)
	require.Contains(t, info.Required, "prompt")

	subagentTypeParam, ok := info.Parameters["subagent_type"]
	require.True(t, ok, "Parameters should have a subagent_type key")

	paramMap, ok := subagentTypeParam.(map[string]any)
	require.True(t, ok, "subagent_type parameter should be a map[string]any")

	enum, ok := paramMap["enum"]
	require.True(t, ok, "subagent_type parameter should have an enum key")

	enumSlice, ok := enum.([]string)
	require.True(t, ok, "enum should be a []string")
	require.Contains(t, enumSlice, "task")
}

func TestBuildAgentDispatchInfo_WithSubagents(t *testing.T) {
	t.Parallel()

	activeSubagents := []*subagents.Subagent{
		{Name: "code-reviewer", Description: "Reviews code"},
		{Name: "tester", Description: "Writes tests"},
	}

	info := buildAgentDispatchInfo(activeSubagents)

	subagentTypeParam, ok := info.Parameters["subagent_type"]
	require.True(t, ok, "Parameters should have a subagent_type key")

	paramMap, ok := subagentTypeParam.(map[string]any)
	require.True(t, ok, "subagent_type parameter should be a map[string]any")

	enum, ok := paramMap["enum"]
	require.True(t, ok, "subagent_type parameter should have an enum key")

	enumSlice, ok := enum.([]string)
	require.True(t, ok, "enum should be a []string")
	require.Contains(t, enumSlice, "task")
	require.Contains(t, enumSlice, "code-reviewer")
	require.Contains(t, enumSlice, "tester")

	// subagent descriptions should appear in the subagent_type parameter description
	desc, ok := paramMap["description"]
	require.True(t, ok, "subagent_type parameter should have a description key")
	descStr, ok := desc.(string)
	require.True(t, ok, "description should be a string")
	require.Contains(t, descStr, "Reviews code")
	require.Contains(t, descStr, "Writes tests")
}

func TestBuildAgentDispatchInfo_PromptRequired(t *testing.T) {
	t.Parallel()

	info := buildAgentDispatchInfo(nil)

	require.Contains(t, info.Required, "prompt")

	// subagent_type is optional — should NOT appear in Required
	for _, r := range info.Required {
		require.NotEqual(t, "subagent_type", r, "subagent_type should not be required")
	}
}

// dispatcherTool tests — exercise the struct's Run and Info methods without a
// full coordinator. The dispatch closure is injected so no provider setup needed.

func TestDispatcherTool_Info_ReturnsBuildInfo(t *testing.T) {
	t.Parallel()

	info := buildAgentDispatchInfo([]*subagents.Subagent{{Name: "my-agent", Description: "Does stuff"}})
	dt := &dispatcherTool{info: info}

	got := dt.Info()
	require.Equal(t, "agent", got.Name)
	require.True(t, got.Parallel)
}

func TestDispatcherTool_Run_ParsesJSONAndCallsDispatch(t *testing.T) {
	t.Parallel()

	var capturedParams AgentDispatchParams
	dt := &dispatcherTool{
		info: buildAgentDispatchInfo(nil),
		dispatch: func(_ context.Context, params AgentDispatchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			capturedParams = params
			return fantasy.NewTextResponse("ok"), nil
		},
	}

	input, _ := json.Marshal(AgentDispatchParams{SubagentType: "my-agent", Prompt: "do the thing"})
	resp, err := dt.Run(context.Background(), fantasy.ToolCall{Input: string(input)})

	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Equal(t, "my-agent", capturedParams.SubagentType)
	require.Equal(t, "do the thing", capturedParams.Prompt)
}

func TestDispatcherTool_Run_InvalidJSON_ReturnsErrorResponse(t *testing.T) {
	t.Parallel()

	dt := &dispatcherTool{
		info: buildAgentDispatchInfo(nil),
		dispatch: func(_ context.Context, _ AgentDispatchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			t.Fatal("dispatch should not be called for invalid JSON")
			return fantasy.ToolResponse{}, nil
		},
	}

	resp, err := dt.Run(context.Background(), fantasy.ToolCall{Input: "not-valid-json{"})

	require.NoError(t, err) // errors are surfaced as error responses, not Go errors
	require.True(t, resp.IsError)
}

func TestDispatcherTool_Run_EmptySubagentType_RoutesToTask(t *testing.T) {
	t.Parallel()

	var capturedParams AgentDispatchParams
	dt := &dispatcherTool{
		info: buildAgentDispatchInfo(nil),
		dispatch: func(_ context.Context, params AgentDispatchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			capturedParams = params
			return fantasy.NewTextResponse("ok"), nil
		},
	}

	input, _ := json.Marshal(AgentDispatchParams{Prompt: "search for something"})
	_, err := dt.Run(context.Background(), fantasy.ToolCall{Input: string(input)})

	require.NoError(t, err)
	require.Empty(t, capturedParams.SubagentType) // dispatch receives params as-is; routing is in the closure
}

func TestDispatcherTool_ProviderOptions_RoundTrip(t *testing.T) {
	t.Parallel()

	dt := &dispatcherTool{info: buildAgentDispatchInfo(nil)}
	require.Nil(t, dt.ProviderOptions())

	opts := fantasy.ProviderOptions{}
	dt.SetProviderOptions(opts)
	require.NotNil(t, dt.ProviderOptions())
}

func TestSubagentBodyPrompt_ReturnsLiteralBody(t *testing.T) {
	t.Parallel()

	body := "# Agent\n\nYou are a specialist.\n\n{{.Provider}} should not be expanded."
	p := subagentBodyPrompt(body)

	result, err := p.Build(context.Background(), "anthropic", "claude-3", nil)
	require.NoError(t, err)
	require.Equal(t, body, result) // body returned verbatim, template metacharacters untouched
}
