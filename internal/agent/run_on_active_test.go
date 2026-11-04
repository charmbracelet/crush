package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_CallsOnActiveOnceWhenRunBecomesActive(t *testing.T) {
	t.Parallel()
	sa, env := newStreamTestAgent(t)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	var calls int
	var busyWhenCalled bool
	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "hi",
		onActive: func() {
			calls++
			busyWhenCalled = sa.IsSessionBusy(sess.ID)
		},
	})
	require.NoError(t, err)

	require.Equal(t, 1, calls)
	require.True(t, busyWhenCalled, "the session must already count as busy when onActive fires")
}
