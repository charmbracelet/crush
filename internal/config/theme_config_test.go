package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestThemeConfig_UnmarshalObject(t *testing.T) {
	t.Parallel()
	var theme ThemeConfig
	require.NoError(t, json.Unmarshal([]byte(`{"base":"charmtone","primary":"#ff0000"}`), &theme))
	require.Equal(t, "charmtone", theme.Base)
	require.True(t, theme.IsObject())
	require.False(t, theme.IsZero())
	require.JSONEq(t, `{"base":"charmtone","primary":"#ff0000"}`, string(theme.RawObject))
}

func TestThemeConfig_UnmarshalNull(t *testing.T) {
	t.Parallel()
	var theme ThemeConfig
	require.NoError(t, json.Unmarshal([]byte(`null`), &theme))
	require.True(t, theme.IsZero())
}

func TestThemeConfig_UnmarshalInvalid(t *testing.T) {
	t.Parallel()
	var theme ThemeConfig
	err := json.Unmarshal([]byte(`123`), &theme)
	require.Error(t, err)
	require.Contains(t, err.Error(), "theme config must be an object")
}

func TestThemeConfig_MarshalObject(t *testing.T) {
	t.Parallel()
	theme := ThemeConfig{RawObject: json.RawMessage(`{"base":"charmtone","primary":"#ff0000"}`)}
	data, err := json.Marshal(theme)
	require.NoError(t, err)
	require.JSONEq(t, `{"base":"charmtone","primary":"#ff0000"}`, string(data))
}

func TestThemeConfig_MarshalZero(t *testing.T) {
	t.Parallel()
	theme := ThemeConfig{}
	data, err := json.Marshal(theme)
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(data))
}

func TestTUIOptions_UnmarshalLegacyStringTheme(t *testing.T) {
	t.Parallel()
	// Older Crush builds stored the theme as a plain string. Loading such a
	// config must not fail; the string becomes the active theme.
	var opts TUIOptions
	require.NoError(t, json.Unmarshal([]byte(`{"theme":"gruvbox-dark"}`), &opts))
	require.Equal(t, "gruvbox-dark", opts.ActiveTheme)
	require.Contains(t, opts.Theme, "gruvbox-dark")
}

func TestTUIOptions_UnmarshalLegacyStringThemeKeepsActive(t *testing.T) {
	t.Parallel()
	// An explicit active_theme wins over the legacy string.
	var opts TUIOptions
	require.NoError(t, json.Unmarshal(
		[]byte(`{"active_theme":"charmtone","theme":"gruvbox-dark"}`), &opts))
	require.Equal(t, "charmtone", opts.ActiveTheme)
}

func TestTUIOptions_UnmarshalMapTheme(t *testing.T) {
	t.Parallel()
	var opts TUIOptions
	require.NoError(t, json.Unmarshal(
		[]byte(`{"active_theme":"my-theme","theme":{"my-theme":{"base":"charmtone"}}}`), &opts))
	require.Equal(t, "my-theme", opts.ActiveTheme)
	require.Equal(t, "charmtone", opts.Theme["my-theme"].Base)
}
