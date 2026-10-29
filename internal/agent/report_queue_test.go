package agent

import (
	"errors"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// A report is saved before the parent is woken, so a turn that fails to
// start, or is cancelled, cannot lose it.
func TestReportIsSavedEvenWhenWakingFails(t *testing.T) {
	t.Parallel()
	c, env := newDispatchTestCoordinator(t)
	c.readyWg.Go(func() error { return errors.New("not ready") })
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	entry := newDispatchedAgent("betty-review", accessRead, "child", parent.ID, newMockAgent("p", 4096, nil))

	c.reportDispatchResult(entry, fantasy.NewTextResponse("all good"), nil)

	msgs, err := env.messages.List(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	reports := msgs[0].SubAgentReports()
	require.Len(t, reports, 1)
	require.Equal(t, "all good", reports[0].Output)
}

// A call delivering a saved message writes nothing new.
func TestSavedCallWritesNoUserMessage(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Models:   noModels,
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)
	saved := message.Message{ID: "saved", SessionID: "s"}
	got, err := a.createUserMessage(t.Context(), SessionAgentCall{SessionID: "s", Saved: &saved})
	require.NoError(t, err)
	require.Equal(t, "saved", got.ID)
	msgs, err := env.messages.List(t.Context(), "s")
	require.NoError(t, err)
	require.Empty(t, msgs)
}

// A queue-only call never starts a turn: on an idle session it is refused,
// and on a busy one it is queued.
func TestQueueOnlyCallNeverStartsATurn(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Models:   noModels,
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "child")
	require.NoError(t, err)

	_, err = a.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi", QueueOnly: true})
	require.ErrorIs(t, err, ErrSessionIdle)
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, msgs, "an idle session gets no turn")

	a.activeRequests.Set(sess.ID, &activeCancel{cancel: func() {}})
	_, err = a.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi", QueueOnly: true})
	require.NoError(t, err)
	require.Equal(t, 1, a.QueuedPrompts(sess.ID))
}

// A queue-only call that was accepted into the queue but missed every step
// still runs as its own turn when the queue drains, instead of being refused
// as idle.
func TestQueuedCallDropsQueueOnly(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Models:   noModels,
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)
	a.enqueueCall(SessionAgentCall{SessionID: "s", Prompt: "more", QueueOnly: true})
	queued, ok := a.messageQueue.Get("s")
	require.True(t, ok)
	require.Len(t, queued, 1)
	require.False(t, queued[0].QueueOnly)
}
