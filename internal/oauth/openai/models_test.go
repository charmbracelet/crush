package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer at-models", r.Header.Get("Authorization"))
		require.Equal(t, "acct-9", r.Header.Get("chatgpt-account-id"))
		require.Equal(t, "crush", r.Header.Get("originator"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"models": [
				{
					"slug": "gpt-5.1-codex",
					"display_name": "GPT-5.1 Codex",
					"visibility": "list",
					"context_window": 272000,
					"default_reasoning_level": "medium",
					"supported_reasoning_levels": [
						{"effort": "low"},
						{"effort": "medium"},
						{"effort": "high"}
					]
				},
				{
					"slug": "gpt-5.1-codex-mini",
					"display_name": "GPT-5.1 Codex Mini",
					"visibility": "list",
					"context_window": 272000
				},
				{
					"slug": "internal-model",
					"display_name": "Internal",
					"visibility": "hide",
					"context_window": 100
				}
			]
		}`))
	}))
	t.Cleanup(server.Close)

	orig := modelsEndpoint
	modelsEndpoint = server.URL
	t.Cleanup(func() { modelsEndpoint = orig })

	models, err := Models(context.Background(), &oauth.Token{
		AccessToken: "at-models",
		AccountID:   "acct-9",
	})
	require.NoError(t, err)

	// The hidden entry is dropped; the other two survive in order.
	require.Len(t, models, 2)
	require.Equal(t, "gpt-5.1-codex", models[0].ID)
	require.Equal(t, "GPT-5.1 Codex", models[0].Name)
	require.True(t, models[0].CanReason)
	require.Equal(t, []string{"low", "medium", "high"}, models[0].ReasoningLevels)
	require.Equal(t, "medium", models[0].DefaultReasoningEffort)
	require.Equal(t, int64(272000), models[0].ContextWindow)

	require.False(t, models[1].CanReason)
	require.Empty(t, models[1].ReasoningLevels)
}

func TestModels_Errors(t *testing.T) {
	t.Run("nil token", func(t *testing.T) {
		_, err := Models(context.Background(), nil)
		require.ErrorContains(t, err, "OAuth token")
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		t.Cleanup(server.Close)

		orig := modelsEndpoint
		modelsEndpoint = server.URL
		t.Cleanup(func() { modelsEndpoint = orig })

		_, err := Models(context.Background(), &oauth.Token{AccessToken: "stale"})
		require.Error(t, err)
	})

	t.Run("empty catalog", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"models": []}`))
		}))
		t.Cleanup(server.Close)

		orig := modelsEndpoint
		modelsEndpoint = server.URL
		t.Cleanup(func() { modelsEndpoint = orig })

		_, err := Models(context.Background(), &oauth.Token{AccessToken: "at"})
		require.ErrorContains(t, err, "empty")
	})
}
