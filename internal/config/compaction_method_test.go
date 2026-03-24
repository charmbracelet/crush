package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetCompactionMethod(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.setDefaults(dir, "")
	store := testStoreWithPath(cfg, dir)

	err := store.SetCompactionMethod(ScopeGlobal, CompactionLLM)
	require.NoError(t, err)

	require.Equal(t, CompactionLLM, cfg.Options.CompactionMethod)

	out := readConfigJSON(t, store.globalDataPath)
	opts, ok := out["options"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, string(CompactionLLM), opts["compaction_method"])
}

func TestSetDefaults_NormalizesCompactionMethodZeroValue(t *testing.T) {
	t.Parallel()

	cfg := &Config{Options: &Options{}}
	cfg.setDefaults(t.TempDir(), "")

	require.Equal(t, CompactionAuto, cfg.Options.CompactionMethod)
}

func TestSetDefaults_PreservesExplicitCompactionMethod(t *testing.T) {
	t.Parallel()

	cfg := &Config{Options: &Options{CompactionMethod: CompactionLLM}}
	cfg.setDefaults(t.TempDir(), "")

	require.Equal(t, CompactionLLM, cfg.Options.CompactionMethod)
}

func TestSetCompactionMethod_NilOptions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &Config{}
	cfg.Options = nil
	store := testStoreWithPath(cfg, dir)

	err := store.SetCompactionMethod(ScopeGlobal, CompactionAuto)
	require.NoError(t, err)

	require.NotNil(t, cfg.Options)
	require.Equal(t, CompactionAuto, cfg.Options.CompactionMethod)
}
