package backend

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

type environmentCoordinator struct {
	errorCoordinator
	env []string
}

func (c *environmentCoordinator) RunAccepted(ctx context.Context, _ *agent.AcceptedRun, _, _ string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	c.env = tools.GetEnvironmentFromContext(ctx)
	return nil, nil
}

func TestRunAgent_WorkspaceEnvironment(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	for _, env := range [][]string{
		{"CRUSH_TEST_WORKSPACE=first"},
		{"CRUSH_TEST_WORKSPACE=second"},
		nil,
	} {
		coord := &environmentCoordinator{}
		ws := insertRunCompleteWorkspace(t, b, t.Context(), coord)
		ws.Env = env

		require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{
			SessionID: "workspace-env",
			Prompt:    "read the workspace environment",
		}))
		ws.runWG.Wait()
		require.Equal(t, env, coord.env)
		require.Nil(t, tools.GetEnvironmentFromContext(ws.ctx))
	}
}
