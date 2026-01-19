package pubsub

import (
	"context"

	"github.com/maniartech/signals"
)

// Broker is a generic pub/sub broker backed by maniartech/signals.
type Broker[T any] struct {
	signal *signals.AsyncSignal[Event[T]]
}

// NewBroker creates a new broker.
func NewBroker[T any]() *Broker[T] {
	return &Broker[T]{
		signal: signals.New[Event[T]](),
	}
}

// NewBrokerWithOptions creates a new broker (options ignored for compatibility).
func NewBrokerWithOptions[T any](_, _ int) *Broker[T] {
	return NewBroker[T]()
}

// Shutdown removes all listeners.
func (b *Broker[T]) Shutdown() {
	b.signal.Reset()
}

// AddListener registers a callback for events.
func (b *Broker[T]) AddListener(key string, fn func(Event[T])) {
	b.signal.AddListener(func(_ context.Context, event Event[T]) {
		fn(event)
	}, key)
}

// RemoveListener removes a listener by key.
func (b *Broker[T]) RemoveListener(key string) {
	b.signal.RemoveListener(key)
}

// Publish emits an event to all listeners without blocking.
func (b *Broker[T]) Publish(ctx context.Context, t EventType, payload T) {
	event := Event[T]{Type: t, Payload: payload}
	go b.signal.Emit(ctx, event)
}

// Len returns the number of listeners.
func (b *Broker[T]) Len() int {
	return b.signal.Len()
}
