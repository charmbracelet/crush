package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTUIOptions_UnmarshalLegacyStringTheme(t *testing.T) {
	t.Parallel()
	// Older Crush builds stored the theme as a plain string. Loading such a
	// config must not fail; the string becomes the active theme.
	var opts TUIOptions
	require.NoError(t, json.Unmarshal([]byte(`{"theme":"gruvbox-dark"}`), &opts))
	require.Equal(t, "gruvbox-dark", opts.ActiveTheme)
}

func TestTUIOptions_UnmarshalLegacyStringThemeKeepsActive(t *testing.T) {
	t.Parallel()
	// An explicit active_theme wins over the legacy string.
	var opts TUIOptions
	require.NoError(t, json.Unmarshal(
		[]byte(`{"active_theme":"charmtone","theme":"gruvbox-dark"}`), &opts))
	require.Equal(t, "charmtone", opts.ActiveTheme)
}

func TestTUIOptions_IgnoresLegacyInlineThemeMap(t *testing.T) {
	t.Parallel()
	var opts TUIOptions
	require.NoError(t, json.Unmarshal(
		[]byte(`{"active_theme":"my-theme","theme":{"my-theme":{"base":"charmtone"}}}`), &opts))
	require.Equal(t, "my-theme", opts.ActiveTheme)
}
