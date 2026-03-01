package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetCompactionMethod(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	cfg.dataConfigDir = filepath.Join(dir, "config.json")

	err := cfg.SetCompactionMethod(CompactionLLM)
	require.NoError(t, err)

	require.Equal(t, CompactionLLM, cfg.Options.CompactionMethod)

	out := readConfigJSON(t, cfg.dataConfigDir)
	opts, ok := out["options"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, string(CompactionLLM), opts["compaction_method"])
}

func TestSetCompactionMethod_NilOptions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.Options = nil
	cfg.dataConfigDir = filepath.Join(dir, "config.json")

	err := cfg.SetCompactionMethod(CompactionAuto)
	require.NoError(t, err)

	require.NotNil(t, cfg.Options)
	require.Equal(t, CompactionAuto, cfg.Options.CompactionMethod)
}
