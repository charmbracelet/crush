package common

import (
	"encoding/json"
	"fmt"
	"image/color"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestThemeStylesFromConfig_StringTheme(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Options: &config.Options{
			TUI: &config.TUIOptions{
				Theme: config.ThemeConfig{ThemeName: "gruvbox-dark"},
			},
		},
	}

	s := ThemeStylesFromConfig(cfg)
	require.Equal(t, "#fabd2f", testColorHex(s.WorkingGradFromColor))
}

func TestThemeStylesFromConfig_ObjectTheme(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Options: &config.Options{
			TUI: &config.TUIOptions{
				Theme: config.ThemeConfig{
					Base:      "gruvbox-dark",
					RawObject: json.RawMessage(`{"base":"gruvbox-dark","primary":"#ff0000"}`),
				},
			},
		},
	}

	s := ThemeStylesFromConfig(cfg)
	require.Equal(t, "#ff0000", testColorHex(s.WorkingGradFromColor))
}

func TestThemeStylesFromConfig_InvalidObjectFallsBackToBase(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Options: &config.Options{
			TUI: &config.TUIOptions{
				Theme: config.ThemeConfig{
					Base:      "gruvbox-dark",
					RawObject: json.RawMessage(`{"base":"gruvbox-dark","primary":"not-a-color"}`),
				},
			},
		},
	}

	s := ThemeStylesFromConfig(cfg)
	require.Equal(t, "#fabd2f", testColorHex(s.WorkingGradFromColor))
}

func testColorHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}
