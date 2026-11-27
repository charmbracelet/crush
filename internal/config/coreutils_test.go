package config_test

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOptionsGetCoreutils(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		options  *config.Options
		expected config.CoreutilsMode
	}{
		{name: "nil options", options: nil, expected: config.DefaultCoreutils},
		{name: "unset field", options: &config.Options{}, expected: config.DefaultCoreutils},
		{name: "turtle", options: &config.Options{Coreutils: config.CoreutilsTurtle}, expected: config.CoreutilsTurtle},
		{name: "system", options: &config.Options{Coreutils: config.CoreutilsSystem}, expected: config.CoreutilsSystem},
		{name: "off", options: &config.Options{Coreutils: config.CoreutilsOff}, expected: config.CoreutilsOff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.expected, tt.options.GetCoreutils())
		})
	}
}

func TestOptionsCoreutilsFromJSON(t *testing.T) {
	t.Parallel()

	var cfg config.Config
	require.NoError(t, json.Unmarshal([]byte(`{"options":{"coreutils":"turtle"}}`), &cfg))
	require.Equal(t, config.CoreutilsTurtle, cfg.Options.Coreutils)
	require.Equal(t, config.CoreutilsTurtle, cfg.Options.GetCoreutils())
	require.NoError(t, cfg.ValidateCoreutils())
}

func TestConfigValidateCoreutils(t *testing.T) {
	t.Parallel()

	for _, mode := range []config.CoreutilsMode{"", "auto", "turtle", "system", "off"} {
		cfg := &config.Config{Options: &config.Options{Coreutils: mode}}
		require.NoError(t, cfg.ValidateCoreutils(), "mode %q should be valid", mode)
	}

	cfg := &config.Config{Options: &config.Options{Coreutils: "bogus"}}
	err := cfg.ValidateCoreutils()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown coreutils mode")
}

func TestShellConfigOptionCoreutils(t *testing.T) {
	store := loadCrushSh(t, `option coreutils turtle`)

	require.Equal(t, config.CoreutilsTurtle, store.Config().Options.Coreutils)
	require.Equal(t, config.CoreutilsTurtle, store.Config().Options.GetCoreutils())
}

func TestShellConfigOptionCoreutilsRejectsInvalidValue(t *testing.T) {
	_, err := loadCrushShErr(t, `option coreutils bogus`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expects turtle, system, auto, or off")
}
