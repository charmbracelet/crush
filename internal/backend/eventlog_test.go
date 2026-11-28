package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// startEventSequencer starts a JSON-encoding sequencer fed by upstream.
func startEventSequencer(ctx context.Context, upstream upstreamFunc) *eventSequencer {
	s := newEventSequencer(defaultEventEncoder)
	s.start(ctx, upstream, nil)
	return s
}

// msgOf decodes the int a test published from an encoded event.
func msgOf(t *testing.T, ev SequencedEvent) int {
	t.Helper()
	var n int
	require.NoError(t, json.Unmarshal(ev.Data, &n))
	return n
}

// newTestSequencer starts a sequencer fed by a fresh broker.
func newTestSequencer(t *testing.T) (*eventSequencer, *pubsub.Broker[tea.Msg]) {
	t.Helper()
	broker := pubsub.NewBroker[tea.Msg]()
	t.Cleanup(broker.Shutdown)
	return startEventSequencer(t.Context(), broker.SubscribeWithOverflow), broker
}

// recvN reads n live events from stream.
func recvN(t *testing.T, stream EventStream, n int) []SequencedEvent {
	t.Helper()
	out := make([]SequencedEvent, 0, n)
	timeout := time.After(5 * time.Second)
	for len(out) < n {
		select {
		case ev, ok := <-stream.Events:
			require.True(t, ok, "stream closed after %d of %d events", len(out), n)
			out = append(out, ev)
		case <-stream.Overflow:
			require.FailNow(t, "stream overflowed")
		case <-timeout:
			require.FailNow(t, "timed out", "got %d of %d events", len(out), n)
		}
	}
	return out
}

// waitSeq waits until s has sequenced n events.
func waitSeq(t *testing.T, s *eventSequencer, n uint64) {
	t.Helper()
	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.lastSeq >= n
	}, 5*time.Second, time.Millisecond)
}

func TestEventSequencer_IDsMonotonic(t *testing.T) {
	t.Parallel()
	s, broker := newTestSequencer(t)
	stream := s.subscribe(t.Context(), "")
	require.Empty(t, stream.Resync)
	require.Empty(t, stream.Replay)

	for i := range 50 {
		broker.Publish(pubsub.UpdatedEvent, i)
	}
	evs := recvN(t, stream, 50)
	for i, ev := range evs {
		require.Equal(t, uint64(i+1), ev.Seq)
		require.Equal(t, i, msgOf(t, ev))
		require.Equal(t, fmt.Sprintf("%s-%d", s.epoch, i+1), ev.ID)
	}
}

func TestEventSequencer_EpochsDiffer(t *testing.T) {
	t.Parallel()
	a, _ := newTestSequencer(t)
	b, _ := newTestSequencer(t)
	require.NotEqual(t, a.epoch, b.epoch)
	require.NotContains(t, a.epoch, "-")
}

func TestEventSequencer_ResumeReplaysMissed(t *testing.T) {
	t.Parallel()
	s, broker := newTestSequencer(t)
	for i := range 10 {
		broker.Publish(pubsub.UpdatedEvent, i)
	}
	waitSeq(t, s, 10)

	stream := s.subscribe(t.Context(), formatEventID(s.epoch, 4))
	require.Empty(t, stream.Resync)
	require.Len(t, stream.Replay, 6)
	for i, ev := range stream.Replay {
		require.Equal(t, uint64(i+5), ev.Seq)
		require.Equal(t, i+4, msgOf(t, ev))
	}

	// Live continues right after the replay.
	broker.Publish(pubsub.UpdatedEvent, 10)
	live := recvN(t, stream, 1)
	require.Equal(t, uint64(11), live[0].Seq)
}

func TestEventSequencer_ResumeAtHeadReplaysNothing(t *testing.T) {
	t.Parallel()
	s, broker := newTestSequencer(t)
	for i := range 3 {
		broker.Publish(pubsub.UpdatedEvent, i)
	}
	waitSeq(t, s, 3)
	stream := s.subscribe(t.Context(), formatEventID(s.epoch, 3))
	require.Empty(t, stream.Resync)
	require.Empty(t, stream.Replay)
}

func TestEventSequencer_Resync(t *testing.T) {
	t.Parallel()
	s, broker := newTestSequencer(t)
	for i := range 3 {
		broker.Publish(pubsub.UpdatedEvent, i)
	}
	waitSeq(t, s, 3)

	for name, tc := range map[string]struct {
		id     string
		reason proto.ResyncReason
	}{
		"garbage":       {"nonsense", proto.ResyncReasonInvalidID},
		"plain number":  {"2", proto.ResyncReasonInvalidID},
		"bad seq":       {s.epoch + "-x", proto.ResyncReasonInvalidID},
		"other epoch":   {"deadbeef0000-2", proto.ResyncReasonEpochMismatch},
		"from the void": {formatEventID(s.epoch, 99), proto.ResyncReasonUnknownID},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stream := s.subscribe(t.Context(), tc.id)
			require.Equal(t, tc.reason, stream.Resync)
			require.Equal(t, formatEventID(s.epoch, 3), stream.ResyncID)
			require.Empty(t, stream.Replay)
		})
	}
}

func TestEventSequencer_ResyncWhenOutOfRing(t *testing.T) {
	t.Parallel()
	s, broker := newTestSequencer(t)
	for i := range eventRingSize + 5 {
		broker.Publish(pubsub.UpdatedEvent, i)
	}
	waitSeq(t, s, eventRingSize+5)

	// Event 5 is the newest that fell out; resuming after it needs it
	// gone no further than the oldest buffered one, seq 6.
	stream := s.subscribe(t.Context(), formatEventID(s.epoch, 5))
	require.Empty(t, stream.Resync)
	require.Len(t, stream.Replay, eventRingSize)

	stream = s.subscribe(t.Context(), formatEventID(s.epoch, 4))
	require.Equal(t, proto.ResyncReasonExpired, stream.Resync)
	require.Empty(t, stream.Replay)
}

func TestEventSequencer_ResyncWhenExpired(t *testing.T) {
	t.Parallel()
	broker := pubsub.NewBroker[tea.Msg]()
	t.Cleanup(broker.Shutdown)
	var mu sync.Mutex
	now := time.Now()
	s := newEventSequencer(defaultEventEncoder)
	s.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	s.start(t.Context(), broker.SubscribeWithOverflow, nil)

	broker.Publish(pubsub.UpdatedEvent, 1)
	broker.Publish(pubsub.UpdatedEvent, 2)
	waitSeq(t, s, 2)
	mu.Lock()
	now = now.Add(eventRingMaxAge + time.Second)
	mu.Unlock()

	stream := s.subscribe(t.Context(), formatEventID(s.epoch, 1))
	require.Equal(t, proto.ResyncReasonExpired, stream.Resync)

	// A client that saw the latest event loses nothing, however old.
	stream = s.subscribe(t.Context(), formatEventID(s.epoch, 2))
	require.Empty(t, stream.Resync)
}

// TestEventSequencer_UpstreamGap covers the sequencer itself falling
// behind its broker: the streams then missed events, so they overflow, and
// resuming from before the gap resyncs while resuming after it works.
func TestEventSequencer_UpstreamGap(t *testing.T) {
	t.Parallel()
	type sub struct {
		events   chan pubsub.Event[tea.Msg]
		overflow chan struct{}
	}
	subs := make(chan sub, 4)
	upstream := func(context.Context) (<-chan pubsub.Event[tea.Msg], <-chan struct{}) {
		u := sub{make(chan pubsub.Event[tea.Msg]), make(chan struct{})}
		subs <- u
		return u.events, u.overflow
	}
	s := newEventSequencer(defaultEventEncoder)
	s.start(t.Context(), upstream, nil)
	first := <-subs

	stream := s.subscribe(t.Context(), "")
	first.events <- pubsub.Event[tea.Msg]{Payload: "a"}
	first.events <- pubsub.Event[tea.Msg]{Payload: "b"}
	recvN(t, stream, 2)

	close(first.overflow)
	second := <-subs
	select {
	case <-stream.Overflow:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "a stream that missed events was kept open")
	}
	second.events <- pubsub.Event[tea.Msg]{Payload: "c"}
	waitSeq(t, s, 3)

	resumed := s.subscribe(t.Context(), formatEventID(s.epoch, 2))
	require.Equal(t, proto.ResyncReasonGap, resumed.Resync)
	resumed = s.subscribe(t.Context(), formatEventID(s.epoch, 1))
	require.Equal(t, proto.ResyncReasonGap, resumed.Resync)
	resumed = s.subscribe(t.Context(), formatEventID(s.epoch, 3))
	require.Empty(t, resumed.Resync)
	resumed = s.subscribe(t.Context(), formatEventID(s.epoch, 0))
	require.Equal(t, proto.ResyncReasonGap, resumed.Resync)
}

// TestEventSequencer_FullStreamOverflowsOnMustDeliver fills one stream's
// queue exactly, then publishes a must-deliver event: fan-out does not
// wait for the stalled stream, which overflows at once, while a healthy
// stream on the same workspace keeps receiving.
func TestEventSequencer_FullStreamOverflowsOnMustDeliver(t *testing.T) {
	t.Parallel()
	s, broker := newTestSequencer(t)
	stalled := s.subscribe(t.Context(), "")
	healthy := s.subscribe(t.Context(), "")

	// Fill the stalled queue in paced batches the healthy stream drains,
	// so neither the sequencer's broker subscription nor the healthy
	// queue backs up.
	const batch = 256
	seen := 0
	for seen < eventStreamQueueSize {
		for range batch {
			broker.Publish(pubsub.UpdatedEvent, seen)
		}
		recvN(t, healthy, batch)
		seen += batch
	}
	select {
	case <-stalled.Overflow:
		require.FailNow(t, "a full queue must not overflow before an event is dropped")
	default:
	}
	require.Len(t, stalled.Events, eventStreamQueueSize)

	start := time.Now()
	broker.PublishMustDeliver(t.Context(), pubsub.UpdatedEvent, -1)
	select {
	case <-stalled.Overflow:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the stalled stream was not overflowed")
	}
	got := recvN(t, healthy, 1)[0]
	require.Equal(t, -1, msgOf(t, got))
	require.Less(t, time.Since(start), time.Second, "fan-out waited on the stalled stream")

	broker.Publish(pubsub.UpdatedEvent, 7)
	require.Equal(t, 7, msgOf(t, recvN(t, healthy, 1)[0]), "the healthy stream must keep receiving")

	// A client resuming from a recent event gets the rest replayed:
	// 4096 fillers, the must-deliver event, and 7 make 4098 events.
	const total = eventStreamQueueSize + 2
	resumed := s.subscribe(t.Context(), formatEventID(s.epoch, total-10))
	require.Empty(t, resumed.Resync)
	require.Len(t, resumed.Replay, 10)
}

func TestEventSequencer_PrunesWhenIdle(t *testing.T) {
	t.Parallel()
	broker := pubsub.NewBroker[tea.Msg]()
	t.Cleanup(broker.Shutdown)
	var mu sync.Mutex
	now := time.Now()
	s := newEventSequencer(defaultEventEncoder)
	s.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	s.pruneInterval = 5 * time.Millisecond
	s.start(t.Context(), broker.SubscribeWithOverflow, nil)

	broker.Publish(pubsub.UpdatedEvent, 1)
	waitSeq(t, s, 1)
	mu.Lock()
	now = now.Add(eventRingMaxAge + time.Second)
	mu.Unlock()

	// Nothing is published or resumed: only the ticker can prune.
	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.ring.n == 0 && s.ring.bytes == 0
	}, 5*time.Second, time.Millisecond)
}

func TestEventRing_ByteCap(t *testing.T) {
	t.Parallel()
	r := newEventRing(10, 1000)
	big := make([]byte, 400-eventRingEntryOverhead)
	for i := range 5 {
		r.push(eventEntry{seq: uint64(i + 1), enc: EncodedEvent{Data: big}})
		require.LessOrEqual(t, r.bytes, 1000)
	}
	// Two 400-byte entries fit in 1000 bytes.
	require.Equal(t, 2, r.n)
	oldest, _ := r.oldest()
	require.Equal(t, uint64(4), oldest)

	require.True(t, r.fits(eventEntry{enc: EncodedEvent{Data: big}}))
	require.False(t, r.fits(eventEntry{enc: EncodedEvent{Data: make([]byte, 1000)}}))

	r.reset()
	require.Zero(t, r.bytes)
}

func TestEventSequencer_ByteCapResync(t *testing.T) {
	t.Parallel()
	s, broker := newTestSequencer(t)
	// Each event encodes to about 1 MiB, so the 16 MiB budget keeps
	// fewer than 16 of them.
	big := strings.Repeat("x", 1<<20)
	for range 20 {
		broker.Publish(pubsub.UpdatedEvent, big)
	}
	waitSeq(t, s, 20)
	s.mu.Lock()
	require.LessOrEqual(t, s.ring.bytes, eventRingMaxBytes)
	kept := s.ring.n
	s.mu.Unlock()
	require.Less(t, kept, 16)

	resumed := s.subscribe(t.Context(), formatEventID(s.epoch, 1))
	require.Equal(t, proto.ResyncReasonExpired, resumed.Resync)
	resumed = s.subscribe(t.Context(), formatEventID(s.epoch, uint64(20-kept)))
	require.Empty(t, resumed.Resync)
	require.Len(t, resumed.Replay, kept)
}

// TestEventSequencer_OversizedEventIsLiveOnly publishes an event larger
// than the whole buffer budget: live streams get it, the buffer does not
// keep it or anything before it, and a cursor before it resyncs rather
// than silently skipping it.
func TestEventSequencer_OversizedEventIsLiveOnly(t *testing.T) {
	t.Parallel()
	s, broker := newTestSequencer(t)
	live := s.subscribe(t.Context(), "")
	broker.Publish(pubsub.UpdatedEvent, "small-1")
	broker.Publish(pubsub.UpdatedEvent, "small-2")
	broker.Publish(pubsub.UpdatedEvent, strings.Repeat("x", eventRingMaxBytes))
	broker.Publish(pubsub.UpdatedEvent, "small-4")
	evs := recvN(t, live, 4)
	require.Greater(t, len(evs[2].Data), eventRingMaxBytes, "live streams get the oversized event")

	s.mu.Lock()
	require.Equal(t, 1, s.ring.n, "only the event after the oversized one is buffered")
	require.LessOrEqual(t, s.ring.bytes, eventRingMaxBytes)
	s.mu.Unlock()

	for _, cursor := range []uint64{0, 1, 2} {
		resumed := s.subscribe(t.Context(), formatEventID(s.epoch, cursor))
		require.Equal(t, proto.ResyncReasonGap, resumed.Resync, "cursor %d", cursor)
		require.Empty(t, resumed.Replay)
	}
	resumed := s.subscribe(t.Context(), formatEventID(s.epoch, 3))
	require.Empty(t, resumed.Resync)
	require.Len(t, resumed.Replay, 1)
	require.Equal(t, uint64(4), resumed.Replay[0].Seq)
}

// TestSubscribeEventStream_ClientSessions checks that client session
// changes are sequenced with the app events, in one id space, and are
// replayed on resume.
func TestSubscribeEventStream_ClientSessions(t *testing.T) {
	t.Parallel()
	b, ws, _ := newPromptsTestBackend(t)
	stream, err := b.SubscribeEventStream(t.Context(), ws.ID, "")
	require.NoError(t, err)

	ws.SendEvent(pubsub.Event[proto.ConfigChanged]{Payload: proto.ConfigChanged{WorkspaceID: ws.ID}})
	first := recvN(t, stream, 1)[0]
	publishClientSession(ws, "c1", "S1")
	ev := recvN(t, stream, 1)[0]
	require.Equal(t, first.Seq+1, ev.Seq)
	var cs pubsub.Event[proto.ClientSession]
	require.NoError(t, json.Unmarshal(ev.Data, &cs))
	require.Equal(t, proto.ClientSession{ClientID: "c1", SessionID: "S1"}, cs.Payload)

	resumed, err := b.SubscribeEventStream(t.Context(), ws.ID, first.ID)
	require.NoError(t, err)
	require.Empty(t, resumed.Resync)
	require.Len(t, resumed.Replay, 1)
	require.Equal(t, ev.ID, resumed.Replay[0].ID)
}

func TestEventSequencer_StopClosesStreams(t *testing.T) {
	t.Parallel()
	broker := pubsub.NewBroker[tea.Msg]()
	s := startEventSequencer(t.Context(), broker.SubscribeWithOverflow)
	stream := s.subscribe(t.Context(), "")
	broker.Shutdown()
	select {
	case _, ok := <-stream.Events:
		require.False(t, ok)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "stream stayed open after the broker stopped")
	}
	_, ok := <-s.subscribe(t.Context(), "").Events
	require.False(t, ok, "a stream subscribed after stop must be closed")
}

func TestEventSequencer_CancelledStreamUnsubscribes(t *testing.T) {
	t.Parallel()
	s, _ := newTestSequencer(t)
	ctx, cancel := context.WithCancel(t.Context())
	s.subscribe(ctx, "")
	cancel()
	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.subs) == 0
	}, 5*time.Second, time.Millisecond)
}

// TestEventSequencer_ResumeBoundaryRace resumes streams while events are
// being published, and checks that each sees replay then live as one
// contiguous run with no gap or duplicate. Run it with -race.
func TestEventSequencer_ResumeBoundaryRace(t *testing.T) {
	t.Parallel()
	s, broker := newTestSequencer(t)
	const total = 3000
	broker.Publish(pubsub.UpdatedEvent, 0)
	waitSeq(t, s, 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 1; i < total; i++ {
			broker.Publish(pubsub.UpdatedEvent, i)
			if i%50 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
	}()

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			s.mu.Lock()
			from := s.lastSeq
			s.mu.Unlock()
			if from > 1 {
				from-- // Resume a little behind, to get a replay.
			}
			stream := s.subscribe(ctx, formatEventID(s.epoch, from))
			if stream.Resync != "" {
				t.Errorf("unexpected resync %q from %d", stream.Resync, from)
				return
			}
			next := from + 1
			for _, ev := range stream.Replay {
				if ev.Seq != next {
					t.Errorf("replay: got seq %d, want %d", ev.Seq, next)
					return
				}
				next++
			}
			for next <= total {
				select {
				case ev := <-stream.Events:
					if ev.Seq != next {
						t.Errorf("live: got seq %d, want %d", ev.Seq, next)
						return
					}
					next++
				case <-stream.Overflow:
					t.Error("stream overflowed")
					return
				case <-time.After(10 * time.Second):
					t.Errorf("timed out at seq %d", next)
					return
				}
			}
		})
		time.Sleep(time.Millisecond)
	}
	wg.Wait()
	<-done
}

func TestParseEventID(t *testing.T) {
	t.Parallel()
	epoch, seq, ok := parseEventID(formatEventID("abc123", 42))
	require.True(t, ok)
	require.Equal(t, "abc123", epoch)
	require.Equal(t, uint64(42), seq)
	for _, bad := range []string{"", "-1", "abc", "abc-", "abc--1", "abc-1.5", strings.Repeat("9", 30)} {
		_, _, ok := parseEventID(bad)
		require.False(t, ok, bad)
	}
}

// TestSubscribeEventStream_Workspace drives the sequencer through a
// workspace, as the SSE handler does.
func TestSubscribeEventStream_Workspace(t *testing.T) {
	t.Parallel()
	b, ws, _ := newPromptsTestBackend(t)
	stream, err := b.SubscribeEventStream(t.Context(), ws.ID, "")
	require.NoError(t, err)
	ws.SendEvent(pubsub.Event[proto.ConfigChanged]{Payload: proto.ConfigChanged{WorkspaceID: ws.ID}})
	ev := recvN(t, stream, 1)[0]
	require.Equal(t, uint64(1), ev.Seq)
	var got pubsub.Event[proto.ConfigChanged]
	require.NoError(t, json.Unmarshal(ev.Data, &got))
	require.Equal(t, ws.ID, got.Payload.WorkspaceID)

	_, err = b.SubscribeEventStream(t.Context(), "missing", "")
	require.ErrorIs(t, err, ErrWorkspaceNotFound)
}
