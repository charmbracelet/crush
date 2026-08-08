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
		{
			name:    "empty_content",
			content: "",
			wantErr: true,
		},
		{
			name:    "only_bom",
			content: "\ufeff",
			wantErr: true,
		},
		{
			name:    "only_whitespace",
			content: "   \n\n   \t\n",
			wantErr: true,
		},
		{
			name:            "empty_frontmatter_no_body",
			content:         "---\n---\n",
			wantName:        "",
			wantDescription: "",
		},
		{
			name: "crlf_line_endings",
			content: "---\r\nname: crlf-agent\r\n" +
				"description: Uses CRLF endings.\r\n---\r\n\r\nBody.\r\n",
			wantName:        "crlf-agent",
			wantDescription: "Uses CRLF endings.",
			wantBody:        "Body.",
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

func TestValidateAgainst(t *testing.T) {
	t.Parallel()

	knownModels := map[string]bool{"gpt-4o": true, "claude-opus-4-7": true}
	isKnown := func(id string) bool { return knownModels[id] }

	tests := []struct {
		name    string
		agent   Subagent
		wantErr bool
		errMsg  string
	}{
		{
			name:  "model_empty_ok",
			agent: Subagent{Name: "a", Description: "d", Model: ""},
		},
		{
			name:  "model_large_ok",
			agent: Subagent{Name: "a", Description: "d", Model: "large"},
		},
		{
			name:  "model_small_ok",
			agent: Subagent{Name: "a", Description: "d", Model: "small"},
		},
		{
			name:  "known_model_id_ok",
			agent: Subagent{Name: "a", Description: "d", Model: "gpt-4o"},
		},
		{
			name:    "unknown_model_rejected",
			agent:   Subagent{Name: "a", Description: "d", Model: "imaginary-99"},
			wantErr: true,
			errMsg:  "model",
		},
		{
			name:    "still_runs_base_validation",
			agent:   Subagent{Name: "", Description: "d", Model: "large"},
			wantErr: true,
			errMsg:  "name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.agent.ValidateAgainst(isKnown)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateAgainst_NilResolver_AcceptsAnyNonEmptyModel(t *testing.T) {
	t.Parallel()

	// Without a resolver, model id strings cannot be validated; ValidateAgainst
	// should accept any non-empty model string and defer enforcement.
	s := Subagent{Name: "a", Description: "d", Model: "gpt-99-future"}
	require.NoError(t, s.ValidateAgainst(nil))
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

func TestDiscoverWithStates(t *testing.T) {
	t.Parallel()

	const validAgent = "---\nname: %s\ndescription: Does the thing.\n---\n\nYou are a specialist agent.\n"

	t.Run("discovers_valid_agents_recursively", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		subdir := filepath.Join(tmp, "subdir")
		require.NoError(t, os.MkdirAll(subdir, 0o755))

		require.NoError(t, os.WriteFile(
			filepath.Join(tmp, "top-agent.md"),
			[]byte("---\nname: top-agent\ndescription: Top level agent.\n---\n\nYou are a specialist agent.\n"),
			0o644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(subdir, "sub-agent.md"),
			[]byte("---\nname: sub-agent\ndescription: Nested agent.\n---\n\nYou are a nested specialist agent.\n"),
			0o644,
		))

		agents, states := DiscoverWithStates([]string{tmp}, nil)

		require.Len(t, agents, 2)
		names := make([]string, 0, len(agents))
		for _, a := range agents {
			names = append(names, a.Name)
		}
		require.Contains(t, names, "top-agent")
		require.Contains(t, names, "sub-agent")
		require.Len(t, states, 2)
	})

	t.Run("invalid_agent_no_frontmatter_appears_as_error_not_in_agents", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(tmp, "bad-agent.md"),
			[]byte("# No frontmatter here\n\nJust markdown.\n"),
			0o644,
		))

		agents, states := DiscoverWithStates([]string{tmp}, nil)

		require.Empty(t, agents)
		require.Len(t, states, 1)
		require.Equal(t, StateError, states[0].State)
		require.Error(t, states[0].Err)
	})

	t.Run("nonexistent_path_silently_skipped", func(t *testing.T) {
		t.Parallel()

		agents, states := DiscoverWithStates([]string{filepath.Join(t.TempDir(), "does-not-exist")}, nil)

		require.Empty(t, agents)
		require.Empty(t, states)
	})

	t.Run("empty_dir_returns_no_results", func(t *testing.T) {
		t.Parallel()

		agents, states := DiscoverWithStates([]string{t.TempDir()}, nil)

		require.Empty(t, agents)
		require.Empty(t, states)
	})

	t.Run("non_md_files_ignored", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(tmp, "agent.txt"),
			[]byte("---\nname: txt-agent\ndescription: Should be ignored.\n---\n\nBody.\n"),
			0o644,
		))

		agents, states := DiscoverWithStates([]string{tmp}, nil)

		require.Empty(t, agents)
		require.Empty(t, states)
	})
}
