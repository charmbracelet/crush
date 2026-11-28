package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
)

const (
	// eventRingSize is the most workspace events kept for resuming
	// client event streams.
	eventRingSize = 1000
	// eventRingMaxAge is how long a workspace event is kept for resuming
	// client event streams.
	eventRingMaxAge = 5 * time.Minute
	// eventRingMaxBytes caps the encoded size of the workspace events
	// kept for resuming client event streams. Whichever of the count,
	// age, and size limits keeps fewer events wins.
	eventRingMaxBytes = 16 << 20
	// eventRingEntryOverhead approximates the memory an entry takes
	// beyond its encoded bytes.
	eventRingEntryOverhead = 64
	// eventRingPruneInterval is how often the sequencer drops events
	// older than eventRingMaxAge, so an idle workspace does not keep
	// them.
	eventRingPruneInterval = 30 * time.Second
	// eventStreamQueueSize is the per-stream queue capacity, matching the
	// pubsub broker queues that client streams used before.
	eventStreamQueueSize = 4096
)

// EncodedEvent is the wire form of a workspace event, produced once when
// the event is sequenced and shared by every stream and replay.
type EncodedEvent struct {
	// Data is the event's SSE data payload.
	Data []byte
	// SessionID is the session the event is scoped to, for interactive
	// prompts that only the session's viewers may get, or empty for
	// events that go to every client.
	SessionID string
}

// EventEncoder encodes a workspace event for client event streams. It
// reports false for events that have no wire form; those are not
// sequenced at all.
type EventEncoder func(msg any) (EncodedEvent, bool)

// defaultEventEncoder encodes events as plain JSON, for backends that no
// server configured.
func defaultEventEncoder(msg any) (EncodedEvent, bool) {
	data, err := json.Marshal(msg)
	if err != nil {
		return EncodedEvent{}, false
	}
	return EncodedEvent{Data: data}, true
}

// SetEventEncoder sets how workspace events are encoded for client event
// streams. It must be called before any stream subscribes.
func (b *Backend) SetEventEncoder(enc EventEncoder) {
	b.eventEncoder = enc
}

// SequencedEvent is an encoded workspace event with the id the workspace
// assigned it. IDs are "<epoch>-<seq>": seq counts the workspace's events
// from 1, and epoch is random per workspace instance, so an id issued by
// another instance (a restarted server, a re-created workspace) is never
// mistaken for one of this instance's.
type SequencedEvent struct {
	ID  string
	Seq uint64
	EncodedEvent
}

// EventStream is a client's subscription to a workspace's sequenced
// events, resumed after a previously seen event id if one was given.
type EventStream struct {
	// Replay holds the buffered events after the resume id, oldest
	// first. Events then continues with the next one, without gaps or
	// duplicates.
	Replay []SequencedEvent
	// Resync is set when the resume id cannot be resumed from (unknown,
	// out of the buffer, from another epoch, or before a gap). The client
	// must re-read the state it shows. ResyncID is then the id of the
	// latest event, which the client is positioned at.
	Resync   proto.ResyncReason
	ResyncID string
	// Events delivers live events. It is closed when the workspace's
	// event pipeline stops.
	Events <-chan SequencedEvent
	// Overflow is closed when events were dropped for this stream, either
	// because its queue was full or because the workspace sequence itself
	// has a gap. The stream should end; a resume from the last event it
	// got replays what it missed, or yields a resync.
	Overflow <-chan struct{}
}

// SubscribeEventStream subscribes to a workspace's sequenced events: its
// app events and the session changes of its clients. A non-empty
// lastEventID resumes after that event: buffered events after it are
// returned in Replay, or Resync says why that is impossible. The
// subscription is registered before this returns, so an event published
// afterwards is delivered.
func (b *Backend) SubscribeEventStream(ctx context.Context, workspaceID, lastEventID string) (EventStream, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return EventStream{}, err
	}
	enc := b.eventEncoder
	if enc == nil {
		enc = defaultEventEncoder
	}
	return ws.eventSequencer(enc).subscribe(ctx, lastEventID), nil
}

// eventSequencer returns the workspace's event sequencer, starting it on
// first use. It lives as long as the workspace's event broker: it is not
// bound to the workspace run context, which is cancelled at the start of
// teardown, before the events that the cancelled runs publish.
func (w *Workspace) eventSequencer(enc EventEncoder) *eventSequencer {
	w.sequencerOnce.Do(func() {
		w.sequencer = newEventSequencer(enc)
		if w.App == nil {
			// Synthetic test workspaces have no event broker: their
			// streams end at once, as they would on a stopped workspace.
			w.sequencer.stop()
			return
		}
		w.sequencer.start(context.Background(),
			w.EventsWithOverflow,
			clientSessionSource(w.clientSessionBroker()))
	})
	return w.sequencer
}

// upstreamFunc subscribes to a source of workspace events; see
// [app.App.EventsWithOverflow].
type upstreamFunc func(context.Context) (<-chan pubsub.Event[tea.Msg], <-chan struct{})

// clientSessionSource adapts the broker of client session changes to a
// source of the sequencer. Each change is passed on as the
// pubsub.Event[proto.ClientSession] it was published as, which is the
// form the SSE encoder knows.
func clientSessionSource(broker *pubsub.Broker[proto.ClientSession]) upstreamFunc {
	return func(ctx context.Context) (<-chan pubsub.Event[tea.Msg], <-chan struct{}) {
		in, overflow := broker.SubscribeWithOverflow(ctx)
		out := make(chan pubsub.Event[tea.Msg])
		go func() {
			defer close(out)
			for ev := range in {
				select {
				case out <- pubsub.Event[tea.Msg]{Type: ev.Type, Payload: ev, MustDeliver: ev.MustDeliver}:
				case <-ctx.Done():
					return
				}
			}
		}()
		return out, overflow
	}
}

// upstream is one subscription of the sequencer to a source. A nil
// subscribe makes a source that never delivers.
type upstream struct {
	subscribe upstreamFunc
	events    <-chan pubsub.Event[tea.Msg]
	overflow  <-chan struct{}
	cancel    context.CancelFunc
}

func (u *upstream) open(ctx context.Context) {
	if u.subscribe == nil {
		return
	}
	subCtx, cancel := context.WithCancel(ctx)
	u.events, u.overflow = u.subscribe(subCtx)
	u.cancel = cancel
}

// reopen subscribes anew before dropping the old subscription, so nothing
// published from now on is missed.
func (u *upstream) reopen(ctx context.Context) {
	old := u.cancel
	u.open(ctx)
	if old != nil {
		old()
	}
}

func (u *upstream) close() {
	if u.cancel != nil {
		u.cancel()
	}
	u.events, u.overflow = nil, nil
}

// eventSequencer is the single subscriber to a workspace's event sources
// that client streams read from: the app event broker and the broker of
// client session changes. It is the one place event ids are assigned. It
// encodes and numbers every event in the order it receives them, keeps
// the recent ones in a ring for resuming streams, and fans them out to
// each stream's own bounded queue. Fan-out never blocks: a stream whose
// queue is full is dropped, so one stalled client cannot hold back the
// others or the sequencer's own broker subscriptions.
//
// The ring stores what the sequencer received, which is the canonical
// sequence: the app broker may coalesce pending message updates for the
// sequencer if it lags, exactly as it did for each client stream before.
// Stream queues do not coalesce, since replacing a queued event would
// deliver ids out of order.
type eventSequencer struct {
	epoch         string
	encode        EventEncoder
	now           func() time.Time
	maxAge        time.Duration
	pruneInterval time.Duration

	mu sync.Mutex
	// lastSeq is the seq of the latest event, 0 before the first one.
	lastSeq uint64
	// floor is the lowest seq a stream can resume from. Events were lost
	// before it, or it is a live-only event too large to buffer, so
	// resuming before it would replay a gapped sequence.
	floor uint64
	ring  eventRing
	subs  map[*eventSub]struct{}
	// subList is an immutable snapshot of subs for fan-out, replaced
	// whenever subs changes.
	subList []*eventSub
	// finished is closed once the sequencer stopped.
	finished chan struct{}
	stopped  bool
}

func newEventSequencer(enc EventEncoder) *eventSequencer {
	return &eventSequencer{
		epoch:         newEpoch(),
		encode:        enc,
		now:           time.Now,
		maxAge:        eventRingMaxAge,
		pruneInterval: eventRingPruneInterval,
		ring:          newEventRing(eventRingSize, eventRingMaxBytes),
		subs:          make(map[*eventSub]struct{}),
		finished:      make(chan struct{}),
	}
}

// start subscribes to the sources synchronously, so an event published
// after start returns is sequenced, then sequences their events until ctx
// ends or primary closes. secondary may be nil.
func (s *eventSequencer) start(ctx context.Context, primary, secondary upstreamFunc) {
	p := &upstream{subscribe: primary}
	sec := &upstream{subscribe: secondary}
	p.open(ctx)
	sec.open(ctx)
	go s.run(ctx, p, sec)
}

func (s *eventSequencer) run(ctx context.Context, p, sec *upstream) {
	defer s.stop()
	defer sec.close()
	defer p.close()
	ticker := time.NewTicker(s.pruneInterval)
	defer ticker.Stop()
	for {
		select {
		case ev, ok := <-p.events:
			if !ok {
				return
			}
			s.publish(ev)
		case ev, ok := <-sec.events:
			if !ok {
				sec.close()
				continue
			}
			s.publish(ev)
		case <-p.overflow:
			// The sequencer itself fell behind a source and the broker
			// dropped an event for it. Resubscribe, then mark the gap.
			slog.Warn("Workspace event sequencer fell behind; resetting resume buffer")
			p.reopen(ctx)
			s.gap()
		case <-sec.overflow:
			slog.Warn("Workspace event sequencer fell behind client sessions; resetting resume buffer")
			sec.reopen(ctx)
			s.gap()
		case <-ticker.C:
			s.prune()
		case <-ctx.Done():
			return
		}
	}
}

// publish encodes and numbers ev, records it, and fans it out to the
// streams that were subscribed when it was recorded. Events without a
// wire form are dropped here and take no id.
func (s *eventSequencer) publish(ev pubsub.Event[tea.Msg]) {
	enc, ok := s.encode(ev.Payload)
	if !ok {
		return
	}
	now := s.now()
	s.mu.Lock()
	s.lastSeq++
	seq := s.lastSeq
	if entry := (eventEntry{seq: seq, at: now, enc: enc}); s.ring.fits(entry) {
		s.ring.push(entry)
		s.ring.expire(now.Add(-s.maxAge))
	} else {
		// Too big to buffer: the event is live-only. Nothing before it
		// can be replayed without skipping it, so drop the older events
		// and resync any cursor before it. A cursor at it resumes.
		slog.Warn("Workspace event too large to buffer for resuming streams",
			"seq", seq, "bytes", len(enc.Data))
		s.ring.reset()
		s.floor = seq
	}
	// A stream that subscribes after this point gets the event from the
	// ring, so it must not get it again live; one subscribed before gets
	// it only live. Taking the snapshot under the lock that records the
	// event is what makes replay and live meet without gaps or
	// duplicates.
	subs := s.subList
	s.mu.Unlock()

	out := SequencedEvent{ID: formatEventID(s.epoch, seq), Seq: seq, EncodedEvent: enc}
	for _, sub := range subs {
		if sub.send(out) {
			continue
		}
		slog.Warn("Event stream fell behind; dropping it",
			"seq", seq, "must_deliver", ev.MustDeliver)
		s.remove(sub)
		sub.overflowed()
	}
}

// prune drops the recorded events that are too old.
func (s *eventSequencer) prune() {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ring.expire(now.Add(-s.maxAge))
}

// gap records that upstream events were lost after lastSeq. Streams
// subscribed now missed them, so they overflow, and no stream may resume
// from before the gap.
func (s *eventSequencer) gap() {
	s.mu.Lock()
	s.floor = s.lastSeq + 1
	s.ring.reset()
	subs := s.subList
	clear(s.subs)
	s.subList = nil
	s.mu.Unlock()
	for _, sub := range subs {
		sub.cancel()
		sub.overflowed()
	}
}

// stop ends every stream. It runs on the sequencer goroutine, the only
// sender on stream channels, so closing them is safe.
func (s *eventSequencer) stop() {
	s.mu.Lock()
	s.stopped = true
	subs := s.subList
	clear(s.subs)
	s.subList = nil
	close(s.finished)
	s.mu.Unlock()
	for _, sub := range subs {
		sub.cancel()
		close(sub.ch)
	}
}

// subscribe registers a stream, resuming after lastEventID if it is not
// empty.
func (s *eventSequencer) subscribe(ctx context.Context, lastEventID string) EventStream {
	sub := newEventSub()
	stream := EventStream{Events: sub.ch, Overflow: sub.overflow}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		close(sub.ch)
		return stream
	}
	if lastEventID != "" {
		stream.Replay, stream.Resync = s.replayLocked(lastEventID)
		if stream.Resync != "" {
			stream.ResyncID = formatEventID(s.epoch, s.lastSeq)
		}
	}
	s.subs[sub] = struct{}{}
	s.rebuildLocked()
	s.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
			s.remove(sub)
			sub.cancel()
		case <-sub.done:
		case <-s.finished:
		}
	}()
	return stream
}

// replayLocked returns the events after lastEventID, or why they cannot
// be replayed.
func (s *eventSequencer) replayLocked(lastEventID string) ([]SequencedEvent, proto.ResyncReason) {
	epoch, seq, ok := parseEventID(lastEventID)
	if !ok {
		return nil, proto.ResyncReasonInvalidID
	}
	if epoch != s.epoch {
		return nil, proto.ResyncReasonEpochMismatch
	}
	if seq > s.lastSeq {
		return nil, proto.ResyncReasonUnknownID
	}
	if seq < s.floor {
		return nil, proto.ResyncReasonGap
	}
	if seq == s.lastSeq {
		return nil, ""
	}
	s.ring.expire(s.now().Add(-s.maxAge))
	oldest, ok := s.ring.oldest()
	if !ok || seq+1 < oldest {
		return nil, proto.ResyncReasonExpired
	}
	return s.ring.after(seq, s.epoch), ""
}

func (s *eventSequencer) remove(sub *eventSub) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[sub]; !ok {
		return
	}
	delete(s.subs, sub)
	s.rebuildLocked()
}

func (s *eventSequencer) rebuildLocked() {
	list := make([]*eventSub, 0, len(s.subs))
	for sub := range s.subs {
		list = append(list, sub)
	}
	s.subList = list
}

// eventSub is one client stream's bounded queue.
type eventSub struct {
	ch       chan SequencedEvent
	overflow chan struct{}
	// done is closed when the stream is gone, so nothing is queued for
	// a stream nobody reads.
	done         chan struct{}
	overflowOnce sync.Once
	doneOnce     sync.Once
}

func newEventSub() *eventSub {
	return &eventSub{
		ch:       make(chan SequencedEvent, eventStreamQueueSize),
		overflow: make(chan struct{}),
		done:     make(chan struct{}),
	}
}

func (sub *eventSub) overflowed() { sub.overflowOnce.Do(func() { close(sub.overflow) }) }
func (sub *eventSub) cancel()     { sub.doneOnce.Do(func() { close(sub.done) }) }

// send queues ev without ever blocking. It reports false if the queue is
// full, must-deliver event or not: a stream that cannot take an event is
// broken, and its client resumes from the last event it got.
func (sub *eventSub) send(ev SequencedEvent) bool {
	select {
	case sub.ch <- ev:
		return true
	case <-sub.done:
		return true
	default:
		return false
	}
}

// eventEntry is one recorded event.
type eventEntry struct {
	seq uint64
	at  time.Time
	enc EncodedEvent
}

func (e eventEntry) size() int {
	return len(e.enc.Data) + len(e.enc.SessionID) + eventRingEntryOverhead
}

// eventRing is a FIFO of recorded events, oldest first, bounded by count
// and by total size.
type eventRing struct {
	buf      []eventEntry
	head     int
	n        int
	bytes    int
	maxBytes int
}

func newEventRing(size, maxBytes int) eventRing {
	return eventRing{buf: make([]eventEntry, size), maxBytes: maxBytes}
}

// fits reports whether e fits in the ring's size budget at all.
func (r *eventRing) fits(e eventEntry) bool { return e.size() <= r.maxBytes }

// push records e, dropping the oldest events to make room. e must fit.
func (r *eventRing) push(e eventEntry) {
	size := e.size()
	for r.n > 0 && (r.n == len(r.buf) || r.bytes+size > r.maxBytes) {
		r.popFront()
	}
	r.buf[(r.head+r.n)%len(r.buf)] = e
	r.n++
	r.bytes += size
}

func (r *eventRing) popFront() {
	r.bytes -= r.buf[r.head].size()
	r.buf[r.head] = eventEntry{} // Release the bytes.
	r.head = (r.head + 1) % len(r.buf)
	r.n--
}

// expire drops the events recorded before cutoff.
func (r *eventRing) expire(cutoff time.Time) {
	for r.n > 0 && r.buf[r.head].at.Before(cutoff) {
		r.popFront()
	}
}

func (r *eventRing) reset() {
	for r.n > 0 {
		r.popFront()
	}
	r.head = 0
}

func (r *eventRing) oldest() (uint64, bool) {
	if r.n == 0 {
		return 0, false
	}
	return r.buf[r.head].seq, true
}

// after returns the events with a seq above seq.
func (r *eventRing) after(seq uint64, epoch string) []SequencedEvent {
	var out []SequencedEvent
	for i := range r.n {
		e := r.buf[(r.head+i)%len(r.buf)]
		if e.seq > seq {
			out = append(out, SequencedEvent{ID: formatEventID(epoch, e.seq), Seq: e.seq, EncodedEvent: e.enc})
		}
	}
	return out
}

// newEpoch returns a random epoch for a sequencer's event ids.
func newEpoch() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func formatEventID(epoch string, seq uint64) string {
	return epoch + "-" + strconv.FormatUint(seq, 10)
}

// parseEventID splits an event id into its epoch and seq.
func parseEventID(id string) (string, uint64, bool) {
	epoch, n, ok := strings.Cut(strings.TrimSpace(id), "-")
	if !ok || epoch == "" {
		return "", 0, false
	}
	seq, err := strconv.ParseUint(n, 10, 64)
	if err != nil {
		return "", 0, false
	}
	return epoch, seq, true
}
