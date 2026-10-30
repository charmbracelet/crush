package pubsub

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type snap struct {
	ID   string
	V    int
	Data []byte
}

func snapKey(ev Event[snap]) string {
	if ev.Type != UpdatedEvent {
		return ""
	}
	return ev.Payload.ID
}

func newTestCoalescingBroker(t *testing.T) *Broker[snap] {
	t.Helper()
	b := NewCoalescingBroker(snapKey)
	t.Cleanup(b.Shutdown)
	return b
}

// subscribeStalled subscribes without reading, then parks the pump on a
// blocker event so every later publish stays in the queue.
func subscribeStalled(t *testing.T, b *Broker[snap]) (*subscription[snap], <-chan Event[snap]) {
	t.Helper()
	ch := b.Subscribe(t.Context())
	b.mu.RLock()
	var sub *subscription[snap]
	for s := range b.subs {
		if s.out == ch {
			sub = s
		}
	}
	b.mu.RUnlock()
	require.NotNil(t, sub)

	b.Publish(CreatedEvent, snap{ID: "blocker"})
	require.Eventually(t, func() bool { return len(sub.queue) == 0 }, time.Second, time.Millisecond)
	return sub, ch
}

// drain reads until no event arrives for a short while.
func drain(t *testing.T, ch <-chan Event[snap]) []Event[snap] {
	t.Helper()
	var got []Event[snap]
	for {
		select {
		case ev := <-ch:
			got = append(got, ev)
		case <-time.After(50 * time.Millisecond):
			return got
		}
	}
}

func describe(evs []Event[snap]) []string {
	out := make([]string, 0, len(evs))
	for _, ev := range evs {
		out = append(out, fmt.Sprintf("%s:%s:%d:%t", ev.Type, ev.Payload.ID, ev.Payload.V, ev.MustDeliver))
	}
	return out
}

func TestCoalescingBroker_StalledSubscriberKeepsOnlyNewestUpdate(t *testing.T) {
	t.Parallel()

	b := newTestCoalescingBroker(t)
	sub, ch := subscribeStalled(t, b)

	const n = 2000
	for i := 1; i <= n; i++ {
		b.Publish(UpdatedEvent, snap{ID: "a", V: i, Data: make([]byte, 64<<10)})
	}

	// 2000 snapshots of 64 KiB would be 125 MiB; only one is pending.
	require.Len(t, sub.queue, 1)
	sub.mu.Lock()
	require.Len(t, sub.open, 1)
	require.Equal(t, n, sub.open["a"].event.Payload.V)
	sub.mu.Unlock()
	require.Zero(t, b.DropCount())

	require.Equal(t, []string{
		"created:blocker:0:false",
		fmt.Sprintf("updated:a:%d:false", n),
	}, describe(drain(t, ch)))

	sub.mu.Lock()
	require.Empty(t, sub.open, "delivered slots must not stay pinned")
	sub.mu.Unlock()
}

func TestCoalescingBroker_Ordering(t *testing.T) {
	t.Parallel()

	b := newTestCoalescingBroker(t)
	_, ch := subscribeStalled(t, b)

	b.Publish(CreatedEvent, snap{ID: "a"})
	b.Publish(UpdatedEvent, snap{ID: "a", V: 1})
	b.Publish(CreatedEvent, snap{ID: "b"})
	b.Publish(UpdatedEvent, snap{ID: "a", V: 2})
	b.Publish(UpdatedEvent, snap{ID: "b", V: 1})
	b.Publish(UpdatedEvent, snap{ID: "a", V: 3})
	b.Publish(DeletedEvent, snap{ID: "b"})

	// Newer snapshots take the slot of the pending one; Created stays
	// first and nothing for b follows its Deleted.
	require.Equal(t, []string{
		"created:blocker:0:false",
		"created:a:0:false",
		"updated:a:3:false",
		"created:b:0:false",
		"updated:b:1:false",
		"deleted:b:0:false",
	}, describe(drain(t, ch)))
}

func TestCoalescingBroker_MustDeliverNeverCoalesced(t *testing.T) {
	t.Parallel()

	b := newTestCoalescingBroker(t)
	_, ch := subscribeStalled(t, b)

	b.Publish(UpdatedEvent, snap{ID: "a", V: 1})
	b.PublishMustDeliver(t.Context(), UpdatedEvent, snap{ID: "a", V: 2})
	b.Publish(UpdatedEvent, snap{ID: "a", V: 3})
	b.PublishMustDeliver(t.Context(), UpdatedEvent, snap{ID: "a", V: 4})
	b.PublishMustDeliver(t.Context(), UpdatedEvent, snap{ID: "a", V: 5})
	b.Publish(UpdatedEvent, snap{ID: "a", V: 6})
	b.Publish(UpdatedEvent, snap{ID: "a", V: 7})

	// Older plain updates are superseded by the must-deliver events after
	// them; must-deliver events all arrive, in order, and are flagged.
	require.Equal(t, []string{
		"created:blocker:0:false",
		"updated:a:2:true",
		"updated:a:4:true",
		"updated:a:5:true",
		"updated:a:7:false",
	}, describe(drain(t, ch)))
	require.Zero(t, b.DropCount())
	require.Zero(t, b.MustDeliverDropCount())
}

func TestCoalescingBroker_MustDeliverTimeoutAndTakeover(t *testing.T) {
	t.Parallel()

	b := NewCoalescingBroker(snapKey)
	b.channelBufferSize = 2
	b.SetMustDeliverTimeout(10 * time.Millisecond)
	t.Cleanup(b.Shutdown)
	_, ch := subscribeStalled(t, b)

	b.Publish(UpdatedEvent, snap{ID: "a", V: 1})
	b.Publish(CreatedEvent, snap{ID: "x"})

	// The queue is full. Taking over the pending update needs no space.
	b.PublishMustDeliver(t.Context(), UpdatedEvent, snap{ID: "a", V: 2})
	require.Zero(t, b.MustDeliverDropCount())

	// With nothing to take over, a must-deliver event times out.
	b.PublishMustDeliver(t.Context(), UpdatedEvent, snap{ID: "b", V: 1})
	require.Equal(t, uint64(1), b.MustDeliverDropCount())

	// The takeover removed the pending item from coalescing, so a plain
	// update for a has no slot to merge into and is dropped.
	b.Publish(UpdatedEvent, snap{ID: "a", V: 3})
	require.Equal(t, uint64(1), b.DropCount())

	require.Equal(t, []string{
		"created:blocker:0:false",
		"updated:a:2:true",
		"created:x:0:false",
	}, describe(drain(t, ch)))
}

func TestCoalescingBroker_PlainUpdateAfterQueuedMustDeliver(t *testing.T) {
	t.Parallel()

	b := newTestCoalescingBroker(t)
	sub, ch := subscribeStalled(t, b)

	// No pending update, so v2 is queued on its own and never replaceable.
	b.PublishMustDeliver(t.Context(), UpdatedEvent, snap{ID: "a", V: 2})
	sub.mu.Lock()
	require.Empty(t, sub.open)
	sub.mu.Unlock()

	// A newer plain update must not be lost behind it.
	b.Publish(UpdatedEvent, snap{ID: "a", V: 3})
	require.Equal(t, []string{
		"created:blocker:0:false",
		"updated:a:2:true",
		"updated:a:3:false",
	}, describe(drain(t, ch)))
}

func TestCoalescingBroker_MustDeliverTakesOverPendingInPlace(t *testing.T) {
	t.Parallel()

	b := newTestCoalescingBroker(t)
	sub, ch := subscribeStalled(t, b)

	b.Publish(UpdatedEvent, snap{ID: "a", V: 1})
	b.Publish(CreatedEvent, snap{ID: "x"})
	b.PublishMustDeliver(t.Context(), UpdatedEvent, snap{ID: "a", V: 2})

	// v2 reuses v1's queue position instead of adding an item.
	require.Len(t, sub.queue, 2)
	sub.mu.Lock()
	require.Empty(t, sub.open)
	sub.mu.Unlock()

	require.Equal(t, []string{
		"created:blocker:0:false",
		"updated:a:2:true",
		"created:x:0:false",
	}, describe(drain(t, ch)))
}

func TestBroker_ShutdownWithStalledReaderStopsPump(t *testing.T) {
	t.Parallel()

	for _, coalesce := range []bool{false, true} {
		t.Run(fmt.Sprintf("coalesce=%t", coalesce), func(t *testing.T) {
			t.Parallel()

			b := NewBroker[snap]()
			if coalesce {
				b = NewCoalescingBroker(snapKey)
			}
			// The context outlives the test so only Shutdown can stop
			// the pump.
			ch := b.Subscribe(context.Background())
			for i := range 10 {
				b.Publish(UpdatedEvent, snap{ID: "a", V: i})
				b.Publish(CreatedEvent, snap{ID: "x", V: i})
			}

			b.Shutdown()

			// The pump closes out when it exits. A live pump would still
			// be blocked handing over its first event, so the first
			// receive tells the two apart without feeding the pump.
			time.Sleep(5 * shutdownDrainTimeout)
			select {
			case ev, ok := <-ch:
				require.False(t, ok, "pump still running, handed over %v", ev)
			case <-time.After(time.Second):
				t.Fatal("pump neither exited nor handed over an event")
			}
		})
	}
}

func TestBroker_ShutdownDeliversPendingToActiveReader(t *testing.T) {
	t.Parallel()

	b := NewBroker[snap]()
	ch := b.Subscribe(context.Background())
	const n = 100
	for i := range n {
		b.Publish(UpdatedEvent, snap{ID: "a", V: i})
	}
	b.Shutdown()

	var got int
	for range ch {
		got++
	}
	require.Equal(t, n, got)
}

func TestBroker_PlainDeliversEveryEventInOrder(t *testing.T) {
	t.Parallel()

	b := NewBroker[snap]()
	t.Cleanup(b.Shutdown)
	sub, ch := subscribeStalled(t, b)

	const n = 1000
	var want []string
	for i := 1; i <= n; i++ {
		b.Publish(UpdatedEvent, snap{ID: "a", V: i})
		want = append(want, fmt.Sprintf("updated:a:%d:false", i))
		if i%100 == 0 {
			b.PublishMustDeliver(t.Context(), UpdatedEvent, snap{ID: "a", V: i})
			want = append(want, fmt.Sprintf("updated:a:%d:true", i))
		}
	}
	require.Len(t, sub.queue, n+n/100)
	sub.mu.Lock()
	require.Empty(t, sub.open)
	sub.mu.Unlock()
	require.Zero(t, b.DropCount())

	require.Equal(t, append([]string{"created:blocker:0:false"}, want...), describe(drain(t, ch)))
}

func TestCoalescingBroker_UnrelatedEventsUnaffected(t *testing.T) {
	t.Parallel()

	b := newTestCoalescingBroker(t)
	_, ch := subscribeStalled(t, b)

	var want []string
	for i := range 50 {
		id := fmt.Sprintf("m%d", i)
		b.Publish(UpdatedEvent, snap{ID: id, V: 1})
		want = append(want, fmt.Sprintf("updated:%s:1:false", id))
	}
	for range 3 {
		b.Publish(CreatedEvent, snap{ID: "a"})
		b.Publish(DeletedEvent, snap{ID: "a"})
		want = append(want, "created:a:0:false", "deleted:a:0:false")
	}
	b.Publish("custom", snap{ID: "a", V: 9})
	b.Publish("custom", snap{ID: "a", V: 10})
	want = append(want, "custom:a:9:false", "custom:a:10:false")

	require.Equal(t, append([]string{"created:blocker:0:false"}, want...), describe(drain(t, ch)))
}

func TestCoalescingBroker_ConcurrentSlowReaderSeesMonotonicSnapshots(t *testing.T) {
	t.Parallel()

	b := newTestCoalescingBroker(t)
	ch := b.Subscribe(t.Context())

	const ids, versions = 8, 500
	done := make(chan struct{})
	var got []Event[snap]
	go func() {
		defer close(done)
		for ev := range ch {
			got = append(got, ev)
			if ev.Type == "end" {
				return
			}
			if rand.IntN(10) == 0 {
				time.Sleep(time.Millisecond)
			}
		}
	}()

	var wg sync.WaitGroup
	for i := range ids {
		id := fmt.Sprintf("m%d", i)
		wg.Go(func() {
			b.Publish(CreatedEvent, snap{ID: id})
			for v := 1; v <= versions; v++ {
				if v%100 == 0 {
					b.PublishMustDeliver(context.Background(), UpdatedEvent, snap{ID: id, V: v})
				} else {
					b.Publish(UpdatedEvent, snap{ID: id, V: v})
				}
			}
			b.Publish(DeletedEvent, snap{ID: id})
		})
	}
	wg.Wait()
	b.PublishMustDeliver(t.Context(), "end", snap{})
	<-done

	require.Zero(t, b.DropCount())
	require.Zero(t, b.MustDeliverDropCount())
	last := map[string]int{}
	created := map[string]bool{}
	deleted := map[string]bool{}
	for _, ev := range got {
		id := ev.Payload.ID
		switch ev.Type {
		case CreatedEvent:
			created[id] = true
		case UpdatedEvent:
			require.True(t, created[id], "update before create for %s", id)
			require.False(t, deleted[id], "update after delete for %s", id)
			require.Greater(t, ev.Payload.V, last[id], "older snapshot after newer for %s", id)
			last[id] = ev.Payload.V
		case DeletedEvent:
			deleted[id] = true
		}
	}
	for i := range ids {
		id := fmt.Sprintf("m%d", i)
		require.Equal(t, versions, last[id])
		require.True(t, deleted[id])
	}
}

// After Shutdown a slow reader gets one drain window in total, not one per
// remaining event.
func TestBroker_ShutdownDrainIsOneDeadline(t *testing.T) {
	t.Parallel()
	b := NewBroker[int]()
	ch := b.Subscribe(t.Context())
	for i := range 50 {
		b.Publish(UpdatedEvent, i)
	}
	time.Sleep(10 * time.Millisecond)
	b.Shutdown()

	start := time.Now()
	for range ch {
		time.Sleep(20 * time.Millisecond)
	}
	require.Less(t, time.Since(start), 5*shutdownDrainTimeout)
}
