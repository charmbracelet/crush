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

	t.Run("loaded instance caps configured context length", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Models: []lmstudioModelEntry{
					{
						Key:              "qwen/qwen3.6-27b",
						MaxContextLength: 262144,
						LoadedInstances: []lmstudioInstance{
							{Config: lmstudioInstanceConfig{ContextLength: 204800}},
						},
					},
				},
			})
		}))
		defer srv.Close()

		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL}
		models := []catwalk.Model{{
			ID:            "qwen/qwen3.6-27b",
			Name:          "Qwen 3.6 27B",
			ContextWindow: 262144,
		}}

		e := &lmstudioEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.Equal(t, int64(204800), result[0].ContextWindow)
	})

	t.Run("uses the smallest context across loaded instances", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(lmstudioModelsResponse{
				Models: []lmstudioModelEntry{
					{
						Key:              "m1",
						MaxContextLength: 262144,
						LoadedInstances: []lmstudioInstance{
							{Config: lmstudioInstanceConfig{ContextLength: 204800}},
							{Config: lmstudioInstanceConfig{ContextLength: 131072}},
							{Config: lmstudioInstanceConfig{ContextLength: 0}},
						},
					},
				},
			})
		}))
		defer srv.Close()

		result, err := (&lmstudioEnricher{}).EnrichModels(
			context.Background(),
			Config{ID: "test-lmstudio", BaseURL: srv.URL},
			&mockResolver{},
			[]catwalk.Model{{ID: "m1", ContextWindow: 230000}},
		)
		require.NoError(t, err)
		require.Equal(t, int64(131072), result[0].ContextWindow)
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

	t.Run("populates SupportsImages from capabilities.vision", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"models": [
					{
						"key": "vision-model",
						"display_name": "Vision Model",
						"max_context_length": 131072,
						"capabilities": {"vision": true}
					},
					{
						"key": "text-only-model",
						"display_name": "Text Only",
						"max_context_length": 32768,
						"capabilities": {"vision": false}
					},
					{
						"key": "no-capabilities",
						"display_name": "No Capabilities Field",
						"max_context_length": 8192
					}
				]
			}`))
		}))
		defer srv.Close()

		cfg := Config{ID: "test-lmstudio", BaseURL: srv.URL}
		models := []catwalk.Model{
			{ID: "vision-model", Name: "vision-model"},
			{ID: "text-only-model", Name: "text-only-model"},
			{ID: "no-capabilities", Name: "no-capabilities"},
		}

		e := &lmstudioEnricher{}
		result, err := e.EnrichModels(context.Background(), cfg, &mockResolver{}, models)
		require.NoError(t, err)
		require.True(t, result[0].SupportsImages, "vision model should have SupportsImages=true")
		require.False(t, result[1].SupportsImages, "text-only model should have SupportsImages=false")
		require.False(t, result[2].SupportsImages, "model without capabilities should default to false")
	})
}

func TestMinPositive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		values   []int64
		expected int64
	}{
		{name: "empty", expected: 0},
		{name: "ignores non-positive", values: []int64{0, -1, -100}, expected: 0},
		{name: "single", values: []int64{204800}, expected: 204800},
		{name: "smallest independent of order", values: []int64{262144, 131072, 204800}, expected: 131072},
		{name: "mixed", values: []int64{0, 204800, -1, 262144}, expected: 204800},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, minPositive(tt.values...))
		})
	}
}
