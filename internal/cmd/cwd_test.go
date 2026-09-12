package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func cmdWithCwd(t testing.TB, cwd string) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().StringP("cwd", "c", "", "Current working directory")
	if cwd != "" {
		require.NoError(t, cmd.Flags().Set("cwd", cwd))
	}
	return cmd
}

// TestResolveClientCwd_NoLocalDirectoryRequired is the thin-client
// guarantee: --cwd names a path on the SERVER, so resolution must not
// fail when the directory does not exist on the client machine (and
// must not chdir the client process either).
func TestResolveClientCwd_NoLocalDirectoryRequired(t *testing.T) {
	t.Parallel()

	before, err := os.Getwd()
	require.NoError(t, err)

	cwd, err := resolveClientCwd(cmdWithCwd(t, "/definitely/not/here/proj"))
	require.NoError(t, err, "a server-side path must not require local existence")
	require.Equal(t, "/definitely/not/here/proj", cwd)

	after, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, before, after, "the client process must not be chdirred")
}

// TestResolveClientCwd_RelativeMadeAbsolute verifies a relative --cwd is
// absolutized against the client's process cwd before being sent to the
// server, so the server never resolves it against its own cwd.
func TestResolveClientCwd_RelativeMadeAbsolute(t *testing.T) {
	t.Parallel()

	cwd, err := resolveClientCwd(cmdWithCwd(t, "some/rel/path"))
	require.NoError(t, err)
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(wd, "some/rel/path"), cwd)
}

// TestResolveClientCwd_DefaultsToProcessCwd verifies the no-flag case.
func TestResolveClientCwd_DefaultsToProcessCwd(t *testing.T) {
	t.Parallel()

	cwd, err := resolveClientCwd(cmdWithCwd(t, ""))
	require.NoError(t, err)
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, wd, cwd)
}

// TestResolveCwd_StillRequiresLocalDirectory guards the local-mode
// resolver: its chdir semantics are load-bearing for local workspaces,
// so a missing directory must stay an error there.
func TestResolveCwd_StillRequiresLocalDirectory(t *testing.T) {
	t.Parallel()

	_, err := ResolveCwd(cmdWithCwd(t, "/definitely/not/here/proj"))
	require.Error(t, err, "local mode resolves --cwd through the filesystem")
}
