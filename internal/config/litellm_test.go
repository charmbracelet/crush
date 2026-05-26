package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchLiteLLMModels(t *testing.T) {
	t.Parallel()

	t.Run("discovers models from /v1/models", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/models", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(litellmModelsResponse{
				Data: []litellmModel{
					{ID: "gpt-4o", Object: "model", OwnedBy: "openai"},
					{ID: "claude-3-opus", Object: "model", OwnedBy: "anthropic"},
				},
			})
		}))
		defer srv.Close()

		models, err := fetchLiteLLMModels(srv.URL, "")
		require.NoError(t, err)
		require.Len(t, models, 2)
		require.Equal(t, "gpt-4o", models[0].ID)
		require.Equal(t, "gpt-4o", models[0].Name)
		require.Equal(t, "claude-3-opus", models[1].ID)
		require.Equal(t, int64(128_000), models[0].ContextWindow)
		require.Equal(t, int64(16_384), models[0].DefaultMaxTokens)
	})

	t.Run("sends bearer token when api key is provided", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "Bearer sk-test-key", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(litellmModelsResponse{
				Data: []litellmModel{
					{ID: "model-1"},
				},
			})
		}))
		defer srv.Close()

		models, err := fetchLiteLLMModels(srv.URL, "sk-test-key")
		require.NoError(t, err)
		require.Len(t, models, 1)
	})

	t.Run("handles base URL with /v1 suffix", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/models", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(litellmModelsResponse{
				Data: []litellmModel{{ID: "model-1"}},
			})
		}))
		defer srv.Close()

		models, err := fetchLiteLLMModels(srv.URL+"/v1", "")
		require.NoError(t, err)
		require.Len(t, models, 1)
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		_, err := fetchLiteLLMModels(srv.URL, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "401")
	})

	t.Run("returns error when no models", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(litellmModelsResponse{Data: []litellmModel{}})
		}))
		defer srv.Close()

		_, err := fetchLiteLLMModels(srv.URL, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no models")
	})
}
