package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestThemeConfig_UnmarshalString(t *testing.T) {
	t.Parallel()
	var theme ThemeConfig
	require.NoError(t, json.Unmarshal([]byte(`"gruvbox-dark"`), &theme))
	require.Equal(t, "gruvbox-dark", theme.ThemeName)
	require.Equal(t, "gruvbox-dark", theme.Name())
	require.False(t, theme.IsObject())
	require.False(t, theme.IsZero())
}

func TestThemeConfig_UnmarshalObject(t *testing.T) {
	t.Parallel()
	var theme ThemeConfig
	require.NoError(t, json.Unmarshal([]byte(`{"base":"charmtone","primary":"#ff0000"}`), &theme))
	require.Empty(t, theme.ThemeName)
	require.Equal(t, "charmtone", theme.Base)
	require.Equal(t, "charmtone", theme.Name())
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
	require.Contains(t, err.Error(), "theme must be a string or object")
}

func TestThemeConfig_MarshalString(t *testing.T) {
	t.Parallel()
	theme := ThemeConfig{ThemeName: "gruvbox-dark"}
	data, err := json.Marshal(theme)
	require.NoError(t, err)
	require.JSONEq(t, `"gruvbox-dark"`, string(data))
}

func TestThemeConfig_MarshalObject(t *testing.T) {
	t.Parallel()
	theme := ThemeConfig{RawObject: json.RawMessage(`{"base":"charmtone","primary":"#ff0000"}`)}
	data, err := json.Marshal(theme)
	require.NoError(t, err)
	require.JSONEq(t, `{"base":"charmtone","primary":"#ff0000"}`, string(data))
}
