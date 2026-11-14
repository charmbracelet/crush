package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

func TestAddAllowedTool(t *testing.T) {
	t.Parallel()

	store := &ConfigStore{
		config:         &Config{},
		globalDataPath: filepath.Join(t.TempDir(), "crush.json"),
		resolver:       NewShellVariableResolver(env.New()),
	}

	require.NoError(t, store.AddAllowedTool(ScopeGlobal, "mcp_atlassian_jira_search"))
	require.Contains(t, store.Config().Permissions.AllowedTools, "mcp_atlassian_jira_search")

	// Adding the same tool twice must not duplicate the entry.
	require.NoError(t, store.AddAllowedTool(ScopeGlobal, "mcp_atlassian_jira_search"))
	require.Len(t, store.Config().Permissions.AllowedTools, 1)

	// The persisted list must keep both an existing and the new entry.
	store = &ConfigStore{
		config:         &Config{Permissions: &Permissions{AllowedTools: []string{"bash"}}},
		globalDataPath: filepath.Join(t.TempDir(), "crush.json"),
		resolver:       NewShellVariableResolver(env.New()),
	}
	require.NoError(t, store.AddAllowedTool(ScopeGlobal, "view"))
	require.Equal(t, []string{"bash", "view"}, store.Config().Permissions.AllowedTools)

	data, err := os.ReadFile(store.globalDataPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "view")
}
