package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

type testDiscoverySource struct {
	cfg        *config.Config
	workingDir string
	resolver   config.VariableResolver
}

func (s testDiscoverySource) Config() *config.Config {
	return s.cfg
}

func (s testDiscoverySource) WorkingDir() string {
	return s.workingDir
}

func (s testDiscoverySource) Resolver() config.VariableResolver {
	return s.resolver
}

func TestCatalogUsesWorkingDirForRelativeSkillsPaths(t *testing.T) {
	workingDir := t.TempDir()
	createTestSkill(t, filepath.Join(workingDir, "project-skills", "example-skill"), "example-skill", "Project skill.", "project instructions")

	wd, err := os.Getwd()
	require.NoError(t, err)
	otherDir := t.TempDir()
	require.NoError(t, os.Chdir(otherDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd))
	})

	entries := Catalog(testDiscoverySource{
		cfg:        &config.Config{Options: &config.Options{SkillsPaths: []string{"./project-skills"}}},
		workingDir: workingDir,
		resolver:   config.IdentityResolver(),
	})

	require.Contains(t, entries, CatalogEntry{
		ID:          filepath.Join(workingDir, "project-skills", "example-skill", SkillFileName),
		Name:        "example-skill",
		Description: "Project skill.",
		Label:       "project:example-skill",
		Source:      SourceProject,
	})
}

func TestEffectiveDeduplicatesAndFiltersVisibleSkills(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	skillsRoot := filepath.Join(workingDir, "skills")
	createTestSkill(t, filepath.Join(skillsRoot, "crush-config"), "crush-config", "User override.", "override instructions")
	createTestSkill(t, filepath.Join(skillsRoot, "hidden-skill"), "hidden-skill", "Hidden skill.", "hidden instructions")

	effective := Effective(testDiscoverySource{
		cfg: &config.Config{Options: &config.Options{
			SkillsPaths:    []string{skillsRoot},
			DisabledSkills: []string{"hidden-skill"},
		}},
		workingDir: workingDir,
		resolver:   config.IdentityResolver(),
	})

	var crushConfig *Skill
	for _, skill := range effective {
		require.NotEqual(t, "hidden-skill", skill.Name)
		if skill.Name == "crush-config" {
			crushConfig = skill
		}
	}

	require.NotNil(t, crushConfig)
	require.False(t, crushConfig.Builtin)
	require.Equal(t, filepath.Join(skillsRoot, "crush-config", SkillFileName), crushConfig.SkillFilePath)
	require.Equal(t, "User override.", crushConfig.Description)
}

func TestReadContentUsesEffectiveSkillIDs(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	skillsRoot := filepath.Join(workingDir, "skills")
	userSkillPath := createTestSkill(t, filepath.Join(skillsRoot, "crush-config"), "crush-config", "User override.", "override instructions")

	src := testDiscoverySource{
		cfg:        &config.Config{Options: &config.Options{SkillsPaths: []string{skillsRoot}}},
		workingDir: workingDir,
		resolver:   config.IdentityResolver(),
	}

	content, err := ReadContent(src, userSkillPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "override instructions")

	_, err = ReadContent(src, BuiltinPrefix+"crush-config/SKILL.md")
	require.ErrorIs(t, err, ErrSkillNotFound)

	_, err = ReadContent(testDiscoverySource{cfg: &config.Config{Options: &config.Options{DisabledSkills: []string{"crush-config"}}}}, BuiltinPrefix+"crush-config/SKILL.md")
	require.ErrorIs(t, err, ErrSkillNotFound)
}

func TestEffectiveUsesStableConfiguredRootPrecedence(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	rootA := filepath.Join(workingDir, "skills-a")
	rootB := filepath.Join(workingDir, "skills-b")
	pathA := createTestSkill(t, filepath.Join(rootA, "shared-skill"), "shared-skill", "Root A skill.", "root a instructions")
	pathB := createTestSkill(t, filepath.Join(rootB, "shared-skill"), "shared-skill", "Root B skill.", "root b instructions")

	src := testDiscoverySource{
		cfg:        &config.Config{Options: &config.Options{SkillsPaths: []string{rootA, rootB}}},
		workingDir: workingDir,
		resolver:   config.IdentityResolver(),
	}

	for range 5 {
		effective := Effective(src)
		var shared *Skill
		for _, skill := range effective {
			if skill.Name == "shared-skill" {
				shared = skill
				break
			}
		}
		require.NotNil(t, shared)
		require.Equal(t, pathB, shared.SkillFilePath)

		content, err := ReadContent(src, pathB)
		require.NoError(t, err)
		require.Contains(t, string(content), "root b instructions")

		_, err = ReadContent(src, pathA)
		require.ErrorIs(t, err, ErrSkillNotFound)
	}
}

func createTestSkill(t *testing.T, dir, name, description, instructions string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, SkillFileName)
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n" + instructions + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
