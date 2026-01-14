package subagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSubagentFile(t *testing.T) {
	content := `---
name: test-agent
description: A test agent
tools: [ls, view]
model: haiku
---
You are a test agent.
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-agent.md")
	err := os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)

	sub, err := ParseSubagentFile(path)
	require.NoError(t, err)
	require.Equal(t, "test-agent", sub.Name)
	require.Equal(t, "A test agent", sub.Description)
	require.Equal(t, []string{"ls", "view"}, sub.Tools)
	require.Equal(t, "haiku", sub.Model)
	require.Equal(t, "You are a test agent.", sub.SystemPrompt)
}

func TestLoaderPriority(t *testing.T) {
	userDir := t.TempDir()
	projectDir := t.TempDir()

	userContent := `---
name: shared-agent
description: User level
---
User prompt
`
	projectContent := `---
name: shared-agent
description: Project level
---
Project prompt
`
	cliJSON := `{"shared-agent": {"description": "CLI level", "prompt": "CLI prompt"}}`

	err := os.WriteFile(filepath.Join(userDir, "shared.md"), []byte(userContent), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(projectDir, "shared.md"), []byte(projectContent), 0o644)
	require.NoError(t, err)

	// Test CLI priority
	loader := NewLoader(userDir, projectDir, cliJSON)
	subs, err := loader.Load()
	require.NoError(t, err)
	require.Equal(t, "CLI level", subs["shared-agent"].Description)
	require.Equal(t, "CLI prompt", subs["shared-agent"].SystemPrompt)
	require.Equal(t, SubagentSourceCLI, subs["shared-agent"].Source)

	// Test Project priority
	loader = NewLoader(userDir, projectDir, "")
	subs, err = loader.Load()
	require.NoError(t, err)
	require.Equal(t, "Project level", subs["shared-agent"].Description)
	require.Equal(t, SubagentSourceProject, subs["shared-agent"].Source)

	// Test User priority
	loader = NewLoader(userDir, "", "")
	subs, err = loader.Load()
	require.NoError(t, err)
	require.Equal(t, "User level", subs["shared-agent"].Description)
	require.Equal(t, SubagentSourceUser, subs["shared-agent"].Source)
}
