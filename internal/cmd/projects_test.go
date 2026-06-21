package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/projects"
	"github.com/stretchr/testify/require"
)

func TestProjectsEmpty(t *testing.T) {
	// Use a temp directory for projects.json
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	var b bytes.Buffer
	projectsCmd.SetOut(&b)
	projectsCmd.SetErr(&b)
	projectsCmd.SetIn(bytes.NewReader(nil))
	err := projectsCmd.RunE(projectsCmd, nil)
	require.NoError(t, err)
	require.Equal(t, "No projects tracked yet.\n", b.String())
}

func TestProjectsJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	// Register a project
	err := projects.Register("/test/project", "/test/project/.crush")
	require.NoError(t, err)

	var b bytes.Buffer
	projectsCmd.SetOut(&b)
	projectsCmd.SetErr(&b)
	projectsCmd.SetIn(bytes.NewReader(nil))

	// Set the --json flag
	projectsCmd.Flags().Set("json", "true")
	defer projectsCmd.Flags().Set("json", "false")

	err = projectsCmd.RunE(projectsCmd, nil)
	require.NoError(t, err)

	// Parse the JSON output
	var result struct {
		Projects []projects.Project `json:"projects"`
	}
	err = json.Unmarshal(b.Bytes(), &result)
	require.NoError(t, err)

	require.Len(t, result.Projects, 1)
	require.Equal(t, "/test/project", result.Projects[0].Path)
	require.Equal(t, "/test/project/.crush", result.Projects[0].DataDir)
}
