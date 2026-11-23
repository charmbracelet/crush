package discover

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestOllamaEnricher(t *testing.T) {
	t.Parallel()

	t.Run("populates context window from /api/show", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/api/show", r.URL.Path)
			require.Equal(t, http.MethodPost, r.Method)

			var req map[string]string
			json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")
			switch req["model"] {
			case "llama3:latest":
				json.NewEncoder(w).Encode(ollamaShowResponse{
					ModelInfo: map[string]any{
						"llama.context_length": float64(8192),
					},
				})
			case "qwen2:latest":
				json.NewEncoder(w).Encode(ollamaShowResponse{
					ModelInfo: map[string]any{
						"qwen2.context_length": float64(32768),
					},
				})
			default:
				json.NewEncoder(w).Encode(ollamaShowResponse{})
			}
		}))
		defer srv.Close()

		// Base URL includes /v1 (as Crush configures it); the enricher
		// strips it so /api/show resolves at the server root.
		cfg := Config{ID: "test-ollama", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{
			{ID: "llama3:latest", Name: "llama3:latest"},
			{ID: "qwen2:latest", Name: "qwen2:latest"},
			{ID: "unknown:latest", Name: "unknown:latest"},
		}

		e := &ollamaEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.Equal(t, int64(8192), result[0].ContextWindow)
		require.Equal(t, int64(32768), result[1].ContextWindow)
		require.Equal(t, int64(0), result[2].ContextWindow)
	})

	t.Run("preserves existing context window", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ollamaShowResponse{
				ModelInfo: map[string]any{
					"llama.context_length": float64(8192),
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-ollama", BaseURL: srv.URL}
		models := []catwalk.Model{
			{ID: "llama3:latest", ContextWindow: 16384},
		}

		e := &ollamaEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(16384), result[0].ContextWindow)
	})

	t.Run("skips /api/show call when context window already set", func(t *testing.T) {
		t.Parallel()
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ollamaShowResponse{})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-ollama", BaseURL: srv.URL}
		models := []catwalk.Model{
			{ID: "m1", ContextWindow: 4096},
			{ID: "m2", ContextWindow: 8192},
		}

		e := &ollamaEnricher{}
		_, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, 0, calls)
	})
}

func TestOllamaEnricherKeepsToolCapableModels(t *testing.T) {
	t.Parallel()
	srv := newOllamaCapabilitiesServer(t, map[string]string{
		"tooled:latest": `{"model_info":{"llama.context_length":32768},"capabilities":["completion","tools"]}`,
	})
	models, err := (&ollamaEnricher{}).EnrichModels(t.Context(),
		Config{ID: "test-ollama", BaseURL: srv.URL + "/v1"}, &mockResolver{},
		[]catwalk.Model{{ID: "tooled:latest"}})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "tooled:latest", models[0].ID)
	require.Equal(t, int64(32768), models[0].ContextWindow)
}

func TestOllamaEnricherDoesNotRejectUnknownCapabilities(t *testing.T) {
	t.Parallel()
	srv := newOllamaCapabilitiesServer(t, map[string]string{
		"legacy:latest": `{"model_info":{"llama.context_length":8192}}`,
		"empty:latest":  `{"capabilities":[]}`,
	})
	models, err := (&ollamaEnricher{}).EnrichModels(t.Context(),
		Config{ID: "test-ollama", BaseURL: srv.URL + "/v1"}, &mockResolver{},
		[]catwalk.Model{{ID: "legacy:latest"}, {ID: "empty:latest"}})
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "legacy:latest", models[0].ID)
	require.Equal(t, int64(8192), models[0].ContextWindow)
	require.Equal(t, "empty:latest", models[1].ID)
}

func TestOllamaEnricherFiltersModelsWithoutTools(t *testing.T) {
	t.Parallel()
	// Literal responses ensure this test also exercises JSON field decoding.
	srv := newOllamaCapabilitiesServer(t, map[string]string{
		"plain:latest":   `{"model_info":{"llama.context_length":32768},"capabilities":["completion"]}`,
		"tooled:latest":  `{"model_info":{"llama.context_length":32768},"capabilities":["completion","tools"]}`,
		"no-info:latest": `{"capabilities":["completion"]}`,
		"legacy:latest":  `{}`,
	})
	models, err := (&ollamaEnricher{}).EnrichModels(t.Context(),
		Config{ID: "test-ollama", BaseURL: srv.URL + "/v1"}, &mockResolver{},
		[]catwalk.Model{
			{ID: "configured:latest", ContextWindow: 4096},
			{ID: "plain:latest"},
			{ID: "tooled:latest"},
			{ID: "no-info:latest"},
			{ID: "legacy:latest"},
		})
	require.NoError(t, err)
	require.Len(t, models, 3)
	require.Equal(t, "configured:latest", models[0].ID)
	require.Equal(t, int64(4096), models[0].ContextWindow)
	require.Equal(t, "tooled:latest", models[1].ID)
	require.Equal(t, int64(32768), models[1].ContextWindow)
	require.Equal(t, "legacy:latest", models[2].ID)
}

func newOllamaCapabilitiesServer(t *testing.T, responses map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		response, ok := responses[req["model"]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestExtractContextLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		info     map[string]any
		expected int64
	}{
		{"llama key", map[string]any{"llama.context_length": float64(8192)}, 8192},
		{"qwen2 key", map[string]any{"qwen2.context_length": float64(32768)}, 32768},
		{"no context key", map[string]any{"llama.block_count": float64(32)}, 0},
		{"empty map", map[string]any{}, 0},
		{"nil map", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, extractContextLength(tt.info))
		})
	}
}
