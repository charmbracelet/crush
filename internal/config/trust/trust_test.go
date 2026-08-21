package trust

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrustStore_TrustAndVerify(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := &TrustStore{
		path: filepath.Join(dir, "trusted_configs.json"),
		data: make(map[string]string),
	}

	cfgPath := filepath.Join(dir, "crush.json")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`{"providers":{}}`), 0o644))

	require.False(t, store.IsTrusted(cfgPath))

	require.NoError(t, store.Trust(cfgPath))
	require.True(t, store.IsTrusted(cfgPath))

	require.NoError(t, store.Save())
	require.NoError(t, store.Load())
	require.True(t, store.IsTrusted(cfgPath))
}

func TestTrustStore_ModifiedFileUntrusted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := &TrustStore{
		path: filepath.Join(dir, "trusted_configs.json"),
		data: make(map[string]string),
	}

	cfgPath := filepath.Join(dir, "crush.json")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`{"providers":{}}`), 0o644))
	require.NoError(t, store.Trust(cfgPath))
	require.True(t, store.IsTrusted(cfgPath))

	require.NoError(t, os.WriteFile(cfgPath, []byte(`{"providers":{"openai":{}}}`), 0o644))
	require.False(t, store.IsTrusted(cfgPath))
}

func TestTrustStore_Untrust(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := &TrustStore{
		path: filepath.Join(dir, "trusted_configs.json"),
		data: make(map[string]string),
	}

	cfgPath := filepath.Join(dir, "crush.json")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`{}`), 0o644))
	require.NoError(t, store.Trust(cfgPath))
	require.True(t, store.IsTrusted(cfgPath))

	store.Untrust(cfgPath)
	require.False(t, store.IsTrusted(cfgPath))
}

func TestTrustStore_UntrustedPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := &TrustStore{
		path: filepath.Join(dir, "trusted_configs.json"),
		data: make(map[string]string),
	}

	trusted := filepath.Join(dir, "trusted.json")
	untrusted := filepath.Join(dir, "untrusted.json")
	require.NoError(t, os.WriteFile(trusted, []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(untrusted, []byte(`{}`), 0o644))
	require.NoError(t, store.Trust(trusted))

	result := store.UntrustedPaths([]string{trusted, untrusted})
	require.Len(t, result, 1)
	require.Equal(t, untrusted, result[0])
}

func TestTrustStore_LoadMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := &TrustStore{
		path: filepath.Join(dir, "nonexistent.json"),
		data: make(map[string]string),
	}

	require.NoError(t, store.Load())
	require.Empty(t, store.data)
}

func TestTrustStore_LoadCorruptFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))

	store := &TrustStore{
		path: path,
		data: make(map[string]string),
	}

	require.NoError(t, store.Load())
	require.Empty(t, store.data)
}
