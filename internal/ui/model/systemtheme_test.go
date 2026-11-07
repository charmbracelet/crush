package model

import (
	"image/color"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func TestSystemThemeAppliesDetectedPalette(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{ActiveTheme: styles.SystemThemeName}}}
	m := newHostThemeTestUI(t, cfg)
	m.caps.TerminalPalette = systemThemeTestPalette()

	m.applySystemThemeFromTerminal(0)

	require.Equal(t, "#101010", hexOf(m.com.Styles.Background))
	require.Equal(t, "#00cccc", hexOf(m.com.Styles.WorkingGradFromColor))
	require.Equal(t, "#00cccc", hexOf(m.com.Styles.ANSI[6]))
}

func TestTerminalColorEventsRefreshSystemTheme(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{ActiveTheme: styles.SystemThemeName}}}
	m := newHostThemeTestUI(t, cfg)
	m.caps.TerminalPalette = systemThemeTestPalette()

	_, cmd := m.Update(uv.BackgroundColorEvent{Color: color.NRGBA{R: 0x20, G: 0x30, B: 0x40, A: 0xff}})
	require.NotNil(t, cmd)
	require.EqualValues(t, 1, m.systemThemeRevision)

	m.applySystemThemeFromTerminal(m.systemThemeRevision)
	require.Equal(t, "#203040", hexOf(m.com.Styles.Background))
}

func TestSystemThemeRefreshHonorsPreviewAndHostPrecedence(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{ActiveTheme: styles.SystemThemeName}}}
	m := newHostThemeTestUI(t, cfg)

	require.True(t, m.shouldApplySystemTheme())

	m.hostTheme.active = true
	require.False(t, m.shouldApplySystemTheme())

	m.preThemeStyles = new(styles.Styles)
	m.themePreviewName = "gruvbox-dark"
	require.False(t, m.shouldApplySystemTheme())

	m.themePreviewName = styles.SystemThemeName
	require.True(t, m.shouldApplySystemTheme())
}

func TestSystemThemePaletteIsQueriedOnlyWhenNeeded(t *testing.T) {
	t.Parallel()

	m := newHostThemeTestUI(t, nil)
	require.False(t, m.wantsSystemThemePalette())

	m.preThemeStyles = new(styles.Styles)
	m.themePreviewName = styles.SystemThemeName
	require.True(t, m.wantsSystemThemePalette())

	cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{ActiveTheme: styles.SystemThemeName}}}
	require.True(t, newHostThemeTestUI(t, cfg).wantsSystemThemePalette())
}

func TestSystemThemePaletteNotificationsDebounce(t *testing.T) {
	t.Parallel()
	require.Equal(t, time.Second, systemThemePaletteNotificationDelay)

	cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{ActiveTheme: styles.SystemThemeName}}}
	m := newHostThemeTestUI(t, cfg)
	require.NotNil(t, m.systemThemePaletteNotificationCmd())
	firstRevision := m.systemThemeNotificationRevision
	require.NotNil(t, m.systemThemePaletteNotificationCmd())
	secondRevision := m.systemThemeNotificationRevision

	require.Greater(t, secondRevision, firstRevision)
	require.Nil(t, m.handleSystemThemePaletteNotification(firstRevision))
	require.NotNil(t, m.handleSystemThemePaletteNotification(secondRevision))
}

func TestSystemThemePaletteNotificationIgnoredAfterThemeChange(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{ActiveTheme: styles.SystemThemeName}}}
	m := newHostThemeTestUI(t, cfg)
	require.NotNil(t, m.systemThemePaletteNotificationCmd())
	revision := m.systemThemeNotificationRevision

	require.NoError(t, m.com.Workspace.SetConfigField(config.ScopeGlobal, "options.tui.active_theme", "gruvbox-dark"))
	require.Nil(t, m.handleSystemThemePaletteNotification(revision))
}

func TestIsTerminalThemeNotification(t *testing.T) {
	t.Parallel()

	require.True(t, isTerminalThemeNotification("\x1b[?997;1n"))
	require.True(t, isTerminalThemeNotification("\x1b[?997;2n"))
	require.False(t, isTerminalThemeNotification("\x1b[?25;1$y"))
}

func TestSystemThemePaletteStatusShowsDetectedDefaults(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"System theme detected terminal colors: fg #f0f0f0, bg #101010",
		systemThemePaletteStatus(systemThemeTestPalette()),
	)
}

func TestClearingHostThemeRestoresDetectedSystemTheme(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{ActiveTheme: styles.SystemThemeName}}}
	m := newHostThemeTestUI(t, cfg)
	m.caps.TerminalPalette = systemThemeTestPalette()

	deliverHostTheme(t, m, hostTheme("#123456"))
	deliverHostTheme(t, m, nil)

	require.Equal(t, "#101010", hexOf(m.com.Styles.Background))
	require.Equal(t, "#00cccc", hexOf(m.com.Styles.WorkingGradFromColor))
}

func TestSystemThemePreviewRevertUsesLatestPalette(t *testing.T) {
	cfg := &config.Config{Options: &config.Options{TUI: &config.TUIOptions{ActiveTheme: styles.SystemThemeName}}}
	m := newHostThemeTestUI(t, cfg)
	previous := m.com.Styles.Clone()
	m.preThemeStyles = &previous
	m.themePreviewName = "gruvbox-dark"
	m.caps.TerminalPalette = styles.TerminalPalette{
		Foreground: color.NRGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff},
		Background: color.NRGBA{R: 0xf8, G: 0xf8, B: 0xf8, A: 0xff},
	}

	m.restoreThemePreview()

	require.Equal(t, "#f8f8f8", hexOf(m.com.Styles.Background))
	require.Nil(t, m.preThemeStyles)
	require.Empty(t, m.themePreviewName)
}

func systemThemeTestPalette() styles.TerminalPalette {
	var ansiColors [16]color.Color
	for index := range ansiColors {
		ansiColors[index] = color.NRGBA{R: uint8(index * 12), G: uint8(index * 9), B: uint8(index * 7), A: 0xff}
	}
	ansiColors[5] = color.NRGBA{R: 0xcc, G: 0, B: 0xcc, A: 0xff}
	ansiColors[6] = color.NRGBA{R: 0, G: 0xcc, B: 0xcc, A: 0xff}
	return styles.TerminalPalette{
		Foreground: color.NRGBA{R: 0xf0, G: 0xf0, B: 0xf0, A: 0xff},
		Background: color.NRGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff},
		ANSI:       ansiColors,
	}
}
