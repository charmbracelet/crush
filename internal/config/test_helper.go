package config

import (
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

// assertSelectedModel asserts selected model properties
func assertSelectedModel(t *testing.T, model SelectedModel, expectedModel, expectedProvider string, expectedMaxTokens int64) {
	require.Equal(t, expectedModel, model.Model)
	require.Equal(t, expectedProvider, model.Provider)
	require.Equal(t, expectedMaxTokens, model.MaxTokens)
}

// createOpenaiProvider creates a standard openai provider for testing
func createOpenaiProvider(hasAPIKey bool) catwalk.Provider {
	apiKey := "$MISSING"
	if hasAPIKey {
		apiKey = "set"
	}
	
	return catwalk.Provider{
		ID:                  "openai",
		APIKey:              apiKey,
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
	}
}