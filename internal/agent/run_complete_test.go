package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
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
	a.activeRequests.Set(sessionID, &activeCancel{cancel: func() {}})

	var called bool
	hook := func(notify.RunComplete) { called = true }

	res, err := a.Run(t.Context(), SessionAgentCall{
		SessionID:  sessionID,
		RunID:      "run-xyz",
		Prompt:     "queued prompt",
		OnComplete: hook,
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
}

// TestDrainQueueForStep_FiltersUnderDispatchLock verifies that the queue
// drain evaluates the per-session cancel mark while holding the dispatch
// mutex (canceledBySeq's documented precondition). Calls queued after the
// cancel (higher seq) are folded into the active turn; calls at or below
// the cancel high-water mark — and untracked enqueues (seq == 0) whenever
// any mark is present — stay in the queue in their original order. A
// cancel ends the turn in progress, so folding a covered prompt into that
// turn would destroy it; discarding it would lose it outright.
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

	fold := a.drainQueueForStep(sessionID)

	require.Len(t, fold, 1,
		"only the follow-up queued after the cancel (seq > mark) must be folded")
	require.Equal(t, "after", fold[0].Prompt)

	kept, ok := a.messageQueue.Get(sessionID)
	require.True(t, ok, "cancel-covered prompts must survive the drain")
	require.Len(t, kept, 3)
	require.Equal(t, []string{"below", "at-mark", "untracked"},
		[]string{kept[0].Prompt, kept[1].Prompt, kept[2].Prompt},
		"the surviving prompts must keep their original order")
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

	fold := a.drainQueueForStep(sessionID)
	require.Len(t, fold, 2, "no cancel mark means all non-RunID queued calls are folded")
}

// TestDrainQueueForStep_KeepsRunIDPromptsQueued is the core of fix 2: a
// queued prompt that carries a RunID must NOT be folded into the active
// turn. Folding it would silently absorb it into another turn and never
// publish a RunComplete for its RunID, hanging a `crush run` caller that
// blocks on that event. Such prompts are left in the queue so the
// recursive run path gives each its own turn and its own RunComplete.
// Non-RunID prompts are still folded.
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
	})

	fold := a.drainQueueForStep(sessionID)

	require.Len(t, fold, 1, "only the non-RunID prompt is folded into the active turn")
	require.Equal(t, "fold-me", fold[0].Prompt)

	kept, ok := a.messageQueue.Get(sessionID)
	require.True(t, ok, "RunID-bearing prompts must remain queued for the recursive run path")
	require.Len(t, kept, 2)
	require.Equal(t, "run-a", kept[0].RunID)
	require.Equal(t, "run-b", kept[1].RunID)
}

// TestDrainQueueForStep_KeepsCanceledPromptsQueued verifies that a cancel
// covering a queued prompt never removes it: the drain leaves it queued
// (RunID or not) so it survives the canceled turn, stays visible in the
// queue pill, and remains poppable. Only prompts queued after the cancel
// are folded into the active turn.
func TestDrainQueueForStep_KeepsCanceledPromptsQueued(t *testing.T) {
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

	fold := a.drainQueueForStep(sessionID)

	require.Empty(t, fold, "a cancel-covered prompt must not be folded into the canceled turn")

	kept, ok := a.messageQueue.Get(sessionID)
	require.True(t, ok)
	require.Len(t, kept, 3, "cancellation must not remove queued prompts")
	require.Equal(t, []string{"canceled", "canceled-no-runid", "survives"},
		[]string{kept[0].Prompt, kept[1].Prompt, kept[2].Prompt})
}

func TestPopQueuedMessage_NewestFirstWithAttachments(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "pop-newest"
	content := []byte("newest content")
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "oldest"},
		{
			SessionID: sessionID,
			Prompt:    "newest",
			Attachments: []message.Attachment{{
				FilePath: "/tmp/newest.txt",
				FileName: "newest.txt",
				MimeType: "text/plain",
				Content:  content,
			}},
		},
	})

	popped, ok := a.PopQueuedMessage(sessionID)
	require.True(t, ok)
	require.Equal(t, "newest", popped.Prompt)
	require.Equal(t, []message.Attachment{{
		FilePath: "/tmp/newest.txt",
		FileName: "newest.txt",
		MimeType: "text/plain",
		Content:  []byte("newest content"),
	}}, popped.Attachments)

	content[0] = 'X'
	require.Equal(t, []byte("newest content"), popped.Attachments[0].Content,
		"the returned attachment content must not alias the queued call")
	remaining, ok := a.messageQueue.Get(sessionID)
	require.True(t, ok)
	require.Len(t, remaining, 1)
	require.Equal(t, "oldest", remaining[0].Prompt)

	last, ok := a.PopQueuedMessage(sessionID)
	require.True(t, ok)
	require.Equal(t, "oldest", last.Prompt)
	_, ok = a.messageQueue.Get(sessionID)
	require.False(t, ok, "popping the last call must remove the queue map entry")

	empty, ok := a.PopQueuedMessage(sessionID)
	require.False(t, ok)
	require.Equal(t, QueuedMessage{}, empty)
}

// TestPopQueuedMessage_DoesNotAliasQueueBackingArray verifies that the
// queue left behind by a pop does not share its backing array with the
// pre-pop queue. QueuedPrompts/QueuedPromptsList read the queue without
// the per-session dispatch mutex, so a reader can be ranging over a slice
// header captured before the pop; if the pop merely resliced, the next
// enqueueCall would append in place over the popped index and mutate that
// reader's window (a torn read of a multi-word struct under -race).
func TestPopQueuedMessage_DoesNotAliasQueueBackingArray(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "pop-alias"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "oldest"},
		{SessionID: sessionID, Prompt: "newest"},
	})
	// Stand in for a concurrent QueuedPromptsList: it holds the slice
	// header as it was before the pop, spare capacity included.
	beforePop, ok := a.messageQueue.Get(sessionID)
	require.True(t, ok)

	popped, ok := a.PopQueuedMessage(sessionID)
	require.True(t, ok)
	require.Equal(t, "newest", popped.Prompt)

	a.enqueueCall(SessionAgentCall{SessionID: sessionID, Prompt: "requeued"})

	require.Equal(t, []string{"oldest", "newest"}, promptsOf(beforePop),
		"an enqueue after a pop must not write into a pre-pop reader's window")
	require.Equal(t, []string{"oldest", "requeued"},
		a.QueuedPromptsList(sessionID),
		"the live queue must keep the surviving prompt and the new one")
}

func promptsOf(calls []SessionAgentCall) []string {
	prompts := make([]string, len(calls))
	for i, call := range calls {
		prompts[i] = call.Prompt
	}
	return prompts
}

func TestPopQueuedMessage_RunIDPublishesCancelledRunComplete(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ch := broker.Subscribe(ctx)
	const sessionID = "pop-runid"
	a.messageQueue.Set(sessionID, []SessionAgentCall{{
		SessionID: sessionID,
		RunID:     "queued-run",
		Prompt:    "queued",
	}})

	popped, ok := a.PopQueuedMessage(sessionID)
	require.True(t, ok)
	require.Equal(t, "queued", popped.Prompt)
	requireCancelledRunCompletes(t, ch, sessionID, "queued-run")
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

// requireCancelledRunCompletes reads exactly len(runIDs) RunCompletes from
// ch, asserts each is a cancelled terminal event for one of runIDs, that
// every runID appears exactly once, and that no further event arrives. It
// observes the published pubsub events rather than internal bookkeeping,
// which is the contract a `crush run` caller blocking on the broker
// actually relies on. Arrival order is not asserted: the contract is that
// every dropped prompt releases its own caller.
func requireCancelledRunCompletes(t *testing.T, ch <-chan pubsub.Event[notify.RunComplete], sessionID string, runIDs ...string) {
	t.Helper()
	seen := make(map[string]struct{}, len(runIDs))
	for range runIDs {
		select {
		case ev := <-ch:
			require.Equal(t, sessionID, ev.Payload.SessionID)
			require.True(t, ev.Payload.Cancelled,
				"a dropped queued prompt must publish a cancelled RunComplete")
			require.NotContains(t, seen, ev.Payload.RunID,
				"a dropped queued prompt must publish exactly one cancelled RunComplete")
			seen[ev.Payload.RunID] = struct{}{}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for cancelled RunCompletes: want %v, got %d", runIDs, len(seen))
		}
	}
	for _, runID := range runIDs {
		require.Contains(t, seen, runID,
			"every dropped queued prompt must publish a cancelled RunComplete carrying its own RunID")
	}
	select {
	case extra := <-ch:
		t.Fatalf("expected exactly %d RunComplete(s), got another: %+v", len(runIDs), extra.Payload)
	case <-time.After(100 * time.Millisecond):
	}
}

// requireNoRunComplete asserts nothing is published on ch. A queued prompt
// that is still queued has not been discarded, so it must not report a
// terminal cancelled RunComplete: a `crush run` caller blocking on that
// RunID would exit as canceled while its prompt was still pending.
func requireNoRunComplete(t *testing.T, ch <-chan pubsub.Event[notify.RunComplete]) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("expected no RunComplete for a still-queued prompt, got %+v", ev.Payload)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestClearQueue_QueuedRunIDPromptPublishesCancelledRunComplete proves the
// terminal-event behavior end-to-end: a RunID-bearing prompt removed from
// the queue by the explicit ClearQueue path (which routes through
// publishCanceledQueueDrops) must emit exactly one cancelled RunComplete
// on the broker for its RunID. A queued prompt without a RunID is dropped
// silently. The assertions observe the published event a `crush run`
// caller awaits.
func TestClearQueue_QueuedRunIDPromptPublishesCancelledRunComplete(t *testing.T) {
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

	const sessionID = "clear-queued-runid"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "no-runid", acceptSeq: 1},
		{SessionID: sessionID, RunID: "run-queued", Prompt: "queued", acceptSeq: 2},
	})

	drained := a.ClearQueue(sessionID)
	require.Len(t, drained, 2)
	require.Equal(t, []string{"no-runid", "queued"},
		[]string{drained[0].Prompt, drained[1].Prompt},
		"the drain must report every removed prompt, RunID-bearing or not")

	requireCancelledRunCompletes(t, ch, sessionID, "run-queued")

	_, ok := a.messageQueue.Get(sessionID)
	require.False(t, ok, "ClearQueue must clear the queue")
}

// TestClearQueue_PublishesCancelledRunCompleteForEveryRunIDBearingDrop
// pins the loop in publishCanceledQueueDrops rather than its first
// iteration: one drain removes the whole queue, so it can retire any
// number of RunID-bearing prompts at once — a burst of scripted
// `crush run` submissions taken off the queue by a single Escape. Each of
// those callers blocks until a RunComplete carrying its own RunID
// arrives, so publishing only the first drop leaves every caller behind
// it hanging until connection teardown. The interleaved RunID-less prompt
// must not consume a publish of its own.
func TestClearQueue_PublishesCancelledRunCompleteForEveryRunIDBearingDrop(t *testing.T) {
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

	const sessionID = "clear-many-queued-runids"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, RunID: "run-a", Prompt: "first", acceptSeq: 1},
		{SessionID: sessionID, Prompt: "no-runid", acceptSeq: 2},
		{SessionID: sessionID, RunID: "run-b", Prompt: "second", acceptSeq: 3},
	})

	require.Len(t, a.ClearQueue(sessionID), 3,
		"the drain must report every removed prompt, RunID-bearing or not")

	requireCancelledRunCompletes(t, ch, sessionID, "run-a", "run-b")
}

// TestClearQueue_DrainsInOrderWithOwnedAttachments pins the
// drain-and-return contract the Escape gesture depends on: the clear
// reports every message it removed, oldest to newest, with attachments
// whose bytes the caller owns, and it deletes the queue map entry. An
// empty queue drains to nothing.
func TestClearQueue_DrainsInOrderWithOwnedAttachments(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "clear-drain"
	content := []byte("oldest content")
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{
			SessionID: sessionID,
			Prompt:    "oldest",
			Attachments: []message.Attachment{{
				FilePath: "/tmp/oldest.txt",
				FileName: "oldest.txt",
				MimeType: "text/plain",
				Content:  content,
			}},
		},
		{SessionID: sessionID, Prompt: "middle"},
		{SessionID: sessionID, Prompt: "newest"},
	})

	drained := a.ClearQueue(sessionID)
	require.Len(t, drained, 3)
	require.Equal(t, []string{"oldest", "middle", "newest"},
		[]string{drained[0].Prompt, drained[1].Prompt, drained[2].Prompt},
		"the drain must preserve queue order, oldest to newest")
	require.Equal(t, []message.Attachment{{
		FilePath: "/tmp/oldest.txt",
		FileName: "oldest.txt",
		MimeType: "text/plain",
		Content:  []byte("oldest content"),
	}}, drained[0].Attachments)

	content[0] = 'X'
	require.Equal(t, []byte("oldest content"), drained[0].Attachments[0].Content,
		"the drained attachment content must not alias the queued call")

	_, ok := a.messageQueue.Get(sessionID)
	require.False(t, ok, "the drain must delete the queue map entry")
	require.Nil(t, a.ClearQueue(sessionID), "draining an empty queue returns nothing")
}

// TestClearQueue_HoldsSessionDispatchMutex pins the synchronization the
// returned payload requires: the drain removes messages the caller then
// acts on, so it must be atomic against the run handoff and the
// PrepareStep drain — a message reported as drained can never also have
// been started. The pre-drain implementation returned nothing and skipped
// the mutex, which was unobservable only because the content was thrown
// away.
func TestClearQueue_HoldsSessionDispatchMutex(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	const sessionID = "clear-dispatch-mutex"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "queued"},
	})

	mu := a.sessionMu(sessionID)
	mu.Lock()
	started := make(chan struct{})
	done := make(chan []QueuedMessage, 1)
	go func() {
		close(started)
		done <- a.ClearQueue(sessionID)
	}()
	// Without waiting for the goroutine to reach the call, the negative
	// window below is satisfied by a goroutine that was never scheduled —
	// which is exactly what happens under -race load, and it would pass with
	// the mutex removed.
	<-started

	select {
	case <-done:
		mu.Unlock()
		t.Fatal("ClearQueue drained the queue without the session dispatch mutex")
	case <-time.After(100 * time.Millisecond):
	}

	mu.Unlock()
	select {
	case drained := <-done:
		require.Len(t, drained, 1)
		require.Equal(t, "queued", drained[0].Prompt)
	case <-time.After(5 * time.Second):
		t.Fatal("ClearQueue never completed after the dispatch mutex was released")
	}
}

// TestSummarize_QueuePromotionHoldsSessionDispatchMutex pins the
// synchronization of the promotion at the end of Summarize. The tail
// releases the active request first, so the session reads idle for the
// whole promotion: a submission landing in that window takes Run's
// idle-with-queue branch, which swaps the submission into the queue and
// starts the queue head. Promoting without the dispatch mutex writes the
// tail's stale pre-swap snapshot back over that swap — the submission is
// wiped from the queue (never run, no terminal RunComplete for its RunID)
// and the head, already active, is re-queued and runs a second time.
func TestSummarize_QueuePromotionHoldsSessionDispatchMutex(t *testing.T) {
	t.Parallel()

	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	// Summarize returns early on an empty session.
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "history"},
		},
	})
	require.NoError(t, err)

	sa.enqueueCall(SessionAgentCall{SessionID: sess.ID, Prompt: "queued"})

	mu := sa.sessionMu(sess.ID)
	mu.Lock()

	done := make(chan error, 1)
	go func() { done <- sa.Summarize(context.WithoutCancel(t.Context()), sess.ID, nil, nil) }()

	// Persisting the summary on the session is the last step before the
	// tail, so once it lands the only work left is the promotion.
	require.Eventually(t, func() bool {
		s, err := env.sessions.Get(t.Context(), sess.ID)
		return err == nil && s.SummaryMessageID != ""
	}, 10*time.Second, 10*time.Millisecond,
		"summarize never reached its queue-promotion tail")

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID),
		"the summarize tail dequeued the queue head without the session dispatch mutex")

	mu.Unlock()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("Summarize never completed after the dispatch mutex was released")
	}

	require.Equal(t, 0, sa.QueuedPrompts(sess.ID),
		"the queued prompt must be promoted once the lock is available")
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var ranQueued bool
	for _, msg := range msgs {
		if msg.Role == message.User && msg.Content().String() == "queued" {
			ranQueued = true
		}
	}
	require.True(t, ranQueued, "the promoted prompt must have run as its own turn")
}

// TestCancel_PreservesQueuedPrompts is the regression test for issue
// #3558: Escape must cancel the turn in progress without discarding
// queued prompts. It drives the public Cancel path with an active request
// and an accepted follow-up so both cancel branches fire, then asserts the
// queue is untouched — order, count, and RunID-bearing entries — and that
// no queued prompt was reported as cancelled to a waiting `crush run`.
func TestCancel_PreservesQueuedPrompts(t *testing.T) {
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

	const sessionID = "cancel-preserves-queue"
	var activeCanceled atomic.Bool
	a.activeRequests.Set(sessionID, &activeCancel{cancel: func() { activeCanceled.Store(true) }})
	accept := a.BeginAccepted(sessionID)
	defer accept.Close()
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, Prompt: "first", acceptSeq: 1},
		{SessionID: sessionID, RunID: "run-queued", Prompt: "second", acceptSeq: 2},
	})

	a.Cancel(sessionID)

	require.True(t, activeCanceled.Load(), "the active turn must still be canceled")
	require.True(t, a.hasPendingCancel(sessionID),
		"the accepted-but-not-active follow-up must still be covered")

	queued, ok := a.messageQueue.Get(sessionID)
	require.True(t, ok, "Cancel must not delete the queue")
	require.Len(t, queued, 2, "Cancel must not discard queued prompts")
	require.Equal(t, "first", queued[0].Prompt)
	require.Equal(t, "second", queued[1].Prompt)
	require.Equal(t, "run-queued", queued[1].RunID)
	require.Equal(t, 2, a.QueuedPrompts(sessionID))

	requireNoRunComplete(t, ch)
}

// TestCancelAll_ShutdownDiscardsQueueAndNotifies covers the one place that
// still discards queued prompts implicitly: shutdown. Cancel keeps the
// queue because a queued prompt can still run later, but CancelAll runs
// when the workspace is going away, so every queued RunID must receive its
// terminal cancelled RunComplete or a `crush run` caller blocking on it
// would never exit.
func TestCancelAll_ShutdownDiscardsQueueAndNotifies(t *testing.T) {
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

	const sessionID = "cancel-all-shutdown"
	// Releasing the active entry mirrors processRequest's deferred
	// cleanup, so CancelAll's busy-wait finishes immediately.
	a.activeRequests.Set(sessionID, &activeCancel{
		cancel: func() { a.activeRequests.Del(sessionID) },
	})
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, RunID: "run-queued", Prompt: "queued", acceptSeq: 1},
	})

	a.CancelAll()

	requireCancelledRunCompletes(t, ch, sessionID, "run-queued")
	require.Equal(t, 0, a.QueuedPrompts(sessionID),
		"shutdown must discard queued prompts it can no longer run")
}

// TestCancelAll_ShutdownDiscardsQueueOnIdleSession is the same shutdown
// contract for the state Cancel actually leaves behind. Cancel is
// turn-scoped: it ends the active turn and keeps the queue, so after
// "esc esc" the session has *no* active request and a non-empty queue.
// Keying the teardown on activeRequests alone would make CancelAll return
// early (nothing is busy), leaving the queued RunIDs without a terminal
// event and hanging any caller blocking on one.
func TestCancelAll_ShutdownDiscardsQueueOnIdleSession(t *testing.T) {
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

	const sessionID = "cancel-all-idle"
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, RunID: "run-queued", Prompt: "queued", acceptSeq: 1},
	})
	require.False(t, a.IsBusy())

	a.CancelAll()

	requireCancelledRunCompletes(t, ch, sessionID, "run-queued")
	require.Equal(t, 0, a.QueuedPrompts(sessionID),
		"shutdown must discard the queue of a session left idle by a cancel")
}

// TestCancelAll_SummarizeKeyClearsSessionQueue guards the key
// normalization: a summarizing session is tracked in activeRequests under
// the synthetic "<id>-summarize" key, while its queue is stored under the
// real session ID. Passing the synthetic key straight to ClearQueue makes
// it a silent no-op, so the queued RunID would never be notified.
func TestCancelAll_SummarizeKeyClearsSessionQueue(t *testing.T) {
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

	const sessionID = "cancel-all-summarize"
	summarizeKey := sessionID + summarizeKeySuffix
	// Releasing the active entry mirrors Summarize's cleanup, so
	// CancelAll's busy-wait finishes immediately.
	a.activeRequests.Set(summarizeKey, &activeCancel{
		cancel: func() { a.activeRequests.Del(summarizeKey) },
	})
	a.messageQueue.Set(sessionID, []SessionAgentCall{
		{SessionID: sessionID, RunID: "run-queued", Prompt: "queued", acceptSeq: 1},
	})

	a.CancelAll()

	requireCancelledRunCompletes(t, ch, sessionID, "run-queued")
	require.Equal(t, 0, a.QueuedPrompts(sessionID),
		"the summarize key must be normalized to the session whose queue it owns")
}
