package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGlobalSubagentsDirs(t *testing.T) {
	t.Parallel()

	dirs := GlobalSubagentsDirs()

	t.Run("returns non-empty slice", func(t *testing.T) {
		t.Parallel()
		require.NotEmpty(t, dirs)
	})

	t.Run("contains path ending with crush/subagents or .config/crush/subagents", func(t *testing.T) {
		t.Parallel()
		found := false
		for _, d := range dirs {
			if strings.HasSuffix(d, filepath.Join("crush", "subagents")) ||
				strings.HasSuffix(d, filepath.Join(".config", "crush", "subagents")) {
				found = true
				break
			}
		}
		require.True(t, found, "expected a path ending with crush/subagents or .config/crush/subagents; got %v", dirs)
	})

	t.Run("contains path ending with .agents/subagents", func(t *testing.T) {
		t.Parallel()
		found := false
		for _, d := range dirs {
			if strings.HasSuffix(d, filepath.Join(".agents", "subagents")) {
				found = true
				break
			}
		}
		require.True(t, found, "expected a path ending with .agents/subagents; got %v", dirs)
	})

	t.Run("does not contain .claude paths", func(t *testing.T) {
		t.Parallel()
		for _, d := range dirs {
			require.False(t, strings.Contains(d, ".claude"),
				"global subagents dirs must not include .claude paths; got %q", d)
		}
	})

	t.Run("all paths are absolute", func(t *testing.T) {
		t.Parallel()
		for _, d := range dirs {
			require.True(t, filepath.IsAbs(d), "expected absolute path, got %q", d)
		}
	})
}

func TestProjectSubagentsDir(t *testing.T) {
	t.Parallel()

	workingDir := "/some/project"
	dirs := ProjectSubagentsDir(workingDir)

	t.Run("contains .agents/subagents under workingDir", func(t *testing.T) {
		t.Parallel()
		require.Contains(t, dirs, filepath.Join(workingDir, ".agents", "subagents"))
	})

	t.Run("contains .crush/subagents under workingDir", func(t *testing.T) {
		t.Parallel()
		require.Contains(t, dirs, filepath.Join(workingDir, ".crush", "subagents"))
	})

	t.Run("does not contain .claude paths", func(t *testing.T) {
		t.Parallel()
		for _, d := range dirs {
			require.False(t, strings.Contains(d, ".claude"),
				"project subagents dirs must not include .claude paths; got %q", d)
		}
	})

	t.Run("does not contain skills paths", func(t *testing.T) {
		t.Parallel()
		for _, d := range dirs {
			require.False(t, strings.Contains(d, "/skills/") || strings.HasSuffix(d, "/skills"),
				"subagents path must not contain a skills segment; got %q", d)
		}
	})

	t.Run("all paths are absolute", func(t *testing.T) {
		t.Parallel()
		for _, d := range dirs {
			require.True(t, filepath.IsAbs(d), "expected absolute path, got %q", d)
		}
	})
}

func TestSetDefaults_SubagentsPathsPopulated(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	cfg.setDefaults("/tmp/workdir", "")

	require.NotEmpty(t, cfg.Options.SubagentsPaths, "SubagentsPaths must be non-empty after setDefaults")

	t.Run("contains a path ending in .agents/subagents", func(t *testing.T) {
		t.Parallel()
		found := false
		for _, p := range cfg.Options.SubagentsPaths {
			if strings.HasSuffix(p, filepath.Join(".agents", "subagents")) {
				found = true
				break
			}
		}
		require.True(t, found, "expected at least one path ending in .agents/subagents; got %v", cfg.Options.SubagentsPaths)
	})

	t.Run("contains a path ending in .crush/subagents", func(t *testing.T) {
		t.Parallel()
		found := false
		for _, p := range cfg.Options.SubagentsPaths {
			if strings.HasSuffix(p, filepath.Join(".crush", "subagents")) {
				found = true
				break
			}
		}
		require.True(t, found, "expected at least one path ending in .crush/subagents; got %v", cfg.Options.SubagentsPaths)
	})

	t.Run("no cross-contamination with skills paths", func(t *testing.T) {
		t.Parallel()
		for _, p := range cfg.Options.SubagentsPaths {
			require.False(t, strings.Contains(p, "/skills/") || strings.HasSuffix(p, "/skills"),
				"SubagentsPaths must not contain a skills segment; got %q", p)
		}
	})
}

func TestSetDefaults_SubagentsPathsNotDuplicated(t *testing.T) {
	t.Parallel()

	preexisting := "/custom/agents/path"
	cfg := &Config{
		Options: &Options{
			SubagentsPaths: []string{preexisting},
		},
	}
	workingDir := t.TempDir()

	cfg.setDefaults(workingDir, "")
	cfg.setDefaults(workingDir, "")

	count := 0
	for _, p := range cfg.Options.SubagentsPaths {
		if p == preexisting {
			count++
		}
	}
	require.Equal(t, 1, count,
		"pre-existing path %q appeared %d times after two setDefaults calls; expected exactly 1",
		preexisting, count)
}

// TestSetDefaults_ProjectSubagentsDirNotDuplicated is the regression test for
// the auto-added project/global dirs specifically (as opposed to a
// pre-existing custom path above): setDefaults runs twice per config reload
// (ConfigStore.reloadFromDiskLocked calls it once on the freshly-loaded
// config and again after merging workspace overrides), and unlike the global
// dirs loop, the project dirs append was unguarded, so every reload duplicated
// them.
func TestSetDefaults_ProjectSubagentsDirNotDuplicated(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	cfg := &Config{Options: &Options{}}

	cfg.setDefaults(workingDir, "")
	firstLen := len(cfg.Options.SubagentsPaths)
	require.NotEmpty(t, firstLen)

	cfg.setDefaults(workingDir, "")
	require.Len(t, cfg.Options.SubagentsPaths, firstLen,
		"a second setDefaults call must not grow SubagentsPaths")

	seen := make(map[string]int)
	for _, p := range cfg.Options.SubagentsPaths {
		seen[p]++
	}
	for p, count := range seen {
		require.Equal(t, 1, count, "path %q appeared %d times; expected exactly 1", p, count)
	}
}

func TestOptions_SubagentsPaths_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	original := Options{
		SubagentsPaths:    []string{"/a", "/b"},
		DisabledSubagents: []string{"my-agent"},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Options
	require.NoError(t, json.Unmarshal(data, &restored))

	require.Equal(t, original.SubagentsPaths, restored.SubagentsPaths)
	require.Equal(t, original.DisabledSubagents, restored.DisabledSubagents)
}

// TestGlobalSubagentsDirs_EnvOverride verifies that CRUSH_SUBAGENTS_DIR, when
// set to a non-empty value, causes GlobalSubagentsDirs to return exactly that
// single path, mirroring the CRUSH_SKILLS_DIR override on GlobalSkillsDirs.
// When unset or empty, the existing default list is returned.
func TestGlobalSubagentsDirs_EnvOverride(t *testing.T) {
	override := t.TempDir()
	t.Setenv("CRUSH_SUBAGENTS_DIR", override)

	dirs := GlobalSubagentsDirs()
	require.Equal(t, []string{override}, dirs,
		"CRUSH_SUBAGENTS_DIR must fully replace the default subagents dirs")

	t.Setenv("CRUSH_SUBAGENTS_DIR", "")

	dirs = GlobalSubagentsDirs()
	found := false
	for _, d := range dirs {
		if strings.HasSuffix(d, filepath.Join("crush", "subagents")) {
			found = true
			break
		}
	}
	require.True(t, found,
		"expected the default list (a path ending in crush/subagents) when CRUSH_SUBAGENTS_DIR is empty; got %v", dirs)
}

// gitInitTempDir resolves a fresh temp dir's symlinks (macOS reports
// t.TempDir() under a symlink while git reports the physical path) and
// initializes a git repository there. It skips the test when git is
// unavailable or init fails.
func gitInitTempDir(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Skip("git init failed")
	}

	return root
}

// TestProjectSubagentsDir_MonorepoGitRoot verifies that when workingDir is a
// subdirectory of a git repository, ProjectSubagentsDir returns the
// repository-root paths FIRST and the working-directory paths LAST. Working
// directory entries must be last because Deduplicate keeps the last
// occurrence of a name, so a working-dir subagent overrides a monorepo-root
// subagent with the same name.
func TestProjectSubagentsDir_MonorepoGitRoot(t *testing.T) {
	t.Parallel()

	root := gitInitTempDir(t)
	sub := filepath.Join(root, "services", "api")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	dirs := ProjectSubagentsDir(sub)

	expected := []string{
		filepath.Join(root, ".agents", "subagents"),
		filepath.Join(root, ".crush", "subagents"),
		filepath.Join(sub, ".agents", "subagents"),
		filepath.Join(sub, ".crush", "subagents"),
	}
	require.Equal(t, expected, dirs)
}

// TestProjectSubagentsDir_AtGitRoot verifies that when workingDir IS the
// repository root, ProjectSubagentsDir returns exactly the two working-dir
// paths with no duplication.
func TestProjectSubagentsDir_AtGitRoot(t *testing.T) {
	t.Parallel()

	root := gitInitTempDir(t)

	dirs := ProjectSubagentsDir(root)

	expected := []string{
		filepath.Join(root, ".agents", "subagents"),
		filepath.Join(root, ".crush", "subagents"),
	}
	require.Equal(t, expected, dirs)
}

// TestProjectSubagentsDir_OutsideGitRepo verifies that when workingDir is not
// inside a git repository at all, ProjectSubagentsDir returns exactly the two
// working-dir paths.
func TestProjectSubagentsDir_OutsideGitRepo(t *testing.T) {
	t.Parallel()

	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	dirs := ProjectSubagentsDir(root)

	expected := []string{
		filepath.Join(root, ".agents", "subagents"),
		filepath.Join(root, ".crush", "subagents"),
	}
	require.Equal(t, expected, dirs)
}
