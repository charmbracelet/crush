package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectNeedsInitializationRespectsInitializeAsPath(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	dataDir := filepath.Join(workingDir, ".crush")

	require.NoError(t, os.WriteFile(filepath.Join(workingDir, ".gitignore"), []byte("bazel-bin\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "main.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(workingDir, "bazel-bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "bazel-bin", "AGENTS.md"), []byte("# Context\n"), 0o644))

	cfg := &Config{
		Options: &Options{
			InitializeAs: "bazel-bin/AGENTS.md",
		},
	}
	cfg.setDefaults(workingDir, dataDir)

	store := testStore(cfg)
	store.workingDir = workingDir

	needsInitialization, err := ProjectNeedsInitialization(store)
	require.NoError(t, err)
	require.False(t, needsInitialization)
}
