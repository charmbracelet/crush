package discover

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestLmstudioEnricher(t *testing.T) {
	t.Parallel()

	t.Run("populates context window and name from /api/v1/models", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/api/v1/models", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			// Raw JSON matching LM Studio's real /api/v1/models wire
			// format: array under "models", not "data".
			_, _ = w.Write([]byte(`{
				"models": [
					{
						"key": "qwen2.5-7b-instruct",
						"display_name": "Qwen 2.5 7B Instruct",
						"max_context_length": 32768
					},
					{
						"key": "llama-3.1-8b",
						"display_name": "Llama 3.1 8B",
						"max_context_length": 131072
					}
				]
			}`))
		}))
		defer srv.Close()

		// Base URL includes /v1 (as Crush configures it); the enricher
		// strips it so the native endpoint resolves at the server root.
		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{
			{ID: "qwen2.5-7b-instruct", Name: "qwen2.5-7b-instruct"},
			{ID: "llama-3.1-8b", Name: "llama-3.1-8b"},
			{ID: "unknown-model", Name: "unknown-model"},
		}

		e := &lmstudioEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.Equal(t, int64(32768), result[0].ContextWindow)
		require.Equal(t, "Qwen 2.5 7B Instruct", result[0].Name)
		require.Equal(t, int64(131072), result[1].ContextWindow)
		require.Equal(t, "Llama 3.1 8B", result[1].Name)
		require.Equal(t, int64(0), result[2].ContextWindow)
		require.Equal(t, "unknown-model", result[2].Name)
	})

	t.Run("marks qwen3 architecture models as reasoning capable", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Models: []lmstudioModelEntry{
					{
						Key:          "qwythos-9b-claude-mythos-5-1m",
						DisplayName:  "Qwythos 9B Claude Mythos 5 1M",
						Architecture: "qwen35",
						Type:         "llm",
					},
					{
						Key:          "qwen2.5-3b-instruct",
						DisplayName:  "Qwen2.5 3B Instruct",
						Architecture: "qwen2",
						Type:         "llm",
					},
					{
						Key:          "text-embedding-nomic-embed-text-v1.5",
						DisplayName:  "Nomic Embed Text v1.5",
						Architecture: "",
						Type:         "embedding",
					},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{
			{ID: "qwythos-9b-claude-mythos-5-1m", Name: "qwythos-9b-claude-mythos-5-1m"},
			{ID: "qwen2.5-3b-instruct", Name: "qwen2.5-3b-instruct"},
			{ID: "text-embedding-nomic-embed-text-v1.5", Name: "text-embedding-nomic-embed-text-v1.5"},
		}

		e := &lmstudioEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.True(t, result[0].CanReason)
		require.False(t, result[1].CanReason)
		require.False(t, result[2].CanReason)
	})

	t.Run("propagates native vision capability", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Models: []lmstudioModelEntry{
					{Key: "vision-model", Capabilities: lmstudioCapabilities{Vision: true}},
					{Key: "text-model", Capabilities: lmstudioCapabilities{Vision: false}},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{
			{ID: "vision-model", Name: "vision-model"},
			{ID: "text-model", Name: "text-model"},
		}

		result, err := (&lmstudioEnricher{}).EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.True(t, result[0].SupportsImages)
		require.False(t, result[1].SupportsImages)
	})

	t.Run("prefers loaded instance context length over model max", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Models: []lmstudioModelEntry{
					{
						Key:              "m1",
						MaxContextLength: 131072,
						LoadedInstances: []lmstudioInstance{
							{Config: lmstudioInstanceConfig{ContextLength: 8192}},
						},
					},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1", Name: "m1"}}

		e := &lmstudioEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(8192), result[0].ContextWindow)
	})

	t.Run("enriches a loaded instance alias", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Models: []lmstudioModelEntry{
					{
						Key:              "qwythos-9b-claude-mythos-5-1m",
						DisplayName:      "Qwythos 9B Claude Mythos 5 1M",
						MaxContextLength: 1_048_576,
						Capabilities:     lmstudioCapabilities{Vision: true},
						LoadedInstances: []lmstudioInstance{
							{ID: "qwythos-9b", Config: lmstudioInstanceConfig{ContextLength: 32_768}},
						},
					},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{{ID: "qwythos-9b", Name: "qwythos-9b"}}
		result, err := (&lmstudioEnricher{}).EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(32_768), result[0].ContextWindow)
		require.Equal(t, "Qwythos 9B Claude Mythos 5 1M", result[0].Name)
		require.True(t, result[0].SupportsImages)
	})

	t.Run("preserves existing non-zero values", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Models: []lmstudioModelEntry{
					{
						Key:              "m1",
						DisplayName:      "Should Not Override",
						MaxContextLength: 131072,
					},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL}
		models := []catwalk.Model{
			{ID: "m1", Name: "My Custom Name", ContextWindow: 65536},
		}

		e := &lmstudioEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(65536), result[0].ContextWindow)
		require.Equal(t, "My Custom Name", result[0].Name)
	})

	t.Run("returns models unchanged on HTTP error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1"}}

		e := &lmstudioEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, int64(0), result[0].ContextWindow)
	})

	t.Run("does not override custom name with display name", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Models: []lmstudioModelEntry{
					{Key: "m1", DisplayName: "API Name"},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1", Name: "User Name"}}

		e := &lmstudioEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, "User Name", result[0].Name)
	})
}
