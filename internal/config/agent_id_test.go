package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_AgentIDs(t *testing.T) {
	cfg := &Config{
		Options: &Options{
			DisabledTools: []string{},
		},
	}
	cfg.SetupAgents()

	t.Run("Coder agent should have correct ID", func(t *testing.T) {
		coderAgent, ok := cfg.Agents[AgentCoder]
		require.True(t, ok)
		assert.Equal(t, AgentCoder, coderAgent.ID, "Coder agent ID should be '%s'", AgentCoder)
	})

	t.Run("Task agent should have correct ID", func(t *testing.T) {
		taskAgent, ok := cfg.Agents[AgentTask]
		require.True(t, ok)
		assert.Equal(t, AgentTask, taskAgent.ID, "Task agent ID should be '%s'", AgentTask)
	})

	t.Run("Task write agent should have correct ID", func(t *testing.T) {
		writeAgent, ok := cfg.Agents[AgentTaskWrite]
		require.True(t, ok)
		assert.Equal(t, AgentTaskWrite, writeAgent.ID, "Task write agent ID should be '%s'", AgentTaskWrite)
	})

	t.Run("Task write agent should have write tools without nesting", func(t *testing.T) {
		writeAgent := cfg.Agents[AgentTaskWrite]
		for _, toolName := range []string{"bash", "edit", "multiedit", "write", "download"} {
			assert.Contains(t, writeAgent.AllowedTools, toolName)
		}
		for _, toolName := range []string{"agent", "agentic_fetch", "question"} {
			assert.NotContains(t, writeAgent.AllowedTools, toolName)
		}

		taskAgent := cfg.Agents[AgentTask]
		for _, toolName := range []string{"bash", "edit", "multiedit", "write", "download"} {
			assert.NotContains(t, taskAgent.AllowedTools, toolName)
		}
	})

	t.Run("Plan agent should have correct ID", func(t *testing.T) {
		planAgent, ok := cfg.Agents[AgentPlan]
		require.True(t, ok)
		assert.Equal(t, AgentPlan, planAgent.ID, "Plan agent ID should be '%s'", AgentPlan)
	})
}
