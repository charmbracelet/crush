package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type preparationHandoffModel struct {
	finishStreamModel
	beforeStream func() error
}

func (m *preparationHandoffModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if err := m.beforeStream(); err != nil {
		return nil, err
	}
	return m.finishStreamModel.Stream(ctx, call)
}

type failedQueuedPreparation struct {
	message.Service
	err error
}

func (m failedQueuedPreparation) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	if params.Role == message.User {
		for _, part := range params.Parts {
			if text, ok := part.(message.TextContent); ok && text.Text == "B" {
				return message.Message{}, m.err
			}
		}
	}
	return m.Service.Create(ctx, sessionID, params)
}

type heldPreparationCompletion struct {
	*pubsub.Broker[notify.RunComplete]
	release <-chan struct{}
}

func (p heldPreparationCompletion) PublishMustDeliver(ctx context.Context, kind pubsub.EventType, complete notify.RunComplete) {
	p.Broker.PublishMustDeliver(ctx, kind, complete)
	if complete.RunID == "B" {
		// The first subscriber has received B, but publication has not
		// returned yet, as when another subscriber applies backpressure.
		select {
		case <-p.release:
		case <-ctx.Done():
		}
	}
}

func TestRun_SubmissionDuringPreparationCompletionIsNotStranded(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	env := testEnv(t)
	sess, err := env.sessions.Create(ctx, "session")
	require.NoError(t, err)
	model := &preparationHandoffModel{finishStreamModel: finishStreamModel{text: "done"}}
	sa := testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system")
	a, ok := sa.(*sessionAgent)
	require.True(t, ok)
	failure := errors.New("B preparation failed")
	a.messages = failedQueuedPreparation{Service: env.messages, err: failure}
	broker := pubsub.NewBroker[notify.RunComplete]()
	defer broker.Shutdown()
	gate := make(chan struct{})
	release := sync.OnceFunc(func() { close(gate) })
	defer release()
	a.runComplete = heldPreparationCompletion{Broker: broker, release: gate}
	events := broker.Subscribe(ctx)
	queued := false
	model.beforeStream = func() error {
		if queued {
			return nil
		}
		queued = true
		_, err := sa.Run(ctx, SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "B"})
		return err
	}
	done := make(chan error, 1)
	go func() {
		_, err := sa.Run(ctx, SessionAgentCall{
			SessionID: sess.ID, RunID: "A", Prompt: "A", OnComplete: func(notify.RunComplete) {},
		})
		done <- err
	}()
	select {
	case event := <-events:
		require.Equal(t, "B", event.Payload.RunID)
		require.Contains(t, event.Payload.Error, failure.Error())
	case <-ctx.Done():
		t.Fatal("B did not publish its preparation failure")
	}
	_, err = sa.Run(ctx, SessionAgentCall{SessionID: sess.ID, RunID: "C", Prompt: "C"})
	require.NoError(t, err)
	release()
	select {
	case err := <-done:
		// The first commit still propagates B's error to A. This test
		// isolates C's liveness, not the later outcome-isolation change.
		if err != nil {
			require.ErrorIs(t, err, failure)
		}
	case <-ctx.Done():
		t.Fatal("the owner did not finish")
	}
	require.False(t, sa.IsSessionBusy(sess.ID))
	require.Zero(t, sa.QueuedPrompts(sess.ID), "C must not remain queued without an owner")
	require.Len(t, events, 1, "C must complete without another submission")
	complete := (<-events).Payload
	require.Equal(t, "C", complete.RunID)
	require.Empty(t, complete.Error)
	require.Equal(t, "done", complete.Text)
}
