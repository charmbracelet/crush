package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSubscribeWithOverflow checks that the overflow channel is closed the
// first time either publish path drops an event for its subscriber, and
// only for that subscriber.
func TestSubscribeWithOverflow(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		publish func(*Broker[int], int)
	}{
		{"Publish", func(b *Broker[int], v int) {
			b.Publish(CreatedEvent, v)
		}},
		{"PublishMustDeliver", func(b *Broker[int], v int) {
			b.PublishMustDeliver(context.Background(), CreatedEvent, v)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := NewBrokerWithOptions[int](1)
			b.SetMustDeliverTimeout(time.Millisecond)
			t.Cleanup(b.Shutdown)

			slow, slowOverflow := b.SubscribeWithOverflow(t.Context())
			fast, fastOverflow := b.SubscribeWithOverflow(t.Context())
			// A plain subscriber that falls behind as well must not get
			// in the way.
			_ = b.Subscribe(t.Context())

			tt.publish(b, 1)
			<-fast
			require.False(t, isClosed(slowOverflow), "nothing was dropped yet")

			// The slow subscriber's buffer is full, so these are dropped
			// for it.
			tt.publish(b, 2)
			<-fast
			tt.publish(b, 3)
			<-fast
			require.True(t, isClosed(slowOverflow), "a drop must close the overflow channel")
			require.False(t, isClosed(fastOverflow), "a subscriber that keeps up must not see an overflow")
			require.Equal(t, 1, (<-slow).Payload, "events buffered before the drop are kept")
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
