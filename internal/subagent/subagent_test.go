package subagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		wantErr     bool
		errContains string
		validate    func(t *testing.T, agent *Subagent)
	}{
		{
			name: "valid full agent",
			content: `---
name: code-reviewer
description: Use for code review tasks
model: large
tools:
  - View
  - Grep
allowed_tools:
  - view
  - grep
yolo_mode: false
max_steps: 10
---

You are a code reviewer. Review code for bugs and style issues.
`,
			wantErr: false,
			validate: func(t *testing.T, agent *Subagent) {
				require.Equal(t, "code-reviewer", agent.Name)
				require.Equal(t, "Use for code review tasks", agent.Description)
				require.Equal(t, "large", agent.Model)
				require.Equal(t, []string{"View", "Grep"}, agent.Tools)
				require.Equal(t, []string{"view", "grep"}, agent.AllowedTools)
				require.False(t, agent.YoloMode)
				require.Equal(t, 10, agent.MaxSteps)
				require.Equal(t, "You are a code reviewer. Review code for bugs and style issues.", agent.Prompt)
			},
		},
		{
			name: "minimal agent",
			content: `---
name: simple-agent
description: A simple agent
---

Do simple things.
`,
			wantErr: false,
			validate: func(t *testing.T, agent *Subagent) {
				require.Equal(t, "simple-agent", agent.Name)
				require.Equal(t, "A simple agent", agent.Description)
				require.Equal(t, "inherit", agent.Model)
				require.Nil(t, agent.Tools)
				require.Nil(t, agent.AllowedTools)
				require.False(t, agent.YoloMode)
				require.Equal(t, 0, agent.MaxSteps)
				require.Equal(t, "Do simple things.", agent.Prompt)
			},
		},
		{
			name: "yolo mode enabled",
			content: `---
name: yolo-agent
description: An agent that auto-approves everything
yolo_mode: true
---

I approve everything.
`,
			wantErr: false,
			validate: func(t *testing.T, agent *Subagent) {
				require.Equal(t, "yolo-agent", agent.Name)
				require.True(t, agent.YoloMode)
			},
		},
		{
			name: "missing name",
			content: `---
description: Missing name field
---

Prompt here.
`,
			wantErr:     true,
			errContains: "missing required 'name' field",
		},
		{
			name: "missing description",
			content: `---
name: no-description
---

Prompt here.
`,
			wantErr:     true,
			errContains: "missing required 'description' field",
		},
		{
			name:        "no frontmatter",
			content:     "Just some text without frontmatter",
			wantErr:     true,
			errContains: "must start with YAML frontmatter",
		},
		{
			name: "unclosed frontmatter",
			content: `---
name: broken
description: Never closed
`,
			wantErr:     true,
			errContains: "unclosed YAML frontmatter",
		},
		{
			name:        "empty file",
			content:     "",
			wantErr:     true,
			errContains: "empty subagent file",
		},
		{
			name: "multiline description",
			content: `---
name: multi-desc
description: |
  Use this agent when you need to:
  - Review code
  - Find bugs
---

System prompt.
`,
			wantErr: false,
			validate: func(t *testing.T, agent *Subagent) {
				require.Contains(t, agent.Description, "Review code")
				require.Contains(t, agent.Description, "Find bugs")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := ParseContent([]byte(tt.content), "test.md")

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, agent)
			if tt.validate != nil {
				tt.validate(t, agent)
			}
		})
	}
}

func TestDiscover(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	// Create valid agent files.
	agent1 := `---
name: agent-one
description: First agent
---

Agent one prompt.
`
	agent2 := `---
name: agent-two
description: Second agent
tools:
  - View
---

Agent two prompt.
`
	// Invalid file (missing name).
	invalidAgent := `---
description: No name
---

Invalid.
`
	// Non-markdown file (should be ignored).
	txtFile := "just some text"

	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "agent-one.md"), []byte(agent1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "agent-two.md"), []byte(agent2), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "invalid.md"), []byte(invalidAgent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "readme.txt"), []byte(txtFile), 0o644))

	agents, err := Discover([]string{agentsDir})
	require.NoError(t, err)
	require.Len(t, agents, 2)

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	require.True(t, names["agent-one"])
	require.True(t, names["agent-two"])
}

func TestDiscoverPriority(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global", "agents")
	localDir := filepath.Join(tmpDir, "local", "agents")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	require.NoError(t, os.MkdirAll(localDir, 0o755))

	// Same name in both dirs - first path should win.
	globalAgent := `---
name: same-name
description: Global version
---

Global prompt.
`
	localAgent := `---
name: same-name
description: Local version
---

Local prompt.
`
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "agent.md"), []byte(globalAgent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "agent.md"), []byte(localAgent), 0o644))

	// Global first.
	agents, err := Discover([]string{globalDir, localDir})
	require.NoError(t, err)
	require.Len(t, agents, 1)
	require.Equal(t, "Global version", agents[0].Description)

	// Local first.
	agents, err = Discover([]string{localDir, globalDir})
	require.NoError(t, err)
	require.Len(t, agents, 1)
	require.Equal(t, "Local version", agents[0].Description)
}

func TestDiscoverNonexistentPath(t *testing.T) {
	t.Parallel()

	agents, err := Discover([]string{"/nonexistent/path/agents"})
	require.NoError(t, err)
	require.Empty(t, agents)
}

func TestFindByName(t *testing.T) {
	t.Parallel()

	agents := []*Subagent{
		{Name: "agent-a", Description: "Agent A"},
		{Name: "agent-b", Description: "Agent B"},
	}

	found := FindByName(agents, "agent-b")
	require.NotNil(t, found)
	require.Equal(t, "Agent B", found.Description)

	notFound := FindByName(agents, "agent-c")
	require.Nil(t, notFound)
}

func TestDefaultDiscoveryPaths(t *testing.T) {
	t.Parallel()

	paths := DefaultDiscoveryPaths("/home/user", "/project")
	require.Contains(t, paths, "/home/user/.config/crush/agents")
	require.Contains(t, paths, "/home/user/.config/agents")
	require.Contains(t, paths, "/project/.crush/agents")
	require.Contains(t, paths, "/project/.claude/agents")
}

func TestParsePermissionControls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		validate func(t *testing.T, agent *Subagent)
	}{
		{
			name: "yolo_mode enables auto-approval",
			content: `---
name: yolo-agent
description: Auto-approves everything
yolo_mode: true
---

YOLO prompt.
`,
			validate: func(t *testing.T, agent *Subagent) {
				require.True(t, agent.YoloMode)
				require.Empty(t, agent.AllowedTools)
			},
		},
		{
			name: "allowed_tools for selective auto-approval",
			content: `---
name: selective-agent
description: Only some tools auto-approved
allowed_tools:
  - view
  - grep
  - glob
---

Selective prompt.
`,
			validate: func(t *testing.T, agent *Subagent) {
				require.False(t, agent.YoloMode)
				require.Equal(t, []string{"view", "grep", "glob"}, agent.AllowedTools)
			},
		},
		{
			name: "allowed_tools with action specifier",
			content: `---
name: action-specific-agent
description: Action-specific permissions
allowed_tools:
  - bash:execute
  - edit:write
---

Action prompt.
`,
			validate: func(t *testing.T, agent *Subagent) {
				require.Contains(t, agent.AllowedTools, "bash:execute")
				require.Contains(t, agent.AllowedTools, "edit:write")
			},
		},
		{
			name: "tools restriction limits available tools",
			content: `---
name: restricted-agent
description: Limited tools
tools:
  - View
  - Grep
  - LS
---

Restricted prompt.
`,
			validate: func(t *testing.T, agent *Subagent) {
				require.Equal(t, []string{"View", "Grep", "LS"}, agent.Tools)
			},
		},
		{
			name: "model inheritance",
			content: `---
name: inherit-model-agent
description: Uses parent model
model: inherit
---

Inherit prompt.
`,
			validate: func(t *testing.T, agent *Subagent) {
				require.Equal(t, "inherit", agent.Model)
			},
		},
		{
			name: "explicit model selection",
			content: `---
name: small-model-agent
description: Uses small model
model: small
---

Small model prompt.
`,
			validate: func(t *testing.T, agent *Subagent) {
				require.Equal(t, "small", agent.Model)
			},
		},
		{
			name: "max_steps limits iterations",
			content: `---
name: limited-steps-agent
description: Limited iterations
max_steps: 5
---

Limited prompt.
`,
			validate: func(t *testing.T, agent *Subagent) {
				require.Equal(t, 5, agent.MaxSteps)
			},
		},
		{
			name: "combined permission controls",
			content: `---
name: combined-agent
description: Multiple permission features
tools:
  - View
  - Grep
  - Bash
allowed_tools:
  - view
  - grep
max_steps: 10
---

Combined prompt with restricted tools but some auto-approved.
`,
			validate: func(t *testing.T, agent *Subagent) {
				require.Equal(t, []string{"View", "Grep", "Bash"}, agent.Tools)
				require.Equal(t, []string{"view", "grep"}, agent.AllowedTools)
				require.Equal(t, 10, agent.MaxSteps)
				require.False(t, agent.YoloMode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := ParseContent([]byte(tt.content), "test.md")
			require.NoError(t, err)
			require.NotNil(t, agent)
			tt.validate(t, agent)
		})
	}
}

func TestParseComplexPrompts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		validate func(t *testing.T, agent *Subagent)
	}{
		{
			name: "multiline system prompt",
			content: `---
name: multiline-agent
description: Has multiline prompt
---

You are a specialized agent.

## Guidelines

1. Follow these rules
2. Do this correctly

## Examples

Here is an example of good output.
`,
			validate: func(t *testing.T, agent *Subagent) {
				require.Contains(t, agent.Prompt, "You are a specialized agent.")
				require.Contains(t, agent.Prompt, "## Guidelines")
				require.Contains(t, agent.Prompt, "## Examples")
			},
		},
		{
			name:    "prompt with code blocks",
			content: "---\nname: code-agent\ndescription: Has code in prompt\n---\n\nWhen writing code, follow this pattern:\n\n```go\nfunc example() {\n    // Do something\n}\n```\n",
			validate: func(t *testing.T, agent *Subagent) {
				require.Contains(t, agent.Prompt, "```go")
				require.Contains(t, agent.Prompt, "func example()")
			},
		},
		{
			name: "prompt with yaml-like content",
			content: `---
name: yaml-content-agent
description: Has yaml in prompt
---

Example configuration:
key: value
nested:
  foo: bar
`,
			validate: func(t *testing.T, agent *Subagent) {
				require.Contains(t, agent.Prompt, "key: value")
				require.Contains(t, agent.Prompt, "nested:")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := ParseContent([]byte(tt.content), "test.md")
			require.NoError(t, err)
			require.NotNil(t, agent)
			tt.validate(t, agent)
		})
	}
}
