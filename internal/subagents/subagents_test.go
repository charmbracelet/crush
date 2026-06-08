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
		wantPermMode    string
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
disallowedTools: Write, Edit
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
disallowedTools: Write, Edit
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
			name: "permission_mode_bypass_decoded",
			content: `---
name: bypass-agent
description: An agent with bypass permissions.
permissionMode: bypassPermissions
---

Body.
`,
			wantName:        "bypass-agent",
			wantDescription: "An agent with bypass permissions.",
			wantPermMode:    PermissionModeBypassPermissions,
			wantBody:        "Body.",
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
			require.Equal(t, tt.wantPermMode, agent.PermissionMode)
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
		{
			name:    "permission_mode_default_valid",
			agent:   Subagent{Name: "my-agent", Description: "Something.", PermissionMode: PermissionModeDefault},
			wantErr: false,
		},
		{
			name:    "permission_mode_bypass_valid",
			agent:   Subagent{Name: "my-agent", Description: "Something.", PermissionMode: PermissionModeBypassPermissions},
			wantErr: false,
		},
		{
			name:    "permission_mode_accept_edits_rejected",
			agent:   Subagent{Name: "my-agent", Description: "Something.", PermissionMode: "acceptEdits"},
			wantErr: true,
			errMsg:  "permissionMode",
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
	isKnown := func(provider, id string) bool { return knownModels[id] }

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

// TestValidateAgainst_ProviderPropagated verifies that ValidateAgainst forwards
// the subagent's Provider field as the first argument to the resolver.
func TestValidateAgainst_ProviderPropagated(t *testing.T) {
	t.Parallel()

	var capturedProvider, capturedModel string
	isKnown := func(provider, model string) bool {
		capturedProvider = provider
		capturedModel = model
		return true
	}

	s := Subagent{Name: "a", Description: "d", Provider: "openai", Model: "gpt-4o"}
	require.NoError(t, s.ValidateAgainst(isKnown))
	require.Equal(t, "openai", capturedProvider)
	require.Equal(t, "gpt-4o", capturedModel)
}

// TestValidateAgainst_EmptyProviderPropagated verifies that when Provider is
// empty, ValidateAgainst calls the resolver with an empty provider string
// (allowing callers to perform an all-provider scan).
func TestValidateAgainst_EmptyProviderPropagated(t *testing.T) {
	t.Parallel()

	var capturedProvider string
	isKnown := func(provider, model string) bool {
		capturedProvider = provider
		return true
	}

	s := Subagent{Name: "a", Description: "d", Provider: "", Model: "gpt-4o"}
	require.NoError(t, s.ValidateAgainst(isKnown))
	require.Equal(t, "", capturedProvider)
}

// TestParseContent_ProviderField verifies that the provider field round-trips
// through YAML frontmatter parsing.
func TestParseContent_ProviderField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string
		wantProvider string
	}{
		{
			name: "provider_set",
			content: `---
name: my-agent
description: A test agent.
provider: openai
model: gpt-4o
---
`,
			wantProvider: "openai",
		},
		{
			name: "provider_absent_is_empty",
			content: `---
name: my-agent
description: A test agent.
---
`,
			wantProvider: "",
		},
		{
			name: "provider_explicit_empty",
			content: `---
name: my-agent
description: A test agent.
provider: ""
---
`,
			wantProvider: "",
		},
		{
			name: "provider_anthropic",
			content: `---
name: my-agent
description: A test agent.
provider: anthropic
model: claude-opus-4-7
---
`,
			wantProvider: "anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := ParseContent([]byte(tt.content))
			require.NoError(t, err)
			require.Equal(t, tt.wantProvider, agent.Provider)
		})
	}
}

// TestValidate_ProviderRequiresSpecificModel verifies that when provider is set,
// model must be a specific model ID (not empty, "large", or "small").
func TestValidate_ProviderRequiresSpecificModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		agent   Subagent
		wantErr bool
		errMsg  string
	}{
		{
			name:    "provider_set_no_model",
			agent:   Subagent{Name: "my-agent", Description: "Does something.", Provider: "openai"},
			wantErr: true,
			errMsg:  "model",
		},
		{
			name:    "provider_set_model_large",
			agent:   Subagent{Name: "my-agent", Description: "Does something.", Provider: "openai", Model: "large"},
			wantErr: true,
			errMsg:  "model",
		},
		{
			name:    "provider_set_model_small",
			agent:   Subagent{Name: "my-agent", Description: "Does something.", Provider: "openai", Model: "small"},
			wantErr: true,
			errMsg:  "model",
		},
		{
			name:  "provider_set_specific_model_ok",
			agent: Subagent{Name: "my-agent", Description: "Does something.", Provider: "openai", Model: "gpt-4o"},
		},
		{
			name:  "no_provider_model_large_ok",
			agent: Subagent{Name: "my-agent", Description: "Does something.", Provider: "", Model: "large"},
		},
		{
			name:  "no_provider_no_model_ok",
			agent: Subagent{Name: "my-agent", Description: "Does something.", Provider: "", Model: ""},
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

// TestParseContent_ColorField verifies that the color field round-trips through
// YAML frontmatter parsing for all defined color values plus absent/empty.
func TestParseContent_ColorField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		content   string
		wantColor string
	}{
		{
			name: "color_red",
			content: `---
name: my-agent
description: A test agent.
color: red
---
`,
			wantColor: "red",
		},
		{
			name: "color_orange",
			content: `---
name: my-agent
description: A test agent.
color: orange
---
`,
			wantColor: "orange",
		},
		{
			name: "color_yellow",
			content: `---
name: my-agent
description: A test agent.
color: yellow
---
`,
			wantColor: "yellow",
		},
		{
			name: "color_green",
			content: `---
name: my-agent
description: A test agent.
color: green
---
`,
			wantColor: "green",
		},
		{
			name: "color_cyan",
			content: `---
name: my-agent
description: A test agent.
color: cyan
---
`,
			wantColor: "cyan",
		},
		{
			name: "color_blue",
			content: `---
name: my-agent
description: A test agent.
color: blue
---
`,
			wantColor: "blue",
		},
		{
			name: "color_purple",
			content: `---
name: my-agent
description: A test agent.
color: purple
---
`,
			wantColor: "purple",
		},
		{
			name: "color_pink",
			content: `---
name: my-agent
description: A test agent.
color: pink
---
`,
			wantColor: "pink",
		},
		{
			name: "color_absent_is_empty",
			content: `---
name: my-agent
description: A test agent.
---
`,
			wantColor: "",
		},
		{
			name: "color_explicit_empty_string",
			content: `---
name: my-agent
description: A test agent.
color: ""
---
`,
			wantColor: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := ParseContent([]byte(tt.content))
			require.NoError(t, err)
			require.Equal(t, tt.wantColor, agent.Color)
		})
	}
}

// TestValidate_ColorField verifies that Validate accepts all eight defined
// color constants and empty, and rejects everything else with an error
// mentioning "color".
func TestValidate_ColorField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		color   string
		wantErr bool
	}{
		{name: "empty_accepted", color: ""},
		{name: "red_accepted", color: ColorRed},
		{name: "orange_accepted", color: ColorOrange},
		{name: "yellow_accepted", color: ColorYellow},
		{name: "green_accepted", color: ColorGreen},
		{name: "cyan_accepted", color: ColorCyan},
		{name: "blue_accepted", color: ColorBlue},
		{name: "purple_accepted", color: ColorPurple},
		{name: "pink_accepted", color: ColorPink},
		{name: "ultra_rejected", color: "ultra", wantErr: true},
		{name: "RED_rejected_case_sensitive", color: "RED", wantErr: true},
		{name: "lime_rejected", color: "lime", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := Subagent{
				Name:        "test-agent",
				Description: "Does something.",
				Color:       tt.color,
			}
			err := s.Validate()
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "color")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestResolvedColor_ExplicitColor verifies that ResolvedColor returns the
// explicitly set Color value when it is non-empty.
func TestResolvedColor_ExplicitColor(t *testing.T) {
	t.Parallel()

	s := Subagent{
		Name:        "my-agent",
		Description: "Does something.",
		Color:       ColorBlue,
	}
	require.Equal(t, ColorBlue, s.ResolvedColor())
}

// TestResolvedColor_AutoFallback verifies that when Color is empty,
// ResolvedColor returns AutoColor(Name): a non-empty string that is one of the
// eight valid color names.
func TestResolvedColor_AutoFallback(t *testing.T) {
	t.Parallel()

	s := Subagent{
		Name:        "my-agent",
		Description: "Does something.",
		Color:       "",
	}

	result := s.ResolvedColor()
	require.NotEmpty(t, result, "ResolvedColor must not return empty when Color is unset")
	require.True(t, IsValidColor(result), "ResolvedColor fallback %q must be a valid color", result)
	require.Equal(t, AutoColor(s.Name), result, "ResolvedColor must equal AutoColor(Name) when Color is empty")
}

func TestDiscoverWithStates(t *testing.T) {
	t.Parallel()

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

	t.Run("resolver_receives_provider_and_model", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(tmp, "specific-agent.md"),
			[]byte("---\nname: specific-agent\ndescription: Uses a specific model.\nprovider: openai\nmodel: gpt-4o\n---\n\nBody.\n"),
			0o644,
		))

		var capturedProvider, capturedModel string
		isKnown := func(provider, model string) bool {
			capturedProvider = provider
			capturedModel = model
			return true
		}

		agents, states := DiscoverWithStates([]string{tmp}, isKnown)

		require.Len(t, agents, 1)
		require.Len(t, states, 1)
		require.Equal(t, StateNormal, states[0].State)
		require.Equal(t, "openai", capturedProvider)
		require.Equal(t, "gpt-4o", capturedModel)
	})

	t.Run("unknown_model_with_resolver_produces_error_state", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(tmp, "unknown-model-agent.md"),
			[]byte("---\nname: unknown-model-agent\ndescription: Uses an unknown model.\nmodel: no-such-model-99\n---\n\nBody.\n"),
			0o644,
		))

		isKnown := func(provider, model string) bool { return false }

		agents, states := DiscoverWithStates([]string{tmp}, isKnown)

		require.Empty(t, agents)
		require.Len(t, states, 1)
		require.Equal(t, StateError, states[0].State)
		require.Error(t, states[0].Err)
	})
}

// TestValidate_ReportsAllToolOverlaps verifies that when multiple tools appear
// in both Tools and DisallowedTools, Validate reports every overlapping tool
// rather than stopping at the first. The fix removed a break so all overlaps
// are joined via errors.Join.
func TestValidate_ReportsAllToolOverlaps(t *testing.T) {
	t.Parallel()

	sa := Subagent{
		Name:            "reviewer",
		Description:     "Reviews things.",
		Tools:           ToolList{"view", "edit", "bash"},
		DisallowedTools: ToolList{"view", "edit", "bash"},
	}

	err := sa.Validate()
	require.Error(t, err)
	require.ErrorContains(t, err, "view")
	require.ErrorContains(t, err, "edit")
	require.ErrorContains(t, err, "bash")
}
