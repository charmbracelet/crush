package model

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// sessionWorkspace extends testWorkspace with a controllable GetSession.
type sessionWorkspace struct {
	workspace.Workspace
	getSession func(ctx context.Context, id string) (session.Session, error)
}

func (w *sessionWorkspace) Config() *config.Config { return nil }

func (w *sessionWorkspace) GetSession(ctx context.Context, id string) (session.Session, error) {
	return w.getSession(ctx, id)
}

func newTestUIWithSession(t *testing.T, sess session.Session, getSession func(context.Context, string) (session.Session, error)) *UI {
	t.Helper()
	return &UI{
		com: &common.Common{
			Workspace: &sessionWorkspace{
				getSession: getSession,
			},
		},
		session: &sess,
	}
}

func TestCheckExternalSessionUpdate_DetectsChange(t *testing.T) {
	t.Parallel()

	const sessionID = "test-session-id"
	cached := session.Session{ID: sessionID, UpdatedAt: 100}
	updated := session.Session{ID: sessionID, UpdatedAt: 200}

	ui := newTestUIWithSession(t, cached, func(_ context.Context, id string) (session.Session, error) {
		require.Equal(t, sessionID, id)
		return updated, nil
	})

	cmd := ui.checkExternalSessionUpdate()
	require.NotNil(t, cmd)

	msg := cmd()
	changed, ok := msg.(externalSessionChangedMsg)
	require.True(t, ok, "expected externalSessionChangedMsg, got %T", msg)
	require.Equal(t, sessionID, changed.sessionID)
}

func TestCheckExternalSessionUpdate_NoChangeWhenTimestampUnchanged(t *testing.T) {
	t.Parallel()

	const sessionID = "test-session-id"
	cached := session.Session{ID: sessionID, UpdatedAt: 100}

	ui := newTestUIWithSession(t, cached, func(_ context.Context, _ string) (session.Session, error) {
		return cached, nil
	})

	cmd := ui.checkExternalSessionUpdate()
	require.NotNil(t, cmd)

	msg := cmd()
	require.Nil(t, msg, "expected nil when session timestamp is unchanged")
}

func TestCheckExternalSessionUpdate_NoChangeWhenTimestampGoesBack(t *testing.T) {
	t.Parallel()

	const sessionID = "test-session-id"
	cached := session.Session{ID: sessionID, UpdatedAt: 100}
	older := session.Session{ID: sessionID, UpdatedAt: 50}

	ui := newTestUIWithSession(t, cached, func(_ context.Context, _ string) (session.Session, error) {
		return older, nil
	})

	cmd := ui.checkExternalSessionUpdate()
	require.NotNil(t, cmd)

	msg := cmd()
	require.Nil(t, msg, "expected nil when remote timestamp is not newer than cached")
}

func TestCheckExternalSessionUpdate_SilentOnError(t *testing.T) {
	t.Parallel()

	const sessionID = "test-session-id"
	cached := session.Session{ID: sessionID, UpdatedAt: 100}

	ui := newTestUIWithSession(t, cached, func(_ context.Context, _ string) (session.Session, error) {
		return session.Session{}, errors.New("db unavailable")
	})

	cmd := ui.checkExternalSessionUpdate()
	require.NotNil(t, cmd)

	msg := cmd()
	require.Nil(t, msg, "expected nil when GetSession returns an error")
}
