package backend

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

type policyRecordingCoordinator struct {
	*blockingCoordinator
	policy permission.RequestPolicy
}

func (c *policyRecordingCoordinator) RunAccepted(ctx context.Context, _ *agent.AcceptedRun, _, _ string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	c.policy = permission.RequestPolicyFromContext(ctx)
	return nil, nil
}

func TestSendMessagePolicyOverridesWorkspaceContext(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	coord := &policyRecordingCoordinator{blockingCoordinator: newBlockingCoordinator()}
	ws := insertAgentWorkspace(t, b, coord)
	ws.ctx = permission.WithAutoApproveRequests(ws.ctx)
	require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "session", Prompt: "hello"}))
	ws.runWG.Wait()
	require.Equal(t, permission.RequestPolicyPrompt, coord.policy)
	require.Equal(t, permission.RequestPolicyAutoApprove, permission.RequestPolicyFromContext(ws.ctx), "the workspace context must not be mutated")
}
