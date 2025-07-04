package config

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/pkg/env"
	"github.com/stretchr/testify/assert"
)

func TestConfig_LoadFromReaders(t *testing.T) {
	data1 := strings.NewReader(`{"providers": {"openai": {"api_key": "key1", "base_url": "https://api.openai.com/v1"}}}`)
	data2 := strings.NewReader(`{"providers": {"openai": {"api_key": "key2", "base_url": "https://api.openai.com/v2"}}}`)
	data3 := strings.NewReader(`{"providers": {"openai": {}}}`)

	loadedConfig, err := loadFromReaders([]io.Reader{data1, data2, data3})

	assert.NoError(t, err)
	assert.NotNil(t, loadedConfig)
	assert.Len(t, loadedConfig.Providers, 1)
	assert.Equal(t, "key2", loadedConfig.Providers["openai"].APIKey)
	assert.Equal(t, "https://api.openai.com/v2", loadedConfig.Providers["openai"].BaseURL)
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
	assert.Equal(t, "/tmp/.crush", cfg.Options.DataDirectory)
	for _, path := range defaultContextPaths {
		assert.Contains(t, cfg.Options.ContextPaths, path)
	}
	assert.Equal(t, "/tmp", cfg.workingDir)
}

func TestConfig_configureProviders(t *testing.T) {
	knownProviders := []provider.Provider{
		{
			ID:          "openai",
			APIKey:      "$OPENAI_API_KEY",
			APIEndpoint: "https://api.openai.com/v1",
			Models: []provider.Model{{
				ID: "test-model",
			}},
		},
	}

	cfg := &Config{}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{
		"OPENAI_API_KEY": "test-key",
	})
	resolver := NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	assert.Len(t, cfg.Providers, 1)

	// We want to make sure that we keep the configured API key as a placeholder
	assert.Equal(t, "$OPENAI_API_KEY", cfg.Providers["openai"].APIKey)
}

func TestConfig_configureProvidersWithOverride(t *testing.T) {
	knownProviders := []provider.Provider{
		{
			ID:          "openai",
			APIKey:      "$OPENAI_API_KEY",
			APIEndpoint: "https://api.openai.com/v1",
			Models: []provider.Model{{
				ID: "test-model",
			}},
		},
	}

	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"openai": {
				APIKey:  "xyz",
				BaseURL: "https://api.openai.com/v2",
				Models: []provider.Model{
					{
						ID:   "test-model",
						Name: "Updated",
					},
					{
						ID: "another-model",
					},
				},
			},
		},
	}
	cfg.setDefaults("/tmp")

	env := env.NewFromMap(map[string]string{
		"OPENAI_API_KEY": "test-key",
	})
	resolver := NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	assert.Len(t, cfg.Providers, 1)

	// We want to make sure that we keep the configured API key as a placeholder
	assert.Equal(t, "xyz", cfg.Providers["openai"].APIKey)
	assert.Equal(t, "https://api.openai.com/v2", cfg.Providers["openai"].BaseURL)
	assert.Len(t, cfg.Providers["openai"].Models, 2)
	assert.Equal(t, "Updated", cfg.Providers["openai"].Models[0].Name)
}

func TestConfig_configureProvidersWithNewProvider(t *testing.T) {
	knownProviders := []provider.Provider{
		{
			ID:          "openai",
			APIKey:      "$OPENAI_API_KEY",
			APIEndpoint: "https://api.openai.com/v1",
			Models: []provider.Model{{
				ID: "test-model",
			}},
		},
	}

	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"custom": {
				APIKey:  "xyz",
				BaseURL: "https://api.someendpoint.com/v2",
				Models: []provider.Model{
					{
						ID: "test-model",
					},
				},
			},
		},
	}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{
		"OPENAI_API_KEY": "test-key",
	})
	resolver := NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	// Should be to because of the env variable
	assert.Len(t, cfg.Providers, 2)

	// We want to make sure that we keep the configured API key as a placeholder
	assert.Equal(t, "xyz", cfg.Providers["custom"].APIKey)
	assert.Equal(t, "https://api.someendpoint.com/v2", cfg.Providers["custom"].BaseURL)
	assert.Len(t, cfg.Providers["custom"].Models, 1)

	_, ok := cfg.Providers["openai"]
	assert.True(t, ok, "OpenAI provider should still be present")
}

func TestConfig_configureProvidersBedrockWithCredentials(t *testing.T) {
	knownProviders := []provider.Provider{
		{
			ID:          provider.InferenceProviderBedrock,
			APIKey:      "",
			APIEndpoint: "",
			Models: []provider.Model{{
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
	resolver := NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	assert.Len(t, cfg.Providers, 1)

	bedrockProvider, ok := cfg.Providers["bedrock"]
	assert.True(t, ok, "Bedrock provider should be present")
	assert.Len(t, bedrockProvider.Models, 1)
	assert.Equal(t, "anthropic.claude-sonnet-4-20250514-v1:0", bedrockProvider.Models[0].ID)
}

func TestConfig_configureProvidersBedrockWithoutCredentials(t *testing.T) {
	knownProviders := []provider.Provider{
		{
			ID:          provider.InferenceProviderBedrock,
			APIKey:      "",
			APIEndpoint: "",
			Models: []provider.Model{{
				ID: "anthropic.claude-sonnet-4-20250514-v1:0",
			}},
		},
	}

	cfg := &Config{}
	cfg.setDefaults("/tmp")
	env := env.NewFromMap(map[string]string{})
	resolver := NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	// Provider should not be configured without credentials
	assert.Len(t, cfg.Providers, 0)
}

func TestConfig_configureProvidersBedrockWithoutUnsupportedModel(t *testing.T) {
	knownProviders := []provider.Provider{
		{
			ID:          provider.InferenceProviderBedrock,
			APIKey:      "",
			APIEndpoint: "",
			Models: []provider.Model{{
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
	resolver := NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.Error(t, err)
}

func TestConfig_configureProvidersVertexAIWithCredentials(t *testing.T) {
	knownProviders := []provider.Provider{
		{
			ID:          provider.InferenceProviderVertexAI,
			APIKey:      "",
			APIEndpoint: "",
			Models: []provider.Model{{
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
	resolver := NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	assert.Len(t, cfg.Providers, 1)

	vertexProvider, ok := cfg.Providers["vertexai"]
	assert.True(t, ok, "VertexAI provider should be present")
	assert.Len(t, vertexProvider.Models, 1)
	assert.Equal(t, "gemini-pro", vertexProvider.Models[0].ID)
	assert.Equal(t, "test-project", vertexProvider.ExtraParams["project"])
	assert.Equal(t, "us-central1", vertexProvider.ExtraParams["location"])
}

func TestConfig_configureProvidersVertexAIWithoutCredentials(t *testing.T) {
	knownProviders := []provider.Provider{
		{
			ID:          provider.InferenceProviderVertexAI,
			APIKey:      "",
			APIEndpoint: "",
			Models: []provider.Model{{
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
	resolver := NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	// Provider should not be configured without proper credentials
	assert.Len(t, cfg.Providers, 0)
}

func TestConfig_configureProvidersVertexAIMissingProject(t *testing.T) {
	knownProviders := []provider.Provider{
		{
			ID:          provider.InferenceProviderVertexAI,
			APIKey:      "",
			APIEndpoint: "",
			Models: []provider.Model{{
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
	resolver := NewEnvironmentVariableResolver(env)
	err := cfg.configureProviders(env, resolver, knownProviders)
	assert.NoError(t, err)
	// Provider should not be configured without project
	assert.Len(t, cfg.Providers, 0)
}
