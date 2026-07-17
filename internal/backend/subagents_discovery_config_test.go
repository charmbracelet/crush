package backend

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestSubagentsDiscoveryConfig_FieldPassthrough verifies that
// subagentsDiscoveryConfig copies SubagentsPaths and DisabledSubagents
// straight from the store's Options, and that IsKnownModel is always
// populated.
func TestSubagentsDiscoveryConfig_FieldPassthrough(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Options: &config.Options{
			SubagentsPaths:    []string{"/a", "/b"},
			DisabledSubagents: []string{"disabled-agent"},
		},
	}
	store := config.NewTestStore(cfg)

	got := subagentsDiscoveryConfig(store)

	require.Equal(t, cfg.Options.SubagentsPaths, got.SubagentsPaths)
	require.Equal(t, cfg.Options.DisabledSubagents, got.DisabledSubagents)
	require.NotNil(t, got.IsKnownModel)
}

// TestSubagentsDiscoveryConfig_NilSafety verifies that
// subagentsDiscoveryConfig does not panic for a store with nil Options
// and no configured resolver, and that the resolver and path fields
// come back empty.
func TestSubagentsDiscoveryConfig_NilSafety(t *testing.T) {
	t.Parallel()

	store := config.NewTestStore(&config.Config{})

	require.NotPanics(t, func() {
		result := subagentsDiscoveryConfig(store)
		require.Empty(t, result.SubagentsPaths)
		require.Empty(t, result.DisabledSubagents)
		require.Nil(t, result.Resolver)
	})
}

// TestSubagentsDiscoveryConfig_ResolverExpandsEnvVar verifies that, for a
// store produced by config.Init (which configures a resolver),
// subagentsDiscoveryConfig's Resolver expands a $VAR-style reference
// against the live process environment.
func TestSubagentsDiscoveryConfig_ResolverExpandsEnvVar(t *testing.T) {
	// Isolate config.Init's filesystem reads from the host, matching
	// backend_skills_test.go's setup for the skills equivalent.
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

	got := subagentsDiscoveryConfig(store)
	require.NotNil(t, got.Resolver)

	resolved, err := got.Resolver("$CRUSH_TEST_SUBAGENT_VAR")
	require.NoError(t, err)
	require.Equal(t, varDir, resolved)
}
