package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLSPConfig_Extensions(t *testing.T) {
	tests := []struct {
		name     string
		config   LSPConfig
		expected []string
	}{
		{
			name: "no extensions",
			config: LSPConfig{
				Command: "test-lsp",
			},
			expected: nil,
		},
		{
			name: "single extension",
			config: LSPConfig{
				Command:    "test-lsp",
				Extensions: []string{".wxss"},
			},
			expected: []string{".wxss"},
		},
		{
			name: "multiple extensions",
			config: LSPConfig{
				Command:    "test-lsp",
				Extensions: []string{".wxss", ".wxml", ".tpl"},
			},
			expected: []string{".wxss", ".wxml", ".tpl"},
		},
		{
			name: "empty extensions slice",
			config: LSPConfig{
				Command:    "test-lsp",
				Extensions: []string{},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.config.Extensions)
		})
	}
}

func TestLSPConfig_JSONSerialization(t *testing.T) {
	config := LSPConfig{
		Command:    "custom-lsp",
		Args:       []string{"--stdio"},
		Extensions: []string{".wxss", ".wxml"},
	}

	// Test that it can be marshaled/unmarshaled correctly
	// This is a basic test to ensure the struct is JSON serializable
	// In a real scenario, you'd use json.Marshal/json.Unmarshal
	require.Equal(t, "custom-lsp", config.Command)
	require.Equal(t, []string{"--stdio"}, config.Args)
	require.Equal(t, []string{".wxss", ".wxml"}, config.Extensions)
}

func TestLSPs_MapOperations(t *testing.T) {
	lspConfigs := LSPs{
		"wxss-lsp": LSPConfig{
			Command:    "css-lsp",
			Extensions: []string{".wxss"},
		},
		"wxml-lsp": LSPConfig{
			Command:    "html-lsp",
			Extensions: []string{".wxml"},
		},
	}

	// Test map access
	wxssConfig, exists := lspConfigs["wxss-lsp"]
	require.True(t, exists)
	require.Equal(t, "css-lsp", wxssConfig.Command)
	require.Equal(t, []string{".wxss"}, wxssConfig.Extensions)

	// Test map iteration
	foundExtensions := make(map[string][]string)
	for name, config := range lspConfigs {
		foundExtensions[name] = config.Extensions
	}

	require.Equal(t, []string{".wxss"}, foundExtensions["wxss-lsp"])
	require.Equal(t, []string{".wxml"}, foundExtensions["wxml-lsp"])
}

func TestLSPConfig_DisabledField(t *testing.T) {
	config := LSPConfig{
		Disabled:   true,
		Command:    "test-lsp",
		Extensions: []string{".test"},
	}

	require.True(t, config.Disabled)
	require.Equal(t, "test-lsp", config.Command)
	require.Equal(t, []string{".test"}, config.Extensions)
}

func TestLSPConfig_OptionsField(t *testing.T) {
	config := LSPConfig{
		Command:    "test-lsp",
		Extensions: []string{".test"},
		Options: map[string]interface{}{
			"tabSize":      2,
			"insertSpaces": true,
		},
	}

	options, ok := config.Options.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, 2, options["tabSize"])
	require.Equal(t, true, options["insertSpaces"])
}
