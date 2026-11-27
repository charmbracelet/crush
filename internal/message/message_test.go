package message

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// slowUpdateQuerier wraps a [db.Querier] and forces UpdateMessage to
// hang on a release channel. Used to simulate an in-flight SQL write.
type slowUpdateQuerier struct {
	db.Querier
	release   chan struct{}
	started   chan struct{}
	startOnce sync.Once
}

func (s *slowUpdateQuerier) UpdateMessage(ctx context.Context, arg db.UpdateMessageParams) error {
	s.startOnce.Do(func() { close(s.started) })
	select {
	case <-s.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.Querier.UpdateMessage(ctx, arg)
}

// deleteHookQuerier wraps a [db.Querier] and runs beforeDelete inside
// DeleteMessage, before the row goes. An error from it fails the delete.
type deleteHookQuerier struct {
	db.Querier
	beforeDelete func() error
}

func (d *deleteHookQuerier) DeleteMessage(ctx context.Context, id string) error {
	if err := d.beforeDelete(); err != nil {
		return err
	}
	return d.Querier.DeleteMessage(ctx, id)
}

// receive returns the next value from ch, failing the test if none
// arrives in time, so a missing event fails instead of hanging.
func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a value")
		var zero T
		return zero
	}
}

// waitForDeleting waits until Delete has marked id.
func waitForDeleting(t *testing.T, impl *service, id string) {
	t.Helper()
	require.Eventually(t, func() bool {
		impl.mu.Lock()
		defer impl.mu.Unlock()
		p, ok := impl.pending[id]
		return ok && p.deletion() == deleting
	}, time.Second, time.Millisecond)
}

// newTestService spins up a fresh in-memory message.Service backed by a
// temporary on-disk SQLite database. Returns the service plus a session
// ID to attach messages to.
func newTestService(t *testing.T, opts ...ServiceOption) (Service, string) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	svc := NewService(q, opts...)
	return svc, sess.ID
}

// eventCollector consumes broker events into a slice in a goroutine
// and exposes thread-safe Snapshot / Reset helpers for assertions.
type eventCollector struct {
	mu     sync.Mutex
	events []pubsub.Event[Message]
}

func collect(ctx context.Context, sub <-chan pubsub.Event[Message]) *eventCollector {
	c := &eventCollector{}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-sub:
				if !ok {
					return
				}
				c.mu.Lock()
				c.events = append(c.events, ev)
				c.mu.Unlock()
			}
		}
	}()
	return c
}

func (c *eventCollector) snapshot() []pubsub.Event[Message] {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]pubsub.Event[Message], len(c.events))
	copy(out, c.events)
	return out
}

// dropCreated waits for the CreatedEvent that Create publishes, then clears
// it, so it cannot land after the reset and be mistaken for a later event.
func (c *eventCollector) dropCreated(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		return len(c.snapshot()) >= 1
	}, time.Second, time.Millisecond)
	require.Equal(t, pubsub.CreatedEvent, c.snapshot()[0].Type)
	c.reset()
}

func (c *eventCollector) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = nil
}

func TestUpdate_DebouncesTextDeltas(t *testing.T) {
	t.Parallel()

	// Long-enough debounce that we can verify nothing flushes prematurely.
	svc, sessionID := newTestService(t, WithDebounce(50*time.Millisecond))

	subCtx, cancelSub := context.WithCancel(t.Context())
	defer cancelSub()
	sub := svc.Subscribe(subCtx)
	collector := collect(subCtx, sub)

	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{
		Role: Assistant,
	})
	require.NoError(t, err)
	collector.dropCreated(t)

	// Push 5 deltas inside a single debounce window.
	for i := 0; i < 5; i++ {
		msg.AppendContent("a")
		require.NoError(t, svc.Update(t.Context(), msg))
	}

	// Before the debounce expires no UpdatedEvent should have landed.
	time.Sleep(10 * time.Millisecond)
	require.Empty(t, collector.snapshot(), "no events should land before debounce window expires")

	// Wait for the debounce timer to fire.
	require.Eventually(t, func() bool {
		return len(collector.snapshot()) >= 1
	}, time.Second, 5*time.Millisecond)
	events := collector.snapshot()
	require.Len(t, events, 1, "5 deltas should coalesce into 1 UpdatedEvent")
	require.Equal(t, pubsub.UpdatedEvent, events[0].Type)
	require.Equal(t, "aaaaa", events[0].Payload.Content().Text)

	// Final state must be persisted.
	got, err := svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Equal(t, "aaaaa", got.Content().Text)
}

func TestUpdate_TerminalUpdatesFlushSynchronously(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t, WithDebounce(time.Hour))

	subCtx, cancelSub := context.WithCancel(t.Context())
	defer cancelSub()
	sub := svc.Subscribe(subCtx)
	collector := collect(subCtx, sub)

	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)
	collector.dropCreated(t)

	// AddFinish makes the message terminal; Update must flush
	// synchronously even with a 1-hour debounce.
	msg.AppendContent("done")
	msg.AddFinish(FinishReasonEndTurn, "", "")
	require.NoError(t, svc.Update(t.Context(), msg))

	require.Eventually(t, func() bool {
		return len(collector.snapshot()) >= 1
	}, time.Second, 5*time.Millisecond,
		"terminal update must publish without waiting for debounce")
	events := collector.snapshot()
	require.Len(t, events, 1)
	require.True(t, events[0].Payload.IsFinished())

	got, err := svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.True(t, got.IsFinished())
}

func TestUpdate_ToolCallStructuralChangeFlushes(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t, WithDebounce(time.Hour))

	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)

	// Adding a new tool call is a structural change → sync flush.
	msg.AddToolCall(ToolCall{ID: "tc1", Name: "view", Finished: false})
	require.NoError(t, svc.Update(t.Context(), msg))

	got, err := svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Len(t, got.ToolCalls(), 1)
	require.Equal(t, "tc1", got.ToolCalls()[0].ID)

	// Marking the tool call finished is also a structural change.
	msg.AddToolCall(ToolCall{ID: "tc1", Name: "view", Input: "{}", Finished: true})
	require.NoError(t, svc.Update(t.Context(), msg))

	got, err = svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.True(t, got.ToolCalls()[0].Finished)
}

func TestUpdate_ReasoningEndFlushes(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t, WithDebounce(time.Hour))

	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)

	// Reasoning deltas alone debounce.
	msg.AppendReasoningContent("hmm")
	require.NoError(t, svc.Update(t.Context(), msg))
	got, err := svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Empty(t, got.ReasoningContent().Thinking, "reasoning delta should still be in the debounce buffer")

	// FinishThinking sets FinishedAt → terminal flush.
	msg.FinishThinking()
	require.NoError(t, svc.Update(t.Context(), msg))

	got, err = svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Equal(t, "hmm", got.ReasoningContent().Thinking)
	require.NotZero(t, got.ReasoningContent().FinishedAt)
}

func TestFlush_DrainsPendingDebouncedUpdates(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t, WithDebounce(time.Hour))

	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)
	msg.AppendContent("buffered")
	require.NoError(t, svc.Update(t.Context(), msg))

	// Without a flush the SQL row is unchanged from Create.
	got, err := svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Empty(t, got.Content().Text)

	require.NoError(t, svc.Flush(t.Context(), msg.ID))

	got, err = svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Equal(t, "buffered", got.Content().Text)

	// Subsequent Flush is a no-op.
	require.NoError(t, svc.Flush(t.Context(), msg.ID))
}

func TestFlushAll_DrainsAllPending(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t, WithDebounce(time.Hour))

	const n = 5
	msgs := make([]Message, n)
	for i := range msgs {
		m, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
		require.NoError(t, err)
		m.AppendContent("hi")
		require.NoError(t, svc.Update(t.Context(), m))
		msgs[i] = m
	}

	require.NoError(t, svc.FlushAll(t.Context()))

	for _, m := range msgs {
		got, err := svc.Get(t.Context(), m.ID)
		require.NoError(t, err)
		require.Equal(t, "hi", got.Content().Text, "FlushAll should drain every pending message")
	}
}

func TestUpdate_OrderingMatchesNonCoalesced(t *testing.T) {
	t.Parallel()

	// Compare the final state after coalesced vs zero-debounce updates.
	// A sequence of interleaved text/reasoning/tool-call updates must
	// converge to the same final DB row either way.
	build := func(svc Service, sessionID string) Message {
		msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
		require.NoError(t, err)
		msg.AppendReasoningContent("thinking 1 ")
		require.NoError(t, svc.Update(t.Context(), msg))
		msg.AppendReasoningContent("thinking 2")
		require.NoError(t, svc.Update(t.Context(), msg))
		msg.FinishThinking()
		require.NoError(t, svc.Update(t.Context(), msg))
		msg.AppendContent("hello ")
		require.NoError(t, svc.Update(t.Context(), msg))
		msg.AppendContent("world")
		require.NoError(t, svc.Update(t.Context(), msg))
		msg.AddToolCall(ToolCall{ID: "tc", Name: "x", Finished: false})
		require.NoError(t, svc.Update(t.Context(), msg))
		msg.AddToolCall(ToolCall{ID: "tc", Name: "x", Input: "{}", Finished: true})
		require.NoError(t, svc.Update(t.Context(), msg))
		msg.AddFinish(FinishReasonEndTurn, "", "")
		require.NoError(t, svc.Update(t.Context(), msg))
		return msg
	}

	coalesced, sid1 := newTestService(t, WithDebounce(20*time.Millisecond))
	a := build(coalesced, sid1)
	require.NoError(t, coalesced.FlushAll(t.Context()))
	gotA, err := coalesced.Get(t.Context(), a.ID)
	require.NoError(t, err)

	immediate, sid2 := newTestService(t, WithDebounce(0))
	b := build(immediate, sid2)
	gotB, err := immediate.Get(t.Context(), b.ID)
	require.NoError(t, err)

	require.Equal(t, gotA.Content().Text, gotB.Content().Text)
	require.Equal(t, gotA.ReasoningContent().Thinking, gotB.ReasoningContent().Thinking)
	require.Equal(t, len(gotA.ToolCalls()), len(gotB.ToolCalls()))
	require.Equal(t, gotA.IsFinished(), gotB.IsFinished())
}

func TestDelete_DropsPendingState(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t, WithDebounce(time.Hour))
	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)
	msg.AppendContent("dropped")
	require.NoError(t, svc.Update(t.Context(), msg))

	require.NoError(t, svc.Delete(t.Context(), msg.ID))

	// FlushAll after Delete must not write to the deleted row.
	require.NoError(t, svc.FlushAll(t.Context()))

	_, err = svc.Get(t.Context(), msg.ID)
	require.Error(t, err, "deleted message must remain deleted")
}

func TestBroker_PublishLossyDropCounter(t *testing.T) {
	t.Parallel()

	// Tiny channel buffer so we can saturate from a single sender.
	b := pubsub.NewBrokerWithOptions[int](1)
	defer b.Shutdown()

	subCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := b.Subscribe(subCtx)
	require.NotNil(t, sub)

	// Don't read from sub. Saturate the buffer.
	for range 100 {
		b.Publish(pubsub.UpdatedEvent, 1)
	}

	require.GreaterOrEqual(t, b.DropCount(), uint64(1),
		"lossy Publish must increment the drop counter under contention")
}

func TestBroker_PublishMustDeliverHonorsTimeout(t *testing.T) {
	t.Parallel()

	b := pubsub.NewBrokerWithOptions[int](1)
	b.SetMustDeliverTimeout(20 * time.Millisecond)
	defer b.Shutdown()

	subCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := b.Subscribe(subCtx)
	require.NotNil(t, sub)

	// Saturate: the subscriber's pump holds one event and its queue
	// (capacity 1) the next. Keep publishing across several ticks so
	// the pump has parked before we stop.
	require.Eventually(t, func() bool {
		b.Publish(pubsub.UpdatedEvent, 1)
		return b.DropCount() >= 3
	}, time.Second, 5*time.Millisecond)

	// PublishMustDeliver should block up to 20ms then drop.
	start := time.Now()
	b.PublishMustDeliver(t.Context(), pubsub.UpdatedEvent, 2)
	elapsed := time.Since(start)

	require.GreaterOrEqual(t, elapsed, 20*time.Millisecond,
		"PublishMustDeliver should block at least the timeout under contention")
	require.Less(t, elapsed, 200*time.Millisecond,
		"PublishMustDeliver must not block indefinitely")
	require.GreaterOrEqual(t, b.MustDeliverDropCount(), uint64(1),
		"timeout must increment the must-deliver drop counter")
}

func TestBroker_PublishMustDeliverWithReader(t *testing.T) {
	t.Parallel()

	b := pubsub.NewBrokerWithOptions[int](1)
	b.SetMustDeliverTimeout(50 * time.Millisecond)
	defer b.Shutdown()

	subCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := b.Subscribe(subCtx)

	var received atomic.Uint64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-subCtx.Done():
				return
			case _, ok := <-sub:
				if !ok {
					return
				}
				received.Add(1)
			}
		}
	}()

	for i := range 10 {
		b.PublishMustDeliver(t.Context(), pubsub.UpdatedEvent, i)
	}

	// All 10 should land within the must-deliver timeout window.
	require.Eventually(t, func() bool { return received.Load() == 10 },
		time.Second, 5*time.Millisecond,
		"all must-deliver events should reach an active subscriber")
	require.Zero(t, b.MustDeliverDropCount(),
		"no drops expected when subscriber drains promptly")
}

func TestUpdate_TerminalEventUsesMustDeliver(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t, WithDebounce(time.Hour))

	subCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := svc.Subscribe(subCtx)

	var seenFinish atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-subCtx.Done():
				return
			case ev, ok := <-sub:
				if !ok {
					return
				}
				if ev.Type == pubsub.UpdatedEvent && ev.Payload.IsFinished() {
					seenFinish.Store(true)
				}
			}
		}
	}()

	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)
	msg.AppendContent("final")
	msg.AddFinish(FinishReasonEndTurn, "", "")
	require.NoError(t, svc.Update(t.Context(), msg))

	require.Eventually(t, func() bool { return seenFinish.Load() },
		time.Second, 10*time.Millisecond,
		"terminal update must reach subscribers via the must-deliver path")
}

func TestUpdate_ZeroDebounceFlushesEveryUpdate(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t, WithDebounce(0))

	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		msg.AppendContent("x")
		require.NoError(t, svc.Update(t.Context(), msg))
		got, err := svc.Get(t.Context(), msg.ID)
		require.NoError(t, err)
		require.Len(t, got.Content().Text, i+1, "every update must land synchronously when debounce is 0")
	}
}

// TestFlush_WaitsForInFlightWrite reproduces the failure where Flush
// or FlushAll could return before a concurrent in-flight SQL write
// completed. We block UpdateMessage on a release channel, fire the
// debounce timer, then call Flush and assert it does not return until
// the in-flight write actually lands.
func TestFlush_WaitsForInFlightWrite(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	slow := &slowUpdateQuerier{
		Querier: q,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
	// Short debounce so the timer fires quickly.
	svc := NewService(slow, WithDebounce(10*time.Millisecond))

	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)
	msg.AppendContent("payload")
	require.NoError(t, svc.Update(t.Context(), msg))

	// Wait for the timer-fired flush to enter UpdateMessage.
	select {
	case <-slow.started:
	case <-time.After(time.Second):
		t.Fatal("timer-fired flush never reached UpdateMessage")
	}

	// At this point the buffer is clean but a flush is in flight. A
	// naive Flush would early-return on !dirty. Spawn Flush in a
	// goroutine and assert it has not returned while the write is
	// still blocked.
	flushDone := make(chan error, 1)
	go func() { flushDone <- svc.Flush(t.Context(), msg.ID) }()

	select {
	case err := <-flushDone:
		t.Fatalf("Flush returned %v while in-flight write was still blocked", err)
	case <-time.After(50 * time.Millisecond):
		// Expected: Flush is correctly waiting.
	}

	// Release the slow write; Flush must now return cleanly.
	close(slow.release)
	select {
	case err := <-flushDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Flush did not return after in-flight write completed")
	}

	// The SQL row should now reflect the buffered payload.
	got, err := svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Equal(t, "payload", got.Content().Text)
}

// TestUpdate_DuringTimerFlushIsWritten asserts that a delta arriving while
// a timer-fired flush writes is written and published too, without waiting
// for a later Update or Flush. The delta cannot arm its own timer while the
// write runs, so the flusher must schedule one.
func TestUpdate_DuringTimerFlushIsWritten(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sess, err := session.NewService(q, conn).Create(t.Context(), "test")
	require.NoError(t, err)

	slow := &slowUpdateQuerier{
		Querier: q,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
	const debounce = 10 * time.Millisecond
	svc := NewService(slow, WithDebounce(debounce))

	subCtx, cancelSub := context.WithCancel(t.Context())
	defer cancelSub()
	collector := collect(subCtx, svc.Subscribe(subCtx))

	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)
	collector.dropCreated(t)
	msg.AppendContent("first")
	require.NoError(t, svc.Update(t.Context(), msg))

	select {
	case <-slow.started:
	case <-time.After(time.Second):
		t.Fatal("timer-fired flush never reached UpdateMessage")
	}
	msg.AppendContent(" second")
	require.NoError(t, svc.Update(t.Context(), msg))
	// Outlast the debounce window, so a timer armed by the Update itself
	// would fire during the write and give up.
	time.Sleep(5 * debounce)
	close(slow.release)

	require.Eventually(t, func() bool {
		got, err := q.GetMessage(t.Context(), msg.ID)
		return err == nil && strings.Contains(got.Parts, "first second")
	}, time.Second, 5*time.Millisecond, "the delta made during the write was never written")
	require.Eventually(t, func() bool {
		events := collector.snapshot()
		return len(events) > 0 && events[len(events)-1].Payload.Content().Text == "first second"
	}, time.Second, 5*time.Millisecond, "the delta made during the write was never published")
}

// TestFlushAll_WaitsForInFlightWrite asserts FlushAll picks up IDs
// whose buffer is currently flushing (dirty=false) so shutdown and
// session-switch callers don't return while an SQL write is mid-flight.
func TestFlushAll_WaitsForInFlightWrite(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	slow := &slowUpdateQuerier{
		Querier: q,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
	svc := NewService(slow, WithDebounce(10*time.Millisecond))

	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)
	msg.AppendContent("payload")
	require.NoError(t, svc.Update(t.Context(), msg))

	select {
	case <-slow.started:
	case <-time.After(time.Second):
		t.Fatal("timer-fired flush never reached UpdateMessage")
	}

	flushDone := make(chan error, 1)
	go func() { flushDone <- svc.FlushAll(t.Context()) }()

	select {
	case err := <-flushDone:
		t.Fatalf("FlushAll returned %v while in-flight write was still blocked", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(slow.release)
	select {
	case err := <-flushDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("FlushAll did not return after in-flight write completed")
	}

	got, err := svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Equal(t, "payload", got.Content().Text)
}

func TestUpdate_PublishesSnapshotsInWriteOrder(t *testing.T) {
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	svc := NewService(q, WithDebounce(time.Hour))
	impl := svc.(*service)
	impl.Shutdown()
	impl.Broker = pubsub.NewBrokerWithOptions[Message](1)
	impl.SetMustDeliverTimeout(2 * time.Second)

	subCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := svc.Subscribe(subCtx)

	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)
	msg.AddToolCall(ToolCall{ID: "tc1", Name: "view"})

	// The subscriber's pump holds the Created event because nothing reads
	// yet. This filler waits for the one-slot queue behind it and then fills
	// that queue, so the next must-deliver publication blocks.
	impl.PublishMustDeliver(t.Context(), pubsub.CreatedEvent, Message{ID: "filler"})

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- svc.Update(t.Context(), msg)
	}()

	// The subscriber queue is full, so this update blocks in
	// PublishMustDeliver after its SQL write. The per-message flushing guard
	// must remain held through publication so a newer update cannot overtake it.
	require.Eventually(t, func() bool {
		stored, getErr := svc.Get(t.Context(), msg.ID)
		return getErr == nil && len(stored.ToolCalls()) == 1
	}, time.Second, time.Millisecond)
	impl.mu.Lock()
	flushing := impl.pending[msg.ID].flushDone != nil
	impl.mu.Unlock()
	require.True(t, flushing)

	newer := msg.Clone()
	newer.FinishToolCall("tc1")
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- svc.Update(t.Context(), newer)
	}()

	select {
	case err := <-secondDone:
		require.Failf(t, "newer update overtook blocked publication", "Update returned: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	created := receive(t, sub)
	require.Equal(t, pubsub.CreatedEvent, created.Type)
	require.Equal(t, msg.ID, created.Payload.ID)
	filler := receive(t, sub)
	require.Equal(t, "filler", filler.Payload.ID)
	first := receive(t, sub)
	require.Equal(t, pubsub.UpdatedEvent, first.Type)
	require.False(t, first.Payload.ToolCalls()[0].Finished)
	second := receive(t, sub)
	require.Equal(t, pubsub.UpdatedEvent, second.Type)
	require.True(t, second.Payload.ToolCalls()[0].Finished)

	require.NoError(t, receive(t, firstDone))
	require.NoError(t, receive(t, secondDone))
}

func TestDeleteSuppressesInFlightUpdatePublication(t *testing.T) {
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	slow := &slowUpdateQuerier{
		Querier: q,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
	svc := NewService(slow, WithDebounce(0))
	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)

	subCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := svc.Subscribe(subCtx)

	msg.AppendContent("stale update")
	updateDone := make(chan error, 1)
	go func() {
		updateDone <- svc.Update(t.Context(), msg)
	}()
	receive(t, slow.started)

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- svc.Delete(t.Context(), msg.ID)
	}()
	waitForDeleting(t, svc.(*service), msg.ID)
	select {
	case err := <-deleteDone:
		require.Failf(t, "delete did not wait for the in-flight update", "Delete returned: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	close(slow.release)
	require.NoError(t, receive(t, updateDone))
	require.NoError(t, receive(t, deleteDone))

	event := receive(t, sub)
	require.Equal(t, pubsub.DeletedEvent, event.Type)
	select {
	case event := <-sub:
		require.Failf(t, "stale event published after delete", "Event type: %s", event.Type)
	case <-time.After(25 * time.Millisecond):
	}
	_, err = svc.Get(t.Context(), msg.ID)
	require.Error(t, err)
}

func TestCanceledDeleteRestoresPendingUpdates(t *testing.T) {
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	slow := &slowUpdateQuerier{
		Querier: q,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
	svc := NewService(slow, WithDebounce(0))
	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)

	msg.AppendContent("before cancel")
	updateDone := make(chan error, 1)
	go func() {
		updateDone <- svc.Update(t.Context(), msg)
	}()
	receive(t, slow.started)

	deleteCtx, cancelDelete := context.WithCancel(t.Context())
	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- svc.Delete(deleteCtx, msg.ID)
	}()
	waitForDeleting(t, svc.(*service), msg.ID)
	msg.AppendContent(" during cancel")
	require.NoError(t, svc.Update(t.Context(), msg))
	cancelDelete()
	require.ErrorIs(t, receive(t, deleteDone), context.Canceled)

	close(slow.release)
	require.NoError(t, receive(t, updateDone))
	// The update held during the delete lands on its own, before a later
	// update could carry the same text.
	require.Eventually(t, func() bool {
		stored, err := svc.Get(t.Context(), msg.ID)
		return err == nil && stored.Content().Text == "before cancel during cancel"
	}, time.Second, time.Millisecond)
	msg.AppendContent(" after cancel")
	require.NoError(t, svc.Update(t.Context(), msg))

	stored, err := svc.Get(t.Context(), msg.ID)
	require.NoError(t, err)
	require.Equal(t, "before cancel during cancel after cancel", stored.Content().Text)
}

// TestUpdate_StructuralFlushUsesMustDeliver covers the second review
// finding: structural terminal events (tool-call add, tool-call
// finish, reasoning end) must publish via the must-deliver path even
// when the message itself is not yet IsFinished.
//
// We detect which path was taken by saturating a subscriber's channel
// buffer with no reader. With a short must-deliver timeout, the
// must-deliver path increments [pubsub.Broker.MustDeliverDropCount]
// after the timeout expires; the lossy path increments
// [pubsub.Broker.DropCount] immediately. The two counters are
// disjoint, so they precisely identify which call site published the
// event.
func TestUpdate_StructuralFlushUsesMustDeliver(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		mut  func(*Message)
	}{
		{
			name: "tool call add",
			mut: func(m *Message) {
				m.AddToolCall(ToolCall{ID: "tc1", Name: "view"})
			},
		},
		{
			name: "tool call finish",
			mut: func(m *Message) {
				m.AddToolCall(ToolCall{ID: "tc1", Name: "view", Input: "{}", Finished: true})
			},
		},
		{
			name: "reasoning end",
			mut: func(m *Message) {
				m.AppendReasoningContent("hmm")
				m.FinishThinking()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			conn, err := db.Connect(t.Context(), t.TempDir())
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })

			q := db.New(conn)
			sessions := session.NewService(q, conn)
			sess, err := sessions.Create(t.Context(), "test")
			require.NoError(t, err)

			// Replace the default broker with a tiny buffer + short
			// must-deliver timeout so we can fully saturate from a
			// single sender and observe drops without long waits.
			svc := NewService(q, WithDebounce(time.Hour))
			impl := svc.(*service)
			impl.Shutdown()
			impl.Broker = pubsub.NewBrokerWithOptions[Message](1)
			impl.SetMustDeliverTimeout(40 * time.Millisecond)

			subCtx, cancel := context.WithCancel(t.Context())
			defer cancel()
			sub := svc.Subscribe(subCtx)

			msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
			require.NoError(t, err)

			// Saturate the subscriber, which we never read: its pump
			// holds one event and its queue (capacity 1) the next.
			// Keep publishing fillers across several ticks so the pump
			// has parked before we stop.
			_ = sub // intentionally not drained.
			require.Eventually(t, func() bool {
				impl.Publish(pubsub.CreatedEvent, Message{})
				return impl.DropCount() >= 3
			}, time.Second, 5*time.Millisecond)
			drops := impl.DropCount()

			// Drive the structural change. With debounce=1h, Update
			// flushes synchronously and routes through whichever
			// publish path the service chose for structural events.
			tc.mut(&msg)
			require.NoError(t, svc.Update(t.Context(), msg))

			// Must-deliver timeout (40ms) should have expired with
			// no drain. If structural events are routed through
			// must-deliver: MustDeliverDropCount > 0, DropCount
			// unchanged. If routed through lossy Publish:
			// DropCount > 0, MustDeliverDropCount == 0.
			require.Eventually(t, func() bool {
				return impl.MustDeliverDropCount() >= 1
			}, time.Second, 5*time.Millisecond,
				"structural terminal event should publish via must-deliver, not lossy Publish")
			require.Equal(t, drops, impl.DropCount(),
				"structural terminal event must not be silently dropped via lossy Publish")
		})
	}
}

// TestPendingStateDoesNotRetainPayload guards the fix for the
// unbounded heap growth reported in #3162: pending entries live for the
// life of the service, so once a message is fully flushed they must
// hold bookkeeping only, never the message parts.
func TestPendingStateDoesNotRetainPayload(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t, WithDebounce(time.Millisecond))
	s := svc.(*service)

	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{
		Role:  Assistant,
		Parts: []ContentPart{},
	})
	require.NoError(t, err)

	msg.AppendReasoningContent(strings.Repeat("x", 64*1024))
	msg.AddToolCall(ToolCall{ID: "tc-1", Name: "bash", Finished: true})
	msg.AddFinish(FinishReasonEndTurn, "", "")
	require.NoError(t, svc.Update(t.Context(), msg))
	require.NoError(t, svc.Flush(t.Context(), msg.ID))

	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.pending[msg.ID]
	require.NotNil(t, p)
	require.False(t, p.dirty)
	require.Empty(t, p.latest.Parts, "flushed pending state still pins the message parts")
	require.True(t, p.hasFlushed)
	require.Equal(t, []bool{true}, p.baseline.toolCallsFinished)
}

// TestShouldFlushNow_AgainstCompactBaseline pins terminal detection to the
// projection stored in flushBaseline rather than to a full Message snapshot.
//
// The existing structural tests all assert the positive direction, which a
// broken projection would still satisfy: an empty baseline yields a tool-call
// length mismatch, so it flushes for the wrong reason. The load-bearing case
// is the negative one — after a flush, a delta that changes nothing structural
// must stay buffered. That only holds if the baseline carries the prior
// finished flags and reasoning finish time accurately.
func TestShouldFlushNow_AgainstCompactBaseline(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (Service, Message) {
		t.Helper()
		svc, sessionID := newTestService(t, WithDebounce(time.Hour))
		msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
		require.NoError(t, err)

		// Land a baseline: one unfinished tool call, reasoning still open.
		msg.AppendReasoningContent("thinking")
		msg.AddToolCall(ToolCall{ID: "tc1", Name: "view", Finished: false})
		require.NoError(t, svc.Update(t.Context(), msg))
		got, err := svc.Get(t.Context(), msg.ID)
		require.NoError(t, err)
		require.Len(t, got.ToolCalls(), 1)
		require.False(t, got.ToolCalls()[0].Finished)
		return svc, msg
	}

	t.Run("non-structural delta stays buffered", func(t *testing.T) {
		t.Parallel()
		svc, msg := setup(t)

		// Same tool call, same finished flag, reasoning still open. A
		// baseline that lost the projection would see a length or flag
		// mismatch here and flush.
		msg.AppendReasoningContent(" some more")
		require.NoError(t, svc.Update(t.Context(), msg))

		got, err := svc.Get(t.Context(), msg.ID)
		require.NoError(t, err)
		require.Equal(t, "thinking", got.ReasoningContent().Thinking,
			"a non-structural delta must not trigger a sync flush against the compact baseline")
	})

	t.Run("finished flip flushes", func(t *testing.T) {
		t.Parallel()
		svc, msg := setup(t)

		msg.AddToolCall(ToolCall{ID: "tc1", Name: "view", Input: "{}", Finished: true})
		require.NoError(t, svc.Update(t.Context(), msg))

		got, err := svc.Get(t.Context(), msg.ID)
		require.NoError(t, err)
		require.True(t, got.ToolCalls()[0].Finished)
	})

	t.Run("new tool call flushes", func(t *testing.T) {
		t.Parallel()
		svc, msg := setup(t)

		msg.AddToolCall(ToolCall{ID: "tc2", Name: "bash", Finished: false})
		require.NoError(t, svc.Update(t.Context(), msg))

		got, err := svc.Get(t.Context(), msg.ID)
		require.NoError(t, err)
		require.Len(t, got.ToolCalls(), 2)
	})

	t.Run("reasoning end flushes", func(t *testing.T) {
		t.Parallel()
		svc, msg := setup(t)

		msg.FinishThinking()
		require.NoError(t, svc.Update(t.Context(), msg))

		got, err := svc.Get(t.Context(), msg.ID)
		require.NoError(t, err)
		require.NotZero(t, got.ReasoningContent().FinishedAt)
	})
}

// drainEvents reads until no event arrives for a short while.
func drainEvents(sub <-chan pubsub.Event[Message]) []pubsub.Event[Message] {
	var got []pubsub.Event[Message]
	for {
		select {
		case ev := <-sub:
			got = append(got, ev)
		case <-time.After(100 * time.Millisecond):
			return got
		}
	}
}

// TestStalledSubscriberHoldsOnlyNewestUpdate guards against a slow
// subscriber retaining one full snapshot per streamed update.
func TestStalledSubscriberHoldsOnlyNewestUpdate(t *testing.T) {
	t.Parallel()

	svc, sessionID := newTestService(t, WithDebounce(0))
	sub := svc.Subscribe(t.Context())

	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)
	chunk := strings.Repeat("x", 1024)
	const n = 100
	for range n {
		msg.AppendContent(chunk)
		require.NoError(t, svc.Update(t.Context(), msg))
	}

	got := drainEvents(sub)
	require.Len(t, got, 2)
	require.Equal(t, pubsub.CreatedEvent, got[0].Type)
	require.Equal(t, pubsub.UpdatedEvent, got[1].Type)
	require.Len(t, got[1].Payload.Content().Text, n*len(chunk))

	// Terminal updates are must-deliver and still arrive.
	msg.AddFinish(FinishReasonEndTurn, "", "")
	require.NoError(t, svc.Update(t.Context(), msg))
	got = drainEvents(sub)
	require.Len(t, got, 1)
	require.True(t, got[0].MustDeliver)
	require.True(t, got[0].Payload.IsFinished())
	require.Zero(t, svc.(*service).DropCount())
}

// TestUpdatesAfterDeleteBeganStayUnpublished covers updates that reach
// the service once Delete has begun but had nothing pending to mark,
// and once it has returned.
func TestUpdatesAfterDeleteBeganStayUnpublished(t *testing.T) {
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	slow := &slowUpdateQuerier{
		Querier: q,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
	hook := &deleteHookQuerier{Querier: slow}
	svc := NewService(hook, WithDebounce(0))
	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)

	subCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := svc.Subscribe(subCtx)

	msg.AppendContent("racing")
	updateDone := make(chan error, 1)
	hook.beforeDelete = func() error {
		go func() {
			updateDone <- svc.Update(context.Background(), msg)
		}()
		// Let the update either be held or start its write.
		select {
		case <-slow.started:
		case err := <-updateDone:
			updateDone <- err
		}
		return nil
	}
	require.NoError(t, svc.Delete(t.Context(), msg.ID))
	close(slow.release)
	require.NoError(t, <-updateDone)

	msg.AppendContent(" trailing")
	require.NoError(t, svc.Update(t.Context(), msg))

	require.Equal(t, pubsub.DeletedEvent, (<-sub).Type)
	select {
	case event := <-sub:
		require.Failf(t, "update published after delete", "Event type: %s", event.Type)
	case <-time.After(25 * time.Millisecond):
	}
}

// A Delete that fails after another one removed the message keeps the
// other's tombstone, so a late update stays unpublished.
func TestFailedDeleteKeepsConcurrentTombstone(t *testing.T) {
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sess, err := session.NewService(q, conn).Create(t.Context(), "test")
	require.NoError(t, err)

	hook := &deleteHookQuerier{Querier: q}
	svc := NewService(hook, WithDebounce(0))
	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)

	subCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := svc.Subscribe(subCtx)

	// The first delete reaches SQL and waits there, then fails once the
	// second one has removed the row.
	reached, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	hook.beforeDelete = func() error {
		if calls.Add(1) == 1 {
			close(reached)
			<-release
			return errors.New("delete failed")
		}
		return nil
	}
	failed := make(chan error, 1)
	go func() { failed <- svc.Delete(context.Background(), msg.ID) }()
	receive(t, reached)
	require.NoError(t, svc.Delete(t.Context(), msg.ID))
	close(release)
	require.Error(t, receive(t, failed))

	msg.AppendContent("late")
	require.NoError(t, svc.Update(t.Context(), msg))

	require.Equal(t, pubsub.DeletedEvent, receive(t, sub).Type)
	select {
	case event := <-sub:
		require.Failf(t, "update published after delete", "Event type: %s", event.Type)
	case <-time.After(25 * time.Millisecond):
	}
}

// A sync update whose write fails while a delete begins reports the
// failure to its caller.
func TestUpdateReportsWriteErrorDuringDelete(t *testing.T) {
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sess, err := session.NewService(q, conn).Create(t.Context(), "test")
	require.NoError(t, err)

	slow := &slowUpdateQuerier{
		Querier: q,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
	svc := NewService(slow, WithDebounce(0))
	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)

	updateCtx, cancelUpdate := context.WithCancel(t.Context())
	msg.AppendContent("payload")
	updateDone := make(chan error, 1)
	go func() { updateDone <- svc.Update(updateCtx, msg) }()
	receive(t, slow.started)

	deleteDone := make(chan error, 1)
	go func() { deleteDone <- svc.Delete(t.Context(), msg.ID) }()
	waitForDeleting(t, svc.(*service), msg.ID)
	cancelUpdate()

	require.ErrorIs(t, receive(t, updateDone), context.Canceled)
	require.NoError(t, receive(t, deleteDone))
}

// Flush waits for a write in progress even while the message is being
// deleted, so it never returns with that write still running.
func TestFlushWaitsForInFlightWriteDuringDelete(t *testing.T) {
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sess, err := session.NewService(q, conn).Create(t.Context(), "test")
	require.NoError(t, err)

	slow := &slowUpdateQuerier{
		Querier: q,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
	svc := NewService(slow, WithDebounce(10*time.Millisecond))
	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)

	msg.AppendContent("payload")
	require.NoError(t, svc.Update(t.Context(), msg))
	receive(t, slow.started)

	deleteDone := make(chan error, 1)
	go func() { deleteDone <- svc.Delete(t.Context(), msg.ID) }()
	waitForDeleting(t, svc.(*service), msg.ID)

	flushDone := make(chan error, 1)
	go func() { flushDone <- svc.Flush(t.Context(), msg.ID) }()
	select {
	case err := <-flushDone:
		require.Failf(t, "Flush returned while a write was still running", "Flush returned: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	close(slow.release)
	require.NoError(t, receive(t, flushDone))
	require.NoError(t, receive(t, deleteDone))
}

// A delete that fails while another is still running keeps holding
// updates until the other one finishes.
func TestFailedDeleteKeepsHoldingWhileAnotherRuns(t *testing.T) {
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sess, err := session.NewService(q, conn).Create(t.Context(), "test")
	require.NoError(t, err)

	hook := &deleteHookQuerier{Querier: q}
	svc := NewService(hook, WithDebounce(0))
	msg, err := svc.Create(t.Context(), sess.ID, CreateMessageParams{Role: Assistant})
	require.NoError(t, err)

	subCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sub := svc.Subscribe(subCtx)

	// Both deletes reach SQL and wait there. The first then fails while
	// the second is still running.
	reachedFirst, releaseFirst := make(chan struct{}), make(chan struct{})
	reachedSecond, releaseSecond := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	hook.beforeDelete = func() error {
		if calls.Add(1) == 1 {
			close(reachedFirst)
			<-releaseFirst
			return errors.New("delete failed")
		}
		close(reachedSecond)
		<-releaseSecond
		return nil
	}
	failed := make(chan error, 1)
	go func() { failed <- svc.Delete(context.Background(), msg.ID) }()
	receive(t, reachedFirst)
	succeeded := make(chan error, 1)
	go func() { succeeded <- svc.Delete(context.Background(), msg.ID) }()
	receive(t, reachedSecond)
	close(releaseFirst)
	require.Error(t, receive(t, failed))

	msg.AppendContent("late")
	require.NoError(t, svc.Update(t.Context(), msg))
	close(releaseSecond)
	require.NoError(t, receive(t, succeeded))

	require.Equal(t, pubsub.DeletedEvent, receive(t, sub).Type)
	select {
	case event := <-sub:
		require.Failf(t, "unexpected event after delete", "Event type: %s", event.Type)
	case <-time.After(25 * time.Millisecond):
	}
}
