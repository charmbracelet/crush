package terminal

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func testRequest() Request {
	return Request{
		SessionID:  "session-1",
		ToolCallID: "call-1",
		Command:    "gh auth login",
		WorkingDir: "/tmp",
	}
}

func newAvailableService() *terminalService {
	svc := NewService()
	svc.SetAvailable(true)
	return svc
}

func TestRunPublishesRequestAndBlocks(t *testing.T) {
	t.Parallel()

	svc := newAvailableService()
	ctx := t.Context()

	events := svc.Subscribe(ctx)

	type outcome struct {
		result Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := svc.Run(ctx, testRequest())
		done <- outcome{result, err}
	}()

	var request Request
	select {
	case event := <-events:
		require.Equal(t, pubsub.CreatedEvent, event.Type)
		request = event.Payload
	case <-time.After(5 * time.Second):
		t.Fatal("request was not published")
	}

	require.NotEmpty(t, request.ID)
	require.Equal(t, "gh auth login", request.Command)

	require.True(t, svc.Complete(Result{Output: "done", ExitCode: 0}))

	select {
	case got := <-done:
		require.NoError(t, got.err)
		require.Equal(t, "done", got.result.Output)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after Complete")
	}

	require.False(t, svc.Pending())
}

func TestCompleteNotification(t *testing.T) {
	t.Parallel()

	svc := newAvailableService()
	ctx := t.Context()

	notifications := svc.SubscribeNotifications(ctx)

	go func() {
		_, _ = svc.Run(ctx, testRequest())
	}()

	require.Eventually(t, svc.Pending, 5*time.Second, 10*time.Millisecond)
	require.True(t, svc.Complete(Result{ExitCode: 1}))

	select {
	case event := <-notifications:
		require.NotEmpty(t, event.Payload.ID)
	case <-time.After(5 * time.Second):
		t.Fatal("notification was not published")
	}
}

func TestCancel(t *testing.T) {
	t.Parallel()

	svc := newAvailableService()
	ctx := t.Context()

	errCh := make(chan error, 1)
	go func() {
		_, err := svc.Run(ctx, testRequest())
		errCh <- err
	}()

	require.Eventually(t, svc.Pending, 5*time.Second, 10*time.Millisecond)
	require.True(t, svc.Cancel())

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, ErrCancelled)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after Cancel")
	}

	require.False(t, svc.Cancel())
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	svc := newAvailableService()
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := svc.Run(ctx, testRequest())
		errCh <- err
	}()

	require.Eventually(t, svc.Pending, 5*time.Second, 10*time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestOnlyOnePendingSession(t *testing.T) {
	t.Parallel()

	svc := newAvailableService()
	ctx := t.Context()

	firstErr := make(chan error, 1)
	go func() {
		_, err := svc.Run(ctx, testRequest())
		firstErr <- err
	}()

	require.Eventually(t, svc.Pending, 5*time.Second, 10*time.Millisecond)

	_, err := svc.Run(ctx, testRequest())
	require.ErrorIs(t, err, ErrBusy)

	require.True(t, svc.Cancel())
	<-firstErr
}

func TestRunValidation(t *testing.T) {
	t.Parallel()

	svc := newAvailableService()
	_, err := svc.Run(context.Background(), Request{Command: "ls"})
	require.Error(t, err)
}

func TestRunUnavailableWithoutTUI(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_, err := svc.Run(context.Background(), testRequest())
	require.ErrorIs(t, err, ErrUnavailable)
}
