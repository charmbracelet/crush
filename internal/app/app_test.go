package app

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

const testSendTimeout = 50 * time.Millisecond

func TestSetupSubscriber_NormalFlow(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var wg sync.WaitGroup
	broker := pubsub.NewBroker[string]()
	defer broker.Shutdown()

	outputCh := make(chan tea.Msg, 10)
	defer close(outputCh)

	subscriber := func(ctx context.Context) <-chan pubsub.Event[string] {
		return broker.Subscribe(ctx)
	}

	setupSubscriber(ctx, &wg, "test", subscriber, outputCh, testSendTimeout)

	time.Sleep(10 * time.Millisecond)

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
}

func TestSetupSubscriber_SlowConsumer(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var wg sync.WaitGroup
	broker := pubsub.NewBroker[string]()
	defer broker.Shutdown()

	slowOutputCh := make(chan tea.Msg)
	defer close(slowOutputCh)

	subscriber := func(ctx context.Context) <-chan pubsub.Event[string] {
		return broker.Subscribe(ctx)
	}

	const numEvents = 5
	setupSubscriber(ctx, &wg, "test", subscriber, slowOutputCh, testSendTimeout)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numEvents; i++ {
			broker.Publish(pubsub.CreatedEvent, "event")
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Let all events be published and timeouts fire.
	time.Sleep(time.Duration(numEvents) * (testSendTimeout + 20*time.Millisecond))

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

	cancel()
	wg.Wait()

	require.Less(t, received, numEvents, "Slow consumer should have dropped some messages")
}

func TestSetupSubscriber_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var wg sync.WaitGroup
	broker := pubsub.NewBroker[string]()
	defer broker.Shutdown()

	outputCh := make(chan tea.Msg, 10)
	defer close(outputCh)

	subscriber := func(ctx context.Context) <-chan pubsub.Event[string] {
		return broker.Subscribe(ctx)
	}

	setupSubscriber(ctx, &wg, "test", subscriber, outputCh, testSendTimeout)

	broker.Publish(pubsub.CreatedEvent, "event1")
	time.Sleep(100 * time.Millisecond)
	cancel()

	wg.Wait()
}

func TestSetupSubscriber_DrainAfterDrop(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var wg sync.WaitGroup
	broker := pubsub.NewBroker[string]()
	defer broker.Shutdown()

	// Unbuffered channel forces drops when consumer isn't reading.
	outputCh := make(chan tea.Msg)
	defer close(outputCh)

	subscriber := func(ctx context.Context) <-chan pubsub.Event[string] {
		return broker.Subscribe(ctx)
	}

	setupSubscriber(ctx, &wg, "test", subscriber, outputCh, testSendTimeout)

	// Give the goroutine time to start.
	time.Sleep(10 * time.Millisecond)

	// First event: nobody reads outputCh so the timer fires (message dropped).
	broker.Publish(pubsub.CreatedEvent, "event1")
	time.Sleep(testSendTimeout + 25*time.Millisecond)

	// Second event: triggers Stop()==false path; without the fix this deadlocks.
	broker.Publish(pubsub.CreatedEvent, "event2")

	// Cancel and wait — if the drain deadlocks, wg.Wait never returns.
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
}

func TestSetupSubscriber_NoTimerLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var wg sync.WaitGroup
	broker := pubsub.NewBroker[string]()
	defer broker.Shutdown()

	outputCh := make(chan tea.Msg, 100)
	defer close(outputCh)

	subscriber := func(ctx context.Context) <-chan pubsub.Event[string] {
		return broker.Subscribe(ctx)
	}

	setupSubscriber(ctx, &wg, "test", subscriber, outputCh, testSendTimeout)

	for i := 0; i < 100; i++ {
		broker.Publish(pubsub.CreatedEvent, "event")
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	wg.Wait()
}
