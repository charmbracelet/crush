package diffview

import (
	"testing"
)

func TestPad(t *testing.T) {
	tests := []struct {
		input    any
		width    int
		expected string
	}{
		{7, 2, " 7"},
		{7, 3, "  7"},
		{"a", 2, " a"},
		{"a", 3, "  a"},
		{"…", 2, " …"},
		{"…", 3, "  …"},
	}

	for _, tt := range tests {
		result := pad(tt.input, tt.width)
		if result != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, result)
		}
	}
}
