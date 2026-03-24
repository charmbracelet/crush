package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestResolveCwdFlagReturnsResolvedPath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "workspace")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	t.Chdir(tmpDir)

	cmd := &cobra.Command{}
	cmd.Flags().String("cwd", "", "")
	require.NoError(t, cmd.Flags().Set("cwd", "workspace"))

	resolved, err := ResolveCwd(cmd)
	require.NoError(t, err)
	require.Equal(t, subDir, resolved)
}

func TestResolveCwdWithoutFlagReturnsCurrentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	cmd := &cobra.Command{}
	cmd.Flags().String("cwd", "", "")

	resolved, err := ResolveCwd(cmd)
	require.NoError(t, err)
	require.Equal(t, tmpDir, resolved)
}
