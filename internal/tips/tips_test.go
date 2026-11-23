package tips

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaimOnlyFirstCallerWins(t *testing.T) {
	t.Parallel()

	s := Open(filepath.Join(t.TempDir(), "tips.json"))
	require.False(t, s.Seen(Permissions))

	ok, err := s.Claim(Permissions)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, s.Seen(Permissions))

	ok, err = s.Claim(Permissions)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestClaimPersistsAcrossOpens(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "tips.json")
	s := Open(path)
	_, err := s.Claim(Folding)
	require.NoError(t, err)

	reopened := Open(path)
	require.True(t, reopened.Seen(Folding))
	require.False(t, reopened.Seen(Permissions))
}

func TestCorruptFileIsTreatedAsEmpty(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "tips.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))

	s := Open(path)
	require.False(t, s.Seen(Permissions))
}

func TestNilStoreNeverShowsTips(t *testing.T) {
	t.Parallel()

	var s *Store
	require.True(t, s.Seen(Permissions))
	ok, err := s.Claim(Permissions)
	require.NoError(t, err)
	require.False(t, ok)
}
