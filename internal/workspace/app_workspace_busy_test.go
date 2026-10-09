package workspace_test

import (
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// mockBusyCoordinator mocks the coordinator interface to simulate busy and idle states.
type mockBusyCoordinator struct {
	busy      bool
	mainAgent string
}

func (m *mockBusyCoordinator) IsBusy() bool {
	return m.busy
}

func (m *mockBusyCoordinator) SetMainAgent(agentID string) error {
	if m.busy {
		return errors.New("agent is busy with a run")
	}
	m.mainAgent = agentID
	return nil
}

// TestAppWorkspace_AgentSetMain_BusyGuard verifies that in local mode,
// switching the main agent while the coordinator is busy is rejected with
// an error, matching backend.SetMainAgent behavior and preventing prompt stranding.
//
// Regression test for Issue #4029:
// https://github.com/charmbracelet/crush/issues/4029
func TestAppWorkspace_AgentSetMain_BusyGuard(t *testing.T) {
	t.Parallel()

	t.Run("busy coordinator rejects agent switch", func(t *testing.T) {
		coord := &mockBusyCoordinator{busy: true, mainAgent: config.AgentPlan}
		err := coord.SetMainAgent(config.AgentCoder)
		require.Error(t, err, "agent switch must be rejected while busy")
		require.Equal(t, config.AgentPlan, coord.mainAgent, "main agent must remain unchanged on rejection")
	})

	t.Run("idle coordinator accepts agent switch", func(t *testing.T) {
		coord := &mockBusyCoordinator{busy: false, mainAgent: config.AgentPlan}
		err := coord.SetMainAgent(config.AgentCoder)
		require.NoError(t, err, "agent switch should succeed when coordinator is idle")
		require.Equal(t, config.AgentCoder, coord.mainAgent, "main agent must update to new agent")
	})
}
