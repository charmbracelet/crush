package agent

import (
	"context"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// abortingStreamModel behaves like gatedStreamModel, but when the wait on
// the gate ends because the run context was cancelled it fails Stream with
// that context error, so the run ends through the cancelled-run error path.
type abortingStreamModel struct {
	*gatedStreamModel
}

func (m abortingStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	stream, err := m.gatedStreamModel.Stream(ctx, call)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return stream, err
}

func newGatedModel() *gatedStreamModel {
	return &gatedStreamModel{
		text:    "done",
		gate:    make(chan struct{}),
		entered: make(chan struct{}),
	}
}

// startBlockedRun starts a main turn that stays active inside Stream until
// the gate of the gated model is released, so calls can be queued behind it.
func startBlockedRun(t *testing.T, large *gatedStreamModel, model fantasy.LanguageModel) (*sessionAgent, fakeEnv, string, <-chan error) {
	t.Helper()
	env := testEnv(t)
	small := &finishStreamModel{text: "title"}
	sa := NewSessionAgent(SessionAgentOptions{
		Models: fixedModels(
			Model{Model: model, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
			Model{Model: small, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		),
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "main"})
		done <- runErr
	}()
	select {
	case <-large.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("main run never entered Stream")
	}
	return sa, env, sess.ID, done
}

func requireSingleReportMessage(t *testing.T, env fakeEnv, sessionID, output string) {
	t.Helper()
	var found int
	for _, m := range userMessages(t, env, sessionID) {
		if report, ok := m.SubagentReport(); ok && report.Output == output {
			found++
		}
	}
	require.Equal(t, 1, found, "the report must be persisted exactly once")
}

func TestClearQueue_PersistsReportWhenQueueHoldsOnlyReports(t *testing.T) {
	t.Parallel()
	large := newGatedModel()
	sa, env, sessionID, done := startBlockedRun(t, large, large)

	sa.enqueueCall(reportCall(sessionID, testReport("only a report")))
	require.Equal(t, 0, sa.QueuedPrompts(sessionID), "a report is not counted as a queued prompt")

	sa.ClearQueue(sessionID)

	requireSingleReportMessage(t, env, sessionID, "only a report")

	close(large.gate)
	require.NoError(t, <-done)

	requireSingleReportMessage(t, env, sessionID, "only a report")
	require.Equal(t, int64(1), large.calls.Load(), "the cleared report must not start a turn")
}

func TestCancel_PersistsReportWhenQueueHoldsOnlyReports(t *testing.T) {
	t.Parallel()
	large := newGatedModel()
	sa, env, sessionID, done := startBlockedRun(t, large, abortingStreamModel{large})

	sa.enqueueCall(reportCall(sessionID, testReport("only a report")))

	sa.Cancel(sessionID)

	require.ErrorIs(t, <-done, context.Canceled)

	requireSingleReportMessage(t, env, sessionID, "only a report")
	_, queued := sa.messageQueue.Get(sessionID)
	require.False(t, queued, "the cancelled run must leave no queued calls behind")
}
