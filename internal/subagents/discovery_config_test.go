package subagents

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

// TestDiscoveryConfigFromStore_FieldPassthrough verifies that
// DiscoveryConfigFromStore copies SubagentsPaths and DisabledSubagents
// straight from the store's Options, and that IsKnownModel is always
// populated.
func TestDiscoveryConfigFromStore_FieldPassthrough(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Options: &config.Options{
			SubagentsPaths:    []string{"/a", "/b"},
			DisabledSubagents: []string{"disabled-agent"},
		},
	}
	store := config.NewTestStore(cfg)

	got := DiscoveryConfigFromStore(store, nil)

	require.Equal(t, cfg.Options.SubagentsPaths, got.SubagentsPaths)
	require.Equal(t, cfg.Options.DisabledSubagents, got.DisabledSubagents)
	require.NotNil(t, got.IsKnownModel)
	require.Nil(t, got.IsKnownSkill, "no skills manager means skill validation is skipped")
}

// TestDiscoveryConfigFromStore_NilSafety verifies that
// DiscoveryConfigFromStore does not panic for a store with nil Options and no
// configured resolver, and that the resolver and path fields come back empty.
func TestDiscoveryConfigFromStore_NilSafety(t *testing.T) {
	t.Parallel()

	store := config.NewTestStore(&config.Config{})

	require.NotPanics(t, func() {
		result := DiscoveryConfigFromStore(store, nil)
		require.Empty(t, result.SubagentsPaths)
		require.Empty(t, result.DisabledSubagents)
		require.Nil(t, result.Resolver)
	})
}

// TestDiscoveryConfigFromStore_ResolverExpandsEnvVar verifies that, for a
// store produced by config.Init (which configures a resolver), the returned
// Resolver expands a $VAR-style reference against the live process
// environment.
func TestDiscoveryConfigFromStore_ResolverExpandsEnvVar(t *testing.T) {
	// Isolate config.Init's filesystem reads from the host.
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

	got := DiscoveryConfigFromStore(store, nil)
	require.NotNil(t, got.Resolver)

	resolved, err := got.Resolver("$CRUSH_TEST_SUBAGENT_VAR")
	require.NoError(t, err)
	require.Equal(t, varDir, resolved)
}

// TestDiscoveryConfigFromStore_KnownSkillFromManager verifies that the
// IsKnownSkill check reflects the skills manager's active set: active skill
// names pass, anything else fails.
func TestDiscoveryConfigFromStore_KnownSkillFromManager(t *testing.T) {
	t.Parallel()

	mgr := skills.NewManager(
		[]*skills.Skill{{Name: "pdf-processing"}},
		[]*skills.Skill{{Name: "pdf-processing"}},
		nil,
	)
	t.Cleanup(mgr.Shutdown)

	got := DiscoveryConfigFromStore(config.NewTestStore(&config.Config{}), mgr)
	require.NotNil(t, got.IsKnownSkill)
	require.True(t, got.IsKnownSkill("pdf-processing"))
	require.False(t, got.IsKnownSkill("nonexistent-skill"))
}
