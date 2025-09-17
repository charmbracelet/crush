package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyLSPDefaults(t *testing.T) {
	t.Parallel()

	// Create a config with an LSP that should get defaults
	config := &Config{
		LSP: map[string]LSPConfig{
			"gopls": {
				Command: "gopls", // This should get defaults from powernap
			},
			"custom": {
				Command:     "custom-lsp",
				RootMarkers: []string{"custom.toml"}, // This should keep its explicit config
			},
		},
	}

	// Apply defaults
	config.applyLSPDefaults()

	// Check that gopls got defaults (it should have some root markers now)
	goplsConfig := config.LSP["gopls"]
	require.NotEmpty(t, goplsConfig.RootMarkers, "gopls should have received default root markers")

	// Check that custom LSP kept its explicit config
	customConfig := config.LSP["custom"]
	require.Equal(t, []string{"custom.toml"}, customConfig.RootMarkers, "custom LSP should keep its explicit root markers")
}
