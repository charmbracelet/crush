package config

import (
	"encoding/json"
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
