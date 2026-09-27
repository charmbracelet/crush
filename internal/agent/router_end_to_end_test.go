// internal/agent/router_end_to_end_test.go
package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestRouterDecision_LowConfidenceIsStillApplied confirms the
// spec-mandated behavior end to end at the resolveRouterDecision seam:
// a low-confidence answer is not discarded, only flagged.
func TestRouterDecision_LowConfidenceIsStillApplied(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"reasoning_effort": map[string]any{"type": "score", "score": 0, "confidence": 0.4},
			},
		})
	}))
	defer server.Close()

	routerCfg := &config.RouterOptions{
		Enabled:             true,
		Provider:            "local",
		BaseURL:             server.URL,
		Model:               "test-model",
		ConfidenceThreshold: 0.7,
	}

	decision, ok, err := resolveRouterDecision(t.Context(), routerCfg, "", "fix this", nil)
	require.NoError(t, err)
	require.True(t, ok, "a low-confidence decision must still be applied, not discarded")
	require.Equal(t, "low", decision.ReasoningEffort)
	require.True(t, decision.LowConfidence)
}
