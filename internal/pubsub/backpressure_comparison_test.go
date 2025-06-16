package pubsub

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestBackpressureComparison demonstrates the differences between the old silent drop behavior
// and the new backpressure handling strategies
func TestBackpressureComparison(t *testing.T) {
	t.Parallel()

	const (
		bufferSize    = 2
		numEvents     = 10
		publishDelay  = 1 * time.Millisecond
		consumerDelay = 50 * time.Millisecond // Slow consumer
	)

	t.Run("Old Behavior - Silent Drops (DropEvents)", func(t *testing.T) {
		t.Parallel()

		broker := NewBrokerWithBackpressure[int](bufferSize, 100, DropEvents, time.Second)
		defer broker.Shutdown()

		ctx := context.Background()
		sub := broker.Subscribe(ctx)

		var receivedEvents []int
		var mu sync.Mutex

		// Slow consumer
		go func() {
			for event := range sub {
				time.Sleep(consumerDelay)
				mu.Lock()
				receivedEvents = append(receivedEvents, event.Payload)
				mu.Unlock()
			}
		}()

		// Fast publisher
		for i := 0; i < numEvents; i++ {
			broker.Publish(CreatedEvent, i)
			time.Sleep(publishDelay)
		}

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		receivedCount := len(receivedEvents)
		mu.Unlock()

		droppedCount := broker.GetDroppedEventCount()

		fmt.Printf("DropEvents Strategy Results:\n")
		fmt.Printf("  Published: %d events\n", numEvents)
		fmt.Printf("  Received: %d events\n", receivedCount)
		fmt.Printf("  Dropped: %d events\n", droppedCount)
		fmt.Printf("  Events lost silently: %d\n", numEvents-receivedCount)

		// Should have dropped some events
		assert.Greater(t, droppedCount, int64(0), "Should have dropped some events")
		assert.Less(t, receivedCount, numEvents, "Should not have received all events")
		assert.Equal(t, int64(numEvents-receivedCount), droppedCount, "Dropped count should match missing events")
	})

	t.Run("New Behavior - Block Publisher", func(t *testing.T) {
		t.Parallel()

		publishTimeout := 10 * time.Millisecond
		broker := NewBrokerWithBackpressure[int](bufferSize, 100, BlockPublisher, publishTimeout)
		defer broker.Shutdown()

		ctx := context.Background()
		sub := broker.Subscribe(ctx)

		var receivedEvents []int
		var mu sync.Mutex

		// Slow consumer
		go func() {
			for event := range sub {
				time.Sleep(consumerDelay)
				mu.Lock()
				receivedEvents = append(receivedEvents, event.Payload)
				mu.Unlock()
			}
		}()

		start := time.Now()

		// Fast publisher - will be throttled by backpressure
		for i := 0; i < numEvents; i++ {
			broker.Publish(CreatedEvent, i)
			time.Sleep(publishDelay)
		}

		publishDuration := time.Since(start)

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		receivedCount := len(receivedEvents)
		mu.Unlock()

		droppedCount := broker.GetDroppedEventCount()

		fmt.Printf("\nBlockPublisher Strategy Results:\n")
		fmt.Printf("  Published: %d events\n", numEvents)
		fmt.Printf("  Received: %d events\n", receivedCount)
		fmt.Printf("  Dropped: %d events\n", droppedCount)
		fmt.Printf("  Publish duration: %v\n", publishDuration)
		fmt.Printf("  Publisher was throttled by backpressure\n")

		// Publisher should have been slowed down by backpressure
		expectedMinDuration := time.Duration(numEvents-bufferSize) * publishTimeout / 2
		assert.Greater(t, publishDuration, expectedMinDuration, "Publisher should have been throttled")
	})

	t.Run("New Behavior - Remove Slow Subscribers", func(t *testing.T) {
		t.Parallel()

		broker := NewBrokerWithBackpressure[int](bufferSize, 100, RemoveSlowSubscribers, time.Second)
		defer broker.Shutdown()

		ctx := context.Background()
		sub := broker.Subscribe(ctx)

		var receivedEvents []int
		var mu sync.Mutex
		channelClosed := false

		// Slow consumer
		go func() {
			for event := range sub {
				time.Sleep(consumerDelay)
				mu.Lock()
				receivedEvents = append(receivedEvents, event.Payload)
				mu.Unlock()
			}
			mu.Lock()
			channelClosed = true
			mu.Unlock()
		}()

		// Fast publisher
		for i := 0; i < numEvents; i++ {
			broker.Publish(CreatedEvent, i)
			time.Sleep(publishDelay)
		}

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		receivedCount := len(receivedEvents)
		closed := channelClosed
		mu.Unlock()

		droppedCount := broker.GetDroppedEventCount()
		removedCount := broker.GetSlowSubscribersRemovedCount()
		subscriberCount := broker.GetSubscriberCount()

		fmt.Printf("\nRemoveSlowSubscribers Strategy Results:\n")
		fmt.Printf("  Published: %d events\n", numEvents)
		fmt.Printf("  Received: %d events\n", receivedCount)
		fmt.Printf("  Dropped: %d events\n", droppedCount)
		fmt.Printf("  Slow subscribers removed: %d\n", removedCount)
		fmt.Printf("  Current subscribers: %d\n", subscriberCount)
		fmt.Printf("  Channel closed: %v\n", closed)

		// Should have removed the slow subscriber
		assert.Equal(t, int64(1), removedCount, "Should have removed one slow subscriber")
		assert.Equal(t, 0, subscriberCount, "Should have no subscribers left")
		assert.True(t, closed, "Channel should be closed")
	})
}

// TestBackpressureMetricsAccuracy verifies that the metrics accurately track what happens
func TestBackpressureMetricsAccuracy(t *testing.T) {
	t.Parallel()

	t.Run("Metrics track drops accurately", func(t *testing.T) {
		t.Parallel()

		broker := NewBrokerWithBackpressure[string](1, 100, DropEvents, time.Second)
		defer broker.Shutdown()

		ctx := context.Background()
		sub := broker.Subscribe(ctx)

		// Fill buffer and cause predictable drops
		broker.Publish(CreatedEvent, "event1") // Should succeed
		broker.Publish(CreatedEvent, "event2") // Should drop
		broker.Publish(CreatedEvent, "event3") // Should drop
		broker.Publish(CreatedEvent, "event4") // Should drop

		assert.Equal(t, int64(3), broker.GetDroppedEventCount(), "Should track 3 dropped events")

		// Consume one event to make space
		select {
		case event := <-sub:
			assert.Equal(t, "event1", event.Payload)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Should receive event1")
		}

		// Publish more events
		broker.Publish(CreatedEvent, "event5") // Should succeed
		broker.Publish(CreatedEvent, "event6") // Should drop

		assert.Equal(t, int64(4), broker.GetDroppedEventCount(), "Should track 4 total dropped events")
	})

	t.Run("Multiple subscribers with different speeds", func(t *testing.T) {
		t.Parallel()

		broker := NewBrokerWithBackpressure[int](2, 100, DropEvents, time.Second)
		defer broker.Shutdown()

		ctx := context.Background()

		// Fast subscriber
		fastSub := broker.Subscribe(ctx)
		var fastEvents []int
		go func() {
			for event := range fastSub {
				fastEvents = append(fastEvents, event.Payload)
			}
		}()

		// Slow subscriber
		slowSub := broker.Subscribe(ctx)
		var slowEvents []int
		go func() {
			for event := range slowSub {
				time.Sleep(10 * time.Millisecond) // Simulate slow processing
				slowEvents = append(slowEvents, event.Payload)
			}
		}()

		assert.Equal(t, 2, broker.GetSubscriberCount())

		// Publish events rapidly
		for i := 0; i < 10; i++ {
			broker.Publish(CreatedEvent, i)
			time.Sleep(1 * time.Millisecond)
		}

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		fmt.Printf("\nMultiple Subscribers Results:\n")
		fmt.Printf("  Fast subscriber received: %d events\n", len(fastEvents))
		fmt.Printf("  Slow subscriber received: %d events\n", len(slowEvents))
		fmt.Printf("  Total dropped events: %d\n", broker.GetDroppedEventCount())

		// Fast subscriber should receive more events than slow subscriber
		assert.GreaterOrEqual(t, len(fastEvents), len(slowEvents), "Fast subscriber should receive at least as many events")
		assert.Greater(t, broker.GetDroppedEventCount(), int64(0), "Should have dropped some events")
	})
}

// BenchmarkBackpressureStrategies compares performance of different strategies
func BenchmarkBackpressureStrategies(b *testing.B) {
	strategies := []struct {
		name     string
		strategy BackpressureStrategy
		timeout  time.Duration
	}{
		{"DropEvents", DropEvents, time.Second},
		{"BlockPublisher_1ms", BlockPublisher, 1 * time.Millisecond},
		{"BlockPublisher_10ms", BlockPublisher, 10 * time.Millisecond},
		{"RemoveSlowSubscribers", RemoveSlowSubscribers, time.Second},
	}

	for _, s := range strategies {
		b.Run(s.name, func(b *testing.B) {
			broker := NewBrokerWithBackpressure[int](10, 1000, s.strategy, s.timeout)
			defer broker.Shutdown()

			ctx := context.Background()
			sub := broker.Subscribe(ctx)

			// Slow consumer
			go func() {
				for range sub {
					time.Sleep(100 * time.Microsecond)
				}
			}()

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				broker.Publish(CreatedEvent, i)
			}
		})
	}
}
