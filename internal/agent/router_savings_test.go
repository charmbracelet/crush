package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestEstimateModelCost pins the pricing formula router savings are built
// on: it must match sessionAgent.updateSessionUsage's real-cost formula
// exactly (cache write, cache read, input, output, all at the model's own
// per-1M rates), or a savings estimate computed against a different model
// would not be comparable to the session's actual accumulated cost.
func TestEstimateModelCost(t *testing.T) {
	model := catwalk.Model{
		CostPer1MIn:        2.0,
		CostPer1MOut:       10.0,
		CostPer1MInCached:  0.5,
		CostPer1MOutCached: 1.0,
	}
	usage := fantasy.Usage{
		InputTokens:         1_000_000,
		OutputTokens:        500_000,
		CacheCreationTokens: 2_000_000,
		CacheReadTokens:     4_000_000,
	}

	got := estimateModelCost(model, usage)
	want := 2.0*1 + 10.0*0.5 + 0.5*2 + 1.0*4
	require.InDelta(t, want, got, 1e-9)
}

// TestRun_SetsLastRouterErrorOnFailedCall proves a failed router call
// (the backend 500s) leaves a human-readable message on LastRouterError,
// so the UI can show that the router isn't working instead of just
// silently showing nothing — the same failure that previously only
// reached a discarded slog.Warn.
func TestRun_SetsLastRouterErrorOnFailedCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	coord := newRouterDecisionTestCoordinator(t, server.URL, []string{"low", "medium", "high"})
	require.Empty(t, coord.LastRouterError(), "sanity: no error before the first run")

	_, _ = coord.run(t.Context(), nil, "test-session", "hello")

	require.NotEmpty(t, coord.LastRouterError(), "a failed router call must leave a visible error message")
}

// TestRun_ClearsLastRouterErrorAfterSuccessfulCall proves a stale error
// from an earlier failure doesn't linger once the router starts working
// again — a fixed local backend (restarted, or a base_url typo
// corrected) must make the error disappear, not require the user to
// notice it cleared on its own.
func TestRun_ClearsLastRouterErrorAfterSuccessfulCall(t *testing.T) {
	server := routerServer(t, 1, 0.9) // score 1 -> "medium"
	coord := newRouterDecisionTestCoordinator(t, server.URL, []string{"low", "medium", "high"})
	coord.setLastRouterError("stale error from an earlier failed call")

	_, _ = coord.run(t.Context(), nil, "test-session", "hello")

	require.Empty(t, coord.LastRouterError(), "a successful call must clear a stale error")
}

// TestMostExpensivePoolModel_PicksHighestCombinedRate proves the router
// savings baseline is "the top of this pool" — the actual alternative the
// router chooses between — not just whichever pool entry sorts first, and
// that it's the combined input+output rate that decides "priciest", since
// a model can be cheap on one axis and expensive on the other.
func TestMostExpensivePoolModel_PicksHighestCombinedRate(t *testing.T) {
	providerCfg := config.ProviderConfig{
		Models: []catwalk.Model{
			{ID: "cheap", CostPer1MIn: 1, CostPer1MOut: 1},
			{ID: "priciest", CostPer1MIn: 5, CostPer1MOut: 15},
			{ID: "mid", CostPer1MIn: 3, CostPer1MOut: 3},
		},
	}

	got, ok := mostExpensivePoolModel(providerCfg, []string{"cheap", "priciest", "mid"})
	require.True(t, ok)
	require.Equal(t, "priciest", got.ID)
}

// TestMostExpensivePoolModel_UnknownPoolFailsOpen proves a pool made
// entirely of ids outside the provider's known catalog reports ok=false
// instead of returning a zero-value catwalk.Model that would silently
// price the baseline at $0.
func TestMostExpensivePoolModel_UnknownPoolFailsOpen(t *testing.T) {
	providerCfg := config.ProviderConfig{
		Models: []catwalk.Model{{ID: "known", CostPer1MIn: 5, CostPer1MOut: 5}},
	}

	_, ok := mostExpensivePoolModel(providerCfg, []string{"totally-unknown-model"})
	require.False(t, ok)
}

// TestRunSubtractsClassifierCallCostEvenWhenDecisionNotApplied proves the
// classifier call's own reported cost is charged against RouterSavings
// unconditionally — including when the decision it produced was not
// applied (the pool model here doesn't support "high") — since the HTTP
// call to the classifier happened and cost that money regardless of
// what crush did with its answer.
func TestRunSubtractsClassifierCallCostEvenWhenDecisionNotApplied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"reasoning_effort": map[string]any{"type": "score", "score": 2, "confidence": 0.95},
			},
			"usage": map[string]any{"input_tokens": 300, "output_tokens": 20, "cost": 0.0000123},
		})
	}))
	t.Cleanup(server.Close)

	coord := newRouterDecisionTestCoordinator(t, server.URL, []string{"low"}) // model doesn't support "high"

	_, _ = coord.run(t.Context(), nil, "test-session", "hello")

	_, hasDecision := coord.LastRouterDecision()
	require.False(t, hasDecision, "sanity: the decision must be discarded (model doesn't support 'high')")
	require.InDelta(t, -0.0000123, coord.RouterSavings("test-session"), 1e-9,
		"the classifier's own call cost must still be subtracted even though its decision wasn't applied")
}

// TestRouterSavings_AccumulatesPerSessionAndDefaultsToZero proves savings
// accumulate independently per session (so one session's router activity
// never leaks into another's total) and that a session the router has
// never touched reads 0 rather than panicking on the lazily-initialized
// map.
func TestRouterSavings_AccumulatesPerSessionAndDefaultsToZero(t *testing.T) {
	var c coordinator

	require.Zero(t, c.RouterSavings("untouched-session"),
		"a session never passed to addRouterSavings must read 0, not panic")

	c.addRouterSavings("session-a", 0.01)
	c.addRouterSavings("session-a", 0.02)
	c.addRouterSavings("session-b", -0.5)

	require.InDelta(t, 0.03, c.RouterSavings("session-a"), 1e-9,
		"multiple router-applied messages in the same session must accumulate")
	require.InDelta(t, -0.5, c.RouterSavings("session-b"), 1e-9,
		"a session where the router cost more than the baseline must record a negative total")
	require.Zero(t, c.RouterSavings("session-a-typo"),
		"a session with no accumulated savings must not see another session's total")
}
