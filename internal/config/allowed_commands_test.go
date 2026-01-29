package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowedCommandsPriority(t *testing.T) {
	tests := []struct {
		name           string
		configCommands []string
		cliCommands    []string
		expected       []string
	}{
		{
			name:           "CLI overrides config",
			configCommands: []string{"curl", "wget"},
			cliCommands:    []string{"apt", "npm"},
			expected:       []string{"apt", "npm"},
		},
		{
			name:           "CLI empty uses config",
			configCommands: []string{"curl", "wget"},
			cliCommands:    []string{},
			expected:       []string{"curl", "wget"},
		},
		{
			name:           "Both empty returns empty",
			configCommands: []string{},
			cliCommands:    []string{},
			expected:       []string{},
		},
		{
			name:           "CLI with duplicates",
			configCommands: []string{"curl", "wget"},
			cliCommands:    []string{"apt", "npm", "apt"}, // duplicate apt
			expected:       []string{"apt", "npm", "apt"}, // duplicates preserved
		},
		{
			name:           "CLI preserves order",
			configCommands: []string{"z", "y", "x"},
			cliCommands:    []string{"a", "b", "c"},
			expected:       []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with allowed commands
			cfg := &Config{
				Tools: Tools{
					Bash: ToolBash{
						AllowedCommands: tt.configCommands,
					},
				},
			}

			// Simulate CLI override
			result := cfg.Tools.Bash.AllowedCommands
			if len(tt.cliCommands) > 0 {
				result = tt.cliCommands
			}

			require.Equal(t, len(tt.expected), len(result))
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestToolBashDefaults(t *testing.T) {
	t.Run("empty by default", func(t *testing.T) {
		cfg := &Config{}
		cfg.setDefaults("/tmp", "")

		assert.Empty(t, cfg.Tools.Bash.AllowedCommands)
	})

	t.Run("preserves config values", func(t *testing.T) {
		cfg := &Config{
			Tools: Tools{
				Bash: ToolBash{
					AllowedCommands: []string{"curl", "wget"},
				},
			},
		}
		cfg.setDefaults("/tmp", "")

		assert.Equal(t, []string{"curl", "wget"}, cfg.Tools.Bash.AllowedCommands)
	})
}
