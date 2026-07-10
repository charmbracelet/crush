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

func TestLlamacppEnricher(t *testing.T) {
	t.Parallel()

	t.Run("populates context window from meta.n_ctx", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/models", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(llamacppModelsResponse{
				Data: []llamacppModelEntry{
					{
						ID:   "ggml-org/Qwen2.5-Coder-1.5B-Instruct-Q8_0-GGUF",
						Meta: llamacppMeta{NCtx: 8192, NCtxTrain: 32768},
					},
					{
						ID:   "ggml-org/Llama-3.2-3B-Instruct-Q8_0-GGUF",
						Meta: llamacppMeta{NCtx: 4096, NCtxTrain: 131072},
					},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-llamacpp", BaseURL: srv.URL}
		models := []catwalk.Model{
			{ID: "ggml-org/Qwen2.5-Coder-1.5B-Instruct-Q8_0-GGUF"},
			{ID: "ggml-org/Llama-3.2-3B-Instruct-Q8_0-GGUF"},
			{ID: "unknown-model"},
		}

		e := &llamacppEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.Equal(t, int64(8192), result[0].ContextWindow)
		require.Equal(t, int64(4096), result[1].ContextWindow)
		require.Equal(t, int64(0), result[2].ContextWindow)
	})

	t.Run("falls back to n_ctx_train when n_ctx is zero", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(llamacppModelsResponse{
				Data: []llamacppModelEntry{
					{
						ID:   "m1",
						Meta: llamacppMeta{NCtx: 0, NCtxTrain: 131072},
					},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-llamacpp", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1"}}

		e := &llamacppEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(131072), result[0].ContextWindow)
	})

	t.Run("preserves existing non-zero context window", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(llamacppModelsResponse{
				Data: []llamacppModelEntry{
					{
						ID:   "m1",
						Meta: llamacppMeta{NCtx: 8192, NCtxTrain: 32768},
					},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-llamacpp", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1", ContextWindow: 65536}}

		e := &llamacppEnricher{}
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

		cfg := Config{ID: "test-llamacpp", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1"}}

		e := &llamacppEnricher{}
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

		cfg := Config{ID: "test-llamacpp", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1"}}

		e := &llamacppEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, int64(0), result[0].ContextWindow)
	})
}
