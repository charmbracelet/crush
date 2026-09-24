package shell

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInteractiveSessionManagerStartAssignsID(t *testing.T) {
	t.Parallel()

	m := &InteractiveSessionManager{sessions: newSessionMap()}

	session, err := m.Start(InteractiveSessionOptions{
		Command:    "sleep 30",
		WorkingDir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Kill(); _ = session.Close() })

	require.NotEmpty(t, session.ID())
	require.Equal(t, session.ID(), session.ID())

	got, ok := m.Get(session.ID())
	require.True(t, ok)
	require.Same(t, session, got)

	active, ok := m.Active()
	require.True(t, ok)
	require.Same(t, session, active)
}

func TestInteractiveSessionManagerRejectsSecondSession(t *testing.T) {
	t.Parallel()

	m := &InteractiveSessionManager{sessions: newSessionMap()}

	first, err := m.Start(InteractiveSessionOptions{Command: "sleep 30", WorkingDir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Kill(); _ = first.Close() })

	_, err = m.Start(InteractiveSessionOptions{Command: "sleep 30", WorkingDir: t.TempDir()})
	require.ErrorIs(t, err, ErrInteractiveActive)
}

func TestInteractiveSessionManagerAllowsNewSessionAfterExit(t *testing.T) {
	t.Parallel()

	m := &InteractiveSessionManager{sessions: newSessionMap()}

	first, err := m.Start(InteractiveSessionOptions{Command: "true", WorkingDir: t.TempDir()})
	require.NoError(t, err)

	select {
	case <-first.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("first session did not exit")
	}

	second, err := m.Start(InteractiveSessionOptions{Command: "sleep 30", WorkingDir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = second.Kill(); _ = second.Close() })

	require.NotEqual(t, first.ID(), second.ID())

	// The completed session stays readable until retention kicks in.
	_, ok := m.Get(first.ID())
	require.True(t, ok)
}

func TestInteractiveSessionManagerRegister(t *testing.T) {
	t.Parallel()

	m := &InteractiveSessionManager{sessions: newSessionMap()}

	session, err := NewInteractiveSession(InteractiveSessionOptions{
		Command:    "sleep 30",
		WorkingDir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Kill(); _ = session.Close() })

	require.Empty(t, session.ID())
	require.NoError(t, m.Register(session))
	require.NotEmpty(t, session.ID())
	require.Equal(t, []string{session.ID()}, m.List())
}

func TestInteractiveSessionManagerCleanupDropsExitedOverCap(t *testing.T) {
	t.Parallel()

	m := &InteractiveSessionManager{sessions: newSessionMap()}

	// Fill past the cap with sessions that exit immediately.
	for range MaxInteractiveSessions + 2 {
		session, err := m.Start(InteractiveSessionOptions{Command: "true", WorkingDir: t.TempDir()})
		require.NoError(t, err)
		select {
		case <-session.Done():
		case <-time.After(15 * time.Second):
			t.Fatal("session did not exit")
		}
	}

	require.LessOrEqual(t, len(m.List()), MaxInteractiveSessions)
}
