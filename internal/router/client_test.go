package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClient_Decide_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/systemone", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "test-model", body["model"])
		require.Equal(t, "help me fix a bug", body["state"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"agent": map[string]any{"type": "choice", "choice": "coder", "confidence": 0.92},
			},
			"usage": map[string]any{"input_tokens": 100, "output_tokens": 10, "cost": 0.0000123},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "/v1/systemone", "test-key", "test-model", time.Second)
	answers, cost, err := c.Decide(t.Context(), "help me fix a bug", map[string]QuestionSpec{
		"agent": {Type: "choice", Criteria: map[string]string{"coder": "writes code"}},
	})
	require.NoError(t, err)
	require.Equal(t, "coder", answers["agent"].Choice)
	require.InDelta(t, 0.92, answers["agent"].Confidence, 0.0001)
	require.InDelta(t, 0.0000123, cost, 1e-9, "the classifier call's own reported cost must be surfaced, not discarded")
}

// TestClient_Decide_NoUsageReportedIsZeroCost proves a backend that omits
// the usage field (a local System One-compatible server, typically —
// self-hosting has no per-call $ cost to report) yields cost 0 rather
// than an error, since that's the expected shape for that backend, not a
// malformed response.
func TestClient_Decide_NoUsageReportedIsZeroCost(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"agent": map[string]any{"type": "choice", "choice": "coder", "confidence": 0.5},
			},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "/v1/systemone", "", "test-model", time.Second)
	_, cost, err := c.Decide(t.Context(), "state", map[string]QuestionSpec{})
	require.NoError(t, err)
	require.Zero(t, cost)
}

func TestClient_Decide_NonOKStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	c := NewClient(server.URL, "/v1/systemone", "", "test-model", time.Second)
	_, _, err := c.Decide(t.Context(), "state", map[string]QuestionSpec{})
	require.Error(t, err)
}

// TestClient_Decide_NestedInputShape proves WithNestedInput wraps state
// and questions under "input" while keeping model at the top level, the
// shape Cloudflare Workers AI expects, instead of the flat System One
// shape every other backend uses.
func TestClient_Decide_NestedInputShape(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "typesafe/jev", body["model"])
		require.NotContains(t, body, "state", "state must live under input, not at the top level")
		require.NotContains(t, body, "questions")

		input, ok := body["input"].(map[string]any)
		require.True(t, ok, "body must carry an input object")
		require.Equal(t, "deploy failed", input["state"])
		require.NotEmpty(t, input["questions"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"reasoning_effort": map[string]any{"type": "score", "score": 2, "confidence": 0.9},
			},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "", "", "typesafe/jev", time.Second, WithNestedInput())
	answers, _, err := c.Decide(t.Context(), "deploy failed", map[string]QuestionSpec{
		"reasoning_effort": {Type: "score", Criteria: []string{"low", "medium", "high"}},
	})
	require.NoError(t, err)
	require.InDelta(t, 2, answers["reasoning_effort"].Score, 0.0001)
}

func TestClient_Decide_Timeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(server.URL, "/systemone", "", "test-model", 5*time.Millisecond)
	_, _, err := c.Decide(t.Context(), "state", map[string]QuestionSpec{})
	require.Error(t, err)
}
