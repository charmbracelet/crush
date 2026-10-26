package agent

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	agentToolTestProviderID = "test-openai-compat"
	agentToolTestModelID    = "test-model"
)

// newAgentToolTestCoordinator builds a coordinator whose selected model
// resolves offline (an openai-compatible provider pointed at an
// unroutable address), so the agent tool and its sub-agents can be
// constructed without network I/O.
func newAgentToolTestCoordinator(t *testing.T, env fakeEnv) *coordinator {
	t.Helper()

	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Providers.Set(agentToolTestProviderID, config.ProviderConfig{
		ID:      agentToolTestProviderID,
		Name:    "Test",
		Type:    openaicompat.Name,
		BaseURL: "http://127.0.0.1:0/v1",
		APIKey:  "test",
		Models:  []catwalk.Model{{ID: agentToolTestModelID, DefaultMaxTokens: 4096}},
	})
	selected := config.SelectedModel{Provider: agentToolTestProviderID, Model: agentToolTestModelID}
	cfg.OverridePreferredModel(config.SelectedModelTypeLarge, selected)
	cfg.OverridePreferredModel(config.SelectedModelTypeSmall, selected)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
	}
	require.NoError(t, coord.rebuildModels(t.Context()))
	return coord
}

func agentToolTestContext(sessionID string) context.Context {
	ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, sessionID)
	return context.WithValue(ctx, tools.MessageIDContextKey, "agent-message-1")
}

func agentToolCall(input string) fantasy.ToolCall {
	return fantasy.ToolCall{ID: "call-1", Name: AgentToolName, Input: input}
}

func mustMarshalParams(t *testing.T, params AgentParams) string {
	t.Helper()
	data, err := json.Marshal(params)
	require.NoError(t, err)
	return string(data)
}

func TestAgentToolValidation(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := newAgentToolTestCoordinator(t, env)

	tool, err := coord.agentTool(t.Context())
	require.NoError(t, err)
	require.Equal(t, AgentToolName, tool.Info().Name)

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)
	ctx := agentToolTestContext(parentSession.ID)

	t.Run("prompt is required", func(t *testing.T) {
		t.Parallel()
		resp, err := tool.Run(ctx, agentToolCall(mustMarshalParams(t, AgentParams{})))
		require.NoError(t, err)
		assert.True(t, resp.IsError)
		assert.Contains(t, resp.Content, "prompt is required")
	})

	t.Run("session id is required", func(t *testing.T) {
		t.Parallel()
		_, err := tool.Run(context.Background(), agentToolCall(mustMarshalParams(t, AgentParams{Prompt: "do something"})))
		require.ErrorContains(t, err, "session id missing from context")
	})

	t.Run("provider requires a model", func(t *testing.T) {
		t.Parallel()
		resp, err := tool.Run(ctx, agentToolCall(mustMarshalParams(t, AgentParams{
			Prompt:   "do something",
			Provider: agentToolTestProviderID,
		})))
		require.NoError(t, err)
		assert.True(t, resp.IsError)
		assert.Contains(t, resp.Content, "requires a model")
	})
}

// TestAgentToolRunsWriteSubAgent exercises the write-tools selection and
// the model override path end to end: the write agent's cache entry is
// pre-seeded with a mock, so the tool runs it instead of a real model.
func TestAgentToolRunsWriteSubAgent(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := newAgentToolTestCoordinator(t, env)

	mock := newMockAgent(agentToolTestProviderID, 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		assert.Equal(t, "write a poem to disk", call.Prompt)
		assert.Equal(t, int64(4096), call.MaxOutputTokens)
		return agentResultWithText("done"), nil
	})
	coord.subAgentCache = map[subAgentKey]SessionAgent{
		{Write: true, Provider: agentToolTestProviderID, Model: agentToolTestModelID}: mock,
	}

	tool, err := coord.agentTool(t.Context())
	require.NoError(t, err)

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	resp, err := tool.Run(agentToolTestContext(parentSession.ID), agentToolCall(mustMarshalParams(t, AgentParams{
		Prompt:     "write a poem to disk",
		WriteTools: true,
		Provider:   agentToolTestProviderID,
		Model:      agentToolTestModelID,
	})))
	require.NoError(t, err)
	assert.False(t, resp.IsError)
	assert.Equal(t, "done", resp.Content)
}

func TestOverrideSubAgent(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := newAgentToolTestCoordinator(t, env)
	tpl, err := taskPrompt()
	require.NoError(t, err)

	readCfg := coord.cfg.Config().Agents[config.AgentTask]
	writeCfg := coord.cfg.Config().Agents[config.AgentTaskWrite]

	t.Run("builds agent with the requested model", func(t *testing.T) {
		t.Parallel()
		agent, err := coord.overrideSubAgent(t.Context(), tpl, readCfg, agentToolTestProviderID, agentToolTestModelID)
		require.NoError(t, err)
		model := agent.Model()
		assert.Equal(t, agentToolTestProviderID, model.ModelCfg.Provider)
		assert.Equal(t, agentToolTestModelID, model.ModelCfg.Model)
	})

	t.Run("caches per provider, model, and write flag", func(t *testing.T) {
		t.Parallel()
		readAgent, err := coord.overrideSubAgent(t.Context(), tpl, readCfg, agentToolTestProviderID, agentToolTestModelID)
		require.NoError(t, err)
		readAgentAgain, err := coord.overrideSubAgent(t.Context(), tpl, readCfg, agentToolTestProviderID, agentToolTestModelID)
		require.NoError(t, err)
		assert.Equal(t, readAgent, readAgentAgain)

		_, err = coord.overrideSubAgent(t.Context(), tpl, writeCfg, agentToolTestProviderID, agentToolTestModelID)
		require.NoError(t, err)

		// Only two distinct keys exist across this test's subtests, and
		// this subtest built both of them, so the cache must hold exactly
		// those two regardless of subtest scheduling.
		coord.subAgentMu.Lock()
		defer coord.subAgentMu.Unlock()
		_, hasRead := coord.subAgentCache[subAgentKey{Write: false, Provider: agentToolTestProviderID, Model: agentToolTestModelID}]
		_, hasWrite := coord.subAgentCache[subAgentKey{Write: true, Provider: agentToolTestProviderID, Model: agentToolTestModelID}]
		assert.True(t, hasRead)
		assert.True(t, hasWrite)
		assert.Len(t, coord.subAgentCache, 2)
	})

	t.Run("model defaults to the selected provider", func(t *testing.T) {
		t.Parallel()
		providerID := coord.subAgentModels().Large.ModelCfg.Provider
		assert.Equal(t, agentToolTestProviderID, providerID)

		agent, err := coord.overrideSubAgent(t.Context(), tpl, readCfg, "", agentToolTestModelID)
		require.NoError(t, err)
		assert.Equal(t, agentToolTestProviderID, agent.Model().ModelCfg.Provider)
	})

	t.Run("provider without model fails", func(t *testing.T) {
		t.Parallel()
		_, err := coord.overrideSubAgent(t.Context(), tpl, readCfg, agentToolTestProviderID, "")
		require.ErrorContains(t, err, "requires a model")
	})

	t.Run("unknown model fails", func(t *testing.T) {
		t.Parallel()
		_, err := coord.overrideSubAgent(t.Context(), tpl, readCfg, agentToolTestProviderID, "no-such-model")
		require.ErrorContains(t, err, "no-such-model")
	})

	t.Run("unconfigured provider fails", func(t *testing.T) {
		t.Parallel()
		_, err := coord.overrideSubAgent(t.Context(), tpl, readCfg, "not-a-provider", agentToolTestModelID)
		require.ErrorContains(t, err, "not-a-provider")
	})

	t.Run("disabled provider fails", func(t *testing.T) {
		t.Parallel()
		coord.cfg.Config().Providers.Set("disabled-provider", config.ProviderConfig{
			ID:      "disabled-provider",
			Name:    "Disabled",
			Type:    openaicompat.Name,
			BaseURL: "http://127.0.0.1:0/v1",
			APIKey:  "test",
			Disable: true,
			Models:  []catwalk.Model{{ID: agentToolTestModelID, DefaultMaxTokens: 4096}},
		})
		_, err := coord.overrideSubAgent(t.Context(), tpl, readCfg, "disabled-provider", agentToolTestModelID)
		require.ErrorContains(t, err, "disabled-provider")
	})
}

// TestRebuildModelsClearsOverrideCache pins the invalidation contract:
// override agents hold models built from one config snapshot, so a
// rebuild must drop them and advance the generation.
func TestRebuildModelsClearsOverrideCache(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := newAgentToolTestCoordinator(t, env)
	tpl, err := taskPrompt()
	require.NoError(t, err)
	readCfg := coord.cfg.Config().Agents[config.AgentTask]

	_, err = coord.overrideSubAgent(t.Context(), tpl, readCfg, agentToolTestProviderID, agentToolTestModelID)
	require.NoError(t, err)

	coord.subAgentMu.Lock()
	require.Len(t, coord.subAgentCache, 1)
	gen := coord.subAgentGen
	coord.subAgentMu.Unlock()

	require.NoError(t, coord.rebuildModels(t.Context()))

	coord.subAgentMu.Lock()
	defer coord.subAgentMu.Unlock()
	assert.Empty(t, coord.subAgentCache)
	assert.Greater(t, coord.subAgentGen, gen)
}

// TestOverrideSubAgentInheritsThink ensures overriding the model keeps
// the user's explicit thinking preference from their selected model.
func TestOverrideSubAgentInheritsThink(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	coord := newAgentToolTestCoordinator(t, env)
	tpl, err := taskPrompt()
	require.NoError(t, err)
	readCfg := coord.cfg.Config().Agents[config.AgentTask]

	modelCfg := coord.cfg.Config().Models[config.SelectedModelTypeLarge]
	modelCfg.Think = true
	coord.cfg.Config().Models[config.SelectedModelTypeLarge] = modelCfg

	agent, err := coord.overrideSubAgent(t.Context(), tpl, readCfg, agentToolTestProviderID, agentToolTestModelID)
	require.NoError(t, err)
	assert.True(t, agent.Model().ModelCfg.Think)
}
