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

func TestOmlxEnricher(t *testing.T) {
	t.Parallel()

	t.Run("populates context window and max tokens from /v1/models/status", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// oMLX serves the status endpoint under /v1 (base URL
			// includes /v1), so the resolved path keeps the prefix.
			require.Equal(t, "/v1/models/status", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(omlxModelsStatusResponse{
				Models: []omlxModelStatus{
					{ID: "qwen3:latest", MaxContextWindow: ptr(int64(32768)), MaxTokens: ptr(int64(16384))},
					{ID: "llama3:latest", MaxContextWindow: ptr(int64(8192)), MaxTokens: ptr(int64(4096))},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-omlx", BaseURL: srv.URL + "/v1"}
		models := []catwalk.Model{
			{ID: "qwen3:latest", Name: "qwen3:latest"},
			{ID: "llama3:latest", Name: "llama3:latest"},
			{ID: "unknown:latest", Name: "unknown:latest"},
		}

		e := &omlxEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.Equal(t, int64(32768), result[0].ContextWindow)
		require.Equal(t, int64(16384), result[0].DefaultMaxTokens)
		require.Equal(t, int64(8192), result[1].ContextWindow)
		require.Equal(t, int64(4096), result[1].DefaultMaxTokens)
		require.Equal(t, int64(0), result[2].ContextWindow)
	})

	t.Run("preserves existing non-zero values", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(omlxModelsStatusResponse{
				Models: []omlxModelStatus{
					{ID: "m1", MaxContextWindow: ptr(int64(32768)), MaxTokens: ptr(int64(16384))},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-omlx", BaseURL: srv.URL}
		models := []catwalk.Model{
			{ID: "m1", ContextWindow: 65536, DefaultMaxTokens: 8192},
		}

		e := &omlxEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(65536), result[0].ContextWindow)
		require.Equal(t, int64(8192), result[0].DefaultMaxTokens)
	})

	t.Run("returns models unchanged on HTTP error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		cfg := Config{ID: "test-omlx", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1"}}

		e := &omlxEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 1)
	})
}

func ptr[T any](v T) *T { return &v }
