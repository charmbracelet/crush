package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetMemoryDefaults(t *testing.T) {
	t.Setenv("CRUSH_MEMORY_DIR", "")

	options := &MemoryOptions{Directory: "custom-memory"}
	setMemoryDefaults(options)

	require.NotNil(t, options.Enabled)
	require.True(t, *options.Enabled)
	require.NotNil(t, options.RecorderEnabled)
	require.True(t, *options.RecorderEnabled)
	require.NotNil(t, options.RecallEnabled)
	require.True(t, *options.RecallEnabled)
	require.Equal(t, 0.88, options.AutoApproveConfidence)
	require.Equal(t, 5, options.MaxRecall)
	require.Equal(t, 80, options.MaxIndexEntries)
	require.Equal(t, 5, options.MaxBackups)
	require.False(t, options.DisableOnExternalContext)
	require.Equal(t, filepath.Join(filepath.Dir(GlobalConfig()), "custom-memory"), options.Directory)
}

func TestSetMemoryDefaultsUsesTrustedEnvironmentOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CRUSH_MEMORY_DIR", dir)

	options := &MemoryOptions{Directory: `C:\untrusted-project-config`}
	setMemoryDefaults(options)

	require.Equal(t, filepath.Clean(dir), options.Directory)
}
