package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
