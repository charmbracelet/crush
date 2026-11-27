package prompt

import (
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

// TestPromptDataRelativeSkillPathResolvesAgainstWorkingDir verifies that a
// relative skills_paths entry resolves against the working directory, not
// the process cwd. The skills live under the temp working dir while the
// test binary runs from the package directory, so a cwd-based resolution
// finds nothing.
func TestPromptDataRelativeSkillPathResolvesAgainstWorkingDir(t *testing.T) {
	workDir, dataDir := isolateSkillEnv(t)

	writeSkillDir(t, filepath.Join(workDir, "skills"), "toggle-me")
	writeSkillDir(t, filepath.Join(workDir, "skills"), "always-on")

	rc := "option skill-path ./skills\n"
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "crushrc"), []byte(rc), 0o644))

	store, err := config.Load(t.Context(), workDir, dataDir, false)
	require.NoError(t, err)

	p, err := NewPrompt("test", "Skills: {{.AvailSkillXML}}")
	require.NoError(t, err)

	out, err := p.Build(t.Context(), "test", "test", store)
	require.NoError(t, err)
	require.Contains(t, out, "toggle-me",
		"relative skills_paths must resolve against the working directory")
	require.Contains(t, out, "always-on")
}
