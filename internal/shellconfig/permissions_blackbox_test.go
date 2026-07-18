package shellconfig_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// allowedTools pulls permissions.allowed_tools out of a decoded config.
func allowedTools(t *testing.T, cfg map[string]any) []any {
	t.Helper()
	perms, ok := cfg["permissions"].(map[string]any)
	require.True(t, ok, "expected a permissions object, got: %v", cfg)
	tools, ok := perms["allowed_tools"].([]any)
	require.True(t, ok, "expected allowed_tools array, got: %v", perms)
	return tools
}

func TestPermissionsAllowSingle(t *testing.T) {
	t.Parallel()

	cfg := loadConfig(t, `permissions allow bash`)
	require.Equal(t, []any{"bash"}, allowedTools(t, cfg))
}

func TestPermissionsAllowMultiplePerCall(t *testing.T) {
	t.Parallel()

	cfg := loadConfig(t, `permissions allow view ls grep`)
	require.Equal(t, []any{"view", "ls", "grep"}, allowedTools(t, cfg))
}

func TestPermissionsAllowAccumulatesAcrossCalls(t *testing.T) {
	t.Parallel()

	cfg := loadConfig(t, `permissions allow view
permissions allow ls`)
	require.Equal(t, []any{"view", "ls"}, allowedTools(t, cfg))
}

func TestPermissionsAllowDeduplicates(t *testing.T) {
	t.Parallel()

	cfg := loadConfig(t, `permissions allow bash
permissions allow view
permissions allow bash`)
	require.Equal(t, []any{"bash", "view"}, allowedTools(t, cfg))
}

func TestPermissionsRejectsLegacyFlag(t *testing.T) {
	t.Parallel()

	err := loadConfigErr(t, `permissions --allow bash`)
	require.Contains(t, err.Error(), "unknown subcommand")
}

func TestPermissionsAllowRequiresTool(t *testing.T) {
	t.Parallel()

	err := loadConfigErr(t, `permissions allow`)
	require.Contains(t, err.Error(), "usage: permissions allow")
}

func TestPermissionsRequiresSubcommand(t *testing.T) {
	t.Parallel()

	err := loadConfigErr(t, `permissions`)
	require.Contains(t, err.Error(), "usage: permissions allow")
}
