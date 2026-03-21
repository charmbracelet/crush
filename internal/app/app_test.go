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
		f := newSubscriberFixture(t, 10)

		time.Sleep(10 * time.Millisecond)
		synctest.Wait()

		f.broker.Publish(pubsub.CreatedEvent, "event1")
		f.broker.Publish(pubsub.CreatedEvent, "event2")

		for range 2 {
			select {
			case <-f.outputCh:
			case <-time.After(5 * time.Second):
				t.Fatal("Timed out waiting for messages")
			}
		}

		f.cancel()
		f.wg.Wait()
	})
}

func TestSetupSubscriber_SlowConsumer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 0)

		const numEvents = 5

		var pubWg sync.WaitGroup
		pubWg.Go(func() {
			for range numEvents {
				f.broker.Publish(pubsub.CreatedEvent, "event")
				time.Sleep(10 * time.Millisecond)
				synctest.Wait()
			}
		})

		time.Sleep(time.Duration(numEvents) * (subscriberSendTimeout + 20*time.Millisecond))
		synctest.Wait()

		received := 0
		for {
			select {
			case <-f.outputCh:
				received++
			default:
				pubWg.Wait()
				f.cancel()
				f.wg.Wait()
				require.Less(t, received, numEvents, "Slow consumer should have dropped some messages")
				return
			}
		}
	})
}

func TestSetupSubscriber_ContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 10)

		f.broker.Publish(pubsub.CreatedEvent, "event1")
		time.Sleep(100 * time.Millisecond)
		synctest.Wait()

		f.cancel()
		f.wg.Wait()
	})
}

func TestSetupSubscriber_DrainAfterDrop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 0)

		time.Sleep(10 * time.Millisecond)
		synctest.Wait()

		// First event: nobody reads outputCh so the timer fires (message dropped).
		f.broker.Publish(pubsub.CreatedEvent, "event1")
		time.Sleep(subscriberSendTimeout + 25*time.Millisecond)
		synctest.Wait()

		// Second event: triggers Stop()==false path; without the fix this deadlocks.
		f.broker.Publish(pubsub.CreatedEvent, "event2")

		// If the timer drain deadlocks, wg.Wait never returns.
		done := make(chan struct{})
		go func() {
			f.cancel()
			f.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("setupSubscriber goroutine hung — likely timer drain deadlock")
		}
	})
}

func TestSetupSubscriber_NoTimerLeak(t *testing.T) {
	defer goleak.VerifyNone(t)
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 100)

		for range 100 {
			f.broker.Publish(pubsub.CreatedEvent, "event")
			time.Sleep(5 * time.Millisecond)
			synctest.Wait()
		}

		f.cancel()
		f.wg.Wait()
	})
}

type subscriberFixture struct {
	broker   *pubsub.Broker[string]
	wg       sync.WaitGroup
	outputCh chan tea.Msg
	cancel   context.CancelFunc
}

func newSubscriberFixture(t *testing.T, bufSize int) *subscriberFixture {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	f := &subscriberFixture{
		broker:   pubsub.NewBroker[string](),
		outputCh: make(chan tea.Msg, bufSize),
		cancel:   cancel,
	}
	t.Cleanup(f.broker.Shutdown)

	setupSubscriber(ctx, &f.wg, "test", func(ctx context.Context) <-chan pubsub.Event[string] {
		return f.broker.Subscribe(ctx)
	}, f.outputCh)

	return f
}
