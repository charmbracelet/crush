package sshaskpass

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPromptsRoundTrip(t *testing.T) {
	svc := NewPrompts()
	sub := svc.Subscribe(context.Background())

	type result struct {
		secret string
		err    error
	}
	done := make(chan result, 1)
	go func() {
		secret, err := svc.Prompt(context.Background(), PromptRequest{
			Prompt: "deploy@example.com's password: ",
			Kind:   KindPassword,
		})
		done <- result{secret: secret, err: err}
	}()

	select {
	case ev := <-sub:
		require.Equal(t, KindPassword, ev.Payload.Kind)
		require.True(t, svc.Respond(ev.Payload.ID, "s3cret"))
	case <-time.After(5 * time.Second):
		t.Fatal("prompt was never published")
	}

	select {
	case r := <-done:
		require.NoError(t, r.err)
		require.Equal(t, "s3cret", r.secret)
	case <-time.After(5 * time.Second):
		t.Fatal("prompt never resolved")
	}
}

func TestPromptsCancel(t *testing.T) {
	svc := NewPrompts()
	sub := svc.Subscribe(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := svc.Prompt(context.Background(), PromptRequest{Prompt: "prompt"})
		errCh <- err
	}()

	var id string
	select {
	case ev := <-sub:
		id = ev.Payload.ID
	case <-time.After(5 * time.Second):
		t.Fatal("prompt was never published")
	}

	require.True(t, svc.Cancel(id))
	select {
	case err := <-errCh:
		require.ErrorIs(t, err, ErrCancelled)
	case <-time.After(5 * time.Second):
		t.Fatal("prompt never cancelled")
	}
}

func TestPromptsNoSubscriber(t *testing.T) {
	svc := NewPrompts()
	_, err := svc.Prompt(context.Background(), PromptRequest{Prompt: "prompt"})
	require.ErrorIs(t, err, ErrNoPrompter)
}

func TestPromptsRespondWithoutPending(t *testing.T) {
	svc := NewPrompts()
	require.False(t, svc.Respond("nope", "secret"))
	require.False(t, svc.Cancel("nope"))
}

func TestPromptsResolutionNotification(t *testing.T) {
	svc := NewPrompts()
	sub := svc.Subscribe(context.Background())
	notifications := svc.SubscribeNotifications(context.Background())

	done := make(chan struct{})
	go func() {
		_, _ = svc.Prompt(context.Background(), PromptRequest{Prompt: "prompt"})
		close(done)
	}()

	ev := <-sub
	require.True(t, svc.Respond(ev.Payload.ID, "secret"))
	<-done

	select {
	case n := <-notifications:
		require.Equal(t, ev.Payload.ID, n.Payload.RequestID)
	case <-time.After(5 * time.Second):
		t.Fatal("resolution notification never published")
	}
}

func TestParsePromptTidiesKeyInfo(t *testing.T) {
	t.Setenv("HOME", "/home/drew")
	t.Setenv("USERPROFILE", "/home/drew")
	got := ParsePrompt("Enter PIN for ED25519-SK key /home/drew/.ssh/id_sk: ")
	if got.KeyInfo != "ED25519-SK key ~/.ssh/id_sk" {
		t.Fatalf("key info: %q", got.KeyInfo)
	}
	if got.Title != "Security Key PIN" {
		t.Fatalf("title: %q", got.Title)
	}
	// "pin" inside a host name is not a PIN prompt.
	if ParsePrompt("drew@pinecone's password: ").Title != "SSH Password" {
		t.Fatal("host name containing pin misclassified")
	}
}

func TestPromptsDoubleResolveDoesNotBlock(t *testing.T) {
	svc := NewPrompts()
	ch := svc.Subscribe(t.Context())
	go svc.Prompt(t.Context(), PromptRequest{Prompt: "p"})
	ev := <-ch
	if !svc.Cancel(ev.Payload.ID) {
		t.Fatal("cancel failed")
	}
	svc.Cancel(ev.Payload.ID)
	svc.Respond(ev.Payload.ID, "x")
	svc.Respond(ev.Payload.ID, "y")
}

// TestNoticeHoldsUntilDoneWithoutBlockingPrompts verifies a touch notice
// is published, stays pending until its context ends (ssh killing the
// helper), then notifies, and never holds up a credential prompt.
func TestNoticeHoldsUntilDoneWithoutBlockingPrompts(t *testing.T) {
	svc := NewPrompts()
	sub := svc.Subscribe(t.Context())
	notifications := svc.SubscribeNotifications(t.Context())

	touchCtx, touchLands := context.WithCancel(t.Context())
	noticeDone := make(chan struct{})
	go func() {
		svc.Notice(touchCtx, PromptRequest{ID: "touch", Kind: KindTouch, Prompt: "Confirm user presence"})
		close(noticeDone)
	}()
	require.Equal(t, "touch", (<-sub).Payload.ID)

	// A password prompt goes through while the touch is still pending.
	secret := make(chan string, 1)
	go func() {
		s, _ := svc.Prompt(t.Context(), PromptRequest{ID: "pw", Prompt: "password: "})
		secret <- s
	}()
	select {
	case ev := <-sub:
		require.Equal(t, "pw", ev.Payload.ID)
		require.True(t, svc.Respond("pw", "hunter2"))
	case <-time.After(5 * time.Second):
		t.Fatal("pending touch blocked the password prompt")
	}
	require.Equal(t, "hunter2", <-secret)
	require.Equal(t, "pw", (<-notifications).Payload.RequestID)

	select {
	case <-noticeDone:
		t.Fatal("notice ended before the touch landed")
	default:
	}

	touchLands()
	select {
	case <-noticeDone:
	case <-time.After(5 * time.Second):
		t.Fatal("notice never ended")
	}
	require.Equal(t, "touch", (<-notifications).Payload.RequestID)
}

func TestNoticeWithoutSubscriberReturns(t *testing.T) {
	NewPrompts().Notice(t.Context(), PromptRequest{Kind: KindTouch})
}
