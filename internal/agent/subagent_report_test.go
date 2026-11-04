package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func testReport(output string) *message.SubagentReport {
	return &message.SubagentReport{Name: "reviewer-1", Status: "completed", Output: output}
}

func reportCall(sessionID string, report *message.SubagentReport) SessionAgentCall {
	return SessionAgentCall{SessionID: sessionID, Prompt: report.PromptText(), Report: report}
}

func userMessages(t *testing.T, env fakeEnv, sessionID string) []message.Message {
	t.Helper()
	msgs, err := env.messages.List(t.Context(), sessionID)
	require.NoError(t, err)
	var users []message.Message
	for _, m := range msgs {
		if m.Role == message.User {
			users = append(users, m)
		}
	}
	return users
}

func TestRun_QueueOnlyOnIdleSessionReturnsErrSessionIdle(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi", QueueOnly: true})
	require.ErrorIs(t, err, ErrSessionIdle)
	require.Nil(t, result)

	require.Empty(t, userMessages(t, env, sess.ID), "an idle queue-only call must not write a message")
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID))
	require.False(t, sa.IsSessionBusy(sess.ID), "an idle queue-only call must not leave the session busy")
}

func TestRun_QueueOnlyOnBusySessionQueues(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	sa.activeRequests.Set(sess.ID, &activeCancel{cancel: func() {}})

	result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "steer", QueueOnly: true})
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))
}

func TestQueuedPrompts_ExcludeReports(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	sa.enqueueCall(SessionAgentCall{SessionID: sess.ID, Prompt: "typed"})
	sa.enqueueCall(reportCall(sess.ID, testReport("done")))

	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))
	require.Equal(t, []string{"typed"}, sa.QueuedPromptsList(sess.ID))
}

func TestRun_FoldsQueuedReportAsReportMessage(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	sa.enqueueCall(reportCall(sess.ID, testReport("all clear")))

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "main"})
	require.NoError(t, err)

	users := userMessages(t, env, sess.ID)
	require.Len(t, users, 2)
	report, ok := users[1].SubagentReport()
	require.True(t, ok, "the folded report must be stored as a report part")
	require.Equal(t, "all clear", report.Output)
	require.False(t, users[1].IsPrompt())
}

func TestRun_ReportCallStoresReportPart(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), reportCall(sess.ID, testReport("fresh")))
	require.NoError(t, err)

	users := userMessages(t, env, sess.ID)
	require.Len(t, users, 1)
	report, ok := users[0].SubagentReport()
	require.True(t, ok)
	require.Equal(t, "fresh", report.Output)
}

func TestClearQueue_PersistsDroppedReports(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	sa.enqueueCall(SessionAgentCall{SessionID: sess.ID, Prompt: "typed"})
	sa.enqueueCall(reportCall(sess.ID, testReport("keep me")))

	sa.ClearQueue(sess.ID)

	users := userMessages(t, env, sess.ID)
	require.Len(t, users, 1, "only the report survives; the typed prompt is discarded")
	report, ok := users[0].SubagentReport()
	require.True(t, ok)
	require.Equal(t, "keep me", report.Output)
}

func TestRun_PendingCancelPersistsQueuedReport(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	sa.enqueueCall(reportCall(sess.ID, testReport("late")))
	sa.cancelMark.Set(sess.ID, 1)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "main"})
	require.NoError(t, err)

	users := userMessages(t, env, sess.ID)
	require.Len(t, users, 2)
	_, ok := users[1].SubagentReport()
	require.True(t, ok, "a report dropped by a cancel must still be written")
}
