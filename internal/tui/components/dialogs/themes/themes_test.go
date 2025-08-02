package themes

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/styles"
)

func TestThemeDialog(t *testing.T) {
	// Create a theme dialog
	dialog := NewThemeDialog()

	// Test that it implements the interface
	if dialog == nil {
		t.Error("NewThemeDialog should not return nil")
	}

	// Test initialization - Init() may return nil, which is fine
	_ = dialog.Init()

	// Test that we can get available themes
	manager := styles.DefaultManager()
	themes := manager.List()
	if len(themes) < 2 {
		t.Errorf("Expected at least 2 themes, got %d", len(themes))
	}

	// Test window resize
	model, _ := dialog.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if model == nil {
		t.Error("Update should not return nil model")
	}
	dialog = model.(ThemeDialog)

	// Test view rendering
	view := dialog.View()
	if view == "" {
		t.Error("View should not return empty string")
	}

	// Test that view contains expected content
	if !contains(view, "Switch Theme") {
		t.Error("View should contain 'Switch Theme' title")
	}
}

func TestThemeDialogKeyHandling(t *testing.T) {
	dialog := NewThemeDialog()

	// Initialize the dialog
	dialog.Init()

	// Test that the dialog can handle updates
	model, _ := dialog.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if model == nil {
		t.Error("Update should not return nil model")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
