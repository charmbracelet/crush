package pubsub

import (
	"context"
	"sync"
)

// Broker is a generic pub/sub broker using callbacks.
type Broker[T any] struct {
	listeners []func(Event[T])
	mu        sync.RWMutex
}

// NewBroker creates a new broker.
func NewBroker[T any]() *Broker[T] {
	return &Broker[T]{
		listeners: make([]func(Event[T]), 0),
	}
}

// AddListener registers a callback for events.
func (b *Broker[T]) AddListener(fn func(Event[T])) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners = append(b.listeners, fn)
}

// Publish emits an event to all listeners without blocking.
func (b *Broker[T]) Publish(ctx context.Context, t EventType, payload T) {
	b.mu.RLock()
	listeners := make([]func(Event[T]), len(b.listeners))
	copy(listeners, b.listeners)
	b.mu.RUnlock()

	event := Event[T]{Type: t, Payload: payload}
	for _, fn := range listeners {
		go fn(event)
	}
}
