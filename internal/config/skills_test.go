package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSetSkillDisabledConfigOnlyTouchesScopeFile(t *testing.T) {
	t.Parallel()

	globalPath := filepath.Join(t.TempDir(), "crush.json")
	require.NoError(t, os.WriteFile(globalPath, []byte(`{"options":{"disabled_skills":["global-one","pair"]}}`), 0o600))

	// The merged config also carries a project-only entry, which must not
	// leak into the global file.
	store := &ConfigStore{
		config:         &Config{Options: &Options{DisabledSkills: []string{"global-one", "pair", "project-only"}}},
		globalDataPath: globalPath,
	}

	require.NoError(t, store.SetSkillDisabledConfig(ScopeGlobal, "charmtone", true))
	require.NoError(t, store.SetSkillDisabledConfig(ScopeGlobal, "pair", false))

	data, err := os.ReadFile(globalPath)
	require.NoError(t, err)
	var got []string
	for _, name := range gjson.GetBytes(data, "options.disabled_skills").Array() {
		got = append(got, name.String())
	}
	require.Equal(t, []string{"charmtone", "global-one"}, got)
}
