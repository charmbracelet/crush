package shellconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHookRemoveByName(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `hook add PreToolUse --command "echo fmt" --name fmt
hook add PreToolUse --command "echo lint" --name lint
hook remove PreToolUse --name fmt`)

	arr := result["hooks"].(map[string]any)["PreToolUse"].([]any)
	require.Len(t, arr, 1)
	require.Equal(t, "lint", arr[0].(map[string]any)["name"])
}

func TestHookRemoveByNameAlias(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `hook add PreToolUse --command "echo fmt" --name fmt
hook add PreToolUse --command "echo lint" --name lint
hook rm PreToolUse --name lint`)

	arr := result["hooks"].(map[string]any)["PreToolUse"].([]any)
	require.Len(t, arr, 1)
	require.Equal(t, "fmt", arr[0].(map[string]any)["name"])
}

func TestHookClearEvent(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `hook add PreToolUse --command "echo a" --name a
hook add PreToolUse --command "echo b" --name b
hook remove PreToolUse`)

	hooks := result["hooks"].(map[string]any)
	require.NotContains(t, hooks, "PreToolUse")
}

func TestHookAddRequiresCommand(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/crush.sh"
	_, err := LoadShellConfig(path, []byte(`hook add PreToolUse --name x`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "--command is required")
}
