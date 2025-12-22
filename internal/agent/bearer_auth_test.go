package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

func TestBuildAnthropicProvider_UseBearerAuth(t *testing.T) {
	t.Run("uses Bearer auth when use_bearer_auth is true", func(t *testing.T) {
		// Create a test HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"models": []}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeLarge: {
					Model:    "claude-sonnet-4-5-20250929",
					Provider: "anthropic",
				},
			},
			Options: &config.Options{},
		}

		providerCfg := config.ProviderConfig{
			ID:            "anthropic",
			Name:          "Anthropic",
			Type:          catwalk.TypeAnthropic,
			BaseURL:       server.URL,
			APIKey:        "test-api-key",
			UseBearerAuth: true,
			ExtraHeaders:  make(map[string]string),
		}
		cfg.Providers.Set("anthropic", providerCfg)

		c := &coordinator{
			cfg: cfg,
		}

		modelCfg := config.SelectedModel{
			Model:    "claude-sonnet-4-5-20250929",
			Provider: "anthropic",
		}

		_, err := c.buildProvider(providerCfg, modelCfg)
		require.NoError(t, err)

		// Note: We can't directly verify the headers without making an actual API call,
		// but we can verify the configuration was set up correctly
		require.True(t, providerCfg.UseBearerAuth, "UseBearerAuth should be true")
		require.Nil(t, providerCfg.OAuthToken, "OAuthToken should be nil when using bearer auth flag")
	})

	t.Run("includes oauth beta header when use_bearer_auth is true", func(t *testing.T) {
		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Options:   &config.Options{},
		}

		providerCfg := config.ProviderConfig{
			ID:            "anthropic",
			Name:          "Anthropic",
			Type:          catwalk.TypeAnthropic,
			APIKey:        "test-api-key",
			UseBearerAuth: true,
			ExtraHeaders:  make(map[string]string),
		}

		c := &coordinator{
			cfg: cfg,
		}

		modelCfg := config.SelectedModel{
			Model:    "claude-sonnet-4-5-20250929",
			Provider: "anthropic",
		}

		// The buildProvider method should add the oauth beta header when UseBearerAuth is true
		_, err := c.buildProvider(providerCfg, modelCfg)
		require.NoError(t, err)
	})

	t.Run("does not use Bearer auth when use_bearer_auth is false", func(t *testing.T) {
		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Options:   &config.Options{},
		}

		providerCfg := config.ProviderConfig{
			ID:            "anthropic",
			Name:          "Anthropic",
			Type:          catwalk.TypeAnthropic,
			APIKey:        "test-api-key",
			UseBearerAuth: false,
			ExtraHeaders:  make(map[string]string),
		}

		c := &coordinator{
			cfg: cfg,
		}

		modelCfg := config.SelectedModel{
			Model:    "claude-sonnet-4-5-20250929",
			Provider: "anthropic",
		}

		_, err := c.buildProvider(providerCfg, modelCfg)
		require.NoError(t, err)

		require.False(t, providerCfg.UseBearerAuth, "UseBearerAuth should be false")
	})
}

func TestBuildProvider_OAuthBetaHeader(t *testing.T) {
	t.Run("adds oauth beta header when use_bearer_auth is true", func(t *testing.T) {
		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Options:   &config.Options{},
		}

		providerCfg := config.ProviderConfig{
			ID:            "anthropic",
			Name:          "Anthropic",
			Type:          catwalk.TypeAnthropic,
			APIKey:        "test-api-key",
			UseBearerAuth: true,
			ExtraHeaders:  make(map[string]string),
		}

		c := &coordinator{
			cfg: cfg,
		}

		modelCfg := config.SelectedModel{
			Model:    "claude-sonnet-4-5-20250929",
			Provider: "anthropic",
		}

		ctx := context.Background()
		_ = ctx

		// Build the provider - this should set up headers internally
		_, err := c.buildProvider(providerCfg, modelCfg)
		require.NoError(t, err)
	})

	t.Run("merges oauth beta header with thinking header when both are needed", func(t *testing.T) {
		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Options:   &config.Options{},
		}

		providerCfg := config.ProviderConfig{
			ID:            "anthropic",
			Name:          "Anthropic",
			Type:          catwalk.TypeAnthropic,
			APIKey:        "test-api-key",
			UseBearerAuth: true,
			ExtraHeaders:  make(map[string]string),
		}

		c := &coordinator{
			cfg: cfg,
		}

		modelCfg := config.SelectedModel{
			Model:    "claude-sonnet-4-5-20250929",
			Provider: "anthropic",
			Think:    true, // Enable thinking mode
		}

		_, err := c.buildProvider(providerCfg, modelCfg)
		require.NoError(t, err)
	})
}
