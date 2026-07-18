package agent

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// TestSessionAgentRun_QueueStripsOnComplete verifies that when a Run
// call is enqueued (because the session is already busy), the
// OnComplete hook is NOT propagated onto the queued copy. The hook
// belongs to the caller's retry/coalesce scope (typically
// coordinator.Run) which has already returned by the time the queue
// drains; carrying it forward would silently funnel the terminal
// event into a closure nobody reads, and subscribers (`crush run`)
// would hang waiting for a RunComplete that never publishes.
func TestSessionAgentRun_QueueStripsOnComplete(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "queued-session"
	// Mark the session as busy so Run takes the queue branch
	// without needing a real model.
	a.activeRequests.Set(sessionID, func() {})

	var called bool
	hook := func(notify.RunComplete) { called = true }

	res, err := a.Run(t.Context(), SessionAgentCall{
		SessionID:        sessionID,
		RunID:            "run-xyz",
		Prompt:           "queued prompt",
		PermissionPolicy: permission.RequestPolicyAutoApprove,
		OnComplete:       hook,
	})
	require.NoError(t, err)
	require.Nil(t, res, "queued Run must return (nil, nil)")
	require.False(t, called,
		"OnComplete must not fire on the enqueue path; the caller's scope is still live")

	queued, ok := a.messageQueue.Get(sessionID)
	require.True(t, ok)
	require.Len(t, queued, 1)
	require.Nil(t, queued[0].OnComplete,
		"queued SessionAgentCall must have OnComplete stripped so the drain falls back to the default broker publish")
	require.Equal(t, "queued prompt", queued[0].Prompt,
		"all other fields must be preserved on the queued copy")
	require.Equal(t, "run-xyz", queued[0].RunID,
		"RunID must be preserved on the queued copy so the drained turn's "+
			"RunComplete still correlates with the originating SendMessage")
	require.Equal(t, permission.RequestPolicyAutoApprove, queued[0].PermissionPolicy,
		"permission policy must be preserved so the queued turn does not inherit the active turn's policy")
}

// TestDrainQueueForStep_FiltersUnderDispatchLock verifies that the queue
// drain evaluates the per-session cancel mark while holding the dispatch
// mutex (canceledBySeq's documented precondition). Queued calls at or
// below the cancel high-water mark are dropped, calls queued after the
// cancel (higher seq) are folded, untracked enqueues (seq == 0) are
// dropped whenever any mark is present, and the queue is cleared. These
// calls carry no RunID, so all foldable survivors are returned for
// folding (the existing follow-up behavior).
func TestDrainQueueForStep_FiltersUnderDispatchLock(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "drain-session"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "below", acceptSeq: 1},
		{SessionID: sessionID, Prompt: "at-mark", acceptSeq: 2},
		{SessionID: sessionID, Prompt: "after", acceptSeq: 3},
		{SessionID: sessionID, Prompt: "untracked", acceptSeq: 0},
	})
	// Cancel high-water mark at seq 2: seq <= 2 and seq == 0 are covered.
	a.cancelMark.Set(sessionID, 2)

	fold, canceledWithRunID := a.drainQueueForStep(sessionID, permission.RequestPolicyPrompt)

	require.Len(t, fold, 1,
		"only the follow-up queued after the cancel (seq > mark) must be folded")
	require.Equal(t, "after", fold[0].Prompt)
	require.Empty(t, canceledWithRunID,
		"no dropped call carried a RunID, so none need a terminal RunComplete")

	_, ok := a.messageQueue.Get(sessionID)
	require.False(t, ok, "drain must clear the session message queue when nothing is kept")
}

// TestDrainQueueForStep_NoMarkFoldsAllNonRunID verifies that with no
// cancel mark recorded, every queued call without a RunID is folded.
func TestDrainQueueForStep_NoMarkFoldsAllNonRunID(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "drain-nomark"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "a", acceptSeq: 0},
		{SessionID: sessionID, Prompt: "b", acceptSeq: 5},
	})

	fold, canceledWithRunID := a.drainQueueForStep(sessionID, permission.RequestPolicyPrompt)
	require.Len(t, fold, 2, "no cancel mark means all non-RunID queued calls are folded")
	require.Empty(t, canceledWithRunID)
}

func TestDrainQueueForStep_KeepsDifferentPermissionPolicyQueued(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "drain-permission-policy"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{
			SessionID:        sessionID,
			Prompt:           "fold-before-barrier",
			PermissionPolicy: permission.RequestPolicyPrompt,
		},
		{
			SessionID:        sessionID,
			Prompt:           "policy-barrier",
			PermissionPolicy: permission.RequestPolicyAutoApprove,
		},
		{
			SessionID:        sessionID,
			Prompt:           "keep-after-barrier",
			PermissionPolicy: permission.RequestPolicyPrompt,
		},
	})

	fold, canceledWithRunID := a.drainQueueForStep(sessionID, permission.RequestPolicyPrompt)
	require.Len(t, fold, 1)
	require.Equal(t, "fold-before-barrier", fold[0].Prompt)
	require.Empty(t, canceledWithRunID)

	kept, ok := a.messageQueue.Get(sessionID)
	require.True(t, ok)
	require.Len(t, kept, 2)
	require.Equal(t, "policy-barrier", kept[0].Prompt)
	require.Equal(t, permission.RequestPolicyAutoApprove, kept[0].PermissionPolicy)
	require.Equal(t, "keep-after-barrier", kept[1].Prompt)
	require.Equal(t, permission.RequestPolicyPrompt, kept[1].PermissionPolicy)
}

// TestDrainQueueForStep_KeepsRunIDPromptsQueued is the core of fix 2: a
// queued prompt that carries a RunID must NOT be folded into the active
// turn. Folding it would silently absorb it into another turn and never
// publish a RunComplete for its RunID, hanging a `crush run` caller that
// blocks on that event. Such prompts are left in the queue so the
// recursive run path gives each its own turn and its own RunComplete.
// Foldable prompts before the first RunID prompt are still folded, while
// later prompts stay queued to preserve submission order.
func TestDrainQueueForStep_KeepsRunIDPromptsQueued(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "drain-runid"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "fold-me", acceptSeq: 1},
		{SessionID: sessionID, RunID: "run-a", Prompt: "keep-me", acceptSeq: 2},
		{SessionID: sessionID, RunID: "run-b", Prompt: "keep-me-too", acceptSeq: 3},
		{SessionID: sessionID, Prompt: "keep-after-runid", acceptSeq: 4},
	})

	fold, canceledWithRunID := a.drainQueueForStep(sessionID, permission.RequestPolicyPrompt)

	require.Len(t, fold, 1, "only the non-RunID prompt is folded into the active turn")
	require.Equal(t, "fold-me", fold[0].Prompt)
	require.Empty(t, canceledWithRunID)

	kept, ok := a.messageQueue.Get(sessionID)
	require.True(t, ok, "RunID-bearing prompts must remain queued for the recursive run path")
	require.Len(t, kept, 3, "prompts after a RunID prompt must remain queued to preserve submission order")
	require.Equal(t, "run-a", kept[0].RunID)
	require.Equal(t, "run-b", kept[1].RunID)
	require.Equal(t, "keep-after-runid", kept[2].Prompt)
}

// TestDrainQueueForStep_ReportsCanceledRunIDDrops verifies that a queued
// prompt carrying a RunID that is dropped because a cancel covers it is
// reported in canceledWithRunID so the caller can publish its terminal
// cancelled RunComplete. A canceled prompt without a RunID is dropped
// silently as before.
func TestDrainQueueForStep_ReportsCanceledRunIDDrops(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "drain-cancel-runid"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, RunID: "run-canceled", Prompt: "canceled", acceptSeq: 1},
		{SessionID: sessionID, Prompt: "canceled-no-runid", acceptSeq: 1},
		{SessionID: sessionID, RunID: "run-survives", Prompt: "survives", acceptSeq: 5},
	})
	a.cancelMark.Set(sessionID, 2)

	fold, canceledWithRunID := a.drainQueueForStep(sessionID, permission.RequestPolicyPrompt)

	require.Empty(t, fold, "no uncanceled non-RunID prompts to fold")
	require.Len(t, canceledWithRunID, 1,
		"only the dropped RunID-bearing prompt needs a terminal RunComplete")
	require.Equal(t, "run-canceled", canceledWithRunID[0].RunID)

	kept, ok := a.messageQueue.Get(sessionID)
	require.True(t, ok)
	require.Len(t, kept, 1, "the uncanceled RunID prompt stays queued")
	require.Equal(t, "run-survives", kept[0].RunID)
}

// TestRunCompletePublisher_MustDeliverOverTakesPublish exercises the
// pubsub.Publisher interface change end-to-end: a Broker is the only
// concrete Publisher implementation and must satisfy both Publish and
// PublishMustDeliver. The coordinator's final RunComplete emit relies
// on PublishMustDeliver to apply bounded-blocking semantics so a
// momentarily-full subscriber buffer can't silently drop the
// authoritative end-of-run event.
func TestRunCompletePublisher_MustDeliverOverTakesPublish(t *testing.T) {
	t.Parallel()

	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	// Subscribe before publishing so the event is delivered.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ch := broker.Subscribe(ctx)

	rc := notify.RunComplete{SessionID: "S", MessageID: "m", Text: "ok"}
	var pub pubsub.Publisher[notify.RunComplete] = broker
	pub.PublishMustDeliver(t.Context(), pubsub.UpdatedEvent, rc)

	select {
	case got := <-ch:
		require.Equal(t, rc, got.Payload)
	case <-time.After(time.Second):
		t.Fatal("PublishMustDeliver did not deliver event")
	}
}

// requireSingleCancelledRunComplete reads exactly one RunComplete from ch,
// asserts it is the cancelled terminal event for runID, and verifies no
// second event arrives. This observes the published pubsub event rather
// than internal bookkeeping, which is the contract a `crush run` caller
// blocking on the broker actually relies on.
func requireSingleCancelledRunComplete(t *testing.T, ch <-chan pubsub.Event[notify.RunComplete], sessionID, runID string) {
	t.Helper()
	select {
	case ev := <-ch:
		require.Equal(t, runID, ev.Payload.RunID,
			"the published RunComplete must carry the dropped queued prompt's RunID")
		require.Equal(t, sessionID, ev.Payload.SessionID)
		require.True(t, ev.Payload.Cancelled,
			"a dropped queued prompt must publish a cancelled RunComplete")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the cancelled RunComplete")
	}
	select {
	case extra := <-ch:
		t.Fatalf("expected exactly one RunComplete, got a second: %+v", extra.Payload)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestCancel_QueuedRunIDPromptPublishesCancelledRunComplete proves the
// terminal-event behavior end-to-end: a RunID-bearing prompt sitting in
// the queue that is canceled while queued (via the public Cancel path,
// which routes through clearQueueAndNotify -> publishCanceledQueueDrops)
// must emit exactly one cancelled RunComplete on the broker for its
// RunID. A queued prompt without a RunID is dropped silently. This is the
// coverage the earlier drain test lacked: it asserted the returned
// bookkeeping slice, not the published event a `crush run` caller awaits.
func TestCancel_QueuedRunIDPromptPublishesCancelledRunComplete(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	a := NewSessionAgent(SessionAgentOptions{
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)

	subCtx, subCancel := context.WithCancel(t.Context())
	defer subCancel()
	ch := broker.Subscribe(subCtx)

	const sessionID = "cancel-queued-runid"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "no-runid", acceptSeq: 1},
		{SessionID: sessionID, RunID: "run-queued", Prompt: "queued", acceptSeq: 2},
	})

	a.Cancel(sessionID)

	requireSingleCancelledRunComplete(t, ch, sessionID, "run-queued")

	_, ok := a.messageQueue.Get(sessionID)
	require.False(t, ok, "Cancel must clear the queue")
}

// TestDrainQueueForStep_DroppedRunIDPublishesCancelledRunComplete drives
// the production drain sequence (drainQueueForStep then
// publishCanceledQueueDrops, mirroring the PrepareStep handoff) and
// asserts the dropped RunID-bearing prompt actually publishes exactly one
// cancelled RunComplete on the broker. The companion bookkeeping test
// covers the returned slice; this one covers the observable terminal
// event.
func TestDrainQueueForStep_DroppedRunIDPublishesCancelledRunComplete(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	a := NewSessionAgent(SessionAgentOptions{
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)

	subCtx, subCancel := context.WithCancel(t.Context())
	defer subCancel()
	ch := broker.Subscribe(subCtx)

	const sessionID = "drain-drop-runid"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, RunID: "run-dropped", Prompt: "dropped", acceptSeq: 1},
		{SessionID: sessionID, Prompt: "dropped-no-runid", acceptSeq: 1},
	})
	a.cancelMark.Set(sessionID, 2)

	_, canceledWithRunID := a.drainQueueForStep(sessionID, permission.RequestPolicyPrompt)
	require.Len(t, canceledWithRunID, 1)
	a.publishCanceledQueueDrops(canceledWithRunID)

	requireSingleCancelledRunComplete(t, ch, sessionID, "run-dropped")
}
