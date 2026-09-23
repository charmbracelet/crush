package terminal

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

func testSession(t *testing.T) *shell.InteractiveSession {
	t.Helper()

	session, err := shell.NewInteractiveSession(shell.InteractiveSessionOptions{
		Command:    "sleep 30",
		WorkingDir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = session.Kill()
		// Wait for the process to be reaped: a live session keeps its
		// working directory locked on Windows, which would fail the
		// temporary directory cleanup that runs right after this one.
		select {
		case <-session.Done():
		case <-time.After(5 * time.Second):
		}
		_ = session.Close()
	})
	return session
}

func testRequest(t *testing.T) Request {
	t.Helper()
	return Request{
		SessionID:  "001",
		ToolCallID: "call-1",
		Command:    "gh auth login",
		WorkingDir: "/tmp",
		Session:    testSession(t),
	}
}

func TestShowPublishesRequest(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.SetAvailable(true)
	require.True(t, svc.Available())

	events := svc.Subscribe(t.Context())

	req, err := svc.Show(t.Context(), testRequest(t))
	require.NoError(t, err)
	require.NotEmpty(t, req.ID)

	select {
	case event := <-events:
		require.Equal(t, pubsub.CreatedEvent, event.Type)
		require.Equal(t, req.ID, event.Payload.ID)
		require.Same(t, req.Session, event.Payload.Session)
		require.Equal(t, "gh auth login", event.Payload.Command)
	case <-t.Context().Done():
		t.Fatal("request was not published")
	}
}

func TestShowUnavailableWithoutTUI(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_, err := svc.Show(t.Context(), testRequest(t))
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestShowRequiresSession(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.SetAvailable(true)

	_, err := svc.Show(t.Context(), Request{Command: "ls"})
	require.Error(t, err)
}

// nextCall waits for the next call published to clients.
func nextCall(t *testing.T, calls <-chan pubsub.Event[Call]) Call {
	t.Helper()
	select {
	case event := <-calls:
		return event.Payload
	case <-time.After(5 * time.Second):
		t.Fatal("call was not published")
		return Call{}
	}
}

func TestExecRunsLocallyWithoutRouter(t *testing.T) {
	t.Parallel()

	_, remote, err := NewService().Exec(t.Context(), Call{Op: "read"})
	require.NoError(t, err)
	require.False(t, remote)
}

func TestExecForwardsToRoutedClient(t *testing.T) {
	t.Parallel()

	svc := NewService()
	var preferred []string
	svc.SetRouter(func(sessionID, pref string) (string, error) {
		require.Equal(t, "chat", sessionID)
		preferred = append(preferred, pref)
		return "client-a", nil
	})
	calls := svc.SubscribeCalls(t.Context())

	for range 2 {
		type outcome struct {
			result Result
			remote bool
			err    error
		}
		done := make(chan outcome, 1)
		go func() {
			result, remote, err := svc.Exec(t.Context(), Call{SessionID: "chat", Op: "read"})
			done <- outcome{result, remote, err}
		}()

		call := nextCall(t, calls)
		require.Equal(t, "client-a", call.ClientID)
		require.NotEmpty(t, call.ID)
		require.False(t, svc.Respond(call.ID, "client-b", Result{Content: "spoofed"}), "only the routed client may answer")
		require.True(t, svc.Respond(call.ID, "client-a", Result{Content: "screen"}))

		got := <-done
		require.NoError(t, got.err)
		require.True(t, got.remote)
		require.Equal(t, "screen", got.result.Content)
	}
	require.Equal(t, []string{"", "client-a"}, preferred, "later calls prefer the client that served the session")
}

func TestExecUnavailableWithoutClient(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.SetRouter(func(string, string) (string, error) { return "", ErrUnavailable })
	_, remote, err := svc.Exec(t.Context(), Call{SessionID: "chat", Op: "start"})
	require.True(t, remote)
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestExecFailsWhenClientDrops(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.SetRouter(func(string, string) (string, error) { return "client-a", nil })
	calls := svc.SubscribeCalls(t.Context())

	done := make(chan error, 1)
	go func() {
		_, _, err := svc.Exec(t.Context(), Call{SessionID: "chat", Op: "read"})
		done <- err
	}()
	nextCall(t, calls)
	svc.DropClient("client-a")
	require.ErrorIs(t, <-done, ErrClientGone)
}

func TestExecCancelWithdrawsCall(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.SetRouter(func(string, string) (string, error) { return "client-a", nil })
	calls := svc.SubscribeCalls(t.Context())

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, _, err := svc.Exec(ctx, Call{SessionID: "chat", Op: "start"})
		done <- err
	}()
	call := nextCall(t, calls)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)

	withdrawn := nextCall(t, calls)
	require.Equal(t, OpCancel, withdrawn.Op)
	require.Equal(t, call.ID, withdrawn.ID)
	require.Equal(t, "client-a", withdrawn.ClientID)
	require.False(t, svc.Respond(call.ID, "client-a", Result{}), "a withdrawn call takes no answer")
}
