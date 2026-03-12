package shell

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewShell_PreservesProvidedEnv(t *testing.T) {
	t.Parallel()

	shell := NewShell(&Options{
		WorkingDir: t.TempDir(),
		Env: []string{
			"PATH=/tmp/bin",
			"TERM=xterm-256color",
		},
	})

	env := shell.GetEnv()
	require.Contains(t, env, "PATH=/tmp/bin")
	require.Contains(t, env, "TERM=xterm-256color")
}

func TestNewShell_SetEnvReplacesExistingEntry(t *testing.T) {
	t.Parallel()

	shell := NewShell(&Options{
		WorkingDir: t.TempDir(),
		Env: []string{
			"PATH=/tmp/bin",
			"TERM=screen",
		},
	})

	shell.SetEnv("TERM", "dumb")
	require.ElementsMatch(t, []string{"PATH=/tmp/bin", "TERM=dumb"}, shell.GetEnv())
}

func TestBackgroundShellManager_StartUsesCurrentAPI(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	manager := newBackgroundShellManager()

	bgShell, err := manager.Start(t.Context(), workingDir, nil, "echo hello", "")
	require.NoError(t, err)

	require.Equal(t, workingDir, bgShell.WorkingDir)
	require.Equal(t, workingDir, bgShell.Shell.GetWorkingDir())
	require.Equal(t, "echo hello", bgShell.Command)

	bgShell.Wait()
	require.NoError(t, manager.Kill(bgShell.ID))
}
