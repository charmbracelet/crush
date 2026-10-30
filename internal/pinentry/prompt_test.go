package pinentry

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestKeyLines(t *testing.T) {
	t.Parallel()

	require.Empty(t, Key{}.Lines())

	require.Equal(t, []string{
		"Number: D2760001240103040006123456780000",
		"Key:    ABC1234567890DEF",
		"Holder: Test Key <test@example.com>",
	}, Key{
		CardNumber: "D2760001240103040006123456780000",
		KeyID:      "ABC1234567890DEF",
		Holder:     "Test Key <test@example.com>",
	}.Lines())

	// The fingerprint stands in when GPG reported no long key ID.
	require.Equal(t, []string{"Key:    0123ABCD"}, Key{Fingerprint: "0123ABCD"}.Lines())
}

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

func TestPromptQueuesConcurrentPrompts(t *testing.T) {
	svc := NewPrompts()
	ctx := context.Background()
	ch := svc.Subscribe(ctx)
	defer svc.broker.Shutdown()

	go svc.Prompt(ctx, PromptRequest{Prompt: "first"})
	first := <-ch

	second := make(chan string, 1)
	go func() {
		s, _ := svc.Prompt(ctx, PromptRequest{Prompt: "second"})
		second <- s
	}()

	// The second prompt waits for the first instead of failing...
	select {
	case ev := <-ch:
		t.Fatalf("second prompt published while the first is pending: %q", ev.Payload.Prompt)
	case <-time.After(50 * time.Millisecond):
	}
	require.True(t, svc.Respond(first.Payload.ID, "one"))

	// ...then gets its turn.
	ev := <-ch
	require.Equal(t, "second", ev.Payload.Prompt)
	require.True(t, svc.Respond(ev.Payload.ID, "two"))
	require.Equal(t, "two", <-second)
}

func TestPromptRespectsContext(t *testing.T) {
	svc := NewPrompts()
	ctx := context.Background()
	ch := svc.Subscribe(ctx)
	notifs := svc.SubscribeNotifications(ctx)
	defer svc.broker.Shutdown()
	defer svc.notificationBroker.Shutdown()

	childCtx, cancel := context.WithCancel(ctx)
	res := make(chan error, 1)
	go func() {
		_, err := svc.Prompt(childCtx, PromptRequest{Prompt: "p"})
		res <- err
	}()
	ev := <-ch
	cancel()
	require.ErrorIs(t, <-res, context.Canceled)
	// The abandoned prompt's dialog is dismissed.
	require.Equal(t, ev.Payload.ID, (<-notifs).Payload.RequestID)
}
