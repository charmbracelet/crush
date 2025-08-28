package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentConfigurationIntegration(t *testing.T) {
	t.Parallel()

	// Test JSON unmarshaling and agent setup
	configJSON := `{
		"agents": {
			"coder": {
				"model": "small"
			},
			"task": {
				"model": "large"
			}
		}
	}`

	var cfg Config
	err := json.Unmarshal([]byte(configJSON), &cfg)
	require.NoError(t, err)

	// Initialize required fields
	cfg.Options = &Options{}
	
	// Setup agents
	cfg.SetupAgents()

	// Verify coder agent configuration
	coderAgent, exists := cfg.GetAgent(AgentIDCoder)
	require.True(t, exists)
	require.Equal(t, SelectedModelTypeSmall, coderAgent.Model)
	require.Equal(t, AgentIDCoder, coderAgent.ID)
	require.Equal(t, "Coder", coderAgent.Name)

	// Verify task agent configuration
	taskAgent, exists := cfg.GetAgent(AgentIDTask)
	require.True(t, exists)
	require.Equal(t, SelectedModelTypeLarge, taskAgent.Model)
	require.Equal(t, AgentIDTask, taskAgent.ID)
	require.Equal(t, "Task", taskAgent.Name)

	// Verify task agent has limited tools
	require.Equal(t, []string{"glob", "grep", "ls", "sourcegraph", "view"}, taskAgent.AllowedTools)
	require.Equal(t, map[string][]string{}, taskAgent.AllowedMCP)
	require.Equal(t, []string{}, taskAgent.AllowedLSP)
}