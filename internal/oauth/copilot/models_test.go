package copilot

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
		require.Equal(t, "crush", r.Header.Get("originator"))
		require.Equal(t, "model-access", r.Header.Get("OpenAI-Intent"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"models": [
				{
					"id": "claude-sonnet-4.5",
					"name": "Claude Sonnet 4.5",
					"model_picker_enabled": true,
					"capabilities": {
						"limits": {"max_context_window_tokens": 200000},
						"supports": {"vision": true, "thinking": true}
					}
				},
				{
					"id": "gpt-5.1",
					"name": "GPT-5.1",
					"model_picker_enabled": true,
					"capabilities": {
						"limits": {"max_context_window_tokens": 128000},
						"supports": {"reasoning_effort": ["low", "medium", "high"]}
					}
				},
				{
					"id": "internal-model",
					"name": "Internal",
					"model_picker_enabled": false,
					"capabilities": {
						"limits": {"max_context_window_tokens": 100}
					}
				}
			]
		}`))
	}))
	t.Cleanup(server.Close)

	orig := modelsEndpoint
	modelsEndpoint = server.URL
	t.Cleanup(func() { modelsEndpoint = orig })

	models, err := Models(context.Background(), &oauth.Token{AccessToken: "at-models"})
	require.NoError(t, err)

	// The disabled entry is dropped; the other two survive in order.
	require.Len(t, models, 2)
	require.Equal(t, "claude-sonnet-4.5", models[0].ID)
	require.Equal(t, "Claude Sonnet 4.5", models[0].Name)
	require.Equal(t, int64(200000), models[0].ContextWindow)
	require.True(t, models[0].CanReason)
	require.True(t, models[0].SupportsImages)

	require.Equal(t, "gpt-5.1", models[1].ID)
	require.True(t, models[1].CanReason)
	require.Equal(t, []string{"low", "medium", "high"}, models[1].ReasoningLevels)
	require.False(t, models[1].SupportsImages)
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
