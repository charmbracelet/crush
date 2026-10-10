package workspace

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// mockBusyCoordinator mocks the coordinator interface without its own busy check,
// ensuring that the guard inside AppWorkspace.AgentSetMain is strictly exercised.
type mockBusyCoordinator struct {
	agent.Coordinator
	busy      bool
	mainAgent string
}

func (m *mockBusyCoordinator) IsBusy() bool {
	return m.busy
}

func (m *mockBusyCoordinator) SetMainAgent(agentID string) error {
	m.mainAgent = agentID
	return nil
}

// TestAppWorkspace_AgentSetMain_BusyGuard verifies that in local mode,
// AppWorkspace.AgentSetMain rejects switching the main agent while the coordinator
// is busy, matching backend.SetMainAgent behavior and preventing prompt stranding.
//
// Regression test for Issue #4029:
// https://github.com/charmbracelet/crush/issues/4029
func TestAppWorkspace_AgentSetMain_BusyGuard(t *testing.T) {
	t.Parallel()

	t.Run("busy coordinator rejects agent switch", func(t *testing.T) {
		coord := &mockBusyCoordinator{busy: true, mainAgent: config.AgentPlan}
		ws := &AppWorkspace{
			app: &app.App{
				AgentCoordinator: coord,
			},
		}

		err := ws.AgentSetMain(config.AgentCoder)
		require.Error(t, err, "AppWorkspace.AgentSetMain must reject switch while coordinator is busy")
		require.Equal(t, config.AgentPlan, coord.mainAgent, "main agent must remain unchanged on rejection")
	})

	t.Run("idle coordinator accepts agent switch", func(t *testing.T) {
		coord := &mockBusyCoordinator{busy: false, mainAgent: config.AgentPlan}
		ws := &AppWorkspace{
			app: &app.App{
				AgentCoordinator: coord,
			},
		}

		err := ws.AgentSetMain(config.AgentCoder)
		require.NoError(t, err, "AppWorkspace.AgentSetMain should succeed when coordinator is idle")
		require.Equal(t, config.AgentCoder, coord.mainAgent, "main agent must update to new agent")
	})
}
