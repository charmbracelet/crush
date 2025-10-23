package themes

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/styles"
)

// Manager handles theme loading, switching, and persistence
type Manager struct {
	themes  map[string]*styles.Theme
	current string
	config  *config.Config
}

// NewManager creates a new theme manager
func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		themes: make(map[string]*styles.Theme),
		config: cfg,
	}

	// Load all preset themes
	for _, theme := range styles.GetAllThemes() {
		m.themes[theme.Name] = theme
	}

	// Set current theme from config or default
	var currentTheme string
	if cfg.Options != nil && cfg.Options.TUI != nil {
		currentTheme = cfg.Options.TUI.Theme
	}
	if currentTheme == "" {
		currentTheme = "charmtone" // default theme
	}

	// Validate that the theme exists
	if _, exists := m.themes[currentTheme]; !exists {
		slog.Warn("Theme not found, using default", "theme", currentTheme)
		currentTheme = "charmtone"
	}

	m.current = currentTheme
	return m
}

// LoadTheme loads a theme by name
func (m *Manager) LoadTheme(name string) (*styles.Theme, error) {
	theme, exists := m.themes[name]
	if !exists {
		return nil, fmt.Errorf("theme %s not found", name)
	}
	return theme, nil
}

// ListThemes returns all available theme names
func (m *Manager) ListThemes() []string {
	names := make([]string, 0, len(m.themes))
	for name := range m.themes {
		names = append(names, name)
	}
	return names
}

// SetTheme sets the current theme and persists it
func (m *Manager) SetTheme(name string) error {
	if _, exists := m.themes[name]; !exists {
		return fmt.Errorf("theme %s not found", name)
	}

	// Update config
	if m.config.Options == nil {
		m.config.Options = &config.Options{}
	}
	if m.config.Options.TUI == nil {
		m.config.Options.TUI = &config.TUIOptions{}
	}

	// Save theme to config
	if err := m.config.SetConfigField("options.tui.theme", name); err != nil {
		return fmt.Errorf("failed to save theme to config: %w", err)
	}

	m.config.Options.TUI.Theme = name
	m.current = name

	// Update the global theme manager
	styles.DefaultManager().SetTheme(name)

	slog.Info("Theme changed", "theme", name)
	return nil
}

// Current returns the current theme
func (m *Manager) Current() *styles.Theme {
	return m.themes[m.current]
}

// CurrentName returns the name of the current theme
func (m *Manager) CurrentName() string {
	return m.current
}

// ValidateTheme checks if a theme name is valid
func (m *Manager) ValidateTheme(name string) error {
	if _, exists := m.themes[name]; !exists {
		return fmt.Errorf("theme %s not found", name)
	}
	return nil
}

// GetThemeInfo returns information about a theme
func (m *Manager) GetThemeInfo(name string) (map[string]interface{}, error) {
	theme, exists := m.themes[name]
	if !exists {
		return nil, fmt.Errorf("theme %s not found", name)
	}

	return map[string]interface{}{
		"name":   theme.Name,
		"isDark": theme.IsDark,
	}, nil
}

// LoadThemeFromConfig loads a theme from the configuration directory
func (m *Manager) LoadThemeFromConfig(themePath string) error {
	// This is for future extensibility - loading custom themes from files
	// For now, we only support preset themes
	if !strings.HasSuffix(themePath, ".yaml") && !strings.HasSuffix(themePath, ".yml") {
		return fmt.Errorf("theme file must be a YAML file")
	}

	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		return fmt.Errorf("theme file does not exist: %s", themePath)
	}

	// TODO: Implement custom theme loading from YAML files
	// This would involve parsing the YAML and creating a Theme struct
	return fmt.Errorf("custom theme loading not yet implemented")
}

// ExportTheme exports a theme to a YAML file
func (m *Manager) ExportTheme(themeName, outputPath string) error {
	if _, exists := m.themes[themeName]; !exists {
		return fmt.Errorf("theme %s not found", themeName)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// TODO: Implement theme export to YAML
	// This would involve converting the Theme struct to YAML format
	return fmt.Errorf("theme export not yet implemented")
}
