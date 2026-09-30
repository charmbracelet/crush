package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newApplySubagentsTestCoordinator builds a coordinator with
// RouterOptions.ApplySubagents enabled, a router pointed at routerBaseURL,
// and (when poolModelID is non-empty) a model pool of one OpenRouter model
// whose reasoning levels are poolModelReasoningLevels — enough for
// runSubAgent's tier-1 (fresh, prompt-scoped) router call to resolve both
// an effort and a model_choice override.
func newApplySubagentsTestCoordinator(t *testing.T, routerBaseURL, poolModelID string, poolModelReasoningLevels []string) *coordinator {
	t.Helper()

	env := testEnv(t)
	levelsJSON, err := json.Marshal(poolModelReasoningLevels)
	require.NoError(t, err)

	modelPoolJSON, providersJSON := `[]`, ""
	if poolModelID != "" {
		modelPoolJSON = fmt.Sprintf("[%q]", poolModelID)
		providersJSON = fmt.Sprintf(`,
    "openrouter": {"id": "openrouter", "name": "OpenRouter", "type": "openrouter",
      "base_url": "http://127.0.0.1:9", "api_key": "test-key",
      "models": [{"id": %q, "name": "Pool Model", "context_window": 200000, "default_max_tokens": 4096, "reasoning_levels": %s}]}`,
			poolModelID, string(levelsJSON))
	}

	crushJSON := fmt.Sprintf(`{
  "options": {
    "disable_default_providers": true,
    "disable_provider_auto_update": true,
    "router": {"enabled": true, "apply_subagents": true, "provider": "local", "base_url": %q, "model": "test-model", "model_pool": %s}
  },
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}%s},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`, routerBaseURL, modelPoolJSON, providersJSON)
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	return &coordinator{
		cfg:      cfg,
		sessions: env.sessions,
		messages: env.messages,
	}
}

func TestRouterSubAgentEffortFromContext_DefaultsToEmpty(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", RouterSubAgentEffortFromContext(t.Context()))
}

func TestRouterSubAgentEffortFromContext_RoundTrips(t *testing.T) {
	t.Parallel()

	ctx := WithRouterSubAgentEffort(t.Context(), "high")
	require.Equal(t, "high", RouterSubAgentEffortFromContext(ctx))
}

func TestRouterSubAgentEffortFromContext_IgnoresUnrelatedContext(t *testing.T) {
	t.Parallel()

	//nolint:staticcheck // deliberately unrelated key to prove isolation.
	ctx := context.WithValue(t.Context(), struct{ x int }{}, "high")
	require.Equal(t, "", RouterSubAgentEffortFromContext(ctx))
}

// TestRouterEffortPropagatesToParallelSubagents simulates a single turn
// where the router picked a reasoning effort for the parent turn, and the
// model then fans that turn out into five parallel "task" sub-agent calls
// (as fantasy.NewParallelAgentTool does for the real "agent"/task tool).
// Every one of those five sub-agents must inherit the router's effort
// override into its own ProviderOptions, and each must land in its own
// independent sub-session.
func TestRouterEffortPropagatesToParallelSubagents(t *testing.T) {
	const providerID = "test-provider"
	const subAgentCount = 5

	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, config.ProviderConfig{
		ID:   providerID,
		Type: catwalk.Type(anthropic.Name),
	})

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	type captured struct {
		sessionID string
		effort    anthropic.Effort
	}
	var (
		mu   sync.Mutex
		seen []captured
	)

	model := Model{
		CatwalkCfg: catwalk.Model{
			ID:               "claude-opus-4-7",
			DefaultMaxTokens: 4096,
			CanReason:        true,
			ReasoningLevels:  []string{"low", "medium", "high"},
		},
		ModelCfg: config.SelectedModel{
			Provider:        providerID,
			ReasoningEffort: "low",
		},
	}
	agent := &mockSessionAgent{
		model: model,
		runFunc: func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			raw, ok := call.ProviderOptions[anthropic.Name]
			require.True(t, ok, "expected anthropic provider options to be set")
			opts, ok := raw.(*anthropic.ProviderOptions)
			require.True(t, ok)
			require.NotNil(t, opts.Effort)

			mu.Lock()
			seen = append(seen, captured{sessionID: call.SessionID, effort: *opts.Effort})
			mu.Unlock()

			return agentResultWithText(fmt.Sprintf("done: %s", call.SessionID)), nil
		},
	}

	// This is what the coordinator does in run() once a router decision
	// with a non-empty reasoning effort is applied and
	// RouterOptions.ApplySubagents is enabled: it tags the turn's context
	// so any "task" sub-agents launched during it pick up the same
	// effort, regardless of their own statically configured one.
	ctx := WithRouterSubAgentEffort(t.Context(), "high")

	var wg sync.WaitGroup
	responses := make([]fantasy.ToolResponse, subAgentCount)
	errs := make([]error, subAgentCount)
	for i := range subAgentCount {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			responses[i], errs[i] = coord.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      parentSession.ID,
				AgentMessageID: fmt.Sprintf("msg-%d", i),
				ToolCallID:     fmt.Sprintf("call-%d", i),
				Prompt:         fmt.Sprintf("sub-task %d", i),
				SessionTitle:   "New Agent Session",
				EffortOverride: RouterSubAgentEffortFromContext(ctx),
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "subagent %d", i)
		assert.False(t, responses[i].IsError, "subagent %d", i)
	}

	require.Len(t, seen, subAgentCount)
	sessionIDs := make(map[string]struct{}, subAgentCount)
	for _, c := range seen {
		assert.Equal(t, anthropic.Effort("high"), c.effort,
			"every sub-agent must inherit the router's effort override")
		sessionIDs[c.sessionID] = struct{}{}
	}
	assert.Len(t, sessionIDs, subAgentCount, "each sub-agent must run in its own sub-session")
}

// TestRouterModelPropagatesFromParentToSubagents verifies that when a router
// chooses a model override for the parent turn, and ApplySubagents is
// enabled, all sub-agents inherit that model choice (not just the reasoning
// effort) — by asserting the actual SessionAgentCall.ModelOverride that
// reaches the sub-agent's Run, which is what agent.go's SessionAgent.Run
// uses to pick the model for the real LLM call (see agent.go:712-717).
func TestRouterModelPropagatesFromParentToSubagents(t *testing.T) {
	const providerID = "test-provider"
	const subAgentCount = 5

	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, config.ProviderConfig{
		ID:   providerID,
		Type: catwalk.Type(anthropic.Name),
	})

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	type captured struct {
		sessionID      string
		appliedModelID string
	}
	var (
		mu   sync.Mutex
		seen []captured
	)

	// Subagent's static config uses haiku.
	staticModel := Model{
		CatwalkCfg: catwalk.Model{
			ID:               "claude-haiku-4.5",
			DefaultMaxTokens: 4096,
		},
		ModelCfg: config.SelectedModel{
			Provider: providerID,
		},
	}

	agent := &mockSessionAgent{
		model: staticModel,
		runFunc: func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			require.NotNil(t, call.ModelOverride, "expected the router's model override to reach SessionAgentCall")
			mu.Lock()
			seen = append(seen, captured{sessionID: call.SessionID, appliedModelID: call.ModelOverride.CatwalkCfg.ID})
			mu.Unlock()
			return agentResultWithText(fmt.Sprintf("done: %s", call.SessionID)), nil
		},
	}

	// Simulate a parent turn where the router chose sonnet + high effort,
	// both propagated to subagents via context.
	routerChosenModel := Model{
		CatwalkCfg: catwalk.Model{
			ID:               "claude-sonnet-5",
			DefaultMaxTokens: 8192,
			CanReason:        true,
			ReasoningLevels:  []string{"low", "medium", "high"},
		},
		ModelCfg: config.SelectedModel{
			Provider: providerID,
		},
	}

	ctx := t.Context()
	ctx = WithRouterSubAgentEffort(ctx, "high")
	ctx = WithRouterSubAgentModel(ctx, &routerChosenModel)

	var wg sync.WaitGroup
	responses := make([]fantasy.ToolResponse, subAgentCount)
	errs := make([]error, subAgentCount)
	for i := range subAgentCount {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			responses[i], errs[i] = coord.runSubAgent(ctx, subAgentParams{
				Agent:          agent,
				SessionID:      parentSession.ID,
				AgentMessageID: fmt.Sprintf("msg-%d", i),
				ToolCallID:     fmt.Sprintf("call-%d", i),
				Prompt:         fmt.Sprintf("sub-task %d", i),
				SessionTitle:   "New Agent Session",
				EffortOverride: RouterSubAgentEffortFromContext(ctx),
				ModelOverride:  RouterSubAgentModelFromContext(ctx),
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "subagent %d", i)
		assert.False(t, responses[i].IsError, "subagent %d", i)
	}

	require.Len(t, seen, subAgentCount)
	sessionIDs := make(map[string]struct{}, subAgentCount)
	for _, c := range seen {
		assert.Equal(t, "claude-sonnet-5", c.appliedModelID,
			"every sub-agent must inherit the router's model choice, not its own static haiku config")
		sessionIDs[c.sessionID] = struct{}{}
	}
	assert.Len(t, sessionIDs, subAgentCount, "each sub-agent must run in its own sub-session")
}

// TestRouterSubAgentKeepsStaticModelWhenApplySubagentsDisabled verifies the
// off-by-default path: with no router model/effort tagged on the context
// (the state whenever RouterOptions.ApplySubagents is false, per
// coordinator.run), runSubAgent passes no override at all and the sub-agent
// runs with its own statically configured model — identical to pre-router
// behavior.
func TestRouterSubAgentKeepsStaticModelWhenApplySubagentsDisabled(t *testing.T) {
	const providerID = "test-provider"

	env := testEnv(t)
	coord := newTestCoordinator(t, env, providerID, config.ProviderConfig{
		ID:   providerID,
		Type: catwalk.Type(anthropic.Name),
	})

	parentSession, err := env.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	staticModel := Model{
		CatwalkCfg: catwalk.Model{
			ID:               "claude-haiku-4.5",
			DefaultMaxTokens: 4096,
		},
		ModelCfg: config.SelectedModel{
			Provider: providerID,
		},
	}

	var captured SessionAgentCall
	agent := &mockSessionAgent{
		model: staticModel,
		runFunc: func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			captured = call
			return agentResultWithText("done"), nil
		},
	}

	// No WithRouterSubAgentEffort/WithRouterSubAgentModel on this context —
	// this is what a plain t.Context() already looks like, and what
	// coordinator.run leaves it as when ApplySubagents is false.
	ctx := t.Context()
	_, err = coord.runSubAgent(ctx, subAgentParams{
		Agent:          agent,
		SessionID:      parentSession.ID,
		AgentMessageID: "msg-0",
		ToolCallID:     "call-0",
		Prompt:         "sub-task",
		SessionTitle:   "New Agent Session",
		EffortOverride: RouterSubAgentEffortFromContext(ctx),
		ModelOverride:  RouterSubAgentModelFromContext(ctx),
	})
	require.NoError(t, err)

	assert.Nil(t, captured.ModelOverride, "sub-agent must keep its static model when the router never tagged the context")
}

// TestRouterSubAgentPrefersOwnPromptOverParentDecision proves tier 1 of the
// resilience chain: with ApplySubagents enabled and a reachable router,
// runSubAgent asks the router about its OWN prompt and uses that answer,
// even when a different decision was already propagated from the parent
// turn via context — the two must never be conflated.
func TestRouterSubAgentPrefersOwnPromptOverParentDecision(t *testing.T) {
	const providerID = "test-provider"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"reasoning_effort": map[string]any{"type": "score", "score": 0, "confidence": 0.95}, // -> "low"
				"model_choice":     map[string]any{"type": "choice", "choice": "pool-model", "confidence": 0.95},
			},
		})
	}))
	t.Cleanup(server.Close)

	coord := newApplySubagentsTestCoordinator(t, server.URL, "pool-model", []string{"low", "medium", "high"})
	parentSession, err := coord.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	staticModel := Model{
		CatwalkCfg: catwalk.Model{ID: "static-model", DefaultMaxTokens: 4096, ReasoningLevels: []string{"low", "medium", "high"}},
		ModelCfg:   config.SelectedModel{Provider: providerID},
	}
	var captured SessionAgentCall
	agent := &mockSessionAgent{
		model: staticModel,
		runFunc: func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			captured = call
			return agentResultWithText("done"), nil
		},
	}

	// The parent turn supposedly applied "high" + a different model — the
	// sub-agent's own low-effort, pool-model decision must win instead.
	parentModel := Model{CatwalkCfg: catwalk.Model{ID: "parent-chosen-model"}}
	ctx := t.Context()
	ctx = WithRouterSubAgentEffort(ctx, "high")
	ctx = WithRouterSubAgentModel(ctx, &parentModel)

	_, err = coord.runSubAgent(ctx, subAgentParams{
		Agent:          agent,
		SessionID:      parentSession.ID,
		AgentMessageID: "msg-0",
		ToolCallID:     "call-0",
		Prompt:         "a simple sub-task",
		SessionTitle:   "New Agent Session",
		EffortOverride: RouterSubAgentEffortFromContext(ctx),
		ModelOverride:  RouterSubAgentModelFromContext(ctx),
	})
	require.NoError(t, err)

	require.NotNil(t, captured.ModelOverride)
	assert.Equal(t, "pool-model", captured.ModelOverride.CatwalkCfg.ID,
		"sub-agent's own fresh decision must win over the parent's propagated one")
}

// TestRouterSubAgentFallsBackToParentWhenOwnRouterCallFails proves tier 2:
// when the sub-agent's own prompt-scoped router call fails outright (the
// router server is unreachable), runSubAgent falls back to the parent
// turn's already-applied decision instead of dropping to the sub-agent's
// static config outright.
func TestRouterSubAgentFallsBackToParentWhenOwnRouterCallFails(t *testing.T) {
	const providerID = "test-provider"

	// A closed local port: the router call fails fast and outright.
	coord := newApplySubagentsTestCoordinator(t, "http://127.0.0.1:9", "pool-model", []string{"low", "medium", "high"})
	parentSession, err := coord.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)

	staticModel := Model{
		CatwalkCfg: catwalk.Model{ID: "static-model", DefaultMaxTokens: 4096, ReasoningLevels: []string{"low", "medium", "high"}},
		ModelCfg:   config.SelectedModel{Provider: providerID},
	}
	var captured SessionAgentCall
	agent := &mockSessionAgent{
		model: staticModel,
		runFunc: func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			captured = call
			return agentResultWithText("done"), nil
		},
	}

	parentModel := Model{
		CatwalkCfg: catwalk.Model{ID: "parent-chosen-model", DefaultMaxTokens: 4096},
		ModelCfg:   config.SelectedModel{Provider: "mock"},
	}
	ctx := t.Context()
	ctx = WithRouterSubAgentEffort(ctx, "high")
	ctx = WithRouterSubAgentModel(ctx, &parentModel)

	_, err = coord.runSubAgent(ctx, subAgentParams{
		Agent:          agent,
		SessionID:      parentSession.ID,
		AgentMessageID: "msg-0",
		ToolCallID:     "call-0",
		Prompt:         "a simple sub-task",
		SessionTitle:   "New Agent Session",
		EffortOverride: RouterSubAgentEffortFromContext(ctx),
		ModelOverride:  RouterSubAgentModelFromContext(ctx),
	})
	require.NoError(t, err)

	require.NotNil(t, captured.ModelOverride)
	assert.Equal(t, "parent-chosen-model", captured.ModelOverride.CatwalkCfg.ID,
		"an unreachable router must fall back to the parent turn's decision, not go straight to static")
}
