package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// hookedStreamModel streams a single text part followed by a clean
// finish, but first runs the hook registered for the Nth Stream call
// (calls past the end of the list stream immediately). That lets a test
// hold one specific turn inside Stream — the active one, or the queued
// one that follows it — while it manipulates the contexts those turns
// run under.
type hookedStreamModel struct {
	text  string
	hooks []func(ctx context.Context) error
	calls atomic.Int64
}

func (m *hookedStreamModel) Provider() string { return "fake" }
func (m *hookedStreamModel) Model() string    { return "fake-model" }

func (m *hookedStreamModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: m.text}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *hookedStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if n := int(m.calls.Add(1)); n <= len(m.hooks) {
		if hook := m.hooks[n-1]; hook != nil {
			if err := hook(ctx); err != nil {
				return nil, err
			}
		}
	}
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

func (m *hookedStreamModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *hookedStreamModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// streamGate is a one-shot rendezvous for a turn parked inside Stream:
// the turn reports that it got there and waits for the test to let it
// proceed, or fails with its run context's error when that run is
// cancelled first — which is how a per-run cancellation (CancelRun, a
// lost client, the duration ceiling) reaches a live turn.
type streamGate struct {
	entered chan struct{}
	release chan struct{}
}

func newStreamGate() *streamGate {
	return &streamGate{entered: make(chan struct{}), release: make(chan struct{})}
}

func (g *streamGate) hold(ctx context.Context) error {
	close(g.entered)
	select {
	case <-g.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *streamGate) await(t *testing.T, what string) {
	t.Helper()
	select {
	case <-g.entered:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func (g *streamGate) let() { close(g.release) }

// lifetimeTestAgent builds a session agent over the given large model,
// publishing terminal events on broker.
func lifetimeTestAgent(env fakeEnv, large fantasy.LanguageModel, broker *pubsub.Broker[notify.RunComplete]) *sessionAgent {
	window := catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}
	return NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: large, CatwalkCfg: window},
		SmallModel:  Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: window},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)
}

// dispatchRun starts a run the way backend.runAgent does: on its own
// goroutine, under its own cancellable run context, carrying the
// lifetime that keeps the call bound to that run even if it has to queue
// behind a busy session. It returns the run's cancel func and a channel
// carrying Run's error.
func dispatchRun(t *testing.T, sa *sessionAgent, sessionID, runID, prompt string) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	ctx = WithRunLifetime(ctx)
	call := SessionAgentCall{
		SessionID: sessionID,
		RunID:     runID,
		Prompt:    prompt,
		Lifetime:  RunLifetimeFromContext(ctx),
	}
	errc := make(chan error, 1)
	go func() {
		_, err := sa.Run(ctx, call)
		errc <- err
	}()
	return cancel, errc
}

// awaitQueueLen waits for the session's queue to reach n prompts.
func awaitQueueLen(t *testing.T, sa *sessionAgent, sessionID string, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return sa.QueuedPrompts(sessionID) == n
	}, 10*time.Second, 5*time.Millisecond, "the session queue never reached %d prompt(s)", n)
}

// TestRun_QueuedRunKeepsItsOwnCancellationScope is the proof that a
// queued prompt is still its own run. It runs in the frame of the turn
// it queued behind, so it used to inherit that turn's context: once that
// earlier run was cancelled — by CancelRun, by the loss of the client
// that asked for it, or by the maximum run duration — the queued turn
// died with it, even though the client waiting on the queued RunID had
// asked for nothing of the sort.
func TestRun_QueuedRunKeepsItsOwnCancellationScope(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	activeGate, queuedGate := newStreamGate(), newStreamGate()
	large := &hookedStreamModel{
		text:  "done",
		hooks: []func(context.Context) error{activeGate.hold, queuedGate.hold},
	}
	sa := lifetimeTestAgent(env, large, broker)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	subCtx, subCancel := context.WithCancel(t.Context())
	defer subCancel()
	ch := broker.Subscribe(subCtx)

	cancelActive, activeDone := dispatchRun(t, sa, sess.ID, "run-active", "active")
	activeGate.await(t, "the active turn to enter Stream")

	_, queuedDone := dispatchRun(t, sa, sess.ID, "run-queued", "queued")
	awaitQueueLen(t, sa, sess.ID, 1)

	// The dispatcher must stay on the stack while its prompt waits: it
	// owns the run handle, the cancellation that reaches the run and the
	// armed duration bound, and all of it is released when Run returns.
	select {
	case err := <-queuedDone:
		t.Fatalf("Run returned %v while its prompt was still queued", err)
	case <-time.After(100 * time.Millisecond):
	}

	// Let the active turn finish. It hands the session to the queued
	// prompt, whose turn parks inside Stream.
	activeGate.let()
	queuedGate.await(t, "the queued turn to enter Stream")

	// The earlier run is cancelled while the queued turn is live. That
	// run is finished and its client is gone; the queued turn belongs to
	// somebody else and must survive.
	cancelActive()
	queuedGate.let()

	require.NoError(t, <-queuedDone, "the queued run must complete on its own terms")
	require.NoError(t, <-activeDone)

	got := collectRunCompletes(t, ch, "run-active", "run-queued")
	require.False(t, got["run-active"].Cancelled)
	require.False(t, got["run-queued"].Cancelled,
		"a queued turn must not inherit the cancellation of the run that dequeued it")
	require.Empty(t, got["run-queued"].Error)
	require.Equal(t, "done", got["run-queued"].Text)
}

// TestRun_QueuedRunCancelledWhileQueued covers the other half of the
// ownership: cancelling a run whose prompt is still queued has to reach
// it. It used to be a silent no-op, because the run was deregistered as
// soon as its prompt was queued, so the prompt ran anyway and the client
// that cancelled it was billed for a turn it had called off.
func TestRun_QueuedRunCancelledWhileQueued(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	activeGate := newStreamGate()
	large := &hookedStreamModel{
		text:  "done",
		hooks: []func(context.Context) error{activeGate.hold},
	}
	sa := lifetimeTestAgent(env, large, broker)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	subCtx, subCancel := context.WithCancel(t.Context())
	defer subCancel()
	ch := broker.Subscribe(subCtx)

	_, activeDone := dispatchRun(t, sa, sess.ID, "run-active", "active")
	activeGate.await(t, "the active turn to enter Stream")

	cancelQueued, queuedDone := dispatchRun(t, sa, sess.ID, "run-queued", "queued")
	awaitQueueLen(t, sa, sess.ID, 1)

	cancelQueued()
	require.ErrorIs(t, <-queuedDone, context.Canceled)
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID),
		"cancelling a queued run must take its prompt out of the queue")

	got := collectRunCompletes(t, ch, "run-queued")
	require.True(t, got["run-queued"].Cancelled,
		"the client waiting on the cancelled RunID needs a terminal event")

	activeGate.let()
	require.NoError(t, <-activeDone)
	require.Equal(t, int64(1), large.calls.Load(), "the cancelled prompt must never run")
}

// TestRun_CancelledTurnStartsTheQueuedDispatchedPrompt covers the
// handoff a failing turn used to skip. The end-of-turn handoff sits
// after the model stream, so every error return jumped over it, leaving
// the queued prompts on a session that is now idle with nothing left to
// start them — and their clients blocked on RunIDs that would never
// report. Per-run cancellation made that ordinary: CancelRun, a lost
// client claim and the duration ceiling all cancel the run context
// directly instead of going through Cancel, which clears the queue
// itself.
func TestRun_CancelledTurnStartsTheQueuedDispatchedPrompt(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	activeGate := newStreamGate()
	large := &hookedStreamModel{
		text:  "done",
		hooks: []func(context.Context) error{activeGate.hold},
	}
	sa := lifetimeTestAgent(env, large, broker)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	subCtx, subCancel := context.WithCancel(t.Context())
	defer subCancel()
	ch := broker.Subscribe(subCtx)

	cancelActive, activeDone := dispatchRun(t, sa, sess.ID, "run-active", "active")
	activeGate.await(t, "the active turn to enter Stream")

	_, queuedDone := dispatchRun(t, sa, sess.ID, "run-queued", "queued")
	awaitQueueLen(t, sa, sess.ID, 1)

	cancelActive()

	require.ErrorIs(t, <-activeDone, context.Canceled)
	require.NoError(t, <-queuedDone)

	got := collectRunCompletes(t, ch, "run-active", "run-queued")
	require.True(t, got["run-active"].Cancelled)
	require.False(t, got["run-queued"].Cancelled,
		"the queued run was not the one that was cancelled")
	require.Equal(t, "done", got["run-queued"].Text)
	require.Equal(t, int64(2), large.calls.Load(),
		"the prompt queued behind the cancelled turn must run as its own turn")
}

// TestRun_CancelledTurnLeavesInteractiveFollowUpQueued keeps the failure
// handoff narrow. A prompt with no RunID is a follow-up typed into the
// TUI while a turn was running: it has no client waiting on a terminal
// event of its own and it belongs to the next turn the user starts, so a
// turn that just failed must not fire every one of them.
func TestRun_CancelledTurnLeavesInteractiveFollowUpQueued(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	activeGate := newStreamGate()
	large := &hookedStreamModel{
		text:  "done",
		hooks: []func(context.Context) error{activeGate.hold},
	}
	sa := lifetimeTestAgent(env, large, broker)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	cancelActive, activeDone := dispatchRun(t, sa, sess.ID, "run-active", "active")
	activeGate.await(t, "the active turn to enter Stream")

	res, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "typed"})
	require.NoError(t, err)
	require.Nil(t, res, "an in-process follow-up still enqueues and returns at once")
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	cancelActive()
	require.ErrorIs(t, <-activeDone, context.Canceled)

	require.Equal(t, 1, sa.QueuedPrompts(sess.ID),
		"an interactive follow-up belongs to the next turn the user starts")
	require.Equal(t, int64(1), large.calls.Load(), "no second turn may run")
}
