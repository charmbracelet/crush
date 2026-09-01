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

func TestGenericEnricher(t *testing.T) {
	t.Parallel()

	t.Run("populates context window from context_length", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/models", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(genericModelsResponse{
				Data: []genericModelEntry{
					{ID: "glm-4.7", ContextLength: 131072},
					{ID: "gemma-4", ContextLength: 262144},
					{ID: "no-context"},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{
			{ID: "glm-4.7"},
			{ID: "gemma-4"},
			{ID: "no-context"},
		}

		e := &genericEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.Equal(t, int64(131072), result[0].ContextWindow)
		require.Equal(t, int64(262144), result[1].ContextWindow)
		require.Equal(t, int64(0), result[2].ContextWindow)
	})

	t.Run("falls back to metadata.max_model_len", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(genericModelsResponse{
				Data: []genericModelEntry{
					{ID: "vllm-model", Metadata: map[string]any{"max_model_len": float64(65536)}},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{{ID: "vllm-model"}}

		e := &genericEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(65536), result[0].ContextWindow)
	})

	t.Run("falls back to metadata.max_context_length", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(genericModelsResponse{
				Data: []genericModelEntry{
					{ID: "ctx-model", Metadata: map[string]any{"max_context_length": float64(32768)}},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{{ID: "ctx-model"}}

		e := &genericEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(32768), result[0].ContextWindow)
	})

	t.Run("context_length takes precedence over metadata", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(genericModelsResponse{
				Data: []genericModelEntry{
					{ID: "both-model", ContextLength: 100000, Metadata: map[string]any{"max_model_len": float64(65536)}},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{{ID: "both-model"}}

		e := &genericEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(100000), result[0].ContextWindow)
	})

	t.Run("preserves existing non-zero context window", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(genericModelsResponse{
				Data: []genericModelEntry{
					{ID: "m1", ContextLength: 131072},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{{ID: "m1", ContextWindow: 65536}}

		e := &genericEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(65536), result[0].ContextWindow)
	})

	t.Run("returns models unchanged on HTTP error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		cfg := Config{ID: "test", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{{ID: "m1"}}

		e := &genericEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, int64(0), result[0].ContextWindow)
	})

	t.Run("returns models unchanged on invalid JSON", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("not json"))
		}))
		defer srv.Close()

		cfg := Config{ID: "test", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{{ID: "m1"}}

		e := &genericEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, int64(0), result[0].ContextWindow)
	})
}
