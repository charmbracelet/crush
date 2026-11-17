package model

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

const systemThemeRefreshDelay = 75 * time.Millisecond
const systemThemePaletteNotificationDelay = time.Second

type systemThemeRefreshMsg struct {
	revision uint64
}

type systemThemePaletteNotificationMsg struct {
	revision uint64
}

func isTerminalThemeNotification(sequence string) bool {
	return sequence == "\x1b[?997;1n" || sequence == "\x1b[?997;2n"
}

func systemThemePaletteStatus(palette styles.TerminalPalette) string {
	foreground := palette.Foreground
	if foreground == nil {
		foreground = palette.ANSI[7]
	}
	background := palette.Background
	if background == nil {
		background = palette.ANSI[0]
	}
	return fmt.Sprintf("System theme detected terminal colors: fg %s, bg %s", terminalColorHex(foreground), terminalColorHex(background))
}

func terminalColorHex(c color.Color) string {
	if c == nil {
		return "unavailable"
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

func (m *UI) resolveThemeStyles(name string) (styles.Styles, error) {
	if strings.EqualFold(name, styles.SystemThemeName) && m.caps.TerminalPalette.IsUsable() {
		return styles.SystemThemeStyles(m.caps.TerminalPalette), nil
	}
	return styles.LoadTheme(name)
}

func (m *UI) systemThemeRefreshCmd() tea.Cmd {
	if !m.shouldApplySystemTheme() {
		return nil
	}
	m.systemThemeRevision++
	revision := m.systemThemeRevision
	return tea.Tick(systemThemeRefreshDelay, func(time.Time) tea.Msg {
		return systemThemeRefreshMsg{revision: revision}
	})
}

func (m *UI) systemThemePaletteNotificationCmd() tea.Cmd {
	if !m.wantsSystemThemePalette() {
		return nil
	}
	m.systemThemeNotificationRevision++
	revision := m.systemThemeNotificationRevision
	return tea.Tick(systemThemePaletteNotificationDelay, func(time.Time) tea.Msg {
		return systemThemePaletteNotificationMsg{revision: revision}
	})
}

func (m *UI) handleSystemThemePaletteNotification(revision uint64) tea.Cmd {
	if revision != m.systemThemeNotificationRevision || !m.wantsSystemThemePalette() {
		return nil
	}
	return common.QueryTerminalPaletteCmd(m.caps.Env, m.caps.TerminalVersion)
}

func (m *UI) shouldApplySystemTheme() bool {
	previewingSystem := m.preThemeStyles != nil && strings.EqualFold(m.themePreviewName, styles.SystemThemeName)
	if previewingSystem {
		return true
	}
	return m.preThemeStyles == nil && !m.hostTheme.active &&
		strings.EqualFold(common.ThemeNameFromConfig(m.com.Config()), styles.SystemThemeName)
}

func (m *UI) wantsSystemThemePalette() bool {
	return strings.EqualFold(common.ThemeNameFromConfig(m.com.Config()), styles.SystemThemeName) ||
		(m.preThemeStyles != nil && strings.EqualFold(m.themePreviewName, styles.SystemThemeName))
}

func (m *UI) applySystemThemeFromTerminal(revision uint64) {
	if revision != m.systemThemeRevision || !m.shouldApplySystemTheme() || !m.caps.TerminalPalette.IsUsable() {
		return
	}
	s := styles.SystemThemeStyles(m.caps.TerminalPalette)
	if m.preThemeStyles != nil {
		m.previewTheme(s)
		return
	}
	m.applyTheme(s)
}

func (m *UI) restoreThemePreview() {
	if m.preThemeStyles == nil {
		m.themePreviewName = ""
		return
	}
	if !m.hostTheme.active && strings.EqualFold(common.ThemeNameFromConfig(m.com.Config()), styles.SystemThemeName) &&
		m.caps.TerminalPalette.IsUsable() {
		m.applyTheme(styles.SystemThemeStyles(m.caps.TerminalPalette))
	} else {
		m.applyTheme(*m.preThemeStyles)
	}
	m.preThemeStyles = nil
	m.themePreviewName = ""
}
