package common

import (
	"encoding/json"
	"fmt"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestThemeStylesFromConfig_ActiveTheme(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Options: &config.Options{
			TUI: &config.TUIOptions{
				ActiveTheme: "gruvbox-dark",
				Theme: map[string]config.ThemeConfig{
					"gruvbox-dark": {},
				},
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
				ActiveTheme: "custom",
				Theme: map[string]config.ThemeConfig{
					"custom": {
						Base:      "gruvbox-dark",
						RawObject: json.RawMessage(`{"base":"gruvbox-dark","primary":"#ff0000"}`),
					},
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
				ActiveTheme: "custom",
				Theme: map[string]config.ThemeConfig{
					"custom": {
						Base:      "gruvbox-dark",
						RawObject: json.RawMessage(`{"base":"gruvbox-dark","primary":"not-a-color"}`),
					},
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

func TestThemeStylesFromConfig_UserFileFallback(t *testing.T) {
	// When config references a theme with no inline override, it should
	// fall through to LoadTheme which checks user files. We verify this
	// by checking that a builtin theme still loads correctly (user files
	// are checked first but absent here).
	cfg := &config.Config{
		Options: &config.Options{
			TUI: &config.TUIOptions{
				ActiveTheme: "gruvbox-dark",
			},
		},
	}
	s := ThemeStylesFromConfig(cfg)
	require.Equal(t, "#fabd2f", testColorHex(s.WorkingGradFromColor))
}

func TestThemeStylesFromConfig_InlineOverridesTakePrecedence(t *testing.T) {
	t.Parallel()
	// Config inline overrides should take precedence over any user file.
	cfg := &config.Config{
		Options: &config.Options{
			TUI: &config.TUIOptions{
				ActiveTheme: "charmtone",
				Theme: map[string]config.ThemeConfig{
					"charmtone": {
						RawObject: json.RawMessage(`{"primary":"#aabbcc"}`),
					},
				},
			},
		},
	}
	s := ThemeStylesFromConfig(cfg)
	require.Equal(t, "#aabbcc", testColorHex(s.WorkingGradFromColor))
}

func TestThemeStylesFromConfig_EmptyObjectFallsThroughToLoadTheme(t *testing.T) {
	t.Parallel()
	// An empty object {} in the theme map should fall through to LoadTheme.
	cfg := &config.Config{
		Options: &config.Options{
			TUI: &config.TUIOptions{
				ActiveTheme: "gruvbox-dark",
				Theme: map[string]config.ThemeConfig{
					"gruvbox-dark": {},
				},
			},
		},
	}
	s := ThemeStylesFromConfig(cfg)
	require.Equal(t, "#fabd2f", testColorHex(s.WorkingGradFromColor))
}

// Verify that ExportResolvedPalette produces a valid theme file that can
// be saved and reloaded through the full resolution pipeline.
func TestExportAndReloadRoundTrip(t *testing.T) {
	dir := t.TempDir()

	exported, err := styles.ExportResolvedPalette("gruvbox-dark")
	require.NoError(t, err)

	path := filepath.Join(dir, "gruvbox-fork.json")
	require.NoError(t, styles.SaveThemeFile(path, exported))

	reloaded, err := styles.LoadThemeFile(path)
	require.NoError(t, err)
	require.Equal(t, exported.Base, reloaded.Base)
	require.Equal(t, exported.Primary, reloaded.Primary)
	require.Equal(t, exported.BgBase, reloaded.BgBase)
}
