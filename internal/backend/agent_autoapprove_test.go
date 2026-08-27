package backend

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

// autoApproveCoordinator records what the dispatched run context said
// about auto-approval, which is the only way the flag reaches the agent.
type autoApproveCoordinator struct {
	errorCoordinator
	seen chan bool
}

func (c *autoApproveCoordinator) RunAccepted(ctx context.Context, _ *agent.AcceptedRun, _, _ string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	c.seen <- agent.AutoApproveFromContext(ctx)
	return nil, nil
}

// TestRunAgent_AutoApproveReachesTheRunContext pins the server half of
// the `crush run` contract for issue 3648: the prompt carries the
// approval, and the dispatched run — not the client's HTTP request — is
// what the agent sees it on.
func TestRunAgent_AutoApproveReachesTheRunContext(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		want bool
	}{
		{name: "requested", want: true},
		{name: "not requested", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			coord := &autoApproveCoordinator{seen: make(chan bool, 1)}
			ws := insertRunCompleteWorkspace(t, b, context.Background(), coord)

			require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{
				SessionID:   "S1",
				Prompt:      "hi",
				AutoApprove: tc.want,
			}))

			ws.runWG.Wait()
			require.Equal(t, tc.want, <-coord.seen)
		})
	}
}
