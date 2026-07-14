package agent

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

type testDeferredToolProvider struct {
	fantasy.AgentTool
	resolved []fantasy.AgentTool
}

func (t *testDeferredToolProvider) ResolveDeferredTools(names []string) []fantasy.AgentTool {
	return slices.Clone(t.resolved)
}

func (t *testDeferredToolProvider) ResolveDeferredToolSearch(tools.MCPToolSearchParams) []fantasy.AgentTool {
	return slices.Clone(t.resolved)
}

func TestDeferredToolSelectionsParsesExactSearchCalls(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{{
		Response: fantasy.Response{Content: fantasy.ResponseContent{
			fantasy.ToolCallContent{
				ToolName: "mcp_tool_search",
				Input:    `{"query":"select:mcp_github_get_me,mcp_memory_search"}`,
			},
		}},
	}}

	require.Equal(t, []string{"mcp_github_get_me", "mcp_memory_search"}, deferredToolSelections(steps))
}

func TestActivateDeferredToolsAddsResolvedNativeToolOnce(t *testing.T) {
	t.Parallel()

	native := fantasy.NewAgentTool(
		"mcp_github_get_me",
		"Get the authenticated GitHub user.",
		func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)
	search := &testDeferredToolProvider{
		AgentTool: fantasy.NewAgentTool(
			"mcp_tool_search",
			"Search tools.",
			func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				return fantasy.NewTextResponse("ok"), nil
			},
		),
		resolved: []fantasy.AgentTool{native, native},
	}
	steps := []fantasy.StepResult{{
		Response: fantasy.Response{Content: fantasy.ResponseContent{
			fantasy.ToolCallContent{
				ToolName: "mcp_tool_search",
				Input:    `{"query":"select:mcp_github_get_me"}`,
			},
		}},
	}}

	result := activateDeferredTools([]fantasy.AgentTool{search}, steps)

	require.Equal(t, []string{"mcp_github_get_me", "mcp_tool_search"}, agentToolNames(result))
}

func TestActivateDeferredToolsAddsKeywordSearchMatchesWithoutSelection(t *testing.T) {
	t.Parallel()

	native := fantasy.NewAgentTool(
		"mcp_github_search_repositories",
		"Search GitHub repositories.",
		func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)
	search := &testDeferredToolProvider{
		AgentTool: fantasy.NewAgentTool(
			"mcp_tool_search",
			"Search tools.",
			func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				return fantasy.NewTextResponse("ok"), nil
			},
		),
		resolved: []fantasy.AgentTool{native},
	}
	steps := []fantasy.StepResult{{
		Response: fantasy.Response{Content: fantasy.ResponseContent{
			fantasy.ToolCallContent{
				ToolName: "mcp_tool_search",
				Input:    `{"query":"find GitHub repositories"}`,
			},
		}},
	}}

	result := activateDeferredTools([]fantasy.AgentTool{search}, steps)

	require.Equal(t, []string{"mcp_github_search_repositories", "mcp_tool_search"}, agentToolNames(result))
}

func TestActivateDeferredToolsPreservesBaseToolsForUnknownSelection(t *testing.T) {
	t.Parallel()

	search := &testDeferredToolProvider{
		AgentTool: fantasy.NewAgentTool(
			"mcp_tool_search",
			"Search tools.",
			func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				return fantasy.NewTextResponse("ok"), nil
			},
		),
	}
	view := fantasy.NewAgentTool(
		"view",
		"View a file.",
		func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)
	steps := []fantasy.StepResult{{
		Response: fantasy.Response{Content: fantasy.ResponseContent{
			fantasy.ToolCallContent{
				ToolName: "mcp_tool_search",
				Input:    `{"query":"select:mcp_github_invented_tool"}`,
			},
		}},
	}}

	result := activateDeferredTools([]fantasy.AgentTool{search, view}, steps)

	require.Equal(t, []string{"mcp_tool_search", "view"}, agentToolNames(result))
}

func TestActivateDeferredToolsResetsWhenNewTurnHasNoDiscoverySteps(t *testing.T) {
	t.Parallel()

	search := &testDeferredToolProvider{
		AgentTool: fantasy.NewAgentTool(
			"mcp_tool_search",
			"Search tools.",
			func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				return fantasy.NewTextResponse("ok"), nil
			},
		),
		resolved: []fantasy.AgentTool{fantasy.NewAgentTool(
			"mcp_github_list_branches",
			"List branches.",
			func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				return fantasy.NewTextResponse("ok"), nil
			},
		)},
	}

	result := activateDeferredTools([]fantasy.AgentTool{search}, nil)

	require.Equal(t, []string{"mcp_tool_search"}, agentToolNames(result))
}

type deferredToolLifecycleModel struct {
	autoReviewStreamModel
	calls    atomic.Int64
	mu       sync.Mutex
	toolSets [][]string
}

func (m *deferredToolLifecycleModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	callNumber := m.calls.Add(1)
	toolNames := make([]string, 0, len(call.Tools))
	for _, tool := range call.Tools {
		toolNames = append(toolNames, tool.GetName())
	}
	slices.Sort(toolNames)
	m.mu.Lock()
	m.toolSets = append(m.toolSets, toolNames)
	m.mu.Unlock()

	return func(yield func(fantasy.StreamPart) bool) {
		switch callNumber {
		case 1:
			yieldToolCall(yield, "search-1", "mcp_tool_search", `{"query":"authenticated GitHub user"}`)
		case 2:
			yieldToolCall(yield, "native-1", "mcp_github_get_me", `{}`)
		default:
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "done"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "done", Delta: "verified"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "done"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
		}
	}, nil
}

func yieldToolCall(yield func(fantasy.StreamPart) bool, id, name, input string) {
	yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: id, ToolCallName: name})
	yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputDelta, ID: id, Delta: input})
	yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: id})
	yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: id, ToolCallName: name, ToolCallInput: input})
	yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
}

func (m *deferredToolLifecycleModel) toolsForCall(index int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.toolSets[index])
}

func TestRunActivatesAndExecutesMatchedDeferredTool(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	model := &deferredToolLifecycleModel{}
	titleModel := &autoReviewStreamModel{text: "deferred tools"}
	modelConfig := catwalk.Model{ContextWindow: 200_000, DefaultMaxTokens: 10_000}
	configured := Model{
		Model:      model,
		CatwalkCfg: modelConfig,
		ModelCfg:   config.SelectedModel{Provider: model.Provider(), Model: model.Model()},
	}
	title := Model{
		Model:      titleModel,
		CatwalkCfg: modelConfig,
		ModelCfg:   config.SelectedModel{Provider: titleModel.Provider(), Model: titleModel.Model()},
	}
	var nativeCalls atomic.Int64
	native := fantasy.NewAgentTool(
		"mcp_github_get_me",
		"Get the authenticated GitHub user.",
		func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			nativeCalls.Add(1)
			return fantasy.NewTextResponse("octocat"), nil
		},
	)
	search := &testDeferredToolProvider{
		AgentTool: fantasy.NewAgentTool(
			"mcp_tool_search",
			"Search deferred tools.",
			func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
				return fantasy.NewTextResponse("selected"), nil
			},
		),
		resolved: []fantasy.AgentTool{native},
	}
	agent := NewSessionAgent(SessionAgentOptions{
		Models:       SessionAgentModels{Large: configured, Small: title, Summary: configured},
		SystemPrompt: "system",
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
		Tools:        []fantasy.AgentTool{search},
	}).(*sessionAgent)
	session, err := env.sessions.Create(t.Context(), "deferred tools")
	require.NoError(t, err)

	result, err := agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "Who am I on GitHub?"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(3), model.calls.Load())
	require.Equal(t, int64(1), nativeCalls.Load())
	require.Equal(t, []string{"mcp_tool_search"}, model.toolsForCall(0))
	require.Equal(t, []string{"mcp_github_get_me", "mcp_tool_search"}, model.toolsForCall(1))
	require.Equal(t, []string{"mcp_github_get_me", "mcp_tool_search"}, model.toolsForCall(2))
}
