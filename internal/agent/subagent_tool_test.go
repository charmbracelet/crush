package agent

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/stretchr/testify/require"
)

func TestBuildSubagentDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		subagents []*subagent.Subagent
		validate  func(t *testing.T, desc string)
	}{
		{
			name:      "no subagents",
			subagents: nil,
			validate: func(t *testing.T, desc string) {
				require.Contains(t, desc, "Launch a new agent")
				require.NotContains(t, desc, "<available_subagents>")
			},
		},
		{
			name:      "empty subagents slice",
			subagents: []*subagent.Subagent{},
			validate: func(t *testing.T, desc string) {
				require.Contains(t, desc, "Launch a new agent")
				require.NotContains(t, desc, "<available_subagents>")
			},
		},
		{
			name: "single subagent",
			subagents: []*subagent.Subagent{
				{
					Name:        "code-reviewer",
					Description: "Use for code review tasks",
				},
			},
			validate: func(t *testing.T, desc string) {
				require.Contains(t, desc, "<available_subagents>")
				require.Contains(t, desc, "code-reviewer")
				require.Contains(t, desc, "Use for code review tasks")
			},
		},
		{
			name: "multiple subagents",
			subagents: []*subagent.Subagent{
				{
					Name:        "code-reviewer",
					Description: "Use for code review tasks",
				},
				{
					Name:        "test-writer",
					Description: "Use for writing tests",
				},
				{
					Name:        "doc-generator",
					Description: "Use for generating documentation",
				},
			},
			validate: func(t *testing.T, desc string) {
				require.Contains(t, desc, "<available_subagents>")
				require.Contains(t, desc, "code-reviewer")
				require.Contains(t, desc, "test-writer")
				require.Contains(t, desc, "doc-generator")
			},
		},
		{
			name: "multiline description uses first line",
			subagents: []*subagent.Subagent{
				{
					Name: "multi-line-agent",
					Description: `Use this agent when you need to:
- Do something
- Do something else`,
				},
			},
			validate: func(t *testing.T, desc string) {
				require.Contains(t, desc, "multi-line-agent")
				require.Contains(t, desc, "Use this agent when you need to:")
				require.NotContains(t, desc, "- Do something")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			desc := buildSubagentDescription(tt.subagents)
			tt.validate(t, desc)
		})
	}
}

func TestFirstLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"single line", "single line"},
		{"first\nsecond", "first"},
		{"first\rsecond", "first"},
		{"first\r\nsecond", "first"},
		{"", ""},
		{"line with trailing newline\n", "line with trailing newline"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, firstLine(tt.input))
		})
	}
}

func TestSubagentDescriptionFormat(t *testing.T) {
	t.Parallel()

	subagents := []*subagent.Subagent{
		{Name: "agent-a", Description: "Description A"},
		{Name: "agent-b", Description: "Description B"},
	}

	desc := buildSubagentDescription(subagents)

	// Should contain proper markdown formatting.
	require.Contains(t, desc, "**agent-a**")
	require.Contains(t, desc, "**agent-b**")
	require.Contains(t, desc, "- **agent-a**: Description A")
	require.Contains(t, desc, "- **agent-b**: Description B")

	// Should have proper structure.
	require.True(t, strings.HasSuffix(desc, "</available_subagents>\n"))
	require.Contains(t, desc, "You can specify a subagent by name using the 'subagent' parameter")
}
