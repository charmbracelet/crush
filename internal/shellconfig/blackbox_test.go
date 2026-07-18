package shellconfig_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/shellconfig"
	"github.com/stretchr/testify/require"
)

// loadConfig runs a crush.sh script through the public LoadShellConfig entry
// point and returns the resulting config as a decoded JSON object. It is the
// shared harness for the black-box tests, which exercise only the exported
// API (no access to package internals).
func loadConfig(t *testing.T, script string) map[string]any {
	t.Helper()
	path := filepath.Join(t.TempDir(), "crush.sh")
	out, err := shellconfig.LoadShellConfig(path, []byte(script))
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(out, &result))
	return result
}

// loadConfigErr runs a script and returns the error from LoadShellConfig,
// asserting that one occurred. Used to verify usage/validation failures.
func loadConfigErr(t *testing.T, script string) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), "crush.sh")
	_, err := shellconfig.LoadShellConfig(path, []byte(script))
	require.Error(t, err)
	return err
}
