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

// AddListener registers a callback for events.
func (b *Broker[T]) AddListener(key string, fn func(Event[T])) {
	b.signal.AddListener(func(_ context.Context, event Event[T]) {
		fn(event)
	}, key)
}

// Publish emits an event to all listeners without blocking.
func (b *Broker[T]) Publish(ctx context.Context, t EventType, payload T) {
	event := Event[T]{Type: t, Payload: payload}
	go b.signal.Emit(ctx, event)
}
