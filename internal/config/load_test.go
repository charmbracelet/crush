package config

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/resolver"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	exitVal := m.Run()
	os.Exit(exitVal)
}

func TestConfig_LoadFromReaders(t *testing.T) {
	data1 := strings.NewReader(`{"providers": {"openai": {"api_key": "key1", "base_url": "https://api.openai.com/v1"}}}`)
	data2 := strings.NewReader(`{"providers": {"openai": {"api_key": "key2", "base_url": "https://api.openai.com/v2"}}}`)
	data3 := strings.NewReader(`{"providers": {"openai": {}}}`)

	loadedConfig, err := loadFromReaders([]io.Reader{data1, data2, data3})

	assert.NoError(t, err)
	assert.NotNil(t, loadedConfig)
	assert.Equal(t, 1, loadedConfig.Providers.Len())
	pc, _ := loadedConfig.Providers.Get("openai")
	assert.Equal(t, "key2", pc.APIKey)
	assert.Equal(t, "https://api.openai.com/v2", pc.BaseURL)
}

func TestConfig_setDefaults(t *testing.T) {
	cfg := &Config{}

	cfg.setDefaults("/tmp")

	assert.NotNil(t, cfg.Options)
	assert.NotNil(t, cfg.Options.TUI)
	assert.NotNil(t, cfg.Options.ContextPaths)
	assert.NotNil(t, cfg.Providers)
	assert.NotNil(t, cfg.Models)
	assert.NotNil(t, cfg.LSP)
	assert.NotNil(t, cfg.MCP)
	assert.Equal(t, filepath.Join("/tmp", ".crush"), cfg.Options.DataDirectory)
	for _, path := range defaultContextPaths {
		assert.Contains(t, cfg.Options.ContextPaths, path)
	}
	assert.Equal(t, "/tmp", cfg.workingDir)
}

func TestConfig_configureProviders(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          "openai",
			APIKey:      "$OPENAI_API_KEY",
			APIEndpoint: "https://api.openai.com/v1",
			Models: []catwalk.Model{{
				ID: "test-model",
			}},
		},
	}

	cfg := &Config{}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{
		"OPENAI_API_KEY": "test-key",
	})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	assert.Equal(t, 1, cfg.Providers.Len())

	// We want to make sure that we keep the configured API key as a placeholder
	pc, _ := cfg.Providers.Get("openai")
	assert.Equal(t, "$OPENAI_API_KEY", pc.APIKey)
}

func TestConfig_configureProvidersWithOverride(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          "openai",
			APIKey:      "$OPENAI_API_KEY",
			APIEndpoint: "https://api.openai.com/v1",
			Models: []catwalk.Model{{
				ID: "test-model",
			}},
		},
	}

	cfg := &Config{
		Providers: csync.NewMap[string, provider.Config](),
	}
	cfg.Providers.Set("openai", provider.Config{
		APIKey:  "xyz",
		BaseURL: "https://api.openai.com/v2",
		Models: []catwalk.Model{
			{
				ID:   "test-model",
				Name: "Updated",
			},
			{
				ID: "another-model",
			},
		},
	})
	cfg.setDefaults("/tmp")

	env := env.NewFromMap(map[string]string{
		"OPENAI_API_KEY": "test-key",
	})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	assert.Equal(t, 1, cfg.Providers.Len())

	// We want to make sure that we keep the configured API key as a placeholder
	pc, _ := cfg.Providers.Get("openai")
	assert.Equal(t, "xyz", pc.APIKey)
	assert.Equal(t, "https://api.openai.com/v2", pc.BaseURL)
	assert.Len(t, pc.Models, 2)
	assert.Equal(t, "Updated", pc.Models[0].Name)
}

func TestConfig_configureProvidersWithNewProvider(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          "openai",
			APIKey:      "$OPENAI_API_KEY",
			APIEndpoint: "https://api.openai.com/v1",
			Models: []catwalk.Model{{
				ID: "test-model",
			}},
		},
	}

	cfg := &Config{
		Providers: csync.NewMapFrom(map[string]provider.Config{
			"custom": {
				APIKey:  "xyz",
				BaseURL: "https://api.someendpoint.com/v2",
				Models: []catwalk.Model{
					{
						ID: "test-model",
					},
				},
			},
		}),
	}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{
		"OPENAI_API_KEY": "test-key",
	})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	// Should be to because of the env variable
	assert.Equal(t, cfg.Providers.Len(), 2)

	// We want to make sure that we keep the configured API key as a placeholder
	pc, _ := cfg.Providers.Get("custom")
	assert.Equal(t, "xyz", pc.APIKey)
	// Make sure we set the ID correctly
	assert.Equal(t, "custom", pc.ID)
	assert.Equal(t, "https://api.someendpoint.com/v2", pc.BaseURL)
	assert.Len(t, pc.Models, 1)

	_, ok := cfg.Providers.Get("openai")
	assert.True(t, ok, "OpenAI provider should still be present")
}

func TestConfig_configureProvidersBedrockWithCredentials(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          catwalk.InferenceProviderBedrock,
			APIKey:      "",
			APIEndpoint: "",
			Models: []catwalk.Model{{
				ID: "anthropic.claude-sonnet-4-20250514-v1:0",
			}},
		},
	}

	cfg := &Config{}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{
		"AWS_ACCESS_KEY_ID":     "test-key-id",
		"AWS_SECRET_ACCESS_KEY": "test-secret-key",
	})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	assert.Equal(t, cfg.Providers.Len(), 1)

	bedrockProvider, ok := cfg.Providers.Get("bedrock")
	assert.True(t, ok, "Bedrock provider should be present")
	assert.Len(t, bedrockProvider.Models, 1)
	assert.Equal(t, "anthropic.claude-sonnet-4-20250514-v1:0", bedrockProvider.Models[0].ID)
}

func TestConfig_configureProvidersBedrockWithoutCredentials(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          catwalk.InferenceProviderBedrock,
			APIKey:      "",
			APIEndpoint: "",
			Models: []catwalk.Model{{
				ID: "anthropic.claude-sonnet-4-20250514-v1:0",
			}},
		},
	}

	cfg := &Config{}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	// Provider should not be configured without credentials
	assert.Equal(t, cfg.Providers.Len(), 0)
}

func TestConfig_configureProvidersBedrockWithoutUnsupportedModel(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          catwalk.InferenceProviderBedrock,
			APIKey:      "",
			APIEndpoint: "",
			Models: []catwalk.Model{{
				ID: "some-random-model",
			}},
		},
	}

	cfg := &Config{}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{
		"AWS_ACCESS_KEY_ID":     "test-key-id",
		"AWS_SECRET_ACCESS_KEY": "test-secret-key",
	})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.Error(t, err)
}

func TestConfig_configureProvidersVertexAIWithCredentials(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          catwalk.InferenceProviderVertexAI,
			APIKey:      "",
			APIEndpoint: "",
			Models: []catwalk.Model{{
				ID: "gemini-pro",
			}},
		},
	}

	cfg := &Config{}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{
		"GOOGLE_GENAI_USE_VERTEXAI": "true",
		"GOOGLE_CLOUD_PROJECT":      "test-project",
		"GOOGLE_CLOUD_LOCATION":     "us-central1",
	})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	assert.Equal(t, cfg.Providers.Len(), 1)

	vertexProvider, ok := cfg.Providers.Get("vertexai")
	assert.True(t, ok, "VertexAI provider should be present")
	assert.Len(t, vertexProvider.Models, 1)
	assert.Equal(t, "gemini-pro", vertexProvider.Models[0].ID)
	assert.Equal(t, "test-project", vertexProvider.ExtraParams["project"])
	assert.Equal(t, "us-central1", vertexProvider.ExtraParams["location"])
}

func TestConfig_configureProvidersVertexAIWithoutCredentials(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          catwalk.InferenceProviderVertexAI,
			APIKey:      "",
			APIEndpoint: "",
			Models: []catwalk.Model{{
				ID: "gemini-pro",
			}},
		},
	}

	cfg := &Config{}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{
		"GOOGLE_GENAI_USE_VERTEXAI": "false",
		"GOOGLE_CLOUD_PROJECT":      "test-project",
		"GOOGLE_CLOUD_LOCATION":     "us-central1",
	})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	// Provider should not be configured without proper credentials
	assert.Equal(t, cfg.Providers.Len(), 0)
}

func TestConfig_configureProvidersVertexAIMissingProject(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          catwalk.InferenceProviderVertexAI,
			APIKey:      "",
			APIEndpoint: "",
			Models: []catwalk.Model{{
				ID: "gemini-pro",
			}},
		},
	}

	cfg := &Config{}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{
		"GOOGLE_GENAI_USE_VERTEXAI": "true",
		"GOOGLE_CLOUD_LOCATION":     "us-central1",
	})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	// Provider should not be configured without project
	assert.Equal(t, cfg.Providers.Len(), 0)
}

func TestConfig_configureProvidersSetProviderID(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          "openai",
			APIKey:      "$OPENAI_API_KEY",
			APIEndpoint: "https://api.openai.com/v1",
			Models: []catwalk.Model{{
				ID: "test-model",
			}},
		},
	}

	cfg := &Config{}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{
		"OPENAI_API_KEY": "test-key",
	})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	assert.Equal(t, cfg.Providers.Len(), 1)

	// Provider ID should be set
	pc, _ := cfg.Providers.Get("openai")
	assert.Equal(t, "openai", pc.ID)
}

func TestConfig_EnabledProviders(t *testing.T) {
	t.Run("all providers enabled", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"openai": {
					ID:      "openai",
					APIKey:  "key1",
					Disable: false,
				},
				"anthropic": {
					ID:      "anthropic",
					APIKey:  "key2",
					Disable: false,
				},
			}),
		}

		enabled := cfg.EnabledProviders()
		assert.Len(t, enabled, 2)
	})

	t.Run("some providers disabled", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"openai": {
					ID:      "openai",
					APIKey:  "key1",
					Disable: false,
				},
				"anthropic": {
					ID:      "anthropic",
					APIKey:  "key2",
					Disable: true,
				},
			}),
		}

		enabled := cfg.EnabledProviders()
		assert.Len(t, enabled, 1)
		assert.Equal(t, "openai", enabled[0].ID)
	})

	t.Run("empty providers map", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMap[string, provider.Config](),
		}

		enabled := cfg.EnabledProviders()
		assert.Len(t, enabled, 0)
	})
}

func TestConfig_IsConfigured(t *testing.T) {
	t.Run("returns true when at least one provider is enabled", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"openai": {
					ID:      "openai",
					APIKey:  "key1",
					Disable: false,
				},
			}),
		}

		assert.True(t, cfg.IsConfigured())
	})

	t.Run("returns false when no providers are configured", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMap[string, provider.Config](),
		}

		assert.False(t, cfg.IsConfigured())
	})

	t.Run("returns false when all providers are disabled", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"openai": {
					ID:      "openai",
					APIKey:  "key1",
					Disable: true,
				},
				"anthropic": {
					ID:      "anthropic",
					APIKey:  "key2",
					Disable: true,
				},
			}),
		}

		assert.False(t, cfg.IsConfigured())
	})
}

func TestConfig_configureProvidersWithDisabledProvider(t *testing.T) {
	knownProviders := []catwalk.Provider{
		{
			ID:          "openai",
			APIKey:      "$OPENAI_API_KEY",
			APIEndpoint: "https://api.openai.com/v1",
			Models: []catwalk.Model{{
				ID: "test-model",
			}},
		},
	}

	cfg := &Config{
		Providers: csync.NewMapFrom(map[string]provider.Config{
			"openai": {
				Disable: true,
			},
		}),
	}
	cfg.setDefaults("/tmp")

	env := env.NewFromMap(map[string]string{
		"OPENAI_API_KEY": "test-key",
	})
	resolver := resolver.NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)

	// Provider should be removed from config when disabled
	assert.Equal(t, cfg.Providers.Len(), 0)
	_, exists := cfg.Providers.Get("openai")
	assert.False(t, exists)
}

func TestConfig_configureProvidersCustomProviderValidation(t *testing.T) {
	t.Run("custom provider with missing API key is allowed, but not known providers", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"custom": {
					BaseURL: "https://api.custom.com/v1",
					Models: []catwalk.Model{{
						ID: "test-model",
					}},
				},
				"openai": {
					APIKey: "$MISSING",
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, []catwalk.Provider{})
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 1)
		_, exists := cfg.Providers.Get("custom")
		assert.True(t, exists)
	})

	t.Run("custom provider with missing BaseURL is removed", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"custom": {
					APIKey: "test-key",
					Models: []catwalk.Model{{
						ID: "test-model",
					}},
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, []catwalk.Provider{})
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 0)
		_, exists := cfg.Providers.Get("custom")
		assert.False(t, exists)
	})

	t.Run("custom provider with no models is removed", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"custom": {
					APIKey:  "test-key",
					BaseURL: "https://api.custom.com/v1",
					Models:  []catwalk.Model{},
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, []catwalk.Provider{})
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 0)
		_, exists := cfg.Providers.Get("custom")
		assert.False(t, exists)
	})

	t.Run("custom provider with unsupported type is removed", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"custom": {
					APIKey:  "test-key",
					BaseURL: "https://api.custom.com/v1",
					Type:    "unsupported",
					Models: []catwalk.Model{{
						ID: "test-model",
					}},
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, []catwalk.Provider{})
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 0)
		_, exists := cfg.Providers.Get("custom")
		assert.False(t, exists)
	})

	t.Run("valid custom provider is kept and ID is set", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"custom": {
					APIKey:  "test-key",
					BaseURL: "https://api.custom.com/v1",
					Type:    catwalk.TypeOpenAI,
					Models: []catwalk.Model{{
						ID: "test-model",
					}},
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, []catwalk.Provider{})
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 1)
		customProvider, exists := cfg.Providers.Get("custom")
		assert.True(t, exists)
		assert.Equal(t, "custom", customProvider.ID)
		assert.Equal(t, "test-key", customProvider.APIKey)
		assert.Equal(t, "https://api.custom.com/v1", customProvider.BaseURL)
	})

	t.Run("custom anthropic provider is supported", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"custom-anthropic": {
					APIKey:  "test-key",
					BaseURL: "https://api.anthropic.com/v1",
					Type:    catwalk.TypeAnthropic,
					Models: []catwalk.Model{{
						ID: "claude-3-sonnet",
					}},
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, []catwalk.Provider{})
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 1)
		customProvider, exists := cfg.Providers.Get("custom-anthropic")
		assert.True(t, exists)
		assert.Equal(t, "custom-anthropic", customProvider.ID)
		assert.Equal(t, "test-key", customProvider.APIKey)
		assert.Equal(t, "https://api.anthropic.com/v1", customProvider.BaseURL)
		assert.Equal(t, catwalk.TypeAnthropic, customProvider.Type)
	})

	t.Run("disabled custom provider is removed", func(t *testing.T) {
		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"custom": {
					APIKey:  "test-key",
					BaseURL: "https://api.custom.com/v1",
					Type:    catwalk.TypeOpenAI,
					Disable: true,
					Models: []catwalk.Model{{
						ID: "test-model",
					}},
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, []catwalk.Provider{})
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 0)
		_, exists := cfg.Providers.Get("custom")
		assert.False(t, exists)
	})
}

func TestConfig_configureProvidersEnhancedCredentialValidation(t *testing.T) {
	t.Run("VertexAI provider removed when credentials missing with existing config", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:          catwalk.InferenceProviderVertexAI,
				APIKey:      "",
				APIEndpoint: "",
				Models: []catwalk.Model{{
					ID: "gemini-pro",
				}},
			},
		}

		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"vertexai": {
					BaseURL: "custom-url",
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{
			"GOOGLE_GENAI_USE_VERTEXAI": "false",
		})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 0)
		_, exists := cfg.Providers.Get("vertexai")
		assert.False(t, exists)
	})

	t.Run("Bedrock provider removed when AWS credentials missing with existing config", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:          catwalk.InferenceProviderBedrock,
				APIKey:      "",
				APIEndpoint: "",
				Models: []catwalk.Model{{
					ID: "anthropic.claude-sonnet-4-20250514-v1:0",
				}},
			},
		}

		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"bedrock": {
					BaseURL: "custom-url",
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 0)
		_, exists := cfg.Providers.Get("bedrock")
		assert.False(t, exists)
	})

	t.Run("provider removed when API key missing with existing config", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:          "openai",
				APIKey:      "$MISSING_API_KEY",
				APIEndpoint: "https://api.openai.com/v1",
				Models: []catwalk.Model{{
					ID: "test-model",
				}},
			},
		}

		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"openai": {
					BaseURL: "custom-url",
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 0)
		_, exists := cfg.Providers.Get("openai")
		assert.False(t, exists)
	})

	t.Run("known provider should still be added if the endpoint is missing the client will use default endpoints", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:          "openai",
				APIKey:      "$OPENAI_API_KEY",
				APIEndpoint: "$MISSING_ENDPOINT",
				Models: []catwalk.Model{{
					ID: "test-model",
				}},
			},
		}

		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"openai": {
					APIKey: "test-key",
				},
			}),
		}
		cfg.setDefaults("/tmp")

		env := env.NewFromMap(map[string]string{
			"OPENAI_API_KEY": "test-key",
		})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)

		assert.Equal(t, cfg.Providers.Len(), 1)
		_, exists := cfg.Providers.Get("openai")
		assert.True(t, exists)
	})
}

func TestConfig_defaultModelSelection(t *testing.T) {
	t.Run("default behavior uses the default models for given provider", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:                  "openai",
				APIKey:              "abc",
				DefaultLargeModelID: "large-model",
				DefaultSmallModelID: "small-model",
				Models: []catwalk.Model{
					{
						ID:               "large-model",
						DefaultMaxTokens: 1000,
					},
					{
						ID:               "small-model",
						DefaultMaxTokens: 500,
					},
				},
			},
		}

		cfg := &Config{}
		cfg.setDefaults("/tmp")
		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)

		large, small, err := cfg.defaultModelSelection(knownProviders)
		assert.NoError(t, err)
		assert.Equal(t, "large-model", large.Model)
		assert.Equal(t, "openai", large.Provider)
		assert.Equal(t, int64(1000), large.MaxTokens)
		assert.Equal(t, "small-model", small.Model)
		assert.Equal(t, "openai", small.Provider)
		assert.Equal(t, int64(500), small.MaxTokens)
	})
	t.Run("should error if no providers configured", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:                  "openai",
				APIKey:              "$MISSING_KEY",
				DefaultLargeModelID: "large-model",
				DefaultSmallModelID: "small-model",
				Models: []catwalk.Model{
					{
						ID:               "large-model",
						DefaultMaxTokens: 1000,
					},
					{
						ID:               "small-model",
						DefaultMaxTokens: 500,
					},
				},
			},
		}

		cfg := &Config{}
		cfg.setDefaults("/tmp")
		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)

		_, _, err = cfg.defaultModelSelection(knownProviders)
		assert.Error(t, err)
	})
	t.Run("should error if model is missing", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:                  "openai",
				APIKey:              "abc",
				DefaultLargeModelID: "large-model",
				DefaultSmallModelID: "small-model",
				Models: []catwalk.Model{
					{
						ID:               "not-large-model",
						DefaultMaxTokens: 1000,
					},
					{
						ID:               "small-model",
						DefaultMaxTokens: 500,
					},
				},
			},
		}

		cfg := &Config{}
		cfg.setDefaults("/tmp")
		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)
		_, _, err = cfg.defaultModelSelection(knownProviders)
		assert.Error(t, err)
	})

	t.Run("should configure the default models with a custom provider", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:                  "openai",
				APIKey:              "$MISSING", // will not be included in the config
				DefaultLargeModelID: "large-model",
				DefaultSmallModelID: "small-model",
				Models: []catwalk.Model{
					{
						ID:               "not-large-model",
						DefaultMaxTokens: 1000,
					},
					{
						ID:               "small-model",
						DefaultMaxTokens: 500,
					},
				},
			},
		}

		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"custom": {
					APIKey:  "test-key",
					BaseURL: "https://api.custom.com/v1",
					Models: []catwalk.Model{
						{
							ID:               "model",
							DefaultMaxTokens: 600,
						},
					},
				},
			}),
		}
		cfg.setDefaults("/tmp")
		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)
		large, small, err := cfg.defaultModelSelection(knownProviders)
		assert.NoError(t, err)
		assert.Equal(t, "model", large.Model)
		assert.Equal(t, "custom", large.Provider)
		assert.Equal(t, int64(600), large.MaxTokens)
		assert.Equal(t, "model", small.Model)
		assert.Equal(t, "custom", small.Provider)
		assert.Equal(t, int64(600), small.MaxTokens)
	})

	t.Run("should fail if no model configured", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:                  "openai",
				APIKey:              "$MISSING", // will not be included in the config
				DefaultLargeModelID: "large-model",
				DefaultSmallModelID: "small-model",
				Models: []catwalk.Model{
					{
						ID:               "not-large-model",
						DefaultMaxTokens: 1000,
					},
					{
						ID:               "small-model",
						DefaultMaxTokens: 500,
					},
				},
			},
		}

		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"custom": {
					APIKey:  "test-key",
					BaseURL: "https://api.custom.com/v1",
					Models:  []catwalk.Model{},
				},
			}),
		}
		cfg.setDefaults("/tmp")
		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)
		_, _, err = cfg.defaultModelSelection(knownProviders)
		assert.Error(t, err)
	})
	t.Run("should use the default provider first", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:                  "openai",
				APIKey:              "set",
				DefaultLargeModelID: "large-model",
				DefaultSmallModelID: "small-model",
				Models: []catwalk.Model{
					{
						ID:               "large-model",
						DefaultMaxTokens: 1000,
					},
					{
						ID:               "small-model",
						DefaultMaxTokens: 500,
					},
				},
			},
		}

		cfg := &Config{
			Providers: csync.NewMapFrom(map[string]provider.Config{
				"custom": {
					APIKey:  "test-key",
					BaseURL: "https://api.custom.com/v1",
					Models: []catwalk.Model{
						{
							ID:               "large-model",
							DefaultMaxTokens: 1000,
						},
					},
				},
			}),
		}
		cfg.setDefaults("/tmp")
		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)
		large, small, err := cfg.defaultModelSelection(knownProviders)
		assert.NoError(t, err)
		assert.Equal(t, "large-model", large.Model)
		assert.Equal(t, "openai", large.Provider)
		assert.Equal(t, int64(1000), large.MaxTokens)
		assert.Equal(t, "small-model", small.Model)
		assert.Equal(t, "openai", small.Provider)
		assert.Equal(t, int64(500), small.MaxTokens)
	})
}

func TestConfig_configureSelectedModels(t *testing.T) {
	t.Run("should override defaults", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:                  "openai",
				APIKey:              "abc",
				DefaultLargeModelID: "large-model",
				DefaultSmallModelID: "small-model",
				Models: []catwalk.Model{
					{
						ID:               "larger-model",
						DefaultMaxTokens: 2000,
					},
					{
						ID:               "large-model",
						DefaultMaxTokens: 1000,
					},
					{
						ID:               "small-model",
						DefaultMaxTokens: 500,
					},
				},
			},
		}

		cfg := &Config{
			Models: map[SelectedModelType]agent.Model{
				"large": {
					Model: "larger-model",
				},
			},
		}
		cfg.setDefaults("/tmp")
		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)

		err = cfg.configureSelectedModels(knownProviders)
		assert.NoError(t, err)
		large := cfg.Models[SelectedModelTypeLarge]
		small := cfg.Models[SelectedModelTypeSmall]
		assert.Equal(t, "larger-model", large.Model)
		assert.Equal(t, "openai", large.Provider)
		assert.Equal(t, int64(2000), large.MaxTokens)
		assert.Equal(t, "small-model", small.Model)
		assert.Equal(t, "openai", small.Provider)
		assert.Equal(t, int64(500), small.MaxTokens)
	})
	t.Run("should be possible to use multiple providers", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:                  "openai",
				APIKey:              "abc",
				DefaultLargeModelID: "large-model",
				DefaultSmallModelID: "small-model",
				Models: []catwalk.Model{
					{
						ID:               "large-model",
						DefaultMaxTokens: 1000,
					},
					{
						ID:               "small-model",
						DefaultMaxTokens: 500,
					},
				},
			},
			{
				ID:                  "anthropic",
				APIKey:              "abc",
				DefaultLargeModelID: "a-large-model",
				DefaultSmallModelID: "a-small-model",
				Models: []catwalk.Model{
					{
						ID:               "a-large-model",
						DefaultMaxTokens: 1000,
					},
					{
						ID:               "a-small-model",
						DefaultMaxTokens: 200,
					},
				},
			},
		}

		cfg := &Config{
			Models: map[SelectedModelType]agent.Model{
				"small": {
					Model:     "a-small-model",
					Provider:  "anthropic",
					MaxTokens: 300,
				},
			},
		}
		cfg.setDefaults("/tmp")
		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)

		err = cfg.configureSelectedModels(knownProviders)
		assert.NoError(t, err)
		large := cfg.Models[SelectedModelTypeLarge]
		small := cfg.Models[SelectedModelTypeSmall]
		assert.Equal(t, "large-model", large.Model)
		assert.Equal(t, "openai", large.Provider)
		assert.Equal(t, int64(1000), large.MaxTokens)
		assert.Equal(t, "a-small-model", small.Model)
		assert.Equal(t, "anthropic", small.Provider)
		assert.Equal(t, int64(300), small.MaxTokens)
	})

	t.Run("should override the max tokens only", func(t *testing.T) {
		knownProviders := []catwalk.Provider{
			{
				ID:                  "openai",
				APIKey:              "abc",
				DefaultLargeModelID: "large-model",
				DefaultSmallModelID: "small-model",
				Models: []catwalk.Model{
					{
						ID:               "large-model",
						DefaultMaxTokens: 1000,
					},
					{
						ID:               "small-model",
						DefaultMaxTokens: 500,
					},
				},
			},
		}

		cfg := &Config{
			Models: map[SelectedModelType]agent.Model{
				"large": {
					MaxTokens: 100,
				},
			},
		}
		cfg.setDefaults("/tmp")
		env := env.NewFromMap(map[string]string{})
		resolver := resolver.NewEnvironmentVariableResolver(env)
		err := cfg.configureProviders(env, resolver, knownProviders)
		assert.NoError(t, err)

		err = cfg.configureSelectedModels(knownProviders)
		assert.NoError(t, err)
		large := cfg.Models[SelectedModelTypeLarge]
		assert.Equal(t, "large-model", large.Model)
		assert.Equal(t, "openai", large.Provider)
		assert.Equal(t, int64(100), large.MaxTokens)
	})
}
