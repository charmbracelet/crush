package prompt

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// isolateSkillEnv points HOME/XDG/CRUSH_* at a throwaway directory so the
// config loader only sees the files the test writes.
func isolateSkillEnv(t *testing.T) (workDir, dataDir string) {
	t.Helper()
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))
	return t.TempDir(), t.TempDir()
}

// writeSkillDir creates a minimal valid skill in skillsDir.
func writeSkillDir(t *testing.T, skillsDir, name string) {
	t.Helper()
	dir := filepath.Join(skillsDir, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content := "---\nname: " + name +
		"\ndescription: Test skill " + name + ".\n---\nSay ok.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))
}

// newSkillPromptStore loads an isolated config with two skills, one of them
// disabled globally, and returns the store plus a Prompt using the given
// repository-scoped overrides.
func newSkillPromptStore(t *testing.T, repoDisabled, repoEnabled []string) (string, *Prompt, *config.ConfigStore) {
	t.Helper()
	workDir, dataDir := isolateSkillEnv(t)

	skillsDir := filepath.Join(workDir, ".crush", "skills")
	writeSkillDir(t, skillsDir, "toggle-me")
	writeSkillDir(t, skillsDir, "always-on")

	rc := "option disable-skill toggle-me\n"
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "crushrc"), []byte(rc), 0o644))

	store, err := config.Load(t.Context(), workDir, dataDir, false)
	require.NoError(t, err)

	overrides := func(disabled, enabled []string) (func(ctx context.Context) []string, func(ctx context.Context) []string) {
		return func(ctx context.Context) []string {
				return disabled
			}, func(ctx context.Context) []string {
				return enabled
			}
	}
	p, err := NewPrompt("test", "Skills: {{.AvailSkillXML}}",
		WithSkillOverrides(overrides(repoDisabled, repoEnabled)))
	require.NoError(t, err)
	return workDir, p, store
}

// TestPromptDataRepoEnabledOverrideRestoresConfigDisabledSkill verifies that
// a repository-scoped enabled override brings back a globally disabled skill.
func TestPromptDataRepoEnabledOverrideRestoresConfigDisabledSkill(t *testing.T) {
	_, p, store := newSkillPromptStore(t, nil, []string{"toggle-me"})

	out, err := p.Build(t.Context(), "test", "test", store)
	require.NoError(t, err)
	require.Contains(t, out, "toggle-me",
		"repo-enabled override must restore a config-disabled skill")
	require.Contains(t, out, "always-on")
}

// TestPromptDataRepoDisabledSkillIsHidden verifies that a repository-scoped
// disable hides a skill that the global config leaves enabled.
func TestPromptDataRepoDisabledSkillIsHidden(t *testing.T) {
	_, p, store := newSkillPromptStore(t, []string{"always-on"}, nil)

	out, err := p.Build(t.Context(), "test", "test", store)
	require.NoError(t, err)
	require.NotContains(t, out, "always-on",
		"repo-disabled override must hide an otherwise enabled skill")
	require.NotContains(t, out, "toggle-me",
		"config-disabled skills stay hidden without an enabled override")
}
