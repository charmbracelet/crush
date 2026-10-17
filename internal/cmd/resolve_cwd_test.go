package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestResolveCwd_RelativeFlagReturnsAbsolutePath guards against returning
// the relative --cwd value after the chdir into it. A relative result
// resolves against the new working directory, so `crush -c sub` put the
// data directory in sub/sub/.crush.
func TestResolveCwd_RelativeFlagReturnsAbsolutePath(t *testing.T) {
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	t.Chdir(tmp)
	require.NoError(t, os.Mkdir("sub", 0o755))

	cmd := &cobra.Command{}
	cmd.Flags().String("cwd", "", "")
	require.NoError(t, cmd.Flags().Set("cwd", "sub"))

	got, err := ResolveCwd(cmd)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(tmp, "sub"), got)

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, got, wd)
}
