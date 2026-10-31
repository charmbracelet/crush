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

// A report call writes the report when it is delivered.
func TestReportCallWritesTheReport(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Models:   noModels,
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	report := message.SubAgentReport{Label: "akiko-bugs", Output: "all good"}
	got, err := a.createUserMessage(t.Context(), SessionAgentCall{SessionID: sess.ID, Report: &report})
	require.NoError(t, err)
	require.Equal(t, []message.SubAgentReport{report}, got.SubAgentReports())
}

// A report for a busy session waits in the queue, out of the transcript,
// until the turn reads it, and a cancel that drops it still records it.
func TestQueuedReportReachesTheTranscriptOnlyWhenDeliveredOrDropped(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Models:   noModels,
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	a.activeRequests.Set(sess.ID, &activeCancel{cancel: func() {}})

	report := message.SubAgentReport{Label: "akiko-bugs", Output: "all good"}
	_, err = a.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: report.PromptText(), Report: &report})
	require.NoError(t, err)
	require.Equal(t, 1, a.QueuedPrompts(sess.ID))
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, msgs, "a queued report is not in the transcript yet")

	a.ClearQueue(sess.ID)
	msgs, err = env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Equal(t, []message.SubAgentReport{report}, msgs[0].SubAgentReports())
}

// The user's own queued prompts go ahead of queued reports, in order, so
// a short line is not buried at the end of a long report.
func TestUserPromptsQueueAheadOfReports(t *testing.T) {
	t.Parallel()
	report := func(label string) SessionAgentCall {
		return SessionAgentCall{Prompt: label, Report: &message.SubAgentReport{Label: label}}
	}
	var queue []SessionAgentCall
	for _, call := range []SessionAgentCall{
		{Prompt: "first"},
		report("r1"),
		report("r2"),
		{Prompt: "pw hunter2"},
		report("r3"),
		{Prompt: "and another"},
	} {
		queue = insertQueued(queue, call)
	}
	var got []string
	for _, call := range queue {
		got = append(got, call.Prompt)
	}
	require.Equal(t, []string{"first", "pw hunter2", "and another", "r1", "r2", "r3"}, got)
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
