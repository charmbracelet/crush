package pubsub

import (
	"context"
	"sync"
)

const bufferSize = 64

type Broker[T any] struct {
	subs      map[chan Event[T]]struct{}
	mu        sync.RWMutex
	done      chan struct{}
	subCount  int
	maxEvents int
}

func NewBroker[T any]() *Broker[T] {
	return NewBrokerWithOptions[T](bufferSize, 1000)
}

func NewBrokerWithOptions[T any](channelBufferSize, maxEvents int) *Broker[T] {
	b := &Broker[T]{
		subs:      make(map[chan Event[T]]struct{}),
		done:      make(chan struct{}),
		maxEvents: maxEvents,
	}
	return b
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

	sub := make(chan Event[T], bufferSize)
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
			// Channel is full, subscriber is slow - skip this event
			// This prevents blocking the publisher
		}
	}
}
