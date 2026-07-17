package cmd

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestLocalSubagentsDiscoveryConfig_FieldPassthrough verifies that
// localSubagentsDiscoveryConfig copies SubagentsPaths and
// DisabledSubagents straight from the store's Options, and that
// IsKnownModel is always populated.
func TestLocalSubagentsDiscoveryConfig_FieldPassthrough(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Options: &config.Options{
			SubagentsPaths:    []string{"/a", "/b"},
			DisabledSubagents: []string{"disabled-agent"},
		},
	}
	store := config.NewTestStore(cfg)

	got := localSubagentsDiscoveryConfig(store)

	require.Equal(t, cfg.Options.SubagentsPaths, got.SubagentsPaths)
	require.Equal(t, cfg.Options.DisabledSubagents, got.DisabledSubagents)
	require.NotNil(t, got.IsKnownModel)
}

// TestLocalSubagentsDiscoveryConfig_NilSafety verifies that
// localSubagentsDiscoveryConfig does not panic for a store with nil
// Options and no configured resolver, and that the resolver and path
// fields come back empty.
func TestLocalSubagentsDiscoveryConfig_NilSafety(t *testing.T) {
	t.Parallel()

	store := config.NewTestStore(&config.Config{})

	require.NotPanics(t, func() {
		result := localSubagentsDiscoveryConfig(store)
		require.Empty(t, result.SubagentsPaths)
		require.Empty(t, result.DisabledSubagents)
		require.Nil(t, result.Resolver)
	})
}

// TestLocalSubagentsDiscoveryConfig_ResolverExpandsEnvVar verifies that,
// for a store produced by config.Init (which configures a resolver),
// localSubagentsDiscoveryConfig's Resolver expands a $VAR-style
// reference against the live process environment.
func TestLocalSubagentsDiscoveryConfig_ResolverExpandsEnvVar(t *testing.T) {
	// Isolate config.Init's filesystem reads from the host, matching the
	// backend package's skills-discovery test setup.
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(hostHome, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(hostHome, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(hostHome, ".cache"))
	t.Setenv("CRUSH_SKILLS_DIR", t.TempDir())
	t.Setenv("CRUSH_SUBAGENTS_DIR", t.TempDir())

	varDir := t.TempDir()
	t.Setenv("CRUSH_TEST_SUBAGENT_VAR", varDir)

	workingDir := t.TempDir()
	store, err := config.Init(workingDir, "", false)
	require.NoError(t, err)

	got := localSubagentsDiscoveryConfig(store)
	require.NotNil(t, got.Resolver)

	resolved, err := got.Resolver("$CRUSH_TEST_SUBAGENT_VAR")
	require.NoError(t, err)
	require.Equal(t, varDir, resolved)
}
