// internal/agent/router_decision_recording_test.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/router"
	"github.com/stretchr/testify/require"
)

// newRouterDecisionTestCoordinator builds a minimal interactive coordinator
// against a hermetic config: one openai-typed provider pointed at a closed
// port (so agent.Run always fails deep inside the real fantasy call, well
// after run()'s router-decision bookkeeping has already executed) and a
// router pointed at routerBaseURL. reasoningLevels controls what the
// selected model advertises as supported reasoning efforts, so tests can
// exercise the "router chose an effort the model doesn't support" path.
func newRouterDecisionTestCoordinator(t *testing.T, routerBaseURL string, reasoningLevels []string) *coordinator {
	t.Helper()

	env := testEnv(t)

	levelsJSON, err := json.Marshal(reasoningLevels)
	require.NoError(t, err)

	crushJSON := fmt.Sprintf(`{
  "options": {
    "disable_default_providers": true,
    "disable_provider_auto_update": true,
    "router": {"enabled": true, "provider": "local", "base_url": %q, "model": "test-model"}
  },
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128, "reasoning_levels": %s}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`, routerBaseURL, string(levelsJSON))
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
		agents:      make(map[string]SessionAgent),
		interactive: true,
	}

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	agentCfg := cfg.Config().Agents[config.AgentCoder]

	agent, err := coord.buildAgent(context.Background(), p, agentCfg, false)
	require.NoError(t, err)
	coord.mainAgent = agent
	coord.mainAgentName = config.AgentCoder
	coord.agents[config.AgentCoder] = agent

	return coord
}

// routerServer returns an httptest server implementing the System One
// endpoint the local router provider calls, always answering with the
// given reasoning-effort score/confidence.
func routerServer(t *testing.T, score int, confidence float64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"reasoning_effort": map[string]any{"type": "score", "score": score, "confidence": confidence},
			},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

// routerServerWithModelChoice is like routerServer but also answers the
// model_choice question, so the router both picks a reasoning effort and
// chooses a model from the pool.
func routerServerWithModelChoice(t *testing.T, score int, confidence float64, modelID string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"reasoning_effort": map[string]any{"type": "score", "score": score, "confidence": confidence},
				"model_choice":     map[string]any{"type": "choice", "choice": modelID, "confidence": confidence},
			},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

// newRouterModelPoolAndDecisionTestCoordinator builds a coordinator with
// both a router (with a model pool) and an OpenRouter provider whose
// catalog gives poolModelReasoningLevels as the reasoning levels the
// resolved override model supports — so tests can exercise "the router
// picked a model, but its chosen effort isn't one that model supports".
func newRouterModelPoolAndDecisionTestCoordinator(t *testing.T, routerBaseURL string, poolModelID string, poolModelReasoningLevels []string) *coordinator {
	t.Helper()

	env := testEnv(t)

	levelsJSON, err := json.Marshal(poolModelReasoningLevels)
	require.NoError(t, err)

	crushJSON := fmt.Sprintf(`{
  "options": {
    "disable_default_providers": true,
    "disable_provider_auto_update": true,
    "router": {"enabled": true, "provider": "local", "base_url": %q, "model": "test-model", "model_pool": [%q]}
  },
  "providers": {
    "mock": {"id": "mock", "name": "Mock", "type": "openai",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
      "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]},
    "openrouter": {"id": "openrouter", "name": "OpenRouter", "type": "openrouter",
      "base_url": "http://127.0.0.1:9", "api_key": "test-key",
      "models": [{"id": %q, "name": "Pool Model", "context_window": 200000, "default_max_tokens": 4096, "reasoning_levels": %s}]}
  },
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`, routerBaseURL, poolModelID, poolModelID, string(levelsJSON))
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
		agents:      make(map[string]SessionAgent),
		interactive: true,
	}

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	agentCfg := cfg.Config().Agents[config.AgentCoder]

	agent, err := coord.buildAgent(context.Background(), p, agentCfg, false)
	require.NoError(t, err)
	coord.mainAgent = agent
	coord.mainAgentName = config.AgentCoder
	coord.agents[config.AgentCoder] = agent

	return coord
}

// TestRunAppliedDecision_ModelOnlyStillCopiesConfidence pins fix 4 from the
// final whole-branch review: when the router's chosen model is applied but
// its chosen reasoning effort is not (because the pool model's
// ReasoningLevels doesn't contain it), the badge's Confidence/LowConfidence
// must still reflect the router's decision instead of being silently
// dropped by the effort-only branch that used to be the sole place they
// were copied from.
func TestRunAppliedDecision_ModelOnlyStillCopiesConfidence(t *testing.T) {
	const poolModelID = "anthropic/claude-opus-4"

	// score 2 -> "high", but the pool model below only advertises
	// "low"/"medium", so the effort is not applied while the model is.
	server := routerServerWithModelChoice(t, 2, 0.42, poolModelID)
	coord := newRouterModelPoolAndDecisionTestCoordinator(t, server.URL, poolModelID, []string{"low", "medium"})

	_, _ = coord.run(t.Context(), nil, "test-session", "hello")

	decision, ok := coord.LastRouterDecision()
	require.True(t, ok, "a decision whose model resolved must be recorded as applied, even without the effort")
	require.Empty(t, decision.ReasoningEffort, "sanity: the effort must not have been applied")
	require.Equal(t, poolModelID, decision.ModelID)
	require.Equal(t, 0.42, decision.Confidence,
		"Confidence must be copied even when only the model (not the effort) was applied")
	require.True(t, decision.LowConfidence,
		"LowConfidence must be copied even when only the model (not the effort) was applied")
}

// TestRunAppliedDecision_ModelChoiceBelowRandomChanceFloorIsNotApplied pins
// the Laya finding: a classifier can score reasoning_effort with real
// confidence while its model_choice answer is indistinguishable from
// guessing among the pool (confirmed live against Laya's actual
// typed-decisions model: every prompt tried, trivial through complex,
// picked the same pool member with 2-6% confidence). Applying that pick
// anyway would silently route every message to whichever model the
// classifier happens to be biased toward, regardless of content — this
// proves the fix: a model_choice confidence at or below chance (1/pool
// size) must not override the model, even though the *effort* confidence
// on the same response is comfortably applied.
func TestRunAppliedDecision_ModelChoiceBelowRandomChanceFloorIsNotApplied(t *testing.T) {
	const chosenModelID = "anthropic/claude-opus-4.5"
	poolModelIDs := []string{"anthropic/claude-haiku-4.5", "anthropic/claude-sonnet-5", chosenModelID}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				// Well-calibrated effort answer (score 2 -> "high", 91%
				// confidence) on the very same response whose model_choice
				// is noise — exactly what was observed against real Laya.
				"reasoning_effort": map[string]any{"type": "score", "score": 2, "confidence": 0.91},
				"model_choice":     map[string]any{"type": "choice", "choice": chosenModelID, "confidence": 0.0376},
			},
		})
	}))
	t.Cleanup(server.Close)

	env := testEnv(t)
	poolJSON, err := json.Marshal(poolModelIDs)
	require.NoError(t, err)

	catalogModels := make([]string, 0, len(poolModelIDs))
	for _, id := range poolModelIDs {
		catalogModels = append(catalogModels, fmt.Sprintf(
			`{"id": %q, "name": %q, "context_window": 200000, "default_max_tokens": 4096, "reasoning_levels": ["low","medium","high"]}`,
			id, id))
	}
	crushJSON := fmt.Sprintf(`{
  "options": {
    "disable_default_providers": true,
    "disable_provider_auto_update": true,
    "router": {"enabled": true, "provider": "local", "base_url": %q, "model": "test-model", "model_pool": %s}
  },
  "providers": {
    "mock": {"id": "mock", "name": "Mock", "type": "openai",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
      "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128, "reasoning_levels": ["low","medium","high"]}]},
    "openrouter": {"id": "openrouter", "name": "OpenRouter", "type": "openrouter",
      "base_url": "http://127.0.0.1:9", "api_key": "test-key",
      "models": [%s]}
  },
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`, server.URL, string(poolJSON), strings.Join(catalogModels, ","))
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
		agents:      make(map[string]SessionAgent),
		interactive: true,
	}
	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	agentCfg := cfg.Config().Agents[config.AgentCoder]
	agent, err := coord.buildAgent(context.Background(), p, agentCfg, false)
	require.NoError(t, err)
	coord.mainAgent = agent
	coord.mainAgentName = config.AgentCoder
	coord.agents[config.AgentCoder] = agent

	_, _ = coord.run(t.Context(), nil, "test-session", "hello")

	decision, ok := coord.LastRouterDecision()
	require.True(t, ok, "the effort answer alone is confident enough to apply")
	require.Equal(t, "high", decision.ReasoningEffort)
	require.Empty(t, decision.ModelID,
		"a model_choice confidence at/below random chance across the pool must not be recorded as applied")
}

// TestRunRecordsRouterDecisionOnlyWhenApplied pins finding 2 from the final
// whole-branch review: the status bar's router badge must reflect "the
// reasoning effort applied to the last dispatched message", not a stale
// decision from an earlier message and not a decision the model never
// actually honored. run() must call exactly one of
// setLastRouterDecision/clearLastRouterDecision on every path through the
// router section.
func TestRunRecordsRouterDecisionOnlyWhenApplied(t *testing.T) {
	t.Run("decision applied when the model supports the chosen effort", func(t *testing.T) {
		server := routerServer(t, 2, 0.95) // score 2 -> "high"
		coord := newRouterDecisionTestCoordinator(t, server.URL, []string{"low", "medium", "high"})

		// run() will ultimately fail (closed provider port), but the router
		// bookkeeping happens before that network call.
		_, _ = coord.run(t.Context(), nil, "test-session", "hello")

		decision, ok := coord.LastRouterDecision()
		require.True(t, ok, "a decision the model supports must be recorded as applied")
		require.Equal(t, "high", decision.ReasoningEffort)
	})

	t.Run("decision cleared when the router fails open", func(t *testing.T) {
		// A server that always 500s makes resolveRouterDecision fail open.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)
		coord := newRouterDecisionTestCoordinator(t, server.URL, []string{"low", "medium", "high"})

		// Seed a stale decision from an earlier, successful message.
		coord.setLastRouterDecision(router.Decision{ReasoningEffort: "high", Confidence: 0.95})
		_, ok := coord.LastRouterDecision()
		require.True(t, ok, "sanity: the seeded decision is present before the run")

		_, _ = coord.run(t.Context(), nil, "test-session", "hello")

		_, ok = coord.LastRouterDecision()
		require.False(t, ok,
			"a failed-open router call must clear the stale decision, not leave it standing")
	})

	t.Run("decision cleared when the chosen effort is unsupported by the model", func(t *testing.T) {
		server := routerServer(t, 2, 0.95) // score 2 -> "high"
		// The active model only supports "low"/"medium": "high" will be
		// filtered out by getProviderOptions's ReasoningLevels check.
		coord := newRouterDecisionTestCoordinator(t, server.URL, []string{"low", "medium"})

		// Seed a stale decision from an earlier message.
		coord.setLastRouterDecision(router.Decision{ReasoningEffort: "low", Confidence: 0.9})

		_, _ = coord.run(t.Context(), nil, "test-session", "hello")

		_, ok := coord.LastRouterDecision()
		require.False(t, ok,
			"a router call whose effort the model doesn't support must clear the decision, "+
				"even though the router call itself succeeded")
	})
}

// TestRunNeverLeavesRouterQueryingStuckOn pins the live "consulting"
// indicator's other half: RouterQuerying must be false once run() returns,
// on every path (router call succeeds, fails, or the decision it returns
// is not honored) — a live indicator that got stuck "on" after a message
// finished would misreport every message after it, not just this one.
func TestRunNeverLeavesRouterQueryingStuckOn(t *testing.T) {
	server := routerServer(t, 2, 0.95) // score 2 -> "high"
	coord := newRouterDecisionTestCoordinator(t, server.URL, []string{"low", "medium", "high"})

	_, ok := coord.RouterQuerying()
	require.False(t, ok, "sanity: nothing is querying before the first run")

	_, _ = coord.run(t.Context(), nil, "test-session", "hello")

	_, ok = coord.RouterQuerying()
	require.False(t, ok, "RouterQuerying must be false once run() has returned")
}

// TestRunRecordsLastRouterModelEvenWhenNotApplied proves LastRouterModel
// keeps naming the router backend's own classifier model after a run —
// distinct from LastRouterDecision, which only reflects a decision that
// was actually applied to the current agent's model/reasoning effort.
// Without this, the sidebar has no way to say which model classified the
// last message once the decision itself was discarded (e.g. the model in
// use doesn't support the effort the router chose).
func TestRunRecordsLastRouterModelEvenWhenNotApplied(t *testing.T) {
	server := routerServer(t, 2, 0.95) // score 2 -> "high"
	// The active model only supports "low", so the applied decision gets
	// cleared even though the router call itself succeeded.
	coord := newRouterDecisionTestCoordinator(t, server.URL, []string{"low"})

	_, _ = coord.run(t.Context(), nil, "test-session", "hello")

	_, hasDecision := coord.LastRouterDecision()
	require.False(t, hasDecision, "sanity: the decision must be discarded (model doesn't support 'high')")
	require.Equal(t, "test-model", coord.LastRouterModel(),
		"LastRouterModel must still name the classifier even when its decision wasn't applied")
}
