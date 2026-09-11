package filehistory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T) *Store {
	t.Helper()
	if os.Getenv("FILESNAP_INTEGRATION_TESTS") != "1" {
		t.Skip("set FILESNAP_INTEGRATION_TESTS=1 to exercise the managed official executable")
	}
	base := t.TempDir()
	cwd := filepath.Join(base, "workspace")
	require.NoError(t, os.Mkdir(cwd, 0o700))
	return New(cwd, filepath.Join(base, "history"), "")
}

func TestPreimageRecoveryAndResumption(t *testing.T) {
	t.Parallel()
	s := fixture(t)
	binary := filepath.Join(s.Cwd, "asset.bin")
	require.NoError(t, os.WriteFile(binary, []byte{0, 255, 3}, 0o600))
	ctx, release, err := s.Begin(t.Context(), "session")
	require.NoError(t, err)
	hidden := filepath.Join(s.Cwd, ".hidden")
	require.NoError(t, os.WriteFile(hidden, []byte("before"), 0o600))
	created := filepath.Join(s.Cwd, "created.bin")
	require.NoError(t, Declare(ctx, hidden))
	require.NoError(t, Declare(ctx, created))
	require.NoError(t, os.WriteFile(hidden, []byte("after"), 0o600))
	require.NoError(t, os.WriteFile(created, []byte("new"), 0o600))
	require.NoError(t, os.WriteFile(binary, []byte{4, 5}, 0o600))
	_, err = s.Command(t.Context(), "session", "list", "")
	require.Error(t, err, "active turn must exclude external operations")
	release()
	resumed := New(s.Cwd, s.DataDir, s.Binary)
	entries, err := resumed.Command(t.Context(), "session", "list", "")
	require.NoError(t, err)
	target := entries[0]["turn"].(string)
	_, err = resumed.Command(t.Context(), "session", "restore", target)
	require.NoError(t, err)
	actual, err := os.ReadFile(binary)
	require.NoError(t, err)
	require.Equal(t, []byte{0, 255, 3}, actual)
	actual, err = os.ReadFile(hidden)
	require.NoError(t, err)
	require.Equal(t, "before", string(actual))
	_, err = os.Stat(created)
	require.True(t, os.IsNotExist(err))
	_, err = resumed.Command(t.Context(), "session", "redo", "")
	require.NoError(t, err)
	actual, err = os.ReadFile(created)
	require.NoError(t, err)
	require.Equal(t, "new", string(actual))
	_, err = resumed.Command(t.Context(), "session", "redo", "")
	require.NoError(t, err)
	actual, err = os.ReadFile(hidden)
	require.NoError(t, err)
	require.Equal(t, "before", string(actual))
	_, err = resumed.Command(t.Context(), "other", "restore", target)
	require.ErrorContains(t, err, "does not belong")
}

func TestFailedCaptureReleasesWorkspaceAndRefusesFalseAbsence(t *testing.T) {
	t.Parallel()
	s := fixture(t)
	broken := New(s.Cwd, s.DataDir, filepath.Join(t.TempDir(), "missing"))
	_, _, err := broken.Begin(t.Context(), "session")
	require.Error(t, err)
	ctx, release, err := s.Begin(t.Context(), "session")
	require.NoError(t, err)
	defer release()
	directory := filepath.Join(s.Cwd, "directory")
	require.NoError(t, os.Mkdir(directory, 0o700))
	require.Error(t, Declare(ctx, directory))
}

func TestStoreInsideWorkspaceIsRefused(t *testing.T) {
	t.Parallel()
	s := fixture(t)
	s.DataDir = filepath.Join(s.Cwd, "history")
	_, _, err := s.Begin(t.Context(), "session")
	require.ErrorContains(t, err, "outside the workspace")
}

func TestDeletePreservesAnotherSessionsHistory(t *testing.T) {
	t.Parallel()
	s := fixture(t)
	path := filepath.Join(s.Cwd, "asset.bin")
	require.NoError(t, os.WriteFile(path, []byte{0, 255, 7}, 0o600))
	_, release, err := s.Begin(t.Context(), "first")
	require.NoError(t, err)
	release()
	_, release, err = s.Begin(t.Context(), "second")
	require.NoError(t, err)
	release()
	entries, err := s.Command(t.Context(), "second", "list", "")
	require.NoError(t, err)
	target := entries[0]["turn"].(string)
	_, err = s.Command(t.Context(), "first", "delete", "")
	require.NoError(t, err)
	entries, err = s.Command(t.Context(), "first", "list", "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "log.done", entries[0]["type"])
	require.NoError(t, os.WriteFile(path, []byte("changed"), 0o600))
	_, err = s.Command(t.Context(), "second", "restore", target)
	require.NoError(t, err)
	actual, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, []byte{0, 255, 7}, actual)
	_, err = s.Command(t.Context(), "first", "delete", "")
	require.NoError(t, err)
}
