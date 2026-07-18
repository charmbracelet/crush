package shellconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPermissionsAllowDedup(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `permissions allow bash
permissions allow view
permissions allow bash`)

	tools := result["permissions"].(map[string]any)["allowed_tools"].([]any)
	require.Equal(t, []any{"bash", "view"}, tools)
}

func TestPermissionsUnknownSubcommand(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/crush.sh"
	_, err := LoadShellConfig(path, []byte(`permissions --allow bash`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown subcommand")
}
