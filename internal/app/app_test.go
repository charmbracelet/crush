package app

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestSetupSubscriber_NormalFlow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var wg sync.WaitGroup
		broker := pubsub.NewBroker[string]()
		defer broker.Shutdown()

		outputCh := make(chan tea.Msg, 10)

		subscriber := func(ctx context.Context) <-chan pubsub.Event[string] {
			return broker.Subscribe(ctx)
		}

		setupSubscriber(ctx, &wg, "test", subscriber, outputCh)

		time.Sleep(10 * time.Millisecond)
		synctest.Wait()

		broker.Publish(pubsub.CreatedEvent, "event1")
		broker.Publish(pubsub.CreatedEvent, "event2")

		received := 0
		timeout := time.After(5 * time.Second)
		for {
			select {
			case <-outputCh:
				received++
				if received >= 2 {
					cancel()
					wg.Wait()
					require.Equal(t, 2, received, "Should have received both messages")
					return
				}
			case <-timeout:
				wg.Wait()
				t.Fatalf("Timed out waiting for messages. Received: %d", received)
			case <-ctx.Done():
				wg.Wait()
				t.Fatalf("Context cancelled before receiving all messages. Received: %d", received)
			}
		}
	})
}

func TestSetupSubscriber_SlowConsumer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var wg sync.WaitGroup
		broker := pubsub.NewBroker[string]()
		defer broker.Shutdown()

		slowOutputCh := make(chan tea.Msg)

		subscriber := func(ctx context.Context) <-chan pubsub.Event[string] {
			return broker.Subscribe(ctx)
		}

		const numEvents = 5

		setupSubscriber(ctx, &wg, "test", subscriber, slowOutputCh)

		var pubWg sync.WaitGroup
		pubWg.Go(func() {
			for range numEvents {
				broker.Publish(pubsub.CreatedEvent, "event")
				time.Sleep(10 * time.Millisecond)
				synctest.Wait()
			}
		})

		// Let all events be published and timeouts fire.
		time.Sleep(time.Duration(numEvents) * (subscriberSendTimeout + 20*time.Millisecond))
		synctest.Wait()

		// Drain whatever made it through.
		received := 0
	drainLoop:
		for {
			select {
			case <-slowOutputCh:
				received++
			default:
				break drainLoop
			}
		}

		pubWg.Wait()
		cancel()
		wg.Wait()

		require.Less(t, received, numEvents, "Slow consumer should have dropped some messages")
	})
}

func TestSetupSubscriber_ContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var wg sync.WaitGroup
		broker := pubsub.NewBroker[string]()
		defer broker.Shutdown()

		outputCh := make(chan tea.Msg, 10)

		subscriber := func(ctx context.Context) <-chan pubsub.Event[string] {
			return broker.Subscribe(ctx)
		}

		setupSubscriber(ctx, &wg, "test", subscriber, outputCh)

		broker.Publish(pubsub.CreatedEvent, "event1")
		time.Sleep(100 * time.Millisecond)
		synctest.Wait()
		cancel()

		wg.Wait()
	})
}

func TestSetupSubscriber_DrainAfterDrop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var wg sync.WaitGroup
		broker := pubsub.NewBroker[string]()
		defer broker.Shutdown()

		// Unbuffered channel forces drops when consumer isn't reading.
		outputCh := make(chan tea.Msg)

		subscriber := func(ctx context.Context) <-chan pubsub.Event[string] {
			return broker.Subscribe(ctx)
		}

		setupSubscriber(ctx, &wg, "test", subscriber, outputCh)

		// Give the goroutine time to start.
		time.Sleep(10 * time.Millisecond)
		synctest.Wait()

		// First event: nobody reads outputCh so the timer fires (message dropped).
		broker.Publish(pubsub.CreatedEvent, "event1")
		time.Sleep(subscriberSendTimeout + 25*time.Millisecond)
		synctest.Wait()

		// Second event: triggers Stop()==false path; without the fix this deadlocks.
		broker.Publish(pubsub.CreatedEvent, "event2")

		// Cancel and wait — if the drain deadlocks, wg.Wait never returns.
		// The goroutine below is spawned only for timeout orchestration;
		// it completes before the synctest bubble exits.
		done := make(chan struct{})
		go func() {
			cancel()
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success: goroutine exited cleanly.
		case <-time.After(5 * time.Second):
			t.Fatal("setupSubscriber goroutine hung — likely timer drain deadlock")
		}
	})
}

func TestSetupSubscriber_NoTimerLeak(t *testing.T) {
	defer goleak.VerifyNone(t)
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var wg sync.WaitGroup
		broker := pubsub.NewBroker[string]()
		defer broker.Shutdown()

		outputCh := make(chan tea.Msg, 100)

		subscriber := func(ctx context.Context) <-chan pubsub.Event[string] {
			return broker.Subscribe(ctx)
		}

		setupSubscriber(ctx, &wg, "test", subscriber, outputCh)

		for range 100 {
			broker.Publish(pubsub.CreatedEvent, "event")
			time.Sleep(5 * time.Millisecond)
			synctest.Wait()
		}

		cancel()
		wg.Wait()
	})
}
