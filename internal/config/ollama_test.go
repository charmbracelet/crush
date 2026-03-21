package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

func TestDiscoverOllamaModels(t *testing.T) {
	t.Run("success with models", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/api/tags", r.URL.Path)
			resp := ollamaTagsResponse{
				Models: []ollamaModel{
					{Name: "llama3:latest", Details: ollamaDetails{ParameterSize: "8B"}},
					{Name: "qwen3:30b", Details: ollamaDetails{ParameterSize: "30B"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
		}))
		defer srv.Close()

		t.Setenv("OLLAMA_HOST", srv.URL)
		models, err := discoverOllamaModels(t.Context())
		require.NoError(t, err)
		require.Len(t, models, 2)
		require.Equal(t, "llama3:latest", models[0].ID)
		require.Equal(t, "qwen3:30b", models[1].ID)
	})

	t.Run("no models", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"models":[]}`)
		}))
		defer srv.Close()

		t.Setenv("OLLAMA_HOST", srv.URL)
		models, err := discoverOllamaModels(t.Context())
		require.NoError(t, err)
		require.Empty(t, models)
	})

	t.Run("server unreachable", func(t *testing.T) {
		t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1") // unlikely to be listening
		_, err := discoverOllamaModels(t.Context())
		require.Error(t, err)
	})

	t.Run("server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		t.Setenv("OLLAMA_HOST", srv.URL)
		_, err := discoverOllamaModels(t.Context())
		require.Error(t, err)
		require.Contains(t, err.Error(), "status 500")
	})
}

func TestMaybeAutoDetectOllama(t *testing.T) {
	t.Run("registers provider when Ollama is available", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := ollamaTagsResponse{
				Models: []ollamaModel{
					{Name: "mistral:latest"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
		}))
		defer srv.Close()

		t.Setenv("OLLAMA_HOST", srv.URL)
		cfg := &Config{}
		cfg.setDefaults("/tmp", "")

		maybeAutoDetectOllama(cfg)

		pc, ok := cfg.Providers.Get(ollamaProviderID)
		require.True(t, ok, "Ollama provider should be registered")
		require.Equal(t, ollamaProviderName, pc.Name)
		require.Equal(t, catwalk.TypeOpenAICompat, pc.Type)
		require.Len(t, pc.Models, 1)
		require.Equal(t, "mistral:latest", pc.Models[0].ID)
		require.Equal(t, srv.URL+ollamaOpenAIPath, pc.BaseURL)
	})

	t.Run("skips when already configured", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := ollamaTagsResponse{
				Models: []ollamaModel{
					{Name: "mistral:latest"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
		}))
		defer srv.Close()

		t.Setenv("OLLAMA_HOST", srv.URL)
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				ollamaProviderID: {
					BaseURL: "http://custom:11434/v1/",
					Models: []catwalk.Model{
						{ID: "custom-model", Name: "Custom Model"},
					},
				},
			}),
		}
		cfg.setDefaults("/tmp", "")

		maybeAutoDetectOllama(cfg)

		pc, ok := cfg.Providers.Get(ollamaProviderID)
		require.True(t, ok)
		// Should keep the user's custom config, not overwrite it.
		require.Equal(t, "http://custom:11434/v1/", pc.BaseURL)
		require.Len(t, pc.Models, 1)
		require.Equal(t, "custom-model", pc.Models[0].ID)
	})

	t.Run("skips when disabled", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := ollamaTagsResponse{
				Models: []ollamaModel{
					{Name: "mistral:latest"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
		}))
		defer srv.Close()

		t.Setenv("OLLAMA_HOST", srv.URL)
		cfg := &Config{}
		cfg.setDefaults("/tmp", "")
		cfg.Options.DisableOllamaAutoDetect = true

		maybeAutoDetectOllama(cfg)

		_, ok := cfg.Providers.Get(ollamaProviderID)
		require.False(t, ok, "Ollama provider should not be registered when disabled")
	})

	t.Run("no-op when Ollama is not running", func(t *testing.T) {
		t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")
		cfg := &Config{}
		cfg.setDefaults("/tmp", "")

		maybeAutoDetectOllama(cfg)

		_, ok := cfg.Providers.Get(ollamaProviderID)
		require.False(t, ok)
	})
}
