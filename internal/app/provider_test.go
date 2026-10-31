package app

import (
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestParseModelStr(t *testing.T) {
	tests := []struct {
		name            string
		modelStr        string
		expectedFilter  string
		expectedModelID string
		setupProviders  func() map[string]config.ProviderConfig
	}{
		{
			name:            "simple model with no slashes",
			modelStr:        "gpt-4o",
			expectedFilter:  "",
			expectedModelID: "gpt-4o",
			setupProviders:  setupMockProviders,
		},
		{
			name:            "valid provider and model",
			modelStr:        "openai/gpt-4o",
			expectedFilter:  "openai",
			expectedModelID: "gpt-4o",
			setupProviders:  setupMockProviders,
		},
		{
			name:            "model with multiple slashes and first part is invalid provider",
			modelStr:        "moonshot/kimi-k2",
			expectedFilter:  "",
			expectedModelID: "moonshot/kimi-k2",
			setupProviders:  setupMockProviders,
		},
		{
			name:            "full path with valid provider and model with slashes",
			modelStr:        "synthetic/moonshot/kimi-k2",
			expectedFilter:  "synthetic",
			expectedModelID: "moonshot/kimi-k2",
			setupProviders:  setupMockProvidersWithSlashes,
		},
		{
			name:            "empty model string",
			modelStr:        "",
			expectedFilter:  "",
			expectedModelID: "",
			setupProviders:  setupMockProviders,
		},
		{
			name:            "model with trailing slash but valid provider",
			modelStr:        "openai/",
			expectedFilter:  "openai",
			expectedModelID: "",
			setupProviders:  setupMockProviders,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers := tt.setupProviders()
			filter, modelID := parseModelStr(providers, tt.modelStr)

			require.Equal(t, tt.expectedFilter, filter, "provider filter mismatch")
			require.Equal(t, tt.expectedModelID, modelID, "model ID mismatch")
		})
	}
}

func setupMockProviders() map[string]config.ProviderConfig {
	return map[string]config.ProviderConfig{
		"openai": {
			ID:     "openai",
			Name:   "OpenAI",
			Models: []catwalk.Model{{ID: "gpt-4o"}, {ID: "gpt-4o-mini"}},
		},
		"anthropic": {
			ID:     "anthropic",
			Name:   "Anthropic",
			Models: []catwalk.Model{{ID: "claude-3-sonnet"}, {ID: "claude-3-opus"}},
		},
	}
}

func setupMockProvidersWithSlashes() map[string]config.ProviderConfig {
	return map[string]config.ProviderConfig{
		"synthetic": {
			ID:   "synthetic",
			Name: "Synthetic",
			Models: []catwalk.Model{
				{ID: "moonshot/kimi-k2"},
				{ID: "deepseek/deepseek-chat"},
			},
		},
		"openai": {
			ID:     "openai",
			Name:   "OpenAI",
			Models: []catwalk.Model{{ID: "gpt-4o"}},
		},
	}
}

func TestFindModels(t *testing.T) {
	tests := []struct {
		name             string
		modelStr         string
		expectedProvider string
		expectedModelID  string
		expectError      bool
		errorContains    string
		setupProviders   func() map[string]config.ProviderConfig
	}{
		{
			name:             "simple model found in one provider",
			modelStr:         "gpt-4o",
			expectedProvider: "openai",
			expectedModelID:  "gpt-4o",
			expectError:      false,
			setupProviders:   setupMockProviders,
		},
		{
			name:             "model with slashes in ID",
			modelStr:         "moonshot/kimi-k2",
			expectedProvider: "synthetic",
			expectedModelID:  "moonshot/kimi-k2",
			expectError:      false,
			setupProviders:   setupMockProvidersWithSlashes,
		},
		{
			name:             "provider and model with slashes in ID",
			modelStr:         "synthetic/moonshot/kimi-k2",
			expectedProvider: "synthetic",
			expectedModelID:  "moonshot/kimi-k2",
			expectError:      false,
			setupProviders:   setupMockProvidersWithSlashes,
		},
		{
			name:           "model not found",
			modelStr:       "nonexistent-model",
			expectError:    true,
			errorContains:  "not found",
			setupProviders: setupMockProviders,
		},
		{
			name:           "invalid provider specified",
			modelStr:       "nonexistent-provider/gpt-4o",
			expectError:    true,
			errorContains:  "provider",
			setupProviders: setupMockProviders,
		},
		{
			name:          "model found in multiple providers without provider filter",
			modelStr:      "shared-model",
			expectError:   true,
			errorContains: "multiple providers",
			setupProviders: func() map[string]config.ProviderConfig {
				return map[string]config.ProviderConfig{
					"openai": {
						ID:     "openai",
						Models: []catwalk.Model{{ID: "shared-model"}},
					},
					"anthropic": {
						ID:     "anthropic",
						Models: []catwalk.Model{{ID: "shared-model"}},
					},
				}
			},
		},
		{
			name:           "empty model string",
			modelStr:       "",
			expectError:    true,
			errorContains:  "not found",
			setupProviders: setupMockProviders,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers := tt.setupProviders()

			// Use findModels with the model as "large" and empty "small".
			matches, _, err := findModels(providers, tt.modelStr, "")
			if err != nil {
				if tt.expectError {
					require.Contains(t, err.Error(), tt.errorContains)
				} else {
					require.NoError(t, err)
				}
				return
			}

			// Validate the matches.
			match, err := validateMatches(matches, tt.modelStr, "large")

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedProvider, match.provider)
				require.Equal(t, tt.expectedModelID, match.modelID)
			}
		})
	}
}

func TestSubscriptionSmallModel(t *testing.T) {
	t.Parallel()

	ids := func(ids ...string) []catwalk.Model {
		models := make([]catwalk.Model, len(ids))
		for i, id := range ids {
			models[i] = catwalk.Model{ID: id}
		}
		return models
	}
	for _, tc := range []struct {
		provider string
		models   []catwalk.Model
		want     string
	}{
		{"openai", ids("gpt-5.1-codex", "gpt-5.1-codex-mini", "gpt-5"), "gpt-5.1-codex-mini"},
		{"xai", ids("grok-4", "grok-4-mini", "grok-4-fast"), "grok-4-fast"},
		{"copilot", ids("gpt-4.1", "claude-haiku-4.5", "claude-sonnet-4.5"), "claude-haiku-4.5"},
		{"copilot", ids("gpt-4.1", "o3"), "o3"},
	} {
		require.Equal(t, tc.want, subscriptionSmallModel(tc.provider, tc.models).ID, tc.provider)
	}
	require.Nil(t, subscriptionSmallModel("openai", nil))
}
