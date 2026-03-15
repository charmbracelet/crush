package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClampToWorkingDir(t *testing.T) {
	t.Parallel()

	t.Run("allows path within working dir", func(t *testing.T) {
		t.Parallel()
		workDir := t.TempDir()
		result, err := clampToWorkingDir(workDir, workDir+"/subdir")
		require.NoError(t, err)
		require.Equal(t, workDir+"/subdir", result)
	})

	t.Run("allows working dir itself", func(t *testing.T) {
		t.Parallel()
		workDir := t.TempDir()
		result, err := clampToWorkingDir(workDir, workDir)
		require.NoError(t, err)
		require.Equal(t, workDir, result)
	})

	t.Run("rejects root path", func(t *testing.T) {
		t.Parallel()
		workDir := t.TempDir()
		_, err := clampToWorkingDir(workDir, "/")
		require.Error(t, err)
		require.Contains(t, err.Error(), "outside the working directory")
	})

	t.Run("rejects parent traversal", func(t *testing.T) {
		t.Parallel()
		workDir := t.TempDir()
		_, err := clampToWorkingDir(workDir, workDir+"/../..")
		require.Error(t, err)
		require.Contains(t, err.Error(), "outside the working directory")
	})

	t.Run("rejects unrelated absolute path", func(t *testing.T) {
		t.Parallel()
		workDir := t.TempDir()
		_, err := clampToWorkingDir(workDir, "/etc")
		require.Error(t, err)
		require.Contains(t, err.Error(), "outside the working directory")
	})
}
