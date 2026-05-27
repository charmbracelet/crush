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
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Data: []lmstudioModelEntry{
					{
						Key:              "qwen2.5-7b-instruct",
						DisplayName:      "Qwen 2.5 7B Instruct",
						MaxContextLength: 32768,
					},
					{
						Key:              "llama-3.1-8b",
						DisplayName:      "Llama 3.1 8B",
						MaxContextLength: 131072,
					},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL}
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

	t.Run("prefers loaded instance context length over model max", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Data: []lmstudioModelEntry{
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

	t.Run("preserves existing non-zero values", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Data: []lmstudioModelEntry{
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
				Data: []lmstudioModelEntry{
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
