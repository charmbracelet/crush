package discover

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestLiteLLMEnricher(t *testing.T) {
	t.Parallel()

	t.Run("enriches models from /v1/model/info", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/model/info", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(litellmModelsResponse{
				Data: []litellmModelInfo{
					{
						ModelName: "gpt-4o",
						ModelInfo: &litellmModelDetail{
							MaxInput:  128000,
							MaxTokens: 16384,
						},
					},
					{
						ModelName: "claude-3-opus",
						ModelInfo: &litellmModelDetail{
							MaxInput:  200000,
							MaxTokens: 4096,
						},
					},
				},
			})
		}))
		defer srv.Close()

		e := &litellmEnricher{}
		models := []catwalk.Model{
			{ID: "gpt-4o", Name: "gpt-4o"},
			{ID: "claude-3-opus", Name: "claude-3-opus"},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		enriched, err := e.EnrichModels(ctx, Config{BaseURL: srv.URL}, models)
		require.NoError(t, err)
		require.Len(t, enriched, 2)
		for _, m := range enriched {
			switch m.ID {
			case "gpt-4o":
				require.Equal(t, int64(128000), m.ContextWindow)
				require.Equal(t, int64(16384), m.DefaultMaxTokens)
			case "claude-3-opus":
				require.Equal(t, int64(200000), m.ContextWindow)
				require.Equal(t, int64(4096), m.DefaultMaxTokens)
			default:
				t.Fatalf("unexpected model: %s", m.ID)
			}
		}
	})

	t.Run("sends bearer token when api key is provided", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "Bearer sk-test-key", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(litellmModelsResponse{
				Data: []litellmModelInfo{
					{
						ModelName: "gpt-4o",
						ModelInfo: &litellmModelDetail{MaxInput: 128000},
					},
				},
			})
		}))
		defer srv.Close()

		e := &litellmEnricher{}
		models := []catwalk.Model{{ID: "gpt-4o", Name: "gpt-4o"}}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		enriched, err := e.EnrichModels(ctx, Config{BaseURL: srv.URL, APIKey: "sk-test-key"}, models)
		require.NoError(t, err)
		require.Len(t, enriched, 1)
	})

	t.Run("handles base URL with /v1 suffix", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/model/info", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(litellmModelsResponse{
				Data: []litellmModelInfo{
					{
						ModelName: "model-1",
						ModelInfo: &litellmModelDetail{MaxInput: 32000},
					},
				},
			})
		}))
		defer srv.Close()

		e := &litellmEnricher{}
		models := []catwalk.Model{{ID: "model-1", Name: "model-1"}}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		enriched, err := e.EnrichModels(ctx, Config{BaseURL: srv.URL + "/v1"}, models)
		require.NoError(t, err)
		require.Len(t, enriched, 1)
		require.Equal(t, int64(32000), enriched[0].ContextWindow)
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		e := &litellmEnricher{}
		models := []catwalk.Model{{ID: "model-1", Name: "model-1"}}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := e.EnrichModels(ctx, Config{BaseURL: srv.URL}, models)
		require.Error(t, err)
		require.Contains(t, err.Error(), "401")
	})

	t.Run("falls back gracefully when info endpoint returns no data", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(litellmModelsResponse{Data: []litellmModelInfo{}})
		}))
		defer srv.Close()

		e := &litellmEnricher{}
		models := []catwalk.Model{{ID: "model-1", Name: "model-1"}}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		enriched, err := e.EnrichModels(ctx, Config{BaseURL: srv.URL}, models)
		require.NoError(t, err)
		require.Len(t, enriched, 1)
	})

	t.Run("uses model_info_map as fallback", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(litellmModelsResponse{
				Data: []litellmModelInfo{
					{
						ModelName: "gpt-4o",
						ModelInfoMap: &litellmModelDetail{
							MaxInput:  100000,
							MaxTokens: 8192,
						},
					},
				},
			})
		}))
		defer srv.Close()

		e := &litellmEnricher{}
		models := []catwalk.Model{{ID: "gpt-4o", Name: "gpt-4o"}}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		enriched, err := e.EnrichModels(ctx, Config{BaseURL: srv.URL}, models)
		require.NoError(t, err)
		require.Len(t, enriched, 1)
		require.Equal(t, int64(100000), enriched[0].ContextWindow)
		require.Equal(t, int64(8192), enriched[0].DefaultMaxTokens)
	})

	t.Run("is registered as known custom provider", func(t *testing.T) {
		require.True(t, IsKnownCustomProvider("litellm"))
	})
}
