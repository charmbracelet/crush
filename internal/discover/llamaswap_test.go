package discover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestLammaswapEnricher(t *testing.T) {
	t.Parallel()

	t.Run("populates context window and name from /models", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/models", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"data": [
					{
						"id": "qwen2.5-7b-instruct",
						"object": "model",
						"created": 1781831908,
						"owned_by": "llama-swap",
						"meta": {
							"llamaswap": {
								"name": "Qwen 2.5 7B Instruct",
								"context_window": 32768,
								"default_max_tokens": 8192
							}
						}
					},
					{
						"id": "llama-3.1-8b",
						"object": "model",
						"created": 1781837945,
						"owned_by": "llama-swap",
						"meta": {
							"llamaswap": {
								"name": "Llama 3.1 8B",
								"context_window": 131072,
								"default_max_tokens": 16384
							}
						}
					}
				]
			}`))
		}))
		defer srv.Close()

		cfg := Config{ID: "test-llamaswap", BaseURL: srv.URL}
		models := []catwalk.Model{
			{ID: "qwen2.5-7b-instruct", Name: "qwen2.5-7b-instruct"},
			{ID: "llama-3.1-8b", Name: "llama-3.1-8b"},
			{ID: "unknown-model", Name: "unknown-model"},
		}

		e := &llamaswapEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 3)
		require.Equal(t, int64(8192), result[0].DefaultMaxTokens)
		require.Equal(t, int64(32768), result[0].ContextWindow)
		require.Equal(t, "Qwen 2.5 7B Instruct", result[0].Name)
		require.Equal(t, int64(16384), result[1].DefaultMaxTokens)
		require.Equal(t, int64(131072), result[1].ContextWindow)
		require.Equal(t, "Llama 3.1 8B", result[1].Name)
		require.Equal(t, int64(0), result[2].DefaultMaxTokens)
		require.Equal(t, int64(0), result[2].ContextWindow)
		require.Equal(t, "unknown-model", result[2].Name)
	})

	t.Run("preserves existing non-zero values", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"data": [
					{
						"id": "m1",
						"object": "model",
						"created": 1781831908,
						"owned_by": "llama-swap",
						"meta": {
							"llamaswap": {
								"name": "Should Not Override",
								"context_window": 131072,
								"default_max_tokens": 8192
							}
						}
					}
				]
			}`))
		}))
		defer srv.Close()

		cfg := Config{ID: "test-llamaswap", BaseURL: srv.URL}
		models := []catwalk.Model{
			{ID: "m1", Name: "My Custom Name", ContextWindow: 65536},
		}

		e := &llamaswapEnricher{}
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

		cfg := Config{ID: "test-llamaswap", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1"}}

		e := &llamaswapEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, int64(0), result[0].ContextWindow)
	})

	t.Run("does not override custom name with display name", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"data": [
					{
						"id": "m1",
						"object": "model",
						"created": 1781831908,
						"owned_by": "llama-swap",
						"meta": {
							"llamaswap": {
								"name": "API Name"
							}
						}
					}
				]
			}`))
		}))
		defer srv.Close()

		cfg := Config{ID: "test-llamaswap", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1", Name: "User Name"}}

		e := &llamaswapEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, "User Name", result[0].Name)
	})

	t.Run("populates cost fields", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"data": [
					{
						"id": "m1",
						"object": "model",
						"created": 1781831908,
						"owned_by": "llama-swap",
						"meta": {
							"llamaswap": {
								"cost_per_1m_in": 2.5,
								"cost_per_1m_out": 10.0,
								"cost_per_1m_in_cached": 0.5,
								"cost_per_1m_out_cached": 0.5
							}
						}
					}
				]
			}`))
		}))
		defer srv.Close()

		cfg := Config{ID: "test-llamaswap", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1"}}

		e := &llamaswapEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, float64(2.5), result[0].CostPer1MIn)
		require.Equal(t, float64(10.0), result[0].CostPer1MOut)
		require.Equal(t, float64(0.5), result[0].CostPer1MInCached)
		require.Equal(t, float64(0.5), result[0].CostPer1MOutCached)
	})

	t.Run("populates reasoning levels", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"data": [
					{
						"id": "m1",
						"object": "model",
						"created": 1781831908,
						"owned_by": "llama-swap",
						"meta": {
							"llamaswap": {
								"reasoning_levels": ["low", "medium", "high"]
							}
						}
					}
				]
			}`))
		}))
		defer srv.Close()

		cfg := Config{ID: "test-llamaswap", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1"}}

		e := &llamaswapEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, []string{"low", "medium", "high"}, result[0].ReasoningLevels)
	})

	t.Run("handles empty data array", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data": []}`))
		}))
		defer srv.Close()

		cfg := Config{ID: "test-llamaswap", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1"}}

		e := &llamaswapEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 1)
	})

	t.Run("handles missing metadata", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"data": [
					{
						"id": "m1",
						"object": "model",
						"created": 1781831908,
						"owned_by": "llama-swap"
					}
				]
			}`))
		}))
		defer srv.Close()

		cfg := Config{ID: "test-llamaswap", BaseURL: srv.URL}
		models := []catwalk.Model{{ID: "m1"}}

		e := &llamaswapEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Len(t, result, 1)
	})
}
