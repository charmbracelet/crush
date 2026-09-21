package pinentry

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestPromptResponds(t *testing.T) {
	svc := NewPrompts()
	ctx := context.Background()
	ch := svc.Subscribe(ctx)
	defer svc.broker.Shutdown()

	type result struct {
		secret string
		err    error
	}
	res := make(chan result, 1)
	go func() {
		s, err := svc.Prompt(ctx, PromptRequest{Prompt: "unlock"})
		res <- result{secret: s, err: err}
	}()

	ev := <-ch
	require.Equal(t, "unlock", ev.Payload.Prompt)
	require.Equal(t, KindPassphrase, ev.Payload.Kind, "defaults to passphrase")
	require.NotEmpty(t, ev.Payload.ID)

	require.True(t, svc.Respond(ev.Payload.ID, "s3cret"))
	r := <-res
	require.NoError(t, r.err)
	require.Equal(t, "s3cret", r.secret)

	// Resolving twice reports false.
	require.False(t, svc.Respond(ev.Payload.ID, "again"))
}

func TestPromptCancel(t *testing.T) {
	svc := NewPrompts()
	ctx := context.Background()
	ch := svc.Subscribe(ctx)
	defer svc.broker.Shutdown()

	type result struct {
		secret string
		err    error
	}
	res := make(chan result, 1)
	go func() {
		s, err := svc.Prompt(ctx, PromptRequest{Prompt: "p"})
		res <- result{secret: s, err: err}
	}()
	ev := <-ch
	require.True(t, svc.Cancel(ev.Payload.ID))
	r := <-res
	require.ErrorIs(t, r.err, ErrCancelled)
}

func TestPromptPublishesNotification(t *testing.T) {
	svc := NewPrompts()
	ctx := context.Background()
	ch := svc.Subscribe(ctx)
	notifs := svc.SubscribeNotifications(ctx)
	defer svc.broker.Shutdown()
	defer svc.notificationBroker.Shutdown()

	go svc.Prompt(ctx, PromptRequest{Prompt: "p"})
	ev := <-ch
	got := make(chan pubsub.Event[Notification], 1)
	go func() {
		got <- <-notifs
	}()
	require.True(t, svc.Respond(ev.Payload.ID, "x"))
	n := <-got
	require.Equal(t, ev.Payload.ID, n.Payload.RequestID)
}

func TestPromptNoSubscribers(t *testing.T) {
	svc := NewPrompts()
	ctx := context.Background()
	// No subscribers: Prompt must fail fast instead of hanging.
	_, err := svc.Prompt(ctx, PromptRequest{Prompt: "p"})
	require.ErrorIs(t, err, ErrNoPrompter)
}

func TestPromptOnlyOnePending(t *testing.T) {
	svc := NewPrompts()
	ctx := context.Background()
	ch := svc.Subscribe(ctx)
	defer svc.broker.Shutdown()

	go svc.Prompt(ctx, PromptRequest{Prompt: "first"})
	<-ch

	_, err := svc.Prompt(ctx, PromptRequest{Prompt: "second"})
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrNoPrompter))
}

func TestPromptRespectsContext(t *testing.T) {
	svc := NewPrompts()
	ctx := context.Background()
	ch := svc.Subscribe(ctx)
	defer svc.broker.Shutdown()

	childCtx, cancel := context.WithCancel(ctx)
	res := make(chan error, 1)
	go func() {
		_, err := svc.Prompt(childCtx, PromptRequest{Prompt: "p"})
		res <- err
	}()
	<-ch
	cancel()
	require.ErrorIs(t, <-res, context.Canceled)
}
