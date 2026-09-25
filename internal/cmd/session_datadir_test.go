package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// TestSessionSetupFromSubdirectoryUsesProjectDataDir mirrors the fix in
// upstream PR #3936: session commands initialized the config with an
// empty working directory, so running from a project subdirectory opened
// a new empty database there instead of the project's. The session data
// dir must resolve from the project root either way.
func TestSessionSetupFromSubdirectoryUsesProjectDataDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("CRUSH_CLIENT_SERVER", "")

	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".crush"), 0o755))
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.Chdir(sub))

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().String("data-dir", "", "")
	cmd.Flags().StringP("cwd", "c", "", "Current working directory")
	require.NoError(t, cmd.Flags().Set("cwd", root))

	_, svc, cleanup, err := sessionSetup(cmd)
	require.NoError(t, err)
	defer cleanup()

	dataDir := svc.cfg.Config().Options.DataDirectory
	require.Equal(t, filepath.Join(root, ".crush"), dataDir,
		"session commands must use the project data dir, not the cwd")

	_, err = os.Stat(filepath.Join(sub, ".crush"))
	require.True(t, os.IsNotExist(err), "no .crush may be created in the subdirectory")
}
