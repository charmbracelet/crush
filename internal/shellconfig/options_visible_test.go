package shellconfig

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// option visible-provider giữ đúng thứ tự khai: alt+p liệt kê theo thứ tự đó.
func TestOption_VisibleProvider(t *testing.T) {
	t.Parallel()

	script := `option visible-provider main
option visible-provider openai
option visible-provider anthropic`
	jsonBytes, err := LoadShellConfig(t.Context(), filepath.Join(t.TempDir(), "crushrc"), []byte(script))
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &result))
	opts := result["options"].(map[string]any)
	require.Equal(t, []any{"main", "openai", "anthropic"}, opts["visible_providers"])
}
