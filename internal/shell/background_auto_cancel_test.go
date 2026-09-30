package shell

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A shell that was moved to the background automatically belongs to the run
// that started it: cancelling that session must terminate it instead of
// letting it run unbounded (#3878). Explicitly backgrounded jobs and other
// sessions' shells are left alone.
func TestKillAutoBackgroundedForSession(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	autoShell, err := manager.Start(ctx, workingDir, nil, "sleep 30", "")
	require.NoError(t, err)
	explicitShell, err := manager.Start(ctx, workingDir, nil, "sleep 30", "")
	require.NoError(t, err)
	otherSessionShell, err := manager.Start(ctx, workingDir, nil, "sleep 30", "")
	require.NoError(t, err)

	require.True(t, manager.MarkAutoBackgrounded(autoShell.ID, "session-a"))
	// Unknown and already-finished shells cannot be tracked as auto-backgrounded.
	require.False(t, manager.MarkAutoBackgrounded("missing", "session-a"))

	killed := manager.KillAutoBackgroundedForSession("session-a")
	require.Equal(t, 1, killed)
	require.True(t, autoShell.IsDone())
	require.False(t, explicitShell.IsDone())
	require.False(t, otherSessionShell.IsDone())

	// The shell is gone from the tracking map, so a second cancel is a no-op.
	require.Equal(t, 0, manager.KillAutoBackgroundedForSession("session-a"))
	require.Equal(t, 0, manager.KillAutoBackgroundedForSession("session-b"))

	manager.KillAll(t.Context())
}

func TestMarkAutoBackgroundedRejectsFinishedShell(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	bgShell, err := manager.Start(ctx, workingDir, nil, "true", "")
	require.NoError(t, err)
	bgShell.Wait()

	require.False(t, manager.MarkAutoBackgrounded(bgShell.ID, "session-a"))
	require.Equal(t, 0, manager.KillAutoBackgroundedForSession("session-a"))
}
