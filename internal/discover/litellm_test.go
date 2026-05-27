package discover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestLitellmEnricher(t *testing.T) {
	t.Parallel()

	t.Run("populates metadata from /model/info", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/model/info", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"model_name": "gpt-4o",
						"model_info": {
							"max_input_tokens": 128000,
							"max_output_tokens": 16384,
							"input_cost_per_token": 2.5e-06,
							"output_cost_per_token": 1e-05
						}
					},
					{
						"model_name": "claude-3-opus",
						"model_info": {
							"max_input_tokens": 200000,
							"max_output_tokens": 4096
						}
					}
				]
			}`))
		}))
		defer srv.Close()

		cfg := Config{
			ID:      "test-litellm",
			BaseURL: srv.URL,
			APIKey:  "test-key",
		}
		models := []catwalk.Model{
			{ID: "gpt-4o", Name: "gpt-4o"},
			{ID: "claude-3-opus", Name: "claude-3-opus"},
			{ID: "unknown-model", Name: "unknown-model"},
		}

		e := &litellmEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 3)

		// gpt-4o should have all fields populated.
		require.Equal(t, int64(128000), result[0].ContextWindow)
		require.Equal(t, int64(16384), result[0].DefaultMaxTokens)
		require.InDelta(t, 2.5, result[0].CostPer1MIn, 0.01)
		require.InDelta(t, 10.0, result[0].CostPer1MOut, 0.01)

		// claude-3-opus should have context window and max tokens.
		require.Equal(t, int64(200000), result[1].ContextWindow)
		require.Equal(t, int64(4096), result[1].DefaultMaxTokens)

		// unknown-model should be unchanged.
		require.Equal(t, int64(0), result[2].ContextWindow)
	})

	t.Run("preserves existing non-zero values", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [{
					"model_name": "gpt-4o",
					"model_info": {
						"max_input_tokens": 128000,
						"max_output_tokens": 16384
					}
				}]
			}`))
		}))
		defer srv.Close()

		cfg := Config{
			ID:      "test-litellm",
			BaseURL: srv.URL,
		}
		models := []catwalk.Model{
			{ID: "gpt-4o", Name: "GPT-4o Custom", ContextWindow: 200000, DefaultMaxTokens: 32768},
		}

		e := &litellmEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)

		// User overrides should be preserved.
		require.Equal(t, int64(200000), result[0].ContextWindow)
		require.Equal(t, int64(32768), result[0].DefaultMaxTokens)
		require.Equal(t, "GPT-4o Custom", result[0].Name)
	})

	t.Run("returns models unchanged on HTTP error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := Config{
			ID:      "test-litellm",
			BaseURL: srv.URL,
		}
		models := []catwalk.Model{{ID: "m1"}}

		e := &litellmEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, "m1", result[0].ID)
	})

	t.Run("sends auth and extra headers", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "Bearer my-key", r.Header.Get("Authorization"))
			require.Equal(t, "custom-val", r.Header.Get("X-Custom"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data": []}`))
		}))
		defer srv.Close()

		cfg := Config{
			ID:      "test-litellm",
			BaseURL: srv.URL,
			APIKey:  "my-key",
			ExtraHeaders: map[string]string{
				"X-Custom": "custom-val",
			},
		}

		e := &litellmEnricher{}
		_, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, nil)
		require.NoError(t, err)
	})
}

func TestGetEnricher(t *testing.T) {
	t.Parallel()

	require.NotNil(t, GetEnricher("litellm"))
	require.Nil(t, GetEnricher("openai-compat"))
	require.Nil(t, GetEnricher(""))
}

func TestIsKnownCustomProvider(t *testing.T) {
	t.Parallel()

	require.True(t, IsKnownCustomProvider("litellm"))
	require.True(t, IsKnownCustomProvider("ollama"))
	require.True(t, IsKnownCustomProvider("omlx"))
	require.True(t, IsKnownCustomProvider("lmstudio"))
	require.False(t, IsKnownCustomProvider("openai-compat"))
	require.False(t, IsKnownCustomProvider("anthropic"))
	require.False(t, IsKnownCustomProvider(""))
}
