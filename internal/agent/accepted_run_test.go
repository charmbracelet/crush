package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCancelTestAgent builds a DB-backed sessionAgent with no model. The
// tests here exercise the dispatch/cancel/persist paths, none of which
// reach agent.Stream, so a model is unnecessary.
func newCancelTestAgent(t *testing.T) (*sessionAgent, fakeEnv) {
	t.Helper()
	env := testEnv(t)
	sa := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)
	return sa, env
}

func (a *sessionAgent) acceptedCount(sessionID string) int {
	c, _ := a.acceptedRuns.Get(sessionID)
	return c
}

func (a *sessionAgent) hasPendingCancel(sessionID string) bool {
	mark, ok := a.cancelMark.Get(sessionID)
	return ok && mark > 0
}

func (a *sessionAgent) pendingCancelMark(sessionID string) uint64 {
	mark, _ := a.cancelMark.Get(sessionID)
	return mark
}

func TestAcceptedRun_CloseIsIdempotent(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	accept := sa.BeginAccepted("sid")
	require.Equal(t, "sid", accept.SessionID())
	require.Equal(t, 1, sa.acceptedCount("sid"))

	accept.Close()
	require.Equal(t, 0, sa.acceptedCount("sid"))

	// Repeated Close must not underflow the counter.
	accept.Close()
	accept.Close()
	require.Equal(t, 0, sa.acceptedCount("sid"))
}

func TestAcceptedRun_MultipleReservations(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	a1 := sa.BeginAccepted("sid")
	a2 := sa.BeginAccepted("sid")
	require.Equal(t, 2, sa.acceptedCount("sid"))

	a1.Close()
	require.Equal(t, 1, sa.acceptedCount("sid"))

	a2.Close()
	require.Equal(t, 0, sa.acceptedCount("sid"))
}

func TestAcceptedRun_NilSafe(t *testing.T) {
	t.Parallel()
	var accept *AcceptedRun
	require.Equal(t, "", accept.SessionID())
	// Must not panic.
	accept.Close()
}

func TestCancel_IdleDoesNotRecordPendingCancel(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	// No accepted run, no active request: a true no-op.
	sa.Cancel("sid")
	require.False(t, sa.hasPendingCancel("sid"))
}

func TestCancel_AcceptedRecordsPendingCancel(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	accept := sa.BeginAccepted("sid")
	defer accept.Close()

	sa.Cancel("sid")
	require.True(t, sa.hasPendingCancel("sid"))
}

func TestCancel_SecondCancelWhilePendingIsNoOp(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	accept := sa.BeginAccepted("sid")
	defer accept.Close()

	sa.Cancel("sid")
	require.True(t, sa.hasPendingCancel("sid"))

	// A second cancel while a pending cancel is already recorded must
	// remain a single pending cancel; one Run consumes exactly one.
	sa.Cancel("sid")
	require.True(t, sa.hasPendingCancel("sid"))
}

func TestRun_CancelOnEntryPersistsCanceledTurn(t *testing.T) {
	t.Parallel()
	sa, env := newCancelTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	accept := sa.BeginAccepted(sess.ID)
	// A cancel arrives in the accepted-but-not-yet-active window.
	sa.Cancel(sess.ID)
	require.True(t, sa.hasPendingCancel(sess.ID))

	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "hello",
		Accepted:  accept,
	})
	require.NoError(t, err)
	require.Nil(t, result)

	// The pending cancel was consumed and the accept released.
	require.False(t, sa.hasPendingCancel(sess.ID))
	require.Equal(t, 0, sa.acceptedCount(sess.ID))

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, message.User, msgs[0].Role)
	assert.Equal(t, message.Assistant, msgs[1].Role)
	assert.Equal(t, message.FinishReasonCanceled, msgs[1].FinishReason())
}

func TestPersistCanceledTurn_WritesBothWhenUserMissing(t *testing.T) {
	t.Parallel()
	sa, env := newCancelTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	err = sa.persistCanceledTurn(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "hello",
	}, false)
	require.NoError(t, err)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, message.User, msgs[0].Role)
	assert.Equal(t, message.Assistant, msgs[1].Role)
	assert.Equal(t, message.FinishReasonCanceled, msgs[1].FinishReason())
}

func TestPersistCanceledTurn_WritesAssistantOnlyWhenUserCreated(t *testing.T) {
	t.Parallel()
	sa, env := newCancelTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	// Simulate PrepareStep having already created the user message.
	_, err = sa.createUserMessage(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "hello",
	})
	require.NoError(t, err)

	err = sa.persistCanceledTurn(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "hello",
	}, true)
	require.NoError(t, err)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, message.User, msgs[0].Role)
	assert.Equal(t, message.Assistant, msgs[1].Role)
	assert.Equal(t, message.FinishReasonCanceled, msgs[1].FinishReason())
}

func TestPersistCanceledTurn_SucceedsWithCanceledContext(t *testing.T) {
	t.Parallel()
	sa, env := newCancelTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	// Simulate workspace shutdown having already canceled the run
	// context. WithoutCancel must let the writes through.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err = sa.persistCanceledTurn(ctx, SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "hello",
	}, false)
	require.NoError(t, err)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
}

func TestClearPendingCancel(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	accept := sa.BeginAccepted("sid")
	defer accept.Close()
	sa.Cancel("sid")
	require.True(t, sa.hasPendingCancel("sid"))

	sa.clearPendingCancel("sid")
	require.False(t, sa.hasPendingCancel("sid"))
}
