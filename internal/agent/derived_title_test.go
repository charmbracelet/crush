package agent

import (
	"context"
	"errors"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

type titleFailureModel struct {
	fastModel
	beforeStream func()
}

func (m titleFailureModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.beforeStream()
	return nil, errors.New("title provider unavailable")
}

func TestGenerateTitleSavesPromptBeforeModelAndKeepsItOnFailure(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), DefaultSessionName)
	require.NoError(t, err)
	calls := 0
	model := titleFailureModel{beforeStream: func() {
		calls++
		saved, err := env.sessions.Get(t.Context(), sess.ID)
		require.NoError(t, err)
		require.Equal(t, "Fix login bug", saved.Title)
	}}
	sa := testSessionAgent(env, model, model, "system")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	sa.GenerateTitle(ctx, sess.ID, "  Fix\nlogin bug  ")
	require.Positive(t, calls)
	saved, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, session.DerivedTitle("Fix login bug"), saved.Title)
}
