package styles

import (
	"testing"
)

func TestCatppuccinLatteTheme(t *testing.T) {
	theme := NewCatppuccinLatteTheme()

	// Test basic properties
	if theme.Name != "catppuccin-latte" {
		t.Errorf("Expected theme name 'catppuccin-latte', got '%s'", theme.Name)
	}

	if theme.IsDark {
		t.Error("Expected Catppuccin Latte to be a light theme (IsDark = false)")
	}

	// Test that colors are properly set (not nil)
	if theme.Primary == nil {
		t.Error("Primary color should not be nil")
	}

	if theme.BgBase == nil {
		t.Error("BgBase color should not be nil")
	}

	if theme.FgBase == nil {
		t.Error("FgBase color should not be nil")
	}

	// Test that styles can be built
	styles := theme.S()
	if styles == nil {
		t.Error("Styles should not be nil")
	}

	// Test that the theme can be registered in a manager
	manager := &Manager{
		themes: make(map[string]*Theme),
	}
	manager.Register(theme)

	if len(manager.themes) != 1 {
		t.Errorf("Expected 1 theme in manager, got %d", len(manager.themes))
	}

	if manager.themes["catppuccin-latte"] == nil {
		t.Error("Theme should be registered with correct name")
	}
}

func TestThemeManager(t *testing.T) {
	// Test that both themes are registered in the default manager
	manager := NewManager("crush")

	themes := manager.List()
	if len(themes) != 2 {
		t.Errorf("Expected 2 themes, got %d", len(themes))
	}

	// Check that both themes exist
	foundCrush := false
	foundCatppuccin := false
	for _, name := range themes {
		if name == "crush" {
			foundCrush = true
		}
		if name == "catppuccin-latte" {
			foundCatppuccin = true
		}
	}

	if !foundCrush {
		t.Error("Expected to find 'crush' theme")
	}

	if !foundCatppuccin {
		t.Error("Expected to find 'catppuccin-latte' theme")
	}

	// Test switching themes
	err := manager.SetTheme("catppuccin-latte")
	if err != nil {
		t.Errorf("Failed to set theme: %v", err)
	}

	current := manager.Current()
	if current.Name != "catppuccin-latte" {
		t.Errorf("Expected current theme to be 'catppuccin-latte', got '%s'", current.Name)
	}

	// Test switching to non-existent theme
	err = manager.SetTheme("non-existent")
	if err == nil {
		t.Error("Expected error when setting non-existent theme")
	}
}
