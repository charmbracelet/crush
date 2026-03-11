package shell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewShell_DetachedSetsNonInteractiveEnv(t *testing.T) {
	t.Parallel()

	shell := NewShell(&Options{
		WorkingDir: t.TempDir(),
		Detached:   true,
		Env: []string{
			"PATH=/tmp/bin",
			"TERM=xterm-256color",
		},
	})

	env := shell.GetEnv()
	require.Contains(t, env, "CRUSH_BACKGROUND_JOB=1")
	require.Contains(t, env, "TERM=dumb")
}

func TestNewShell_DetachedReplacesTermOnce(t *testing.T) {
	t.Parallel()

	shell := NewShell(&Options{
		WorkingDir: t.TempDir(),
		Detached:   true,
		Env: []string{
			"PATH=/tmp/bin",
			"TERM=screen",
		},
	})

	termCount := 0
	for _, entry := range shell.GetEnv() {
		if strings.HasPrefix(entry, "TERM=") {
			termCount++
		}
	}

	require.Equal(t, 1, termCount)
}

func TestBackgroundShellManager_StartUsesDetachedShell(t *testing.T) {
	t.Parallel()

	manager := newBackgroundShellManager()

	bgShell, err := manager.Start(t.Context(), "session-123", t.TempDir(), nil, "echo hello", "")
	require.NoError(t, err)

	require.True(t, bgShell.Shell.detached)
	require.Equal(t, "session-123", bgShell.SessionID)
	require.Contains(t, bgShell.Shell.GetEnv(), "CRUSH_BACKGROUND_JOB=1")

	require.NoError(t, manager.Kill(bgShell.ID))
}
