package prompt

import (
	"testing"

	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

func TestWithSkills_StoresProvidedSkills(t *testing.T) {
	t.Parallel()

	provided := []*skills.Skill{
		{Name: "my-skill", Description: "A test skill.", SkillFilePath: "/skills/my-skill/SKILL.md"},
	}

	p, err := NewPrompt("test", "Test template", WithSkills(provided))
	require.NoError(t, err)

	require.Equal(t, provided, p.skills,
		"WithSkills must store the slice on the Prompt")
}

func TestWithoutSkills_LeavesNil(t *testing.T) {
	t.Parallel()

	p, err := NewPrompt("test", "Test template")
	require.NoError(t, err)

	require.Nil(t, p.skills,
		"Without WithSkills, p.skills must be nil so promptData falls back to discovery")
}
