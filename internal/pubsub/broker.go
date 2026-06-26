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
	// bufferSize is the per-subscriber channel capacity for any broker
	// created via NewBroker. Publish is non-blocking, so a full buffer
	// drops events (with a warning log); sized to cover a long
	// streaming assistant turn (~one UpdatedEvent per token) even under
	// TUI render stalls.
	bufferSize = 4096

	// defaultMustDeliverTimeout is the per-subscriber upper bound on how
	// long [Broker.PublishMustDeliver] will block waiting for buffer
	// space before giving up on that subscriber.
	defaultMustDeliverTimeout = 50 * time.Millisecond
)

type Broker[T any] struct {
	subs                 map[chan Event[T]]struct{}
	mu                   sync.RWMutex
	done                 chan struct{}
	subCount             int
	channelBufferSize    int
	mustDeliverTimeout   time.Duration
	dropCount            atomic.Uint64
	mustDeliverDropCount atomic.Uint64

	// Name identifies this broker in log messages. Set via
	// NewBrokerWithName; empty for brokers created with NewBroker.
	Name string

	// lastDropLog gates drop warnings to at most one per interval to
	// avoid flooding logs during burst drops.
	lastDropLog atomic.Int64

	// lastDropCount snapshots the drop count at the time of the last
	// log message so we can report the delta (drops since last log)
	// rather than just the cumulative total.
	lastDropCount atomic.Uint64

	// lastMustDeliverLog gates must-deliver timeout errors similarly.
	lastMustDeliverLog atomic.Int64

	// lastMustDeliverDropCount snapshots the must-deliver drop count
	// at the time of the last log message.
	lastMustDeliverDropCount atomic.Uint64
}

func NewBroker[T any]() *Broker[T] {
	return NewBrokerWithOptions[T](bufferSize)
}

// NewBrokerWithName creates a broker with a human-readable name used in
// log messages and debug endpoints. Prefer this over NewBroker for any
// broker that will be registered in app.setupEvents so drop warnings
// identify the source.
func NewBrokerWithName[T any](name string) *Broker[T] {
	b := NewBrokerWithOptions[T](bufferSize)
	b.Name = name
	return b
}

func NewBrokerWithOptions[T any](channelBufferSize int) *Broker[T] {
	return &Broker[T]{
		subs:               make(map[chan Event[T]]struct{}),
		done:               make(chan struct{}),
		channelBufferSize:  channelBufferSize,
		mustDeliverTimeout: defaultMustDeliverTimeout,
	}
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

func (b *Broker[T]) Shutdown() {
	select {
	case <-b.done: // Already closed
		return
	default:
		close(b.done)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.subs {
		delete(b.subs, ch)
		close(ch)
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

	sub := make(chan Event[T], b.channelBufferSize)
	b.subs[sub] = struct{}{}
	b.subCount++

	go func() {
		<-ctx.Done()

		b.mu.Lock()
		defer b.mu.Unlock()

		select {
		case <-b.done:
			return
		default:
		}

		delete(b.subs, sub)
		close(sub)
		b.subCount--
	}()

	return sub
}

func (b *Broker[T]) GetSubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.subCount
}

// BrokerStats holds a point-in-time snapshot of broker health.
type BrokerStats struct {
	Name             string `json:"name"`
	Subscribers      int    `json:"subscribers"`
	BufferCap        int    `json:"buffer_cap"`
	BufferLengths    []int  `json:"buffer_lengths"`
	DropCount        uint64 `json:"drop_count"`
	MustDeliverDrops uint64 `json:"must_deliver_drops"`
}

// Stats returns a point-in-time snapshot of broker health including
// per-subscriber buffer occupancy and cumulative drop counts.
func (b *Broker[T]) Stats() BrokerStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	lengths := make([]int, 0, len(b.subs))
	for sub := range b.subs {
		lengths = append(lengths, len(sub))
	}
	return BrokerStats{
		Name:             b.Name,
		Subscribers:      len(b.subs),
		BufferCap:        b.channelBufferSize,
		BufferLengths:    lengths,
		DropCount:        b.dropCount.Load(),
		MustDeliverDrops: b.mustDeliverDropCount.Load(),
	}
}

// DropCount returns the cumulative number of events dropped by
// [Broker.Publish] because a subscriber's channel was full.
func (b *Broker[T]) DropCount() uint64 {
	return b.dropCount.Load()
}

// MustDeliverDropCount returns the cumulative number of events dropped
// by [Broker.PublishMustDeliver] after the per-subscriber timeout
// expired.
func (b *Broker[T]) MustDeliverDropCount() uint64 {
	return b.mustDeliverDropCount.Load()
}

// logDrop emits a rate-limited warning when events are dropped. At most
// one warning per second to prevent log flooding during burst drops
// (e.g., when fan-in goroutines drain full source buffers faster than
// the downstream consumer can process). Reports the number of drops
// since the last log message so you can see the rate, not just the
// cumulative total.
func (b *Broker[T]) logDrop(t EventType) {
	now := time.Now().UnixNano()
	last := b.lastDropLog.Load()
	if now-last < int64(time.Second) {
		return
	}
	if !b.lastDropLog.CompareAndSwap(last, now) {
		return // Another goroutine won the race; skip.
	}
	total := b.dropCount.Load()
	delta := total - b.lastDropCount.Load()
	b.lastDropCount.Store(total)
	name := b.Name
	if name == "" {
		name = "unknown"
	}
	slog.Warn("Pubsub buffer full; dropping events",
		"broker", name,
		"type", t,
		"dropsLastSecond", delta,
		"totalDrops", total,
	)
}

// logMustDeliverDrop emits a rate-limited error when must-deliver events
// time out. Same cadence as logDrop to prevent log flooding during
// sustained backpressure. Reports drops since last log.
func (b *Broker[T]) logMustDeliverDrop(t EventType, timeout time.Duration) {
	now := time.Now().UnixNano()
	last := b.lastMustDeliverLog.Load()
	if now-last < int64(time.Second) {
		return
	}
	if !b.lastMustDeliverLog.CompareAndSwap(last, now) {
		return
	}
	total := b.mustDeliverDropCount.Load()
	delta := total - b.lastMustDeliverDropCount.Load()
	b.lastMustDeliverDropCount.Store(total)
	name := b.Name
	if name == "" {
		name = "unknown"
	}
	slog.Error("PublishMustDeliver timed out delivering events",
		"broker", name,
		"type", t,
		"timeout", timeout,
		"dropsLastSecond", delta,
		"totalDrops", total,
	)
}

// Publish delivers an event to every active subscriber.
//
// Delivery is non-blocking and lossy: if a subscriber's channel is full
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

	for sub := range b.subs {
		select {
		case sub <- event:
		default:
			// Channel is full, subscriber is slow — skip this event.
			// Lossy by design; counted and logged so saturation is
			// observable.
			b.dropCount.Add(1)
			b.logDrop(t)
		}
	}
}

// PublishMustDeliver delivers an event with bounded-blocking semantics.
// For each subscriber it first attempts a non-blocking send, then falls
// back to a blocking send bounded by a per-subscriber timeout (default
// [defaultMustDeliverTimeout]). On timeout the event is dropped for
// that subscriber, [Broker.MustDeliverDropCount] is incremented, and an
// error is logged. The publisher never blocks indefinitely.
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

	event := Event[T]{Type: t, Payload: payload}
	timeout := b.mustDeliverTimeout

	for sub := range b.subs {
		// Fast path: non-blocking send.
		select {
		case sub <- event:
			continue
		default:
		}

		// Slow path: bounded blocking send.
		timer := time.NewTimer(timeout)
		select {
		case sub <- event:
			timer.Stop()
		case <-timer.C:
			b.mustDeliverDropCount.Add(1)
			b.logMustDeliverDrop(t, timeout)
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}
