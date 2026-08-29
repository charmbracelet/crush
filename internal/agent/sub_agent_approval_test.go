package agent

import (
	"context"
	"testing"
	"time"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
)

// newDelegatingTool returns a tool that starts a child turn the way the
// `agent` tool does: on its own session, carrying the approval of the
// turn that called it.
func newDelegatingTool(child SessionAgent, childSessionID string, done chan<- error) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		"delegate",
		"Runs a child turn.",
		func(ctx context.Context, _ echoToolParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			_, err := child.Run(ctx, SessionAgentCall{
				SessionID:      childSessionID,
				Prompt:         "child work",
				SubAgent:       true,
				NonInteractive: true,
				AutoApprove:    AutoApproveFromContext(ctx),
			})
			done <- err
			return fantasy.NewTextResponse("delegated"), nil
		},
	)
}

// newApprovalProbeTool returns a tool that reports the approval its own
// turn is carrying, which is what a sub-agent tool call reads to decide
// whether the child turn is part of an approved run.
func newApprovalProbeTool(probes chan<- bool) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		"probe",
		"Reports the turn's approval.",
		func(ctx context.Context, _ echoToolParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			probes <- AutoApproveFromContext(ctx)
			return fantasy.NewTextResponse("probed"), nil
		},
	)
}

// oneToolCallScript drives a turn that makes exactly one call to
// toolName and then finishes.
func oneToolCallScript(callID, toolName string) []scriptedStep {
	return []scriptedStep{
		{
			toolCallID:   callID,
			toolName:     toolName,
			toolInput:    `{"value":"hi"}`,
			finishReason: fantasy.FinishReasonToolCalls,
		},
		{text: "done", finishReason: fantasy.FinishReasonStop},
	}
}

// TestRun_SubAgentTurnInheritsTheRunsApproval is the point of the
// change: `crush run --yolo` (or any non-interactive run) approves the
// run, not one session. A sub-agent turn started by one of that run's
// tool calls happens on a fresh child session that nobody has approved,
// so before this it blocked on a prompt no client could answer — the run
// hung until it was killed.
//
// The child turn must take its own hold, and give it back with the turn:
// the child session must be back to prompting afterwards.
func TestRun_SubAgentTurnInheritsTheRunsApproval(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	perms := permission.NewPermissionService(env.workingDir, false, nil)

	parentSess, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	childSess, err := env.sessions.CreateTaskSession(t.Context(), "child-id", parentSess.ID, "child")
	require.NoError(t, err)

	verdicts := make(chan bool, 1)
	childModel := &scriptedStreamModel{steps: oneToolCallScript("child-call", "ask")}
	child := autoApproveAgent(t, env, perms, childModel,
		newAskingTool(t, perms, childSess.ID, verdicts))

	childErrs := make(chan error, 1)
	parentModel := &scriptedStreamModel{steps: oneToolCallScript("parent-call", "delegate")}
	parent := autoApproveAgent(t, env, perms, parentModel,
		newDelegatingTool(child, childSess.ID, childErrs))

	_, err = parent.Run(t.Context(), SessionAgentCall{
		SessionID:   parentSess.ID,
		Prompt:      "delegate this",
		AutoApprove: true,
	})
	require.NoError(t, err)
	require.NoError(t, <-childErrs)

	require.True(t, <-verdicts,
		"a tool call in the sub-agent's turn is part of an approved run, so it must not prompt")

	granted, err := askPermission(t, perms, childSess.ID, 200*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded,
		"the child session must not stay approved once its turn is over")
	require.False(t, granted)
}

// TestRun_SubAgentTurnOfAnOrdinaryRunStillPrompts pins the other side:
// inheriting the approval must not mean sub-agent turns are always
// approved. A sub-agent started by an interactive turn still asks.
func TestRun_SubAgentTurnOfAnOrdinaryRunStillPrompts(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	perms := permission.NewPermissionService(env.workingDir, false, nil)

	parentSess, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	childSess, err := env.sessions.CreateTaskSession(t.Context(), "child-id", parentSess.ID, "child")
	require.NoError(t, err)

	verdicts := make(chan bool, 1)
	childModel := &scriptedStreamModel{steps: oneToolCallScript("child-call", "ask")}
	child := autoApproveAgent(t, env, perms, childModel,
		newAskingTool(t, perms, childSess.ID, verdicts))

	childErrs := make(chan error, 1)
	parentModel := &scriptedStreamModel{steps: oneToolCallScript("parent-call", "delegate")}
	parent := autoApproveAgent(t, env, perms, parentModel,
		newDelegatingTool(child, childSess.ID, childErrs))

	_, err = parent.Run(t.Context(), SessionAgentCall{
		SessionID: parentSess.ID,
		Prompt:    "delegate this",
	})
	require.NoError(t, err)
	require.NoError(t, <-childErrs)

	require.False(t, <-verdicts,
		"a sub-agent of a turn that was never approved must still wait for a human")
}

// TestRun_ToolCallsSeeTheirOwnTurnsApproval pins where the decision has
// to come from. A queued prompt is run by the frame of the turn it
// queued behind, so a tool call that reads the incoming context reads
// the *previous* turn's request. The decision must be stamped onto each
// turn's own run context instead.
func TestRun_ToolCallsSeeTheirOwnTurnsApproval(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	perms := permission.NewPermissionService(env.workingDir, false, nil)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	probes := make(chan bool, 2)
	large := &gatedToolStreamModel{
		gate:       make(chan struct{}),
		entered:    make(chan struct{}),
		toolName:   "probe",
		toolInput:  `{"value":"hi"}`,
		toolCallID: "call",
	}
	sa := autoApproveAgent(t, env, perms, large, newApprovalProbeTool(probes))

	mainDone := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{
			SessionID:   sess.ID,
			RunID:       "run-main",
			Prompt:      "main",
			AutoApprove: true,
		})
		mainDone <- runErr
	}()

	select {
	case <-large.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("main run never entered Stream")
	}
	require.True(t, sa.IsSessionBusy(sess.ID))

	// A RunID keeps the follow-up out of the fold path, so it runs as
	// its own turn — inside the main turn's frame.
	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-follow",
		Prompt:    "follow",
	})
	require.NoError(t, err)
	require.Nil(t, res)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	close(large.gate)
	require.NoError(t, <-mainDone)

	require.True(t, <-probes,
		"the approved turn's tool call must see the approval, even though its caller's context carried none")
	require.False(t, <-probes,
		"the queued turn asked for no approval, so its tool calls must not inherit the previous turn's")
}
