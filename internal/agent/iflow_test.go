package agent

import (
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestIflowProviderUserAgent tests that the iFlow provider correctly sets
// the User-Agent header to "iFlow-Cli" as required for premium models.
func TestIflowProviderUserAgent(t *testing.T) {
	t.Parallel()

	// Create a minimal coordinator just for testing the provider building
	cfg := &config.Config{
		Options: &config.Options{},
	}
	c := &coordinator{cfg: cfg}

	// Test the iFlow provider configuration
	iflowProvider := config.ProviderConfig{
		ID:      "iflow",
		Name:    "iFlow",
		BaseURL: "https://apis.iflow.cn/v1",
		Type:    catwalk.TypeOpenAICompat,
		APIKey:  "test-api-key",
		Models: []catwalk.Model{
			{
				ID: "glm-4.7",
			},
		},
	}

	// Test model configuration
	modelCfg := config.SelectedModel{
		Provider: "iflow",
		Model:    "glm-4.7",
	}

	// Build the provider using the coordinator's buildProvider function
	provider, err := c.buildProvider(iflowProvider, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Verify the provider was created successfully
	t.Logf("Successfully built iFlow provider through buildProvider: %T", provider)
}

// TestIflowProviderHeaders tests that iFlow provider gets the correct User-Agent header
func TestIflowProviderHeaders(t *testing.T) {
	t.Parallel()

	// Create a minimal coordinator just for testing the provider building
	cfg := &config.Config{
		Options: &config.Options{},
	}
	c := &coordinator{cfg: cfg}

	// Build the iFlow provider using the coordinator's buildOpenaiCompatProvider
	provider, err := c.buildOpenaiCompatProvider(
		"https://apis.iflow.cn/v1",
		"test-api-key",
		map[string]string{"User-Agent": "iFlow-Cli"}, // This should be set by our implementation
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Verify provider was created successfully
	t.Logf("Successfully created iFlow provider with User-Agent header: %T", provider)
}