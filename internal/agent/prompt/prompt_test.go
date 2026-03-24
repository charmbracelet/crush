package prompt

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildInjectsSkillsIntoPrompt(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	cfg, err := config.Init(workingDir, "", false)
	require.NoError(t, err)

	skillRoot := t.TempDir()
	skillDir := filepath.Join(skillRoot, "example-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: example-skill
description: Example description.
---
Use this skill carefully.
`), 0o644))
	cfg.Config().Options.SkillsPaths = []string{skillRoot}

	templateBytes, err := os.ReadFile(filepath.Join("..", "templates", "coder.md.tpl"))
	require.NoError(t, err)

	p, err := NewPrompt("coder", string(templateBytes))
	require.NoError(t, err)

	output, err := p.Build(context.Background(), "anthropic", "claude-sonnet-4-5", cfg)
	require.NoError(t, err)
	require.Contains(t, output, "<available_skills>")
	require.Contains(t, output, "example-skill")
	require.Contains(t, output, "<skills_usage>")
}

// Not parallel: test calls os.Chdir which affects the whole process.
func TestBuildResolvesRelativeSkillsPathsFromWorkingDir(t *testing.T) {
	workingDir := t.TempDir()
	cfg, err := config.Init(workingDir, "", false)
	require.NoError(t, err)

	skillDir := filepath.Join(workingDir, "project-skills", "example-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: example-skill
description: Example description.
---
Use this project skill carefully.
`), 0o644))
	cfg.Config().Options.SkillsPaths = []string{"./project-skills"}
	templateBytes, err := os.ReadFile(filepath.Join("..", "templates", "coder.md.tpl"))
	require.NoError(t, err)

	wd, err := os.Getwd()
	require.NoError(t, err)
	otherDir := t.TempDir()
	require.NoError(t, os.Chdir(otherDir))
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	})

	p, err := NewPrompt("coder", string(templateBytes))
	require.NoError(t, err)

	output, err := p.Build(context.Background(), "anthropic", "claude-sonnet-4-5", cfg)
	require.NoError(t, err)
	require.Contains(t, output, "example-skill")
}
