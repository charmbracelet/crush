package grok

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
		require.Equal(t, "application/json", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "grok-4",
					"context_length": 256000,
					"prompt_text_token_price": 30000,
					"prompt_image_token_price": 12500,
					"completion_text_token_price": 15000,
					"capabilities": {
						"reasoning_effort": ["low", "high"],
						"default_reasoning_effort": "high"
					}
				},
				{
					"id": "grok-code-fast-1",
					"context_length": 131072,
					"prompt_text_token_price": 1500,
					"completion_text_token_price": 7500,
					"prompt_image_token_price": 0
				},
				{
					"id": "grok-imagine-image",
					"context_length": 1024,
					"image_price": 200000000
				}
			],
			"object": "list"
		}`))
	}))
	t.Cleanup(server.Close)

	orig := modelsEndpoint
	modelsEndpoint = server.URL
	t.Cleanup(func() { modelsEndpoint = orig })

	models, err := Models(context.Background(), &oauth.Token{AccessToken: "at-models"})
	require.NoError(t, err)

	// The image generation entry is dropped; the other two survive in
	// order.
	require.Len(t, models, 2)
	require.Equal(t, "grok-4", models[0].ID)
	require.Equal(t, "Grok 4", models[0].Name)
	require.True(t, models[0].CanReason)
	require.Equal(t, []string{"low", "high"}, models[0].ReasoningLevels)
	require.Equal(t, "high", models[0].DefaultReasoningEffort)
	require.Equal(t, int64(256000), models[0].ContextWindow)
	require.True(t, models[0].SupportsImages)

	require.False(t, models[1].CanReason)
	require.Empty(t, models[1].ReasoningLevels)
	require.Empty(t, models[1].DefaultReasoningEffort)
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
			_, _ = w.Write([]byte(`{"data": []}`))
		}))
		t.Cleanup(server.Close)

		orig := modelsEndpoint
		modelsEndpoint = server.URL
		t.Cleanup(func() { modelsEndpoint = orig })

		_, err := Models(context.Background(), &oauth.Token{AccessToken: "at"})
		require.ErrorContains(t, err, "empty")
	})
}
