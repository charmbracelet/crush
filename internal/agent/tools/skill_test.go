package tools

import (
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

func TestSkillToolLoadsExactSkillAndTracksIt(t *testing.T) {
	t.Parallel()

	available := []*skills.Skill{
		{Name: "mcp-setup", Description: "Configure MCP servers.", Instructions: "Use structured MCP tools.", SkillFilePath: "crush://skills/mcp-setup/SKILL.md"},
		{Name: "hidden", Instructions: "Do not load.", DisableModelInvocation: true},
	}
	tracker := skills.NewTracker(available)
	tool := NewSkillTool(available, tracker)

	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: SkillToolName, Input: `{"name":"mcp-setup"}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Contains(t, response.Content, "<name>mcp-setup</name>")
	require.Contains(t, response.Content, "<description>Configure MCP servers.</description>")
	require.Contains(t, response.Content, "<location>crush://skills/mcp-setup/SKILL.md</location>")
	require.Contains(t, response.Content, "Use structured MCP tools.")
	require.True(t, tracker.IsLoaded("mcp-setup"))
}

func TestSkillToolRejectsUnknownOrHiddenSkill(t *testing.T) {
	t.Parallel()

	tool := NewSkillTool([]*skills.Skill{
		{Name: "mcp-setup", Instructions: "Use structured MCP tools."},
		{Name: "hidden", Instructions: "Do not load.", DisableModelInvocation: true},
	}, nil)

	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: SkillToolName, Input: `{"name":"MCP-SETUP"}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, "available skills: mcp-setup")
	require.NotContains(t, response.Content, "hidden")
}

func TestSkillToolRequiresName(t *testing.T) {
	t.Parallel()

	tool := NewSkillTool(nil, nil)
	response, err := tool.Run(t.Context(), fantasy.ToolCall{Name: SkillToolName, Input: `{}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, "name is required")
}
