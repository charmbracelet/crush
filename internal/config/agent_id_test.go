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
		assert.Equal(t, "orchestrator", coderAgent.Role)
	})

	t.Run("Task alias should resolve to explore", func(t *testing.T) {
		_, ok := cfg.Agents[AgentTask]
		require.False(t, ok)
		assert.Equal(t, AgentExplore, CanonicalSubagentID(AgentTask))
	})

	t.Run("General agent should have correct ID", func(t *testing.T) {
		generalAgent, ok := cfg.Agents[AgentGeneral]
		require.True(t, ok)
		assert.Equal(t, AgentGeneral, generalAgent.ID, "General agent ID should be '%s'", AgentGeneral)
		assert.Equal(t, "executor", generalAgent.Role)
	})

	t.Run("Explore agent should have correct ID", func(t *testing.T) {
		exploreAgent, ok := cfg.Agents[AgentExplore]
		require.True(t, ok)
		assert.Equal(t, AgentExplore, exploreAgent.ID, "Explore agent ID should be '%s'", AgentExplore)
		assert.Equal(t, "reviewer", exploreAgent.Role)
	})
}
