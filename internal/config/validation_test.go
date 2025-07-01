package config

import (
	"testing"

	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Models: PreferredModels{
			Large: PreferredModel{
				ModelID:  "gpt-4",
				Provider: provider.InferenceProviderOpenAI,
			},
			Small: PreferredModel{
				ModelID:  "gpt-3.5-turbo",
				Provider: provider.InferenceProviderOpenAI,
			},
		},
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "test-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "gpt-4",
				DefaultSmallModel: "gpt-3.5-turbo",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
						CostPer1MIn:      30.0,
						CostPer1MOut:     60.0,
					},
					{
						ID:               "gpt-3.5-turbo",
						Name:             "GPT-3.5 Turbo",
						ContextWindow:    4096,
						DefaultMaxTokens: 2048,
						CostPer1MIn:      1.5,
						CostPer1MOut:     2.0,
					},
				},
			},
		},
		Agents: map[AgentID]Agent{
			AgentCoder: {
				ID:           AgentCoder,
				Name:         "Coder",
				Description:  "An agent that helps with executing coding tasks.",
				Model:        LargeModel,
				ContextPaths: []string{"CRUSH.md"},
			},
			AgentTask: {
				ID:           AgentTask,
				Name:         "Task",
				Description:  "An agent that helps with searching for context and finding implementation details.",
				Model:        LargeModel,
				ContextPaths: []string{"CRUSH.md"},
				AllowedTools: []string{"glob", "grep", "ls", "sourcegraph", "view"},
				AllowedMCP:   map[string][]string{},
				AllowedLSP:   []string{},
			},
		},
		MCP: map[string]MCP{},
		LSP: map[string]LSPConfig{},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_MissingAPIKey(t *testing.T) {
	cfg := &Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:           provider.InferenceProviderOpenAI,
				ProviderType: provider.TypeOpenAI,
				// Missing APIKey
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key is required")
}

func TestConfig_Validate_InvalidProviderType(t *testing.T) {
	cfg := &Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:           provider.InferenceProviderOpenAI,
				APIKey:       "test-key",
				ProviderType: provider.Type("invalid"),
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid provider type")
}

func TestConfig_Validate_CustomProviderMissingBaseURL(t *testing.T) {
	customProvider := provider.InferenceProvider("custom-provider")
	cfg := &Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			customProvider: {
				ID:           customProvider,
				APIKey:       "test-key",
				ProviderType: provider.TypeOpenAI,
				// Missing BaseURL for custom provider
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BaseURL is required for custom providers")
}

func TestConfig_Validate_DuplicateModelIDs(t *testing.T) {
	cfg := &Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:           provider.InferenceProviderOpenAI,
				APIKey:       "test-key",
				ProviderType: provider.TypeOpenAI,
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
					{
						ID:               "gpt-4", // Duplicate ID
						Name:             "GPT-4 Duplicate",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate model ID")
}

func TestConfig_Validate_InvalidModelFields(t *testing.T) {
	cfg := &Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:           provider.InferenceProviderOpenAI,
				APIKey:       "test-key",
				ProviderType: provider.TypeOpenAI,
				Models: []Model{
					{
						ID:               "", // Empty ID
						Name:             "GPT-4",
						ContextWindow:    0,    // Invalid context window
						DefaultMaxTokens: -1,   // Invalid max tokens
						CostPer1MIn:      -5.0, // Negative cost
					},
				},
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	validationErr := err.(ValidationErrors)
	assert.True(t, len(validationErr) >= 4) // Should have multiple validation errors
}

func TestConfig_Validate_DefaultModelNotFound(t *testing.T) {
	cfg := &Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:                provider.InferenceProviderOpenAI,
				APIKey:            "test-key",
				ProviderType:      provider.TypeOpenAI,
				DefaultLargeModel: "nonexistent-model",
				Models: []Model{
					{
						ID:               "gpt-4",
						Name:             "GPT-4",
						ContextWindow:    8192,
						DefaultMaxTokens: 4096,
					},
				},
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default large model 'nonexistent-model' not found")
}

func TestConfig_Validate_AgentIDMismatch(t *testing.T) {
	cfg := &Config{
		Agents: map[AgentID]Agent{
			AgentCoder: {
				ID:   AgentTask, // Wrong ID
				Name: "Coder",
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent ID mismatch")
}

func TestConfig_Validate_InvalidAgentModelType(t *testing.T) {
	cfg := &Config{
		Agents: map[AgentID]Agent{
			AgentCoder: {
				ID:    AgentCoder,
				Name:  "Coder",
				Model: ModelType("invalid"),
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid model type")
}

func TestConfig_Validate_UnknownTool(t *testing.T) {
	cfg := &Config{
		Agents: map[AgentID]Agent{
			AgentID("custom-agent"): {
				ID:           AgentID("custom-agent"),
				Name:         "Custom Agent",
				Model:        LargeModel,
				AllowedTools: []string{"unknown-tool"},
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestConfig_Validate_MCPReference(t *testing.T) {
	cfg := &Config{
		Agents: map[AgentID]Agent{
			AgentID("custom-agent"): {
				ID:         AgentID("custom-agent"),
				Name:       "Custom Agent",
				Model:      LargeModel,
				AllowedMCP: map[string][]string{"nonexistent-mcp": nil},
			},
		},
		MCP: map[string]MCP{}, // Empty MCP map
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "referenced MCP 'nonexistent-mcp' not found")
}

func TestConfig_Validate_InvalidMCPType(t *testing.T) {
	cfg := &Config{
		MCP: map[string]MCP{
			"test-mcp": {
				Type: MCPType("invalid"),
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid MCP type")
}

func TestConfig_Validate_MCPMissingCommand(t *testing.T) {
	cfg := &Config{
		MCP: map[string]MCP{
			"test-mcp": {
				Type: MCPStdio,
				// Missing Command
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command is required for stdio MCP")
}

func TestConfig_Validate_LSPMissingCommand(t *testing.T) {
	cfg := &Config{
		LSP: map[string]LSPConfig{
			"test-lsp": {
				// Missing Command
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command is required for LSP")
}

func TestConfig_Validate_NoValidProviders(t *testing.T) {
	cfg := &Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:           provider.InferenceProviderOpenAI,
				APIKey:       "test-key",
				ProviderType: provider.TypeOpenAI,
				Disabled:     true, // Disabled
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one non-disabled provider is required")
}

func TestConfig_Validate_MissingDefaultAgents(t *testing.T) {
	cfg := &Config{
		Providers: map[provider.InferenceProvider]ProviderConfig{
			provider.InferenceProviderOpenAI: {
				ID:           provider.InferenceProviderOpenAI,
				APIKey:       "test-key",
				ProviderType: provider.TypeOpenAI,
			},
		},
		Agents: map[AgentID]Agent{}, // Missing default agents
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "coder agent is required")
	assert.Contains(t, err.Error(), "task agent is required")
}

func TestConfig_Validate_KnownAgentProtection(t *testing.T) {
	cfg := &Config{
		Agents: map[AgentID]Agent{
			AgentCoder: {
				ID:          AgentCoder,
				Name:        "Modified Coder",       // Should not be allowed
				Description: "Modified description", // Should not be allowed
				Model:       LargeModel,
			},
		},
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "coder agent name cannot be changed")
	assert.Contains(t, err.Error(), "coder agent description cannot be changed")
}

func TestConfig_Validate_EmptyDataDirectory(t *testing.T) {
	cfg := &Config{
		Options: Options{
			DataDirectory: "", // Empty
			ContextPaths:  []string{"CRUSH.md"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data directory is required")
}

func TestConfig_Validate_EmptyContextPath(t *testing.T) {
	cfg := &Config{
		Options: Options{
			DataDirectory: ".crush",
			ContextPaths:  []string{""}, // Empty context path
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context path cannot be empty")
}
