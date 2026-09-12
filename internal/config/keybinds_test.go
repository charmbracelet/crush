package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateKeybinds_DropsUnknown verifies that unknown actions are
// dropped with a warning instead of failing the load. Tokens are kept
// verbatim.
func TestValidateKeybinds_DropsUnknown(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Keybinds: map[string][]string{
			"global.quit": {"ctrl+q"},
			"chat.tab":    {"x"},
			"bogus":       {"y"},
		},
	}
	cfg.ValidateKeybinds()

	require.Equal(t, []string{"ctrl+q"}, cfg.Keybinds["global.quit"])
	require.NotContains(t, cfg.Keybinds, "chat.tab", "excluded binding survived validation")
	require.NotContains(t, cfg.Keybinds, "bogus", "unknown action survived validation")
}

// TestValidateKeybinds_NilIsNoop verifies an empty config loads clean.
func TestValidateKeybinds_NilIsNoop(t *testing.T) {
	t.Parallel()
	cfg := &Config{}
	require.NotPanics(t, func() { cfg.ValidateKeybinds() })
	require.Empty(t, cfg.Keybinds)
}
