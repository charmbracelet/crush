package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
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

func TestFindSubagentByName(t *testing.T) {
	t.Parallel()

	active := []*subagents.Subagent{
		{Name: "alpha"},
		{Name: "beta"},
	}

	require.NotNil(t, findSubagentByName(active, "alpha"))
	require.Equal(t, "alpha", findSubagentByName(active, "alpha").Name)
	require.Equal(t, "beta", findSubagentByName(active, "beta").Name)
	require.Nil(t, findSubagentByName(active, "missing"))
	require.Nil(t, findSubagentByName(active, ""))
	require.Nil(t, findSubagentByName(nil, "alpha"))
}

// TestDispatcherTool_Run_UnknownSubagent_ReturnsErrorResponse exercises the
// dispatcher routing for a subagent_type not in the active list. The closure
// here mirrors the lookup performed by (*coordinator).agentTool.
func TestDispatcherTool_Run_UnknownSubagent_ReturnsErrorResponse(t *testing.T) {
	t.Parallel()

	active := []*subagents.Subagent{
		{Name: "code-reviewer", Description: "ok"},
	}

	dt := &dispatcherTool{
		info: buildAgentDispatchInfo(active),
		dispatch: func(_ context.Context, params AgentDispatchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sa := findSubagentByName(active, params.SubagentType)
			if sa == nil {
				return fantasy.NewTextErrorResponse("unknown subagent type: \"" + params.SubagentType + "\""), nil
			}
			return fantasy.NewTextResponse("would have run " + sa.Name), nil
		},
	}

	input, _ := json.Marshal(AgentDispatchParams{SubagentType: "imaginary", Prompt: "do thing"})
	resp, err := dt.Run(context.Background(), fantasy.ToolCall{Input: string(input)})

	require.NoError(t, err)
	require.True(t, resp.IsError)
}

// TestAgentTool_SubagentToolsCappedByOwner verifies that a custom subagent's
// tool pool is capped by the AllowedTools of the agent that dispatched it
// (the "owner" passed to agentTool), not always the coder's full tool set.
// In plan mode the plan agent's read-only tool pool owns the dispatcher, so
// its subagents must not inherit coder-only tools such as edit/write/bash.
//
// The dispatcher is built with the plan agent's config as owner. The custom
// subagent has no tools:/disallowed_tools: restrictions of its own, so its
// effective tool pool is exactly what ToConfigAgent copies from the owner.
// The subagent is actually dispatched end-to-end against a local fake
// OpenAI-compatible server so the real request payload's tool list — the
// ground truth for what the built agent could actually call — can be
// inspected directly, rather than trusting an intermediate config value.
func TestAgentTool_SubagentToolsCappedByOwner(t *testing.T) {
	t.Parallel()

	env := testEnv(t)

	var (
		mu        sync.Mutex
		sawTools  bool
		toolNames []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		_ = json.Unmarshal(body, &payload)
		if len(payload.Tools) > 0 {
			mu.Lock()
			if !sawTools {
				sawTools = true
				for _, tl := range payload.Tools {
					toolNames = append(toolNames, tl.Function.Name)
				}
			}
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"z\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"z\",\"created\":1,\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"z\",\"created\":1,\"model\":\"m\",\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	const (
		providerID = "test-openai-compat"
		modelID    = "test-model"
	)
	cfg.Config().Providers.Set(providerID, config.ProviderConfig{
		ID:      providerID,
		Name:    "Test",
		Type:    openaicompat.Name,
		BaseURL: srv.URL,
		APIKey:  "test",
		Models:  []catwalk.Model{{ID: modelID, DefaultMaxTokens: 4096}},
	})
	selected := config.SelectedModel{Provider: providerID, Model: modelID}
	cfg.Config().Models[config.SelectedModelTypeLarge] = selected
	cfg.Config().Models[config.SelectedModelTypeSmall] = selected
	cfg.SetupAgents()

	// Coder/task are irrelevant to this test; clear their AllowedTools like
	// newOfflineCoordinator does, keeping the run cheap.
	for _, agentID := range []string{config.AgentCoder, config.AgentTask} {
		a := cfg.Config().Agents[agentID]
		a.AllowedTools = nil
		cfg.Config().Agents[agentID] = a
	}

	c, err := NewCoordinator(t.Context(), CoordinatorOptions{
		Config:      cfg,
		Sessions:    env.sessions,
		Messages:    env.messages,
		Permissions: permission.NewPermissionService(env.workingDir, true, nil),
	})
	require.NoError(t, err)
	coord := c.(*coordinator)
	require.NoError(t, coord.readyWg.Wait())

	owner := cfg.Config().Agents[config.AgentPlan]
	require.Contains(t, owner.AllowedTools, AgentToolName,
		"precondition: the plan agent must retain the dispatcher tool")
	require.NotContains(t, owner.AllowedTools, "edit")
	require.NotContains(t, owner.AllowedTools, "write")
	require.NotContains(t, owner.AllowedTools, "bash")

	coord.activeSubagents = []*subagents.Subagent{
		{Name: "custom", Description: "a custom subagent with no tool restrictions of its own"},
	}

	tool, err := coord.agentTool(t.Context(), owner)
	require.NoError(t, err)
	dt := tool.(*dispatcherTool)

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	runCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx := context.WithValue(runCtx, tools.SessionIDContextKey, parentSession.ID)
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "msg-1")

	input, err := json.Marshal(AgentDispatchParams{SubagentType: "custom", Prompt: "do something"})
	require.NoError(t, err)

	_, err = dt.Run(ctx, fantasy.ToolCall{ID: "call-1", Input: string(input)})
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.True(t, sawTools, "expected at least one request carrying a tool list")
	require.NotContains(t, toolNames, "edit",
		"a subagent's tools must be capped by the dispatching (owner) agent's AllowedTools, not the coder's")
	require.NotContains(t, toolNames, "write")
	require.NotContains(t, toolNames, "bash")
}
