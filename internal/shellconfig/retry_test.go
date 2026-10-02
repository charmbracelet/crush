package shellconfig

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetryShellConfig(t *testing.T) {
	t.Parallel()
	script := `option retry max-retries 0
option retry initial-delay-ms 100
option retry jitter none
provider add example --retry '{"max_retries":5}'`
	data, err := LoadShellConfig(t.Context(), filepath.Join(t.TempDir(), "crushrc"), []byte(script))
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	global := decoded["options"].(map[string]any)["retry"].(map[string]any)
	require.Equal(t, float64(0), global["max_retries"])
	require.Equal(t, float64(100), global["initial_delay_ms"])
	require.Equal(t, "none", global["jitter"])
	provider := decoded["providers"].(map[string]any)["example"].(map[string]any)["retry"].(map[string]any)
	require.Equal(t, float64(5), provider["max_retries"])
}
