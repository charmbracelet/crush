package agent

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// handoffAgent builds a sessionAgent wired to a RunComplete broker, the
// only way to observe the terminal event a queued prompt gets (or fails
// to get) through the handoff. The small model is separate on purpose:
// title generation streams it concurrently and a shared script would be
// consumed out from under the test.
func handoffAgent(t *testing.T, env fakeEnv, large fantasy.LanguageModel, tools ...fantasy.AgentTool) (*sessionAgent, <-chan pubsub.Event[notify.RunComplete]) {
	t.Helper()
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	window := catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: large, CatwalkCfg: window},
		SmallModel:  Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: window},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		Tools:       tools,
		RunComplete: broker,
	}).(*sessionAgent)
	return sa, broker.Subscribe(t.Context())
}

// drainRunCompletes collects every RunComplete already published for
// runID. Callers use it after the run under test has returned, so every
// terminal event it owed has been published by then and a short settle
// window is enough.
func drainRunCompletes(ch <-chan pubsub.Event[notify.RunComplete], runID string) []notify.RunComplete {
	var got []notify.RunComplete
	settle := time.After(500 * time.Millisecond)
	for {
		select {
		case ev := <-ch:
			if ev.Payload.RunID == runID {
				got = append(got, ev.Payload)
			}
		case <-settle:
			return got
		}
	}
}

// userPrompts returns the text of every user message in the session, in
// order, which is how these tests tell which prompts actually ran.
func userPrompts(t *testing.T, env fakeEnv, sessionID string) []string {
	t.Helper()
	msgs, err := env.messages.List(t.Context(), sessionID)
	require.NoError(t, err)
	var prompts []string
	for _, m := range msgs {
		if m.Role == message.User {
			prompts = append(prompts, m.Content().String())
		}
	}
	return prompts
}

// failSummarySaveService fails the session save that ends a summarize —
// the one that records SummaryMessageID — so the summarize returns an
// error after doing all its work.
type failSummarySaveService struct {
	session.Service
	failed atomic.Bool
}

func (s *failSummarySaveService) Save(ctx context.Context, sess session.Session) (session.Session, error) {
	if sess.SummaryMessageID != "" && s.failed.CompareAndSwap(false, true) {
		return session.Session{}, errors.New("summary save failed")
	}
	return s.Service.Save(ctx, sess)
}

// TestSummarize_HandoffDropsQueuedPromptCoveredByCancel pins the first
// half of Defect F. Summarize used to end with an unlocked
// messageQueue.Get -> Set(tail) -> Run, with no cancel-mark check at
// all, so a prompt the session's pending cancel covered ran anyway: the
// user said stop and the agent carried on. Worse, the drop was invisible
// — no terminal event was ever published for it either way.
//
// The handoff is now Run's guarded one, so the covered prompt is dropped
// and its RunID gets a cancelled RunComplete.
func TestSummarize_HandoffDropsQueuedPromptCoveredByCancel(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	blocking := &blockingListService{
		Service: env.messages,
		n:       1,
		entered: make(chan struct{}),
		gate:    make(chan struct{}),
	}
	env.messages = blocking

	model := &scriptedStreamModel{
		steps: []scriptedStep{{text: "summary", finishReason: fantasy.FinishReasonStop}},
	}
	sa, events := handoffAgent(t, env, model)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	seedUserMessage(t, env, sess.ID)

	// Record a pending cancel while the session is idle but a prompt is
	// accepted and not yet started: Cancel raises the mark without
	// cancelling anything, which is the only way to reach the handoff
	// with a mark in place and a queued prompt it covers.
	stale := sa.BeginAccepted(sess.ID)
	defer stale.Close()
	sa.Cancel(sess.ID)
	require.True(t, sa.hasPendingCancel(sess.ID))

	summarizeErr := make(chan error, 1)
	go func() {
		summarizeErr <- sa.Summarize(t.Context(), SummarizeCall{SessionID: sess.ID})
	}()

	select {
	case <-blocking.entered:
	case <-time.After(5 * time.Second):
		close(blocking.gate)
		t.Fatal("summarize never reached the transcript read")
	}

	// Queued behind the summarize with no accept reservation of its own,
	// so the session's pending cancel covers it.
	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-queued",
		Prompt:    "queued",
	})
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	close(blocking.gate)
	require.NoError(t, <-summarizeErr)

	require.Equal(t, 0, sa.QueuedPrompts(sess.ID))
	require.Equal(t, int64(1), model.calls.Load(),
		"only the summarize may stream: the cancelled prompt must not run")
	require.NotContains(t, userPrompts(t, env, sess.ID), "queued",
		"a prompt the pending cancel covers must not run through the summarize handoff")

	got := drainRunCompletes(events, "run-queued")
	require.Len(t, got, 1, "the dropped prompt must get exactly one terminal event")
	require.True(t, got[0].Cancelled,
		"a dropped queued prompt must be reported cancelled, or its client waits forever")
}

// TestSummarize_FailedSummarizeStillRunsQueuedPrompts pins the second
// half of Defect F. Summarize returned on its own error, which stranded
// every RunID-bearing prompt queued behind it: drainQueueForStep leaves
// those in the queue on purpose and only the end-of-turn handoff starts
// them, so the session went idle with a prompt nobody would ever run and
// a client blocked on a RunID that would never carry a terminal event.
func TestSummarize_FailedSummarizeStillRunsQueuedPrompts(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	env.sessions = &failSummarySaveService{Service: env.sessions}
	blocking := &blockingListService{
		Service: env.messages,
		n:       1,
		entered: make(chan struct{}),
		gate:    make(chan struct{}),
	}
	env.messages = blocking

	model := &scriptedStreamModel{
		steps: []scriptedStep{
			{text: "summary", finishReason: fantasy.FinishReasonStop},
			{text: "done", finishReason: fantasy.FinishReasonStop},
		},
	}
	sa, events := handoffAgent(t, env, model)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	seedUserMessage(t, env, sess.ID)

	summarizeErr := make(chan error, 1)
	go func() {
		summarizeErr <- sa.Summarize(t.Context(), SummarizeCall{SessionID: sess.ID})
	}()

	select {
	case <-blocking.entered:
	case <-time.After(5 * time.Second):
		close(blocking.gate)
		t.Fatal("summarize never reached the transcript read")
	}

	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-queued",
		Prompt:    "queued",
	})
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	close(blocking.gate)
	err = <-summarizeErr
	require.ErrorContains(t, err, "summary save failed",
		"Summarize must still report its own failure")

	require.Contains(t, userPrompts(t, env, sess.ID), "queued",
		"a failed summarize must still run the prompt queued behind it")
	got := drainRunCompletes(events, "run-queued")
	require.Len(t, got, 1)
	require.Empty(t, got[0].Error)
	require.False(t, got[0].Cancelled)
	require.Equal(t, "done", got[0].Text)
	require.False(t, sa.IsSessionBusy(sess.ID))
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID))
}

// TestHandOffQueue_ReservesAcceptForTheDequeuedPrompt pins the last of
// Defect F's three bullets: no accept reservation covered Summarize's
// dequeue-to-register window. With the queue emptied and nothing
// registered in activeRequests or acceptedRuns, a Cancel arriving there
// was a complete no-op — it cancelled nothing, recorded nothing, and
// found nothing to clear — so the dequeued prompt streamed anyway.
//
// The shared handoff mints the reservation before it drops the lock, so
// the cancel records a mark that the prompt's own Run observes as
// cancel-on-entry.
func TestHandOffQueue_ReservesAcceptForTheDequeuedPrompt(t *testing.T) {
	t.Parallel()
	// No model: if the dequeued prompt reached the provider instead of
	// the cancel-on-entry path, this test would panic rather than pass.
	sa, env := newCancelTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	sa.enqueueCall(SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-queued",
		Prompt:    "queued",
	})

	handoff := sa.handOffQueue(sess.ID, "", nil, false)
	require.True(t, handoff.Started)
	require.Equal(t, "queued", handoff.Next.Prompt)
	require.NotNil(t, handoff.Next.Accepted,
		"the dequeued prompt must carry an accept reservation across the handoff")
	require.Equal(t, 1, sa.acceptedCount(sess.ID))
	require.False(t, handoff.OwesRunComplete, "a summarize has no RunID of its own")

	// A cancel in the dequeue -> register window.
	sa.Cancel(sess.ID)
	require.True(t, sa.hasPendingCancel(sess.ID))

	result, err := sa.Run(t.Context(), handoff.Next)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 0, sa.acceptedCount(sess.ID))

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	require.Equal(t, message.User, msgs[0].Role)
	require.Equal(t, message.Assistant, msgs[1].Role)
	require.Equal(t, message.FinishReasonCanceled, msgs[1].FinishReason(),
		"the cancel must reach the prompt the handoff dequeued")
}

// compactionSteps is the script for an auto-compaction turn: a step that
// calls a tool and reports enough prompt tokens to trip the summarize
// stop condition (testSessionAgent's 200k window puts the threshold at
// 40k), then the summary, then one more turn.
func compactionSteps(last string) []scriptedStep {
	return []scriptedStep{
		{
			text:         "working",
			toolCallID:   "call-1",
			toolName:     "echo",
			toolInput:    `{"value":"hi"}`,
			usage:        fantasy.Usage{InputTokens: 180_000, OutputTokens: 100},
			finishReason: fantasy.FinishReasonToolCalls,
		},
		{text: "summary", finishReason: fantasy.FinishReasonStop},
		{text: last, finishReason: fantasy.FinishReasonStop},
	}
}

// TestRun_AutoCompactionDoesNotResumeAfterAQueuedPrompt pins the related
// fault in the same stretch: the auto-compaction continuation used to be
// appended to the queue unconditionally, and through an unlocked
// Get/Set pair at that. So a turn interrupted by compaction ran the
// prompt the user queued and then resumed the work that prompt replaced,
// with the user's newer instruction already acknowledged.
func TestRun_AutoCompactionDoesNotResumeAfterAQueuedPrompt(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	echo, echoRuns := newEchoTool()
	model := &scriptedStreamModel{
		steps:   compactionSteps("done"),
		entered: make(chan struct{}),
		gate:    make(chan struct{}),
	}
	sa, _ := handoffAgent(t, env, model, echo)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	runErr := make(chan error, 1)
	go func() {
		_, err := sa.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			RunID:     "run-main",
			Prompt:    "fill the context window",
		})
		runErr <- err
	}()

	select {
	case <-model.entered:
	case <-time.After(10 * time.Second):
		close(model.gate)
		t.Fatal("the turn never entered Stream")
	}

	// Queued while the turn is active, with a RunID so it runs as its
	// own turn rather than being folded into this one.
	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-follow",
		Prompt:    "follow",
	})
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	close(model.gate)
	select {
	case err := <-runErr:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the turn never returned")
	}

	require.Equal(t, int64(1), echoRuns.Load())
	require.Equal(t, []string{"fill the context window", "follow"}, userPrompts(t, env, sess.ID),
		"the queued prompt supersedes the interrupted work; the original must not be resumed")
	require.Equal(t, int64(3), model.calls.Load(),
		"one turn step, the summary, and the queued prompt's turn")
	require.False(t, sa.IsSessionBusy(sess.ID))
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID))
}

// TestRun_AutoCompactionResumesInterruptedWorkWhenNothingIsQueued is the
// complement: with nothing queued, nothing supersedes the interrupted
// work, so compacting mid-work must still resume it. That is the whole
// point of auto-compaction, and it now runs through the same guarded
// handoff as any queued prompt — including the accept reservation and
// the stripped OnComplete hook.
func TestRun_AutoCompactionResumesInterruptedWorkWhenNothingIsQueued(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	echo, _ := newEchoTool()
	model := &scriptedStreamModel{steps: compactionSteps("resumed")}
	sa, events := handoffAgent(t, env, model, echo)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-main",
		Prompt:    "fill the context window",
	})
	require.NoError(t, err)

	prompts := userPrompts(t, env, sess.ID)
	require.Len(t, prompts, 2)
	require.Equal(t, "fill the context window", prompts[0])
	require.True(t, strings.HasPrefix(prompts[1], "The previous session was interrupted"),
		"the turn must resume its interrupted work after the summary, got %q", prompts[1])
	require.Equal(t, int64(3), model.calls.Load(),
		"one turn step, the summary, and the resumed turn")

	// The resumed turn carries the original RunID, so the run's client
	// must see exactly one terminal event: the resumed turn's.
	got := drainRunCompletes(events, "run-main")
	require.Len(t, got, 1, "the resumed turn owns the RunID's single terminal event")
	require.Empty(t, got[0].Error)
	require.Equal(t, "resumed", got[0].Text)
}
