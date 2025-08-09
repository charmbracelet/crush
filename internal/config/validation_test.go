package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateProviderJSON(t *testing.T) {
	tests := []struct {
		name           string
		config         string
		expectedFields []string
		valid          bool
	}{
		{
			name: "valid configuration",
			config: `{
				"id": "test-provider",
				"name": "Test Provider",
				"type": "openai",
				"base_url": "https://api.test.com/v1",
				"api_key": "test-key",
				"models": [{
					"id": "test-model",
					"name": "Test Model",
					"cost_per_1m_in": 0.5,
					"cost_per_1m_out": 1.5,
					"cost_per_1m_in_cached": 0.25,
					"cost_per_1m_out_cached": 1.5,
					"context_window": 128000,
					"default_max_tokens": 4096,
					"can_reason": false,
					"has_reasoning_efforts": false,
					"supports_attachments": true
				}]
			}`,
			expectedFields: []string{},
			valid:          true,
		},
		{
			name: "invalid field context_windows",
			config: `{
				"id": "test-provider",
				"name": "Test Provider",
				"type": "openai",
				"base_url": "https://api.test.com/v1",
				"api_key": "test-key",
				"models": [{
					"id": "test-model",
					"name": "Test Model",
					"cost_per_1m_in": 0.5,
					"cost_per_1m_out": 1.5,
					"cost_per_1m_in_cached": 0.25,
					"cost_per_1m_out_cached": 1.5,
					"context_windows": 128000,
					"default_max_tokens": 4096,
					"can_reason": false,
					"has_reasoning_efforts": false,
					"supports_attachments": true
				}]
			}`,
			expectedFields: []string{"provider.models[0].context_windows"},
			valid:          false,
		},
		{
			name: "invalid provider field baseurl",
			config: `{
				"id": "test-provider",
				"name": "Test Provider",
				"type": "openai",
				"baseurl": "https://api.test.com/v1",
				"api_key": "test-key",
				"models": [{
					"id": "test-model",
					"name": "Test Model",
					"cost_per_1m_in": 0.5,
					"cost_per_1m_out": 1.5,
					"cost_per_1m_in_cached": 0.25,
					"cost_per_1m_out_cached": 1.5,
					"context_window": 128000,
					"default_max_tokens": 4096,
					"can_reason": false,
					"has_reasoning_efforts": false,
					"supports_attachments": true
				}]
			}`,
			expectedFields: []string{"provider.baseurl"},
			valid:          false,
		},
		{
			name: "multiple invalid fields",
			config: `{
				"id": "test-provider",
				"name": "Test Provider",
				"type": "openai",
				"baseurl": "https://api.test.com/v1",
				"apikey": "test-key",
				"models": [{
					"id": "test-model",
					"name": "Test Model",
					"cost_per_1m_in": 0.5,
					"costper1mout": 1.5,
					"cost_per_1m_in_cached": 0.25,
					"cost_per_1m_out_cached": 1.5,
					"context_windows": 128000,
					"default_max_tokens": 4096,
					"can_reason": false,
					"has_reasoning_efforts": false,
					"supports_attachments": true
				}]
			}`,
			expectedFields: []string{"provider.baseurl", "provider.apikey", "provider.models[0].costper1mout", "provider.models[0].context_windows"},
			valid:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rawJSON json.RawMessage
			err := json.Unmarshal([]byte(tt.config), &rawJSON)
			require.NoError(t, err)

			result := validateProviderJSON("test-provider", rawJSON)
			require.Equal(t, tt.valid, result.Valid)
			require.ElementsMatch(t, tt.expectedFields, result.UnknownFields)
		})
	}
}

func TestGetValidFields(t *testing.T) {
	tests := []struct {
		context  string
		expected []string
	}{
		{
			context: "provider",
			expected: []string{
				"id", "name", "base_url", "type", "api_key", "disable",
				"system_prompt_prefix", "extra_headers", "extra_body", "models",
			},
		},
		{
			context: "model",
			expected: []string{
				"id", "name", "cost_per_1m_in", "cost_per_1m_out", "cost_per_1m_in_cached",
				"cost_per_1m_out_cached", "context_window", "default_max_tokens",
				"can_reason", "has_reasoning_efforts", "default_reasoning_effort",
				"supports_attachments",
			},
		},
		{
			context: "unknown",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.context, func(t *testing.T) {
			result := getValidFields(tt.context)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestLoadReaderWithValidation(t *testing.T) {
	// Test that LoadReader properly validates configuration
	configWithInvalidField := `{
		"providers": {
			"test-provider": {
				"id": "test-provider",
				"name": "Test Provider",
				"type": "openai",
				"base_url": "https://api.test.com/v1",
				"api_key": "test-key",
				"models": [{
					"id": "test-model",
					"name": "Test Model",
					"cost_per_1m_in": 0.5,
					"cost_per_1m_out": 1.5,
					"cost_per_1m_in_cached": 0.25,
					"cost_per_1m_out_cached": 1.5,
					"context_windows": 128000,
					"default_max_tokens": 4096,
					"can_reason": false,
					"has_reasoning_efforts": false,
					"supports_attachments": true
				}]
			}
		}
	}`

	// This should not return an error, but should log warnings
	config, err := LoadReader(strings.NewReader(configWithInvalidField))
	require.NoError(t, err)
	require.NotNil(t, config)

	// The provider should still be loaded despite the invalid field
	provider, exists := config.Providers.Get("test-provider")
	require.True(t, exists)
	require.Equal(t, "test-provider", provider.ID)
	require.Equal(t, "Test Provider", provider.Name)

	// The model should have the correct context_window (not context_windows)
	require.Len(t, provider.Models, 1)
	require.Equal(t, "test-model", provider.Models[0].ID)
}

func TestLoadReaderValidConfig(t *testing.T) {
	// Test that valid configuration loads without issues
	validConfig := `{
		"providers": {
			"test-provider": {
				"id": "test-provider",
				"name": "Test Provider",
				"type": "openai",
				"base_url": "https://api.test.com/v1",
				"api_key": "test-key",
				"models": [{
					"id": "test-model",
					"name": "Test Model",
					"cost_per_1m_in": 0.5,
					"cost_per_1m_out": 1.5,
					"cost_per_1m_in_cached": 0.25,
					"cost_per_1m_out_cached": 1.5,
					"context_window": 128000,
					"default_max_tokens": 4096,
					"can_reason": false,
					"has_reasoning_efforts": false,
					"supports_attachments": true
				}]
			}
		}
	}`

	config, err := LoadReader(strings.NewReader(validConfig))
	require.NoError(t, err)
	require.NotNil(t, config)

	provider, exists := config.Providers.Get("test-provider")
	require.True(t, exists)
	require.Equal(t, "test-provider", provider.ID)
	require.Equal(t, 1, len(provider.Models))
	require.Equal(t, int64(128000), provider.Models[0].ContextWindow)
}
