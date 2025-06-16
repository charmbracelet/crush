package pubsub

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const bufferSize = 64

type BackpressureStrategy int

const (
	DropEvents BackpressureStrategy = iota
	BlockPublisher
	RemoveSlowSubscribers
)

type Broker[T any] struct {
	subs                 map[chan Event[T]]struct{}
	mu                   sync.RWMutex
	done                 chan struct{}
	subCount             int
	maxEvents            int
	channelBufferSize    int
	backpressureStrategy BackpressureStrategy
	publishTimeout       time.Duration
	droppedEvents        int64
	slowSubsRemoved      int64
}

func NewBroker[T any]() *Broker[T] {
	return NewBrokerWithOptions[T](bufferSize, 1000)
}

func NewBrokerWithOptions[T any](channelBufferSize, maxEvents int) *Broker[T] {
	return NewBrokerWithBackpressure[T](channelBufferSize, maxEvents, DropEvents, 5*time.Second)
}

func NewBrokerWithBackpressure[T any](channelBufferSize, maxEvents int, strategy BackpressureStrategy, publishTimeout time.Duration) *Broker[T] {
	b := &Broker[T]{
		subs:                 make(map[chan Event[T]]struct{}),
		done:                 make(chan struct{}),
		subCount:             0,
		maxEvents:            maxEvents,
		channelBufferSize:    channelBufferSize,
		backpressureStrategy: strategy,
		publishTimeout:       publishTimeout,
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

func (b *Broker[T]) GetDroppedEventCount() int64 {
	return atomic.LoadInt64(&b.droppedEvents)
}

func (b *Broker[T]) GetSlowSubscribersRemovedCount() int64 {
	return atomic.LoadInt64(&b.slowSubsRemoved)
}

func (b *Broker[T]) Publish(t EventType, payload T) {
	b.mu.RLock()
	select {
	case <-b.done:
		b.mu.RUnlock()
		return
	default:
	}

	subscribers := make([]chan Event[T], 0, len(b.subs))
	for sub := range b.subs {
		subscribers = append(subscribers, sub)
	}
	b.mu.RUnlock()

	event := Event[T]{Type: t, Payload: payload}

	switch b.backpressureStrategy {
	case DropEvents:
		b.publishWithDrop(subscribers, event)
	case BlockPublisher:
		b.publishWithBlock(subscribers, event)
	case RemoveSlowSubscribers:
		b.publishWithRemoval(subscribers, event)
	}
}

func (b *Broker[T]) publishWithDrop(subscribers []chan Event[T], event Event[T]) {
	for _, sub := range subscribers {
		select {
		case sub <- event:
		default:
			atomic.AddInt64(&b.droppedEvents, 1)
		}
	}
}

func (b *Broker[T]) publishWithBlock(subscribers []chan Event[T], event Event[T]) {
	ctx, cancel := context.WithTimeout(context.Background(), b.publishTimeout)
	defer cancel()

	for _, sub := range subscribers {
		select {
		case sub <- event:
		case <-ctx.Done():
			atomic.AddInt64(&b.droppedEvents, 1)
		}
	}
}

func (b *Broker[T]) publishWithRemoval(subscribers []chan Event[T], event Event[T]) {
	slowSubs := make([]chan Event[T], 0)

	for _, sub := range subscribers {
		select {
		case sub <- event:
		default:
			slowSubs = append(slowSubs, sub)
			atomic.AddInt64(&b.droppedEvents, 1)
		}
	}

	if len(slowSubs) > 0 {
		b.mu.Lock()
		for _, slowSub := range slowSubs {
			if _, exists := b.subs[slowSub]; exists {
				delete(b.subs, slowSub)
				close(slowSub)
				b.subCount--
				atomic.AddInt64(&b.slowSubsRemoved, 1)
			}
		}
		b.mu.Unlock()
	}
}
