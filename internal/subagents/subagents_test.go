package subagents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		content         string
		wantTools       []string
		wantDisallowed  []string
		wantName        string
		wantDescription string
		wantModel       string
		wantSkills      []string
		wantMCPServers  []string
		wantBody        string
		wantErr         bool
	}{
		{
			name: "comma_separated_tools",
			content: `---
name: my-agent
description: A test agent.
tools: Read, Grep, Bash
---
`,
			wantName:        "my-agent",
			wantDescription: "A test agent.",
			wantTools:       []string{"Read", "Grep", "Bash"},
		},
		{
			name: "yaml_array_tools",
			content: `---
name: my-agent
description: A test agent.
tools:
  - Read
  - Grep
---
`,
			wantName:        "my-agent",
			wantDescription: "A test agent.",
			wantTools:       []string{"Read", "Grep"},
		},
		{
			name: "no_tools_field",
			content: `---
name: my-agent
description: A test agent.
---
`,
			wantName:        "my-agent",
			wantDescription: "A test agent.",
			wantTools:       nil,
		},
		{
			name: "disallowed_tools_comma",
			content: `---
name: my-agent
description: A test agent.
disallowed_tools: Write, Edit
---
`,
			wantName:        "my-agent",
			wantDescription: "A test agent.",
			wantDisallowed:  []string{"Write", "Edit"},
		},
		{
			name: "all_fields",
			content: `---
name: my-agent
description: A fully specified agent.
model: large
tools:
  - Read
  - Bash
disallowed_tools: Write, Edit
skills:
  - pdf-processing
  - data-analysis
mcp_servers:
  - filesystem
---

This is the system prompt body.
`,
			wantName:        "my-agent",
			wantDescription: "A fully specified agent.",
			wantModel:       "large",
			wantTools:       []string{"Read", "Bash"},
			wantDisallowed:  []string{"Write", "Edit"},
			wantSkills:      []string{"pdf-processing", "data-analysis"},
			wantMCPServers:  []string{"filesystem"},
			wantBody:        "This is the system prompt body.",
		},
		{
			name: "body_extracted",
			content: `---
name: my-agent
description: A test agent.
---

# System Prompt

Do the thing.
`,
			wantName:        "my-agent",
			wantDescription: "A test agent.",
			wantBody:        "# System Prompt\n\nDo the thing.",
		},
		{
			name: "utf8_bom_stripped",
			content: "\uFEFF---\n" +
				"name: bom-agent\n" +
				"description: Agent with BOM.\n" +
				"---\n\n" +
				"Body here.\n",
			wantName:        "bom-agent",
			wantDescription: "Agent with BOM.",
			wantBody:        "Body here.",
		},
		{
			name: "leading_blank_lines",
			content: "\n\n---\n" +
				"name: blank-prefix\n" +
				"description: Agent with leading blank lines.\n" +
				"---\n\n" +
				"Body here.\n",
			wantName:        "blank-prefix",
			wantDescription: "Agent with leading blank lines.",
			wantBody:        "Body here.",
		},
		{
			name:    "no_frontmatter",
			content: "# Just Markdown\n\nNo frontmatter here.",
			wantErr: true,
		},
		{
			name: "unclosed_frontmatter",
			content: `---
name: my-agent
description: Never closed.
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := ParseContent([]byte(tt.content))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, agent)

			require.Equal(t, tt.wantName, agent.Name)
			require.Equal(t, tt.wantDescription, agent.Description)
			require.Equal(t, tt.wantTools, []string(agent.Tools))
			require.Equal(t, tt.wantDisallowed, []string(agent.DisallowedTools))

			if tt.wantModel != "" {
				require.Equal(t, tt.wantModel, agent.Model)
			}
			if tt.wantSkills != nil {
				require.Equal(t, tt.wantSkills, agent.Skills)
			}
			if tt.wantMCPServers != nil {
				require.Equal(t, tt.wantMCPServers, agent.MCPServers)
			}
			if tt.wantBody != "" {
				require.Equal(t, tt.wantBody, agent.Body)
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("reads file and sets filepath", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "my-agent.md")
		require.NoError(t, os.WriteFile(path, []byte(`---
name: my-agent
description: A test agent.
---

Body here.
`), 0o644))

		agent, err := Parse(path)
		require.NoError(t, err)
		require.Equal(t, "my-agent", agent.Name)
		require.Equal(t, "A test agent.", agent.Description)
		require.Equal(t, "Body here.", agent.Body)
		require.Equal(t, path, agent.FilePath)
	})

	t.Run("missing file returns error", func(t *testing.T) {
		t.Parallel()

		_, err := Parse(filepath.Join(t.TempDir(), "nonexistent.md"))
		require.Error(t, err)
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		agent   Subagent
		wantErr bool
		errMsg  string
	}{
		{
			name:  "valid_minimal",
			agent: Subagent{Name: "my-agent", Description: "Does something."},
		},
		{
			name:    "missing_name",
			agent:   Subagent{Description: "Something."},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name:    "missing_description",
			agent:   Subagent{Name: "my-agent"},
			wantErr: true,
			errMsg:  "description is required",
		},
		{
			name:    "uppercase_in_name",
			agent:   Subagent{Name: "MyAgent", Description: "Something."},
			wantErr: true,
			errMsg:  "lowercase",
		},
		{
			name:    "name_too_long",
			agent:   Subagent{Name: strings.Repeat("a", 65), Description: "Something."},
			wantErr: true,
			errMsg:  "exceeds",
		},
		{
			name:    "reserved_name_agent",
			agent:   Subagent{Name: "agent", Description: "Something."},
			wantErr: true,
			errMsg:  "reserved",
		},
		{
			name:    "reserved_name_task",
			agent:   Subagent{Name: "task", Description: "Something."},
			wantErr: true,
			errMsg:  "reserved",
		},
		{
			name:    "reserved_name_bash",
			agent:   Subagent{Name: "bash", Description: "Something."},
			wantErr: true,
			errMsg:  "reserved",
		},
		{
			name: "tools_disallowed_overlap",
			agent: Subagent{
				Name:            "my-agent",
				Description:     "Something.",
				Tools:           ToolList{"bash", "grep"},
				DisallowedTools: ToolList{"bash"},
			},
			wantErr: true,
			errMsg:  "both",
		},
		{
			name:    "description_too_long",
			agent:   Subagent{Name: "my-agent", Description: strings.Repeat("a", 1025)},
			wantErr: true,
			errMsg:  "description",
		},
		{
			name:    "starts_with_hyphen",
			agent:   Subagent{Name: "-my-agent", Description: "Something."},
			wantErr: true,
			errMsg:  "lowercase",
		},
		{
			name:    "consecutive_hyphens",
			agent:   Subagent{Name: "my--agent", Description: "Something."},
			wantErr: true,
			errMsg:  "lowercase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.agent.Validate()
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()

	all := []*Subagent{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}

	tests := []struct {
		name     string
		disabled []string
		wantLen  int
	}{
		{"nil_disabled", nil, 3},
		{"filter_one", []string{"b"}, 2},
		{"filter_all", []string{"a", "b", "c"}, 0},
		{"filter_nonexistent", []string{"z"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := Filter(all, tt.disabled)
			require.Len(t, result, tt.wantLen)
		})
	}
}

func TestDeduplicate(t *testing.T) {
	t.Parallel()

	t.Run("no_duplicates", func(t *testing.T) {
		t.Parallel()

		input := []*Subagent{{Name: "a", FilePath: "/a"}, {Name: "b", FilePath: "/b"}}
		result := Deduplicate(input)
		require.Len(t, result, 2)
	})

	t.Run("last_wins", func(t *testing.T) {
		t.Parallel()

		input := []*Subagent{
			{Name: "a", FilePath: "/a"},
			{Name: "a", FilePath: "/b"},
		}
		result := Deduplicate(input)
		require.Len(t, result, 1)
		require.Equal(t, "/b", result[0].FilePath)
	})

	t.Run("empty_input", func(t *testing.T) {
		t.Parallel()

		result := Deduplicate(nil)
		require.Empty(t, result)
	})
}
