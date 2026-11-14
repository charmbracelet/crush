// Package pubsub provides a lightweight in-process broker for fan-out
// event delivery between services and the UI.
//
// Delivery semantics:
//
//   - [Broker.Publish] is best-effort and lossy under contention. If a
//     subscriber's channel is full, the event is dropped for that
//     subscriber, a warning is logged, and a counter is incremented.
//     This is the right choice for high-frequency intermediate updates
//     (e.g. streaming token deltas) where only the latest state
//     matters.
//
//   - [Broker.PublishMustDeliver] is bounded-blocking. For each
//     subscriber it first tries a non-blocking send, then falls back to
//     a per-subscriber blocking send with a hard timeout. On timeout the
//     event is dropped for that subscriber, an error is logged, and the
//     must-deliver drop counter is incremented. The publisher never
//     blocks indefinitely. This is the right choice for terminal events
//     (finish, tool result, error, cancel) that must not be silently
//     coalesced away.
//
//   - A broker created with [NewCoalescingBroker] keeps at most one
//     pending event per key for each subscriber; a newer event replaces
//     it in place and takes its delivery position. Events with the same
//     key arrive in order, but a replacement can arrive before unrelated
//     events published after the event it replaced.
//
// Drop counters ([Broker.DropCount], [Broker.MustDeliverDropCount]) are
// exposed so callers can surface saturation in telemetry.
package pubsub

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// bufferSize is the per-subscriber queue capacity for any broker
	// created via NewBroker. Publish is non-blocking, so a full buffer
	// drops events (with a warning log); sized to cover a long
	// streaming assistant turn (~one UpdatedEvent per token) even under
	// TUI render stalls.
	bufferSize = 4096

	// defaultMustDeliverTimeout is the per-subscriber upper bound on how
	// long [Broker.PublishMustDeliver] will block waiting for buffer
	// space before giving up on that subscriber.
	defaultMustDeliverTimeout = 50 * time.Millisecond

	// shutdownDrainTimeout bounds how long a subscription keeps delivering
	// its remaining events after [Broker.Shutdown].
	shutdownDrainTimeout = 100 * time.Millisecond
)

type Broker[T any] struct {
	subs                 map[*subscription[T]]struct{}
	coalesceKey          func(Event[T]) string
	mu                   sync.RWMutex
	done                 chan struct{}
	subCount             int
	channelBufferSize    int
	mustDeliverTimeout   time.Duration
	dropCount            atomic.Uint64
	mustDeliverDropCount atomic.Uint64
}

func NewBroker[T any]() *Broker[T] {
	return NewBrokerWithOptions[T](bufferSize)
}

func NewBrokerWithOptions[T any](channelBufferSize int) *Broker[T] {
	return &Broker[T]{
		subs:               make(map[*subscription[T]]struct{}),
		done:               make(chan struct{}),
		channelBufferSize:  channelBufferSize,
		mustDeliverTimeout: defaultMustDeliverTimeout,
	}
}

// NewCoalescingBroker returns a broker whose subscribers keep at most one
// pending [Broker.Publish] event per non-empty key returned by key. Events
// for which key returns "" are delivered in order without coalescing.
func NewCoalescingBroker[T any](key func(Event[T]) string) *Broker[T] {
	b := NewBroker[T]()
	b.coalesceKey = key
	return b
}

// SetMustDeliverTimeout overrides the per-subscriber timeout used by
// [Broker.PublishMustDeliver]. A zero or negative value resets to the
// default. Intended primarily for tests.
func (b *Broker[T]) SetMustDeliverTimeout(d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if d <= 0 {
		b.mustDeliverTimeout = defaultMustDeliverTimeout
		return
	}
	b.mustDeliverTimeout = d
}

// Shutdown stops the broker. Each subscriber's channel closes once its
// pending events are delivered, or once its reader stops taking them for
// [shutdownDrainTimeout].
func (b *Broker[T]) Shutdown() {
	select {
	case <-b.done: // Already closed
		return
	default:
		close(b.done)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for sub := range b.subs {
		delete(b.subs, sub)
		close(sub.queue)
	}

	b.subCount = 0
}

func (b *Broker[T]) Subscribe(ctx context.Context) <-chan Event[T] {
	b.mu.Lock()
	defer b.mu.Unlock()

	select {
	case <-b.done:
		ch := make(chan Event[T])
		close(ch)
		return ch
	default:
	}

	sub := &subscription[T]{
		// Unbuffered so pending events stay in the queue where they can
		// still be coalesced.
		out:   make(chan Event[T]),
		queue: make(chan *item[T], b.channelBufferSize),
		open:  make(map[string]*item[T]),
	}
	b.subs[sub] = struct{}{}
	b.subCount++
	go sub.pump(ctx, b.done)

	go func() {
		select {
		case <-ctx.Done():
		case <-b.done:
			return
		}

		b.mu.Lock()
		defer b.mu.Unlock()

		select {
		case <-b.done:
			return
		default:
		}

		delete(b.subs, sub)
		close(sub.queue)
		b.subCount--
	}()

	return sub.out
}

func (b *Broker[T]) GetSubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.subCount
}

// DropCount returns the cumulative number of events dropped by
// [Broker.Publish] because a subscriber's queue was full.
func (b *Broker[T]) DropCount() uint64 {
	return b.dropCount.Load()
}

// MustDeliverDropCount returns the cumulative number of events dropped
// by [Broker.PublishMustDeliver] after the per-subscriber timeout
// expired.
func (b *Broker[T]) MustDeliverDropCount() uint64 {
	return b.mustDeliverDropCount.Load()
}

// Publish delivers an event to every active subscriber.
//
// Delivery is non-blocking and lossy: if a subscriber's queue is full
// the event is dropped for that subscriber, a warning is logged, and
// [Broker.DropCount] is incremented. Use [Broker.PublishMustDeliver]
// for events that must not be silently dropped.
func (b *Broker[T]) Publish(t EventType, payload T) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	select {
	case <-b.done:
		return
	default:
	}

	event := Event[T]{Type: t, Payload: payload}
	key := b.key(event)

	for sub := range b.subs {
		if !sub.publish(event, key) {
			// Queue is full, subscriber is slow — skip this event.
			// Lossy by design; counted and logged so saturation is
			// observable.
			b.dropCount.Add(1)
			slog.Warn("Pubsub buffer full; dropping event", "type", t)
		}
	}
}

// PublishMustDeliver delivers an event with bounded-blocking semantics.
// For each subscriber it first attempts a non-blocking send, then falls
// back to a blocking send bounded by a per-subscriber timeout (default
// [defaultMustDeliverTimeout]). On timeout the event is dropped for
// that subscriber, [Broker.MustDeliverDropCount] is incremented, and an
// error is logged. The publisher never blocks indefinitely. On a
// coalescing broker the event takes over a pending event with the same
// key instead, which cannot time out, and is never replaced itself.
//
// Use this for terminal events that must reach subscribers (finish,
// tool result, error, cancel). Callers must still tolerate rare drops
// after timeout — recovery is the subscriber's responsibility (e.g. a
// re-fetch on the next session-visible event).
func (b *Broker[T]) PublishMustDeliver(ctx context.Context, t EventType, payload T) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	select {
	case <-b.done:
		return
	default:
	}

	event := Event[T]{Type: t, Payload: payload, MustDeliver: true}
	key := b.key(event)
	timeout := b.mustDeliverTimeout

	for sub := range b.subs {
		if sub.publishMustDeliver(ctx, event, key, timeout) {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		b.mustDeliverDropCount.Add(1)
		slog.Error("PublishMustDeliver timed out delivering event",
			"type", t, "timeout", timeout)
	}
}

func (b *Broker[T]) key(event Event[T]) string {
	if b.coalesceKey == nil {
		return ""
	}
	return b.coalesceKey(event)
}

// item is one queued event. While it is pending under a non-empty key,
// later events with that key overwrite it in place.
type item[T any] struct {
	event Event[T]
	key   string
}

// subscription is one subscriber: publishers fill a bounded queue and a
// pump goroutine feeds out.
type subscription[T any] struct {
	out   chan Event[T]
	queue chan *item[T]

	mu   sync.Mutex
	open map[string]*item[T] // Pending replaceable items by key.
}

// publish queues event without blocking, or overwrites a pending item
// with the same key. It reports false if the queue was full.
func (s *subscription[T]) publish(event Event[T], key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if it, ok := s.open[key]; ok {
		it.event = event
		return true
	}
	it := &item[T]{event: event, key: key}
	select {
	case s.queue <- it:
		if key != "" {
			s.open[key] = it
		}
		return true
	default:
		return false
	}
}

// publishMustDeliver takes over a pending item with the same key, or
// queues a new item that is never replaced, waiting up to timeout for
// space. It reports false if the event was not delivered.
func (s *subscription[T]) publishMustDeliver(ctx context.Context, event Event[T], key string, timeout time.Duration) bool {
	it := &item[T]{event: event, key: key}
	s.mu.Lock()
	if open, ok := s.open[key]; ok {
		open.event = event
		delete(s.open, key)
		s.mu.Unlock()
		return true
	}
	// The fast path stays under the lock so a concurrent plain update for
	// key cannot queue ahead of this event.
	select {
	case s.queue <- it:
		s.mu.Unlock()
		return true
	default:
	}
	s.mu.Unlock()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case s.queue <- it:
		return true
	case <-timer.C:
	case <-ctx.Done():
	}
	return false
}

// pump hands queued items to out until the queue is closed or ctx ends.
// After done closes it keeps going only while the reader keeps up.
func (s *subscription[T]) pump(ctx context.Context, done <-chan struct{}) {
	defer close(s.out)
	var deadline <-chan time.Time
	for it := range s.queue {
		s.mu.Lock()
		event := it.event
		it.event = Event[T]{}
		if s.open[it.key] == it {
			delete(s.open, it.key)
		}
		s.mu.Unlock()

		if deadline == nil {
			select {
			case s.out <- event:
				continue
			case <-ctx.Done():
				return
			case <-done:
				timer := time.NewTimer(shutdownDrainTimeout)
				defer timer.Stop()
				deadline = timer.C
			}
		}
		select {
		case s.out <- event:
		case <-ctx.Done():
			return
		case <-deadline:
			return
		}
	}
}
