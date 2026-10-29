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
	}
}
