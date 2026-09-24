package terminal

import (
	"testing"

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
