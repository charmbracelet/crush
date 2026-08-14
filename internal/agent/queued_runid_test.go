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
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// gatedStreamModel streams a single text part followed by a clean finish,
// but blocks the very first Stream call until its gate is released. That
// lets a test hold a run "active" (past PrepareStep, inside Stream) just
// long enough to enqueue a follow-up prompt behind the busy session. If
// the run context is canceled while the gate is held, Stream fails with
// the context error, the way a real provider call does.
// Subsequent Stream calls (e.g. the recursive run draining the queue)
// proceed immediately.
type gatedStreamModel struct {
	text    string
	gate    chan struct{}
	entered chan struct{}
	calls   atomic.Int64
}

func (m *gatedStreamModel) Provider() string { return "fake" }
func (m *gatedStreamModel) Model() string    { return "fake-model" }

func (m *gatedStreamModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: m.text}},
		FinishReason: fantasy.FinishReasonStop,
	}, nil
}

func (m *gatedStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if m.calls.Add(1) == 1 {
		close(m.entered)
		select {
		case <-m.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
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

func (m *gatedStreamModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *gatedStreamModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// TestRun_QueuedRunIDPromptRunsRecursivelyAndPublishesRunComplete is the
// end-to-end proof of fix 2: a prompt carrying a RunID that is queued
// behind a busy session must NOT be silently folded into the active turn.
// It runs as its own turn via the recursive run path and publishes its
// own terminal RunComplete, so a `crush run` caller blocking on that
// RunID does not hang. The active turn keeps its own RunComplete too.
func TestRun_QueuedRunIDPromptRunsRecursivelyAndPublishesRunComplete(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	large := &gatedStreamModel{
		text:    "done",
		gate:    make(chan struct{}),
		entered: make(chan struct{}),
	}
	small := &finishStreamModel{text: "title"}

	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: large, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		SmallModel:  Model{Model: small, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	subCtx, subCancel := context.WithCancel(t.Context())
	defer subCancel()
	ch := broker.Subscribe(subCtx)

	// Start the main turn; it blocks inside Stream once active.
	mainDone := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			RunID:     "run-main",
			Prompt:    "main",
		})
		mainDone <- runErr
	}()

	// Wait until the main turn is active (inside Stream).
	select {
	case <-large.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("main run never entered Stream")
	}
	require.True(t, sa.IsSessionBusy(sess.ID), "main run must be active before enqueueing the follow-up")

	// Enqueue a RunID-bearing follow-up behind the busy session.
	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-follow",
		Prompt:    "follow",
	})
	require.NoError(t, err)
	require.Nil(t, res, "a busy-session follow-up must enqueue and return (nil, nil)")
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID), "the follow-up must be queued, not folded")

	// Release the main turn so it completes and hands off to the queue.
	close(large.gate)
	require.NoError(t, <-mainDone)

	// Both turns must publish their own terminal RunComplete.
	got := map[string]notify.RunComplete{}
	deadline := time.After(5 * time.Second)
	for len(got) < 2 {
		select {
		case ev := <-ch:
			got[ev.Payload.RunID] = ev.Payload
		case <-deadline:
			t.Fatalf("timed out waiting for both RunCompletes; got %v", got)
		}
	}

	main, ok := got["run-main"]
	require.True(t, ok, "the active turn must publish its own RunComplete")
	require.Empty(t, main.Error)
	require.False(t, main.Cancelled)

	follow, ok := got["run-follow"]
	require.True(t, ok,
		"the queued RunID prompt must publish its own RunComplete instead of being folded silently")
	require.Empty(t, follow.Error)
	require.False(t, follow.Cancelled)
	require.Equal(t, "done", follow.Text, "the queued prompt ran as its own turn")

	// Two distinct assistant turns prove the follow-up was not folded.
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var assistants, follows int
	for _, m := range msgs {
		switch m.Role {
		case message.Assistant:
			assistants++
		case message.User:
			if m.Content().String() == "follow" {
				follows++
			}
		}
	}
	require.Equal(t, 2, assistants, "the active turn and the recursive turn each produce one assistant message")
	require.Equal(t, 1, follows, "the follow-up prompt is its own user turn")
}

// TestRun_CancelKeepsQueuedPromptsForEditing is the end-to-end proof for
// issue #3558: with a turn active and prompts queued behind it, "esc esc"
// must cancel the active turn and nothing else. Before this, Cancel called
// clearQueueAndNotify, so cancelling destroyed every queued prompt — the
// data loss the pop feature exists to prevent. The canceled turn returns
// through the error path (before the completion handoff), so the queue is
// left intact, in order, for the user to pop and edit; still-queued
// prompts are not reported as cancelled to a waiting caller, because they
// have not been discarded.
func TestRun_CancelKeepsQueuedPromptsForEditing(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	large := &gatedStreamModel{
		text:    "done",
		gate:    make(chan struct{}),
		entered: make(chan struct{}),
	}
	small := &finishStreamModel{text: "title"}

	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: large, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		SmallModel:  Model{Model: small, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	subCtx, subCancel := context.WithCancel(t.Context())
	defer subCancel()
	ch := broker.Subscribe(subCtx)

	// Start the main turn; it blocks inside Stream once active.
	mainDone := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			RunID:     "run-main",
			Prompt:    "main",
		})
		mainDone <- runErr
	}()
	select {
	case <-large.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("main run never entered Stream")
	}
	require.True(t, sa.IsSessionBusy(sess.ID))

	// The user queues two prompts behind the busy turn: one typed in the
	// TUI (no RunID) and one submitted by a non-interactive caller.
	for _, queued := range []SessionAgentCall{
		{SessionID: sess.ID, Prompt: "queued-one"},
		{SessionID: sess.ID, RunID: "run-two", Prompt: "queued-two"},
	} {
		res, runErr := sa.Run(t.Context(), queued)
		require.NoError(t, runErr)
		require.Nil(t, res, "a busy-session prompt must enqueue and return (nil, nil)")
	}
	require.Equal(t, 2, sa.QueuedPrompts(sess.ID))

	// esc esc.
	sa.Cancel(sess.ID)
	require.ErrorIs(t, <-mainDone, context.Canceled,
		"the active turn must end canceled")

	// The queue survived the cancel, in its original order, and the
	// newest entry is still poppable for editing.
	require.Equal(t, []string{"queued-one", "queued-two"}, sa.QueuedPromptsList(sess.ID),
		"cancelling must not discard queued prompts")
	popped, ok := sa.PopQueuedMessage(sess.ID)
	require.True(t, ok)
	require.Equal(t, "queued-two", popped.Prompt)
	require.Equal(t, []string{"queued-one"}, sa.QueuedPromptsList(sess.ID))

	// Neither queued prompt ran: only the canceled main turn produced
	// messages.
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	require.Equal(t, message.User, msgs[0].Role)
	require.Equal(t, "main", msgs[0].Content().String())
	require.Equal(t, message.Assistant, msgs[1].Role)
	require.Equal(t, message.FinishReasonCanceled, msgs[1].FinishReason())

	// Exactly two terminal events: the canceled main turn, and the popped
	// prompt (a pop does discard it, so its waiting caller is released).
	// A prompt that merely sat in the queue across the cancel publishes
	// nothing.
	got := map[string]notify.RunComplete{}
	deadline := time.After(5 * time.Second)
	for len(got) < 2 {
		select {
		case ev := <-ch:
			got[ev.Payload.RunID] = ev.Payload
		case <-deadline:
			t.Fatalf("timed out waiting for terminal events; got %v", got)
		}
	}
	require.True(t, got["run-main"].Cancelled, "the active turn must report cancelled")
	require.True(t, got["run-two"].Cancelled, "the popped prompt must release its caller")
	select {
	case extra := <-ch:
		t.Fatalf("unexpected extra terminal event: %+v", extra.Payload)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestRun_IdleWithQueueRunsSubmissionsOldestFirst covers restarting the
// agent after "esc esc": cancellation is turn-scoped, so the session is
// left idle with its queue intact, and the prompt the user sends to get
// going again must run *behind* what is already queued. Keying the
// dispatch decision on IsSessionBusy alone made the new prompt the active
// run, so it overtook every queued prompt and they drained after it.
//
// Each prompt carries a RunID, so none of them can be folded into another
// turn: every one runs as its own turn and owes exactly one terminal
// RunComplete. The order of those events is the contract — a caller
// waiting on the new submission's RunID must not be released before the
// prompts queued ahead of it have completed.
func TestRun_IdleWithQueueRunsSubmissionsOldestFirst(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	model := &finishStreamModel{text: "done"}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:  Model{Model: model, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		SmallModel:  Model{Model: model, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
		IsYolo:      true,
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	subCtx, subCancel := context.WithCancel(t.Context())
	defer subCancel()
	ch := broker.Subscribe(subCtx)

	// Two prompts survived a cancel and sit queued on an idle session.
	sa.enqueueCall(SessionAgentCall{SessionID: sess.ID, RunID: "run-one", Prompt: "queued-one"})
	sa.enqueueCall(SessionAgentCall{SessionID: sess.ID, RunID: "run-two", Prompt: "queued-two"})
	require.False(t, sa.IsSessionBusy(sess.ID))

	// The user restarts the agent with a new prompt.
	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-new",
		Prompt:    "new",
	})
	require.NoError(t, err)

	require.Equal(t, 0, sa.QueuedPrompts(sess.ID), "the whole queue must have drained")

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	roles := make([]message.MessageRole, len(msgs))
	prompts := make([]string, 0, 3)
	for i, m := range msgs {
		roles[i] = m.Role
		if m.Role == message.User {
			prompts = append(prompts, m.Content().String())
		}
	}
	require.Equal(t, []string{"queued-one", "queued-two", "new"}, prompts,
		"queued prompts must run in order, with the new submission last")
	require.Equal(t, []message.MessageRole{
		message.User, message.Assistant,
		message.User, message.Assistant,
		message.User, message.Assistant,
	}, roles, "each prompt runs as its own turn")

	// One terminal event per RunID, in execution order.
	var order []string
	deadline := time.After(5 * time.Second)
	for len(order) < 3 {
		select {
		case ev := <-ch:
			require.Empty(t, ev.Payload.Error)
			require.False(t, ev.Payload.Cancelled)
			order = append(order, ev.Payload.RunID)
		case <-deadline:
			t.Fatalf("timed out waiting for terminal events; got %v", order)
		}
	}
	require.Equal(t, []string{"run-one", "run-two", "run-new"}, order,
		"the new submission's RunID must not complete before the prompts queued ahead of it")
	select {
	case extra := <-ch:
		t.Fatalf("unexpected extra terminal event: %+v", extra.Payload)
	case <-time.After(200 * time.Millisecond):
	}
}
