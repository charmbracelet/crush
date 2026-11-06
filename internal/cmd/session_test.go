package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionSetupFromSubdirectoryUsesProjectDataDir(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")

	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	gitInit := exec.CommandContext(t.Context(), "git", "init", "-q")
	gitInit.Dir = root
	require.NoError(t, gitInit.Run())
	require.NoError(t, os.Mkdir(filepath.Join(root, ".crush"), 0o700))
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))
	t.Chdir(sub)

	sessionListCmd.SetContext(t.Context())
	_, svc, cleanup, err := sessionSetup(sessionListCmd)
	require.NoError(t, err)
	defer cleanup()

	require.Equal(t, filepath.Join(root, ".crush"), svc.cfg.Config().Options.DataDirectory)
	require.NoDirExists(t, filepath.Join(sub, ".crush"))
}
