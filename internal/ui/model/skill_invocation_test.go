package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

func TestFormatLoadedSkillInvocationIncludesResolvedBody(t *testing.T) {
	t.Parallel()

	skill := &skills.Skill{
		Name:          "project-context-init",
		Description:   "fallback description",
		SkillFilePath: "crush://skills/project-context-init/SKILL.md",
	}
	result := skills.SkillReadResult{
		Name:        "project-context-init",
		Description: "resolved description",
		Builtin:     true,
	}

	got := formatLoadedSkillInvocation(skill, []byte("Inspect the exact project root before writing."), result, "Run it now.")
	require.Contains(t, got, "<loaded_skill>")
	require.Contains(t, got, "resolved description")
	require.Contains(t, got, "Inspect the exact project root before writing.")
	require.Contains(t, got, "Run it now.")
}
