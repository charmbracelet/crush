package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/stretchr/testify/require"
)

func TestRealCopilotModelsClientGetMapsModelsAndSendsHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/models", r.URL.Path)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		for key, value := range copilot.Headers() {
			require.Equal(t, value, r.Header.Get(key))
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"data": [
				{
					"id": "claude-sonnet-4.6",
					"name": "Claude Sonnet 4.6",
					"version": "claude-sonnet-4.6",
					"capabilities": {
						"limits": {
							"max_context_window_tokens": 264000,
							"max_output_tokens": 64000
						},
						"supports": {
							"vision": true,
							"reasoning_effort": ["low", "medium", "high"]
						}
					}
				}
			]
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := &realCopilotModelsClient{baseURL: server.URL + "/", apiKey: " test-token "}
	provider, err := client.Get(t.Context())

	require.NoError(t, err)
	require.Equal(t, catwalk.InferenceProviderCopilot, provider.ID)
	require.Equal(t, server.URL, provider.APIEndpoint)
	require.Equal(t, catwalk.TypeOpenAICompat, provider.Type)
	require.Equal(t, []catwalk.Model{
		{
			ID:               "claude-sonnet-4.6",
			Name:             "Claude Sonnet 4.6",
			ContextWindow:    264000,
			DefaultMaxTokens: 64000,
			CanReason:        true,
			ReasoningLevels:  []string{"low", "medium", "high"},
			SupportsImages:   true,
		},
	}, provider.Models)
}

func TestRealCopilotModelsClientGetRefreshesExpiredOAuthToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer refreshed-token", r.Header.Get("Authorization"))
		_, err := w.Write([]byte(`{
			"data": [
				{
					"id": "gpt-5.1",
					"name": "GPT 5.1",
					"capabilities": {
						"limits": {
							"max_context_window_tokens": 128000,
							"max_output_tokens": 16000
						}
					}
				}
			]
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := &realCopilotModelsClient{
		baseURL: server.URL,
		oauthToken: &oauth.Token{
			AccessToken:  "expired-token",
			RefreshToken: "github-token",
			ExpiresIn:    3600,
			ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
		},
		refreshToken: func(_ctx context.Context, githubToken string) (*oauth.Token, error) {
			require.Equal(t, "github-token", githubToken)
			return &oauth.Token{
				AccessToken:  "refreshed-token",
				RefreshToken: githubToken,
				ExpiresIn:    3600,
				ExpiresAt:    time.Now().Add(time.Hour).Unix(),
			}, nil
		},
	}

	provider, err := client.Get(t.Context())

	require.NoError(t, err)
	require.Equal(t, []catwalk.Model{{ID: "gpt-5.1", Name: "GPT 5.1", ContextWindow: 128000, DefaultMaxTokens: 16000}}, provider.Models)
	require.Equal(t, "refreshed-token", client.apiKey)
}

func TestCopilotModelsToCatwalkModelsFiltersAndDeduplicates(t *testing.T) {
	t.Parallel()

	models := copilotModelsToCatwalkModels(copilotModelsResponse{Data: []copilotModel{
		{
			ID:      "gpt-5.1",
			Name:    "GPT 5.1",
			Version: "gpt-5.1",
			Capabilities: copilotModelCapabilities{
				Limits: copilotModelLimits{MaxContextWindowTokens: 128000, MaxOutputTokens: 16000},
				Supports: copilotModelSupports{
					Vision:           true,
					ReasoningEffort:  []string{"low", "medium", "high"},
					AdaptiveThinking: true,
				},
			},
		},
		{
			ID:      "gpt-5.1",
			Name:    "GPT 5.1 Duplicate",
			Version: "gpt-5.1",
		},
		{
			ID:      "aliased-model",
			Name:    "Aliased Model",
			Version: "versioned-model-2026-01-01",
		},
		{
			ID:      "versioned-model-2026-01-01",
			Name:    "Versioned Model",
			Version: "versioned-model-2026-01-01",
		},
		{
			ID:      "unaliased-model-2026-01-01",
			Name:    "Unaliased Dated Model",
			Version: "unaliased-model-2026-01-01",
			Capabilities: copilotModelCapabilities{
				Type: "chat",
			},
		},
		{
			ID:   "some-embeddings-model",
			Name: "Embeddings via Type",
			Capabilities: copilotModelCapabilities{
				Type: "embeddings",
			},
		},
		{ID: "text-embedding-3", Name: "Embedding"},
		{ID: "accounts/msft/routers/test", Name: "Router"},
		{ID: "oswe-vscode-test", Name: "OSWE"},
		{ID: "lark-test", Name: "Lark"},
		{ID: "mai-code-test", Name: "MAI"},
		{ID: "gpt-4-o-preview", Name: "Preview"},
		{ID: "trajectory-compaction", Name: "Compaction"},
	}})

	require.Equal(t, []catwalk.Model{
		{
			ID:   "aliased-model",
			Name: "Aliased Model",
		},
		{
			ID:               "gpt-5.1",
			Name:             "GPT 5.1",
			ContextWindow:    128000,
			DefaultMaxTokens: 16000,
			CanReason:        true,
			ReasoningLevels:  []string{"low", "medium", "high"},
			SupportsImages:   true,
		},
		{
			ID:   "unaliased-model-2026-01-01",
			Name: "Unaliased Dated Model",
		},
	}, models)
}
