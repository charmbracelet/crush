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

// runFlags is what a dispatched run context says about the per-run flags
// the prompt carried.
type runFlags struct {
	autoApprove    bool
	nonInteractive bool
}

// runFlagsCoordinator records the flags on the dispatched run context,
// which is the only way they reach the agent.
type runFlagsCoordinator struct {
	errorCoordinator
	seen chan runFlags
}

func (c *runFlagsCoordinator) RunAccepted(ctx context.Context, _ *agent.AcceptedRun, _, _ string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	c.seen <- runFlags{
		autoApprove:    agent.AutoApproveFromContext(ctx),
		nonInteractive: agent.NonInteractiveFromContext(ctx),
	}
	return nil, nil
}

// TestRunAgent_FlagsReachTheRunContext pins the server half of the
// `crush run` contract for issue 3648: the prompt carries the flags, and
// the dispatched run — not the client's HTTP request — is what the agent
// sees them on.
//
// Both flags are per run, not per workspace, because one workspace
// serves an attached TUI and headless `crush run` prompts at the same
// time.
func TestRunAgent_FlagsReachTheRunContext(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		want runFlags
	}{
		{name: "neither"},
		{name: "auto approve only", want: runFlags{autoApprove: true}},
		{name: "non interactive only", want: runFlags{nonInteractive: true}},
		{name: "crush run", want: runFlags{autoApprove: true, nonInteractive: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)
			coord := &runFlagsCoordinator{seen: make(chan runFlags, 1)}
			ws := insertRunCompleteWorkspace(t, b, context.Background(), coord)

			require.NoError(t, b.SendMessage(ws.ID, proto.AgentMessage{
				SessionID:      "S1",
				Prompt:         "hi",
				AutoApprove:    tc.want.autoApprove,
				NonInteractive: tc.want.nonInteractive,
			}))

			ws.runWG.Wait()
			require.Equal(t, tc.want, <-coord.seen)
		})
	}
}
