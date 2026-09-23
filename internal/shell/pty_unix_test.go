//go:build !windows

package shell

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlatformInteractiveShell(t *testing.T) {
	t.Setenv("SHELL", "sh")

	t.Run("empty command starts an interactive shell", func(t *testing.T) {
		path, args := platformInteractiveShell("")
		require.Equal(t, "sh", path)
		require.Equal(t, []string{"-i"}, args)
	})

	t.Run("command runs through -c", func(t *testing.T) {
		path, args := platformInteractiveShell("echo hi")
		require.Equal(t, "sh", path)
		require.Equal(t, []string{"-c", "echo hi"}, args)
	})

	t.Run("falls back to sh", func(t *testing.T) {
		t.Setenv("SHELL", "")
		path, _ := platformInteractiveShell("echo hi")
		require.Equal(t, "sh", path)
	})
}

func TestSignaledExitCode(t *testing.T) {
	t.Parallel()

	_, ok := signaledExitCode(nil)
	require.False(t, ok)

	cmd := exec.Command("sh", "-c", "kill -TERM $$")
	err := cmd.Run()
	require.Error(t, err)

	code, ok := signaledExitCode(err)
	require.True(t, ok)
	require.Equal(t, 128+15, code)
	require.Equal(t, 128+15, InteractiveExitCode(err))
}
