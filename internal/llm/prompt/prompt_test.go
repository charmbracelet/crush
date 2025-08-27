package prompt

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/home"
)

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected func() string
	}{
		{
			name:  "regular path unchanged",
			input: "/absolute/path",
			expected: func() string {
				return "/absolute/path"
			},
		},
		{
			name:  "tilde expansion",
			input: "~/documents",
			expected: func() string {
				return home.Dir() + "/documents"
			},
		},
		{
			name:  "tilde only",
			input: "~",
			expected: func() string {
				return home.Dir()
			},
		},
		{
			name:  "environment variable expansion",
			input: "$HOME",
			expected: func() string {
				return os.Getenv("HOME")
			},
		},
		{
			name:  "relative path unchanged",
			input: "relative/path",
			expected: func() string {
				return "relative/path"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			expected := tt.expected()

			// Skip test if environment variable is not set
			if strings.HasPrefix(tt.input, "$") && expected == "" {
				t.Skip("Environment variable not set")
			}

			if result != expected {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, expected)
			}
		})
	}
}
