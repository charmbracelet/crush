package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBrokerBackpressureStrategies(t *testing.T) {
	t.Parallel()

	t.Run("DropEvents strategy", func(t *testing.T) {
		t.Parallel()
		broker := NewBrokerWithBackpressure[string](1, 100, DropEvents, time.Second)
		defer broker.Shutdown()

		ctx := context.Background()
		sub := broker.Subscribe(ctx)

		// Fill the buffer (size 1)
		broker.Publish(CreatedEvent, "event1")
		// This should be dropped since buffer is full
		broker.Publish(CreatedEvent, "event2")

		// Verify first event received
		select {
		case event := <-sub:
			assert.Equal(t, "event1", event.Payload)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected to receive event1")
		}

		// Verify dropped event count
		assert.Equal(t, int64(1), broker.GetDroppedEventCount())
	})

	t.Run("BlockPublisher strategy", func(t *testing.T) {
		t.Parallel()
		broker := NewBrokerWithBackpressure[string](1, 100, BlockPublisher, 50*time.Millisecond)
		defer broker.Shutdown()

		ctx := context.Background()
		sub := broker.Subscribe(ctx)

		// Fill the buffer (size 1)
		broker.Publish(CreatedEvent, "event1")

		start := time.Now()
		// This should timeout since buffer is full
		broker.Publish(CreatedEvent, "event2")
		duration := time.Since(start)

		// Should have waited for timeout
		assert.True(t, duration >= 50*time.Millisecond)
		assert.Equal(t, int64(1), broker.GetDroppedEventCount())

		// Consume first event
		select {
		case event := <-sub:
			assert.Equal(t, "event1", event.Payload)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected to receive event1")
		}
	})

	t.Run("RemoveSlowSubscribers strategy", func(t *testing.T) {
		t.Parallel()
		broker := NewBrokerWithBackpressure[string](1, 100, RemoveSlowSubscribers, time.Second)
		defer broker.Shutdown()

		ctx := context.Background()
		sub := broker.Subscribe(ctx)

		// Fill the buffer (size 1)
		broker.Publish(CreatedEvent, "event1")
		// This should cause subscriber removal since buffer is full
		broker.Publish(CreatedEvent, "event2")

		// Give some time for the removal to happen
		time.Sleep(10 * time.Millisecond)

		// Verify subscriber was removed
		assert.Equal(t, 0, broker.GetSubscriberCount())
		assert.Equal(t, int64(1), broker.GetSlowSubscribersRemovedCount())

		// Channel should be closed
		select {
		case event, ok := <-sub:
			if ok {
				assert.Equal(t, "event1", event.Payload)
				// Try to read again to check if channel is closed
				_, ok = <-sub
				assert.False(t, ok, "Channel should be closed")
			} else {
				// Channel was closed immediately, which is also valid
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected channel to be closed or receive event")
		}
	})
}

func TestBrokerMetrics(t *testing.T) {
	t.Parallel()

	broker := NewBrokerWithBackpressure[string](1, 100, DropEvents, time.Second)
	defer broker.Shutdown()

	// Initial metrics should be zero
	assert.Equal(t, int64(0), broker.GetDroppedEventCount())
	assert.Equal(t, int64(0), broker.GetSlowSubscribersRemovedCount())
	assert.Equal(t, 0, broker.GetSubscriberCount())

	ctx := context.Background()
	sub := broker.Subscribe(ctx)
	assert.Equal(t, 1, broker.GetSubscriberCount())

	// Fill buffer (size 1) and cause drops
	broker.Publish(CreatedEvent, "event1")
	broker.Publish(CreatedEvent, "event2") // This should be dropped

	assert.Equal(t, int64(1), broker.GetDroppedEventCount())

	// Consume event
	select {
	case <-sub:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected to receive event")
	}
}

func TestBrokerBackwardCompatibility(t *testing.T) {
	t.Parallel()

	// Test that existing constructors still work
	broker1 := NewBroker[string]()
	defer broker1.Shutdown()

	broker2 := NewBrokerWithOptions[string](32, 500)
	defer broker2.Shutdown()

	// Both should use DropEvents strategy by default
	ctx := context.Background()
	sub1 := broker1.Subscribe(ctx)
	sub2 := broker2.Subscribe(ctx)

	broker1.Publish(CreatedEvent, "test1")
	broker2.Publish(CreatedEvent, "test2")

	// Should receive events
	select {
	case event := <-sub1:
		assert.Equal(t, "test1", event.Payload)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected to receive event from broker1")
	}

	select {
	case event := <-sub2:
		assert.Equal(t, "test2", event.Payload)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected to receive event from broker2")
	}
}
