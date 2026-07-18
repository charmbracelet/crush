package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// finishStreamModel is a minimal fantasy.LanguageModel that streams a
// single text part followed by a normal (FinishReasonStop) finish. It
// is enough to drive sessionAgent.Run through PrepareStep and a clean
// completion without a recorded provider cassette.
type finishStreamModel struct {
	text string
}

func (m *finishStreamModel) Provider() string { return "fake" }
func (m *finishStreamModel) Model() string    { return "fake-model" }

func (m *finishStreamModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: m.text}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *finishStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	text := m.text
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *finishStreamModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *finishStreamModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func newStreamTestAgent(t *testing.T) (*sessionAgent, fakeEnv) {
	t.Helper()
	env := testEnv(t)
	model := &finishStreamModel{text: "done"}
	sa := testSessionAgent(env, model, model, "system").(*sessionAgent)
	return sa, env
}

// TestCancel_ActiveAndAcceptedFiresBothBranches covers the case where a
// session is actively running (activeRequests set) AND a follow-up has
// been accepted (acceptedRuns > 0). A single Cancel must fire both: it
// invokes the active cancel func and records a pending cancel for the
// accepted follow-up.
func TestCancel_ActiveAndAcceptedFiresBothBranches(t *testing.T) {
	t.Parallel()
	sa, _ := newCancelTestAgent(t)

	const sid = "sid"
	var activeCanceled atomic.Bool
	sa.activeRequests.Set(sid, func() { activeCanceled.Store(true) })

	accept := sa.BeginAccepted(sid)
	defer accept.Close()

	sa.Cancel(sid)

	require.True(t, activeCanceled.Load(), "active cancel func must fire")
	require.True(t, sa.hasPendingCancel(sid), "accepted follow-up must record a pending cancel")
}

// TestRun_BusyWithPendingCancelTakesCancelOnEntry covers the busy-queue
// branch consulting pendingCancels: when the session is busy AND a
// cancel has been recorded for an accepted follow-up, Run must take the
// cancel-on-entry path (persist a canceled turn) instead of enqueueing
// the call behind the active run.
func TestRun_BusyWithPendingCancelTakesCancelOnEntry(t *testing.T) {
	t.Parallel()
	sa, env := newCancelTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	// Make the session look busy: an earlier prompt is active.
	sa.activeRequests.Set(sess.ID, func() {})

	accept := sa.BeginAccepted(sess.ID)
	// A cancel arrives while this follow-up is accepted-but-not-active.
	sa.Cancel(sess.ID)
	require.True(t, sa.hasPendingCancel(sess.ID))

	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "follow-up",
		Accepted:  accept,
	})
	require.NoError(t, err)
	require.Nil(t, result)

	// The follow-up was canceled on entry, not enqueued.
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID),
		"cancel-on-entry must not enqueue the follow-up behind the active run")
	require.False(t, sa.hasPendingCancel(sess.ID), "pending cancel must be consumed")
	require.Equal(t, 0, sa.acceptedCount(sess.ID), "accept reservation must be released")

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, message.User, msgs[0].Role)
	assert.Equal(t, message.Assistant, msgs[1].Role)
	assert.Equal(t, message.FinishReasonCanceled, msgs[1].FinishReason())
}

// TestRun_PrepareStepDrainSkipsQueuedOnPendingCancel verifies that the
// queue drain inside PrepareStep skips queued follow-up prompts when a
// cancel has been recorded for the session: the queued prompt must not
// be folded into the active turn as an extra user message.
func TestRun_PrepareStepDrainSkipsQueuedOnPendingCancel(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	// A follow-up prompt sits queued for the session.
	sa.enqueueCall(SessionAgentCall{SessionID: sess.ID, Prompt: "queued-followup"})
	// A cancel was recorded for the session while it sat in the queue.
	sa.cancelMark.Set(sess.ID, 1)

	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "main",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Only the main prompt produced a user message; the queued
	// follow-up was skipped, not folded into the turn.
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var userMsgs []message.Message
	for _, m := range msgs {
		if m.Role == message.User {
			userMsgs = append(userMsgs, m)
		}
	}
	require.Len(t, userMsgs, 1, "queued follow-up must not create a user message")
	assert.Equal(t, "main", userMsgs[0].Content().String())

	// The queue was drained and the pending cancel consumed.
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID))
	require.False(t, sa.hasPendingCancel(sess.ID))
}

// TestRun_NormalCompletionClearsStalePendingCancel verifies that a Run
// which completes normally clears any stale pending-cancel entry for the
// session, so it cannot catch a future run.
func TestRun_NormalCompletionClearsStalePendingCancel(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	// A stale cancel mark lingers (no queued work, no accepted run).
	sa.cancelMark.Set(sess.ID, 1)

	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "main",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.False(t, sa.hasPendingCancel(sess.ID),
		"normal completion must clear the stale pending cancel")

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, message.Assistant, msgs[1].Role)
	assert.Equal(t, message.FinishReasonEndTurn, msgs[1].FinishReason())
}

// newCancelTestAgentWithRunComplete builds a DB-backed sessionAgent wired
// to a RunComplete broker so tests can observe the terminal event a
// RunID-bearing caller (e.g. `crush run`) blocks on.
func newCancelTestAgentWithRunComplete(t *testing.T) (*sessionAgent, fakeEnv, *pubsub.Broker[notify.RunComplete]) {
	t.Helper()
	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	sa := NewSessionAgent(SessionAgentOptions{
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)
	return sa, env, broker
}

// TestRun_CancelOnEntryPublishesRunComplete covers the first review
// finding: the cancel-on-entry path returned before the streaming defer
// that publishes RunComplete was installed. A caller that dispatches a
// run with a RunID and blocks on RunComplete (ignoring message events,
// like `crush run`) would hang on an immediately-canceled accepted run.
// The cancel-on-entry path must publish a terminal RunComplete carrying
// the originating RunID.
func TestRun_CancelOnEntryPublishesRunComplete(t *testing.T) {
	t.Parallel()
	sa, env, broker := newCancelTestAgentWithRunComplete(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ch := broker.Subscribe(ctx)

	accept := sa.BeginAccepted(sess.ID)
	// A cancel arrives in the accepted-but-not-yet-active window.
	sa.Cancel(sess.ID)
	require.True(t, sa.hasPendingCancel(sess.ID))

	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-cancel-on-entry",
		Prompt:    "hello",
		Accepted:  accept,
	})
	require.NoError(t, err)
	require.Nil(t, result)

	select {
	case got := <-ch:
		assert.Equal(t, "run-cancel-on-entry", got.Payload.RunID,
			"RunComplete must echo the originating RunID")
		assert.Equal(t, sess.ID, got.Payload.SessionID)
		assert.True(t, got.Payload.Cancelled,
			"cancel-on-entry RunComplete must be marked cancelled")
	case <-time.After(2 * time.Second):
		t.Fatal("cancel-on-entry must publish RunComplete; a RunID caller would hang otherwise")
	}
}

// TestCancel_TwoAcceptedBothObserveCancellation covers the second review
// finding: a single cancel with two accepted-not-yet-active prompts must
// cancel both. The cancel raises the session's high-water mark to the
// latest accept sequence, so every prompt accepted-but-not-yet-active at
// cancel time is covered and both take the cancel-on-entry path.
func TestCancel_TwoAcceptedBothObserveCancellation(t *testing.T) {
	t.Parallel()
	sa, env := newCancelTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	// Two prompts are accepted-but-not-yet-active for the same session.
	accept1 := sa.BeginAccepted(sess.ID)
	accept2 := sa.BeginAccepted(sess.ID)
	require.Equal(t, 2, sa.acceptedCount(sess.ID))

	// A single cancel arrives before either becomes active.
	sa.Cancel(sess.ID)
	require.Equal(t, accept2.seq, sa.pendingCancelMark(sess.ID),
		"one cancel must mark every currently-accepted prompt as canceled")
	require.GreaterOrEqual(t, sa.pendingCancelMark(sess.ID), accept1.seq,
		"the mark must cover the earlier accepted prompt too")

	// Both prompts enter Run; each must take cancel-on-entry, not run.
	r1, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "first",
		Accepted:  accept1,
	})
	require.NoError(t, err)
	require.Nil(t, r1)

	r2, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "second",
		Accepted:  accept2,
	})
	require.NoError(t, err)
	require.Nil(t, r2)

	require.False(t, sa.hasPendingCancel(sess.ID),
		"both reserved units must be consumed")
	require.Equal(t, 0, sa.acceptedCount(sess.ID),
		"both accept reservations must be released")

	// Each canceled-on-entry turn writes a user + canceled assistant
	// message, and neither prompt was enqueued to run normally.
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID),
		"neither accepted prompt may be enqueued to run normally")
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 4, "two canceled turns produce two user + two assistant messages")
	var canceled int
	for _, m := range msgs {
		if m.Role == message.Assistant {
			assert.Equal(t, message.FinishReasonCanceled, m.FinishReason())
			canceled++
		}
	}
	require.Equal(t, 2, canceled, "both turns must finish canceled")
}

// TestRun_IdleCancelDoesNotPoisonNextPrompt covers the idle-cancel
// no-op guarantee end-to-end: an Escape on an idle session must not
// record a pending cancel that leaks into the next accepted prompt, which
// must run normally to completion.
func TestRun_IdleCancelDoesNotPoisonNextPrompt(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	// Idle Escape: no accepted run, no active request.
	sa.Cancel(sess.ID)
	require.False(t, sa.hasPendingCancel(sess.ID),
		"idle cancel must not record a pending cancel")

	// The next accepted prompt must run normally, not cancel on entry.
	accept := sa.BeginAccepted(sess.ID)
	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "next",
		Accepted:  accept,
	})
	require.NoError(t, err)
	require.NotNil(t, result, "next prompt must run normally after an idle cancel")

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, message.User, msgs[0].Role)
	assert.Equal(t, message.Assistant, msgs[1].Role)
	assert.Equal(t, message.FinishReasonEndTurn, msgs[1].FinishReason(),
		"the prompt must finish normally, not canceled")
}

// TestCancel_AcceptedAfterCancelIsNotPoisoned is the regression test for
// the reviewer's finding: a counted session-level pending cancel let a
// prompt accepted after the cancel enter Run first and consume a unit
// reserved for the earlier prompts. With a sequence high-water mark, a
// single cancel covers exactly the prompts accepted-but-not-yet-active at
// cancel time (A and B); a prompt accepted afterwards (C) gets a higher
// sequence and must run normally without consuming A or B's cancellation.
// C is run first to prove it neither cancels nor drains the mark, then A
// and B are run and must both cancel on entry.
func TestCancel_AcceptedAfterCancelIsNotPoisoned(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	// A and B are accepted-but-not-yet-active.
	acceptA := sa.BeginAccepted(sess.ID)
	acceptB := sa.BeginAccepted(sess.ID)

	// One cancel arrives covering both A and B.
	sa.Cancel(sess.ID)
	require.True(t, sa.hasPendingCancel(sess.ID))
	require.Equal(t, acceptB.seq, sa.pendingCancelMark(sess.ID),
		"the mark must cover every prompt accepted before the cancel")

	// C is accepted AFTER the cancel; its sequence is above the mark.
	acceptC := sa.BeginAccepted(sess.ID)
	require.Greater(t, acceptC.seq, sa.pendingCancelMark(sess.ID),
		"a prompt accepted after the cancel must not be covered by the mark")

	// Run C first. It must run normally to completion and must not
	// consume or clear the cancellation reserved for A and B.
	rc, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "C",
		Accepted:  acceptC,
	})
	require.NoError(t, err)
	require.NotNil(t, rc, "C was accepted after the cancel and must run normally")
	require.True(t, sa.hasPendingCancel(sess.ID),
		"running C must not drain the cancellation reserved for A and B")

	// Now A and B run. Both were accepted before the cancel and must
	// take the cancel-on-entry path.
	ra, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "A",
		Accepted:  acceptA,
	})
	require.NoError(t, err)
	require.Nil(t, ra, "A must cancel on entry, not run")

	rb, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "B",
		Accepted:  acceptB,
	})
	require.NoError(t, err)
	require.Nil(t, rb, "B must cancel on entry, not run")

	require.False(t, sa.hasPendingCancel(sess.ID),
		"the mark clears once all covered handles are resolved")
	require.Equal(t, 0, sa.acceptedCount(sess.ID))
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID),
		"neither A nor B may be enqueued to run normally")

	// C produced a normal turn; A and B each produced a canceled turn.
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 6, "C normal + A canceled + B canceled = 3 user + 3 assistant")

	var normal, canceled int
	for _, m := range msgs {
		if m.Role != message.Assistant {
			continue
		}
		switch m.FinishReason() {
		case message.FinishReasonEndTurn:
			normal++
		case message.FinishReasonCanceled:
			canceled++
		}
	}
	require.Equal(t, 1, normal, "only C finished normally")
	require.Equal(t, 2, canceled, "both A and B finished canceled")
}
