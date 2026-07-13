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

	t.Run("Plan agent should have correct ID", func(t *testing.T) {
		planAgent, ok := cfg.Agents[AgentPlan]
		require.True(t, ok)
		assert.Equal(t, AgentPlan, planAgent.ID, "Plan agent ID should be '%s'", AgentPlan)
	})

	t.Run("Task agent should have correct ID", func(t *testing.T) {
		taskAgent, ok := cfg.Agents[AgentTask]
		require.True(t, ok)
		assert.Equal(t, AgentTask, taskAgent.ID, "Task agent ID should be '%s'", AgentTask)
	})

	t.Run("Goal agent should have correct ID", func(t *testing.T) {
		goalAgent, ok := cfg.Agents[AgentGoal]
		require.True(t, ok)
		assert.Equal(t, AgentGoal, goalAgent.ID, "Goal agent ID should be '%s'", AgentGoal)
	})

	t.Run("Review agent should have correct ID", func(t *testing.T) {
		reviewAgent, ok := cfg.Agents[AgentReview]
		require.True(t, ok)
		assert.Equal(t, AgentReview, reviewAgent.ID, "Review agent ID should be '%s'", AgentReview)
	})
}
