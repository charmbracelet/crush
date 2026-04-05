package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

func TestEscalationIntegration(t *testing.T) {
	t.Run("escalation bridge is initialized", func(t *testing.T) {
		coord := &coordinator{
			escalationBridge: permission.NewEscalationBridge(),
		}

		require.NotNil(t, coord.EscalationBridge())
	})

	t.Run("worker identity is set in subagent context", func(t *testing.T) {
		ctx := context.Background()

		bridge := permission.NewEscalationBridge()
		identity := permission.WorkerIdentity{
			AgentID:   "worker-1",
			AgentName: "researcher",
			Color:     "blue",
		}

		ctx = permission.WithWorkerIdentity(ctx, identity)
		ctx = permission.WithEscalationBridge(ctx, bridge)

		retrieved := permission.WorkerIdentityFromContext(ctx)
		require.Equal(t, identity.AgentID, retrieved.AgentID)
		require.Equal(t, identity.AgentName, retrieved.AgentName)

		retrievedBridge := permission.EscalationBridgeFromContext(ctx)
		require.Same(t, bridge, retrievedBridge)
	})
}

func TestBackgroundAgentNaming(t *testing.T) {
	registry := newBackgroundAgentRegistry()

	t.Run("register with custom name", func(t *testing.T) {
		agentID := registry.RegisterNamed("researcher", "explore", "Research codebase", nil)
		require.NotEmpty(t, agentID)

		resolvedID, found := registry.ResolveAddress("researcher")
		require.True(t, found)
		require.Equal(t, agentID, resolvedID)

		resolvedID2, found := registry.ResolveAddress(agentID)
		require.True(t, found)
		require.Equal(t, agentID, resolvedID2)
	})

	t.Run("resume or create", func(t *testing.T) {
		runner := func(_ context.Context, _ backgroundAgentCommand) backgroundAgentRunResult {
			return backgroundAgentRunResult{Status: backgroundAgentStatusCompleted}
		}

		agentID1, resumed1 := registry.ResumeOrCreate("tester", "test", "Run tests", runner)
		require.NotEmpty(t, agentID1)
		require.False(t, resumed1)

		agentID2, resumed2 := registry.ResumeOrCreate("tester", "test", "Run tests", runner)
		require.Equal(t, agentID1, agentID2)
		require.True(t, resumed2)
	})

	t.Run("update artifacts", func(t *testing.T) {
		agentID := registry.RegisterNamed("builder", "build", "Build project", nil)

		registry.UpdateArtifacts(agentID,
			"Build completed",
			[]string{"/src/main.go", "/src/utils.go"},
			[]string{"binary:app", "session:build-1"},
		)

		entry, found := registry.Get(agentID)
		require.True(t, found)
		require.Equal(t, "Build completed", entry.Summary)
		require.Len(t, entry.FilesTouched, 2)
		require.Len(t, entry.Artifacts, 2)
	})
}

func TestEscalationUIHelpers(t *testing.T) {
	bridge := permission.NewEscalationBridge()
	ui := NewEscalationUI(bridge)

	t.Run("no pending escalations initially", func(t *testing.T) {
		require.False(t, ui.HasPendingEscalations())
		require.Empty(t, ui.GetPendingEscalations())
	})

	t.Run("approve escalation", func(t *testing.T) {
		req := permission.EscalationRequest{
			WorkerID:    "worker-1",
			WorkerName:  "researcher",
			WorkerColor: "blue",
			ToolName:    "bash",
			Description: "Run tests",
		}

		errCh := make(chan error, 1)
		go func() {
			deadline := time.After(500 * time.Millisecond)
			tick := time.NewTicker(5 * time.Millisecond)
			defer tick.Stop()
			for {
				select {
				case <-deadline:
					errCh <- fmt.Errorf("timeout waiting for pending escalation")
					return
				case <-tick.C:
					pending := ui.GetPendingEscalations()
					if len(pending) == 0 {
						continue
					}
					err := ui.ApproveEscalation(pending[0].RequestID, "Looks safe")
					errCh <- err
					return
				}
			}
		}()

		resp, err := bridge.RequestEscalation(context.Background(), req)
		require.NoError(t, err)
		require.True(t, resp.Approved)
		require.Equal(t, "Looks safe", resp.Reason)
		require.NoError(t, <-errCh)
	})

	t.Run("format escalation prompt", func(t *testing.T) {
		req := permission.EscalationRequest{
			WorkerName:  "researcher",
			WorkerColor: "blue",
			ToolName:    "bash",
			Description: "Remove temp files",
		}

		prompt := FormatEscalationPrompt(req)
		require.Contains(t, prompt, "[blue] researcher")
		require.Contains(t, prompt, "bash")
		require.Contains(t, prompt, "Remove temp files")
	})
}
