package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestResolveCwdReturnsAbsolutePath guards a relative --cwd being sent to
// the server, which resolves it against its own working directory.
func TestResolveCwdReturnsAbsolutePath(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "proj", "sub"), 0o755))
	for flag, want := range map[string]string{"sub": "proj/sub", "..": "."} {
		t.Chdir(filepath.Join(root, "proj"))
		cmd := &cobra.Command{}
		cmd.Flags().String("cwd", "", "")
		require.NoError(t, cmd.Flags().Set("cwd", flag))
		got, err := ResolveCwd(cmd)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, filepath.FromSlash(want)), got)

		again, err := ResolveCwd(cmd)
		require.NoError(t, err)
		require.Equal(t, got, again, "a second call must not re-resolve the relative flag")
	}
}

func TestClientServerModeEnvDecides(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("CRUSH_SERVER_IDLE_TIMEOUT", "")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crush.json"),
		[]byte(`{"options":{"experimental":{"client_server":true,"server_idle_timeout":7}}}`), 0o644))

	cmd := &cobra.Command{}
	cmd.Flags().String("cwd", "", "")

	t.Setenv("CRUSH_CLIENT_SERVER", "false")
	on, store, err := clientServerMode(cmd)
	require.NoError(t, err)
	require.False(t, on, "the environment must win over the config")
	require.Nil(t, store)
	require.Empty(t, os.Getenv("CRUSH_SERVER_IDLE_TIMEOUT"))

	t.Setenv("CRUSH_CLIENT_SERVER", "true")
	on, store, err = clientServerMode(cmd)
	require.NoError(t, err)
	require.True(t, on)
	require.Nil(t, store)
	require.Equal(t, "7", os.Getenv("CRUSH_SERVER_IDLE_TIMEOUT"))
}
