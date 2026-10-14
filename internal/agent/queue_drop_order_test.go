package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestClearQueueFreezesBatchBeforePublication(t *testing.T) {
	t.Parallel()
	_, sess, model, a := newLifecycleTestAgent(t, "session")
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer cancel()
	entered, finish := make(chan struct{}), make(chan struct{})
	finishModel := sync.OnceFunc(func() { close(finish) })
	defer finishModel()
	failure := errors.New("active turn failed")
	model.stream = func(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
		close(entered)
		select {
		case <-finish:
			return nil, failure
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	broker := pubsub.NewBroker[notify.RunComplete]()
	defer broker.Shutdown()
	events := broker.Subscribe(ctx)
	gate := make(chan struct{})
	release := sync.OnceFunc(func() { close(gate) })
	defer release()
	a.runComplete = heldPreparationCompletion{Broker: broker, release: gate}
	outcome := make(chan notify.RunComplete, 1)
	returned := make(chan error, 1)
	go func() {
		_, err := a.Run(ctx, SessionAgentCall{SessionID: sess.ID, RunID: "A", Prompt: "A", OnComplete: func(rc notify.RunComplete) { outcome <- rc }})
		returned <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	for _, id := range []string{"B", "C"} {
		result, err := a.Run(ctx, SessionAgentCall{SessionID: sess.ID, RunID: id, Prompt: id})
		require.NoError(t, err)
		require.Nil(t, result)
	}
	cleared := make(chan struct{})
	go func() { a.ClearQueue(sess.ID); close(cleared) }()
	var b notify.RunComplete
	select {
	case event := <-events:
		b = event.Payload
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.Zero(t, a.QueuedPrompts(sess.ID))
	finishModel()
	var active notify.RunComplete
	select {
	case active = <-outcome:
	case <-ctx.Done():
		t.Fatal("completion must precede blocked owner cleanup", ctx.Err())
	}
	release()
	<-cleared
	require.ErrorIs(t, <-returned, failure)
	c := (<-events).Payload
	require.Equal(t, "B", b.RunID)
	require.Equal(t, "C", c.RunID)
	require.True(t, b.Cancelled)
	require.True(t, c.Cancelled)
	require.Contains(t, active.Error, failure.Error())
	require.Less(t, b.Sequence, active.Sequence)
	require.Less(t, c.Sequence, active.Sequence)
}
