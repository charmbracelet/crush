package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
)

func TestCoordinator_PreAgentFailureCompletion(t *testing.T) {
	t.Parallel()
	for _, runID := range []string{"", "correlated"} {
		for _, failure := range []error{errors.New("initialization failed"), context.Canceled} {
			t.Run(runID+"/"+failure.Error(), func(t *testing.T) {
				t.Parallel()
				broker := pubsub.NewBroker[notify.RunComplete]()
				defer broker.Shutdown()
				events := broker.Subscribe(t.Context())
				coord := &coordinator{runComplete: broker, completionSequence: new(atomic.Uint64)}
				coord.readyWg.Go(func() error { return failure })
				ctx, cancel := context.WithCancel(WithRunCompleteMarker(WithRunID(t.Context(), runID)))
				if errors.Is(failure, context.Canceled) {
					cancel()
				}
				defer cancel()
				_, err := coord.Run(ctx, "session", "prompt")
				require.ErrorIs(t, err, failure)
				require.True(t, RunCompletePublished(ctx))
				require.Len(t, events, 1)
				rc := (<-events).Payload
				require.Equal(t, runID, rc.RunID)
				require.Equal(t, "session", rc.SessionID)
				require.Equal(t, failure.Error(), rc.Error)
				require.Equal(t, errors.Is(failure, context.Canceled), rc.Cancelled)
			})
		}
	}
}

func TestCoordinator_CancelledPreAgentFailureWaitsForDelivery(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		broker := pubsub.NewBrokerWithOptions[notify.RunComplete](0)
		defer broker.Shutdown()
		events := broker.Subscribe(t.Context())
		coord := &coordinator{runComplete: broker, completionSequence: new(atomic.Uint64)}
		coord.readyWg.Go(func() error { return context.Canceled })
		ctx, cancel := context.WithCancel(WithRunCompleteMarker(t.Context()))
		cancel()
		done := make(chan error, 1)
		go func() { _, err := coord.Run(ctx, "session", "prompt"); done <- err }()
		synctest.Wait()
		require.Empty(t, done, "a cancelled context must not drop reliable completion")
		require.False(t, RunCompletePublished(ctx), "ownership transfers only after publication")
		rc := (<-events).Payload
		require.True(t, rc.Cancelled)
		require.ErrorIs(t, <-done, context.Canceled)
		require.True(t, RunCompletePublished(ctx))
	})
}
