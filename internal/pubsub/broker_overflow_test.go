package pubsub

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBroker_ConcurrentShutdown verifies that Shutdown is idempotent when
// several goroutines call it at once, as workspaces sharing the
// process-global MCP broker do when they shut down in parallel. Closing the
// done channel twice would panic and crash the server.
func TestBroker_ConcurrentShutdown(t *testing.T) {
	t.Parallel()

	const (
		iterations = 2000
		callers    = 4
	)
	for n := range iterations {
		b := NewBroker[int]()
		start := make(chan struct{})
		recovered := make([]any, callers)
		var wg sync.WaitGroup
		for i := range callers {
			wg.Go(func() {
				defer func() { recovered[i] = recover() }()
				<-start
				b.Shutdown()
			})
		}
		close(start)
		wg.Wait()

		for _, r := range recovered {
			require.Nil(t, r, "Shutdown panicked on iteration %d", n)
		}
	}
}

// TestSubscribeWithOverflow guards that a subscriber that falls behind is
// told so (its overflow channel closes) instead of silently losing events.
func TestSubscribeWithOverflow(t *testing.T) {
	t.Parallel()

	for name, publish := range map[string]func(*Broker[int], int){
		"Publish": func(b *Broker[int], v int) { b.Publish(CreatedEvent, v) },
		"PublishMustDeliver": func(b *Broker[int], v int) {
			b.PublishMustDeliver(context.Background(), CreatedEvent, v)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := NewBrokerWithOptions[int](1)
			b.SetMustDeliverTimeout(time.Millisecond)
			t.Cleanup(b.Shutdown)

			slow, slowOverflow := b.SubscribeWithOverflow(t.Context())
			fast, fastOverflow := b.SubscribeWithOverflow(t.Context())

			publish(b, 1)
			<-fast
			require.False(t, isClosed(slowOverflow))

			publish(b, 2)
			<-fast
			publish(b, 3)
			<-fast
			require.True(t, isClosed(slowOverflow))
			require.False(t, isClosed(fastOverflow))
			require.Equal(t, 1, (<-slow).Payload)
		})
	}
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
