package shellconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyAdd(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `verify add --command "go test ./..." --name tests --timeout 300
verify add --command "golangci-lint run"`)

	arr := result["verify"].([]any)
	require.Len(t, arr, 2)
	require.Equal(t, "go test ./...", arr[0].(map[string]any)["command"])
	require.Equal(t, "tests", arr[0].(map[string]any)["name"])
	require.Equal(t, float64(300), arr[0].(map[string]any)["timeout"])
	require.Equal(t, "golangci-lint run", arr[1].(map[string]any)["command"])
}

func TestVerifyRemoveByName(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `verify add --command "echo a" --name a
verify add --command "echo b" --name b
verify remove --name a`)

	arr := result["verify"].([]any)
	require.Len(t, arr, 1)
	require.Equal(t, "b", arr[0].(map[string]any)["name"])
}

func TestVerifyRemoveAll(t *testing.T) {
	t.Parallel()

	// Clearing the only config section produces an empty config —
	// LoadShellConfig returns nil when the builder has nothing.
	path := t.TempDir() + "/crushrc"
	data, err := LoadShellConfig(t.Context(), path, []byte(`verify add --command "echo a" --name a
verify rm`))
	require.NoError(t, err)
	require.Empty(t, data)
}

func TestVerifyAddRequiresCommand(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/crushrc"
	_, err := LoadShellConfig(t.Context(), path, []byte(`verify add --name x`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "--command is required")
}
