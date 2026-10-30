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
