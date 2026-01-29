package core_test

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/stretchr/testify/require"
)

func TestFormatModelWithProvider(t *testing.T) {
	t.Parallel()

	// Create a simple test style
	testStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	tests := []struct {
		name     string
		model    *catwalk.Model
		provider *config.ProviderConfig
		want     string
	}{
		{
			name: "WithProviderName",
			model: &catwalk.Model{
				Name: "gpt-4o",
			},
			provider: &config.ProviderConfig{
				ID:   "openai",
				Name: "OpenAI",
			},
			want: "OpenAI / gpt-4o",
		},
		{
			name: "WithProviderIDOnly",
			model: &catwalk.Model{
				Name: "claude-3-5-sonnet-20241022",
			},
			provider: &config.ProviderConfig{
				ID:   "anthropic",
				Name: "", // Empty name, should fallback to ID
			},
			want: "anthropic / claude-3-5-sonnet-20241022",
		},
		{
			name: "WithCustomProvider",
			model: &catwalk.Model{
				Name: "alibaba/qwen3-coder",
			},
			provider: &config.ProviderConfig{
				ID:   "vercel",
				Name: "Vercel",
			},
			want: "Vercel / alibaba/qwen3-coder",
		},
		{
			name: "WithoutProvider",
			model: &catwalk.Model{
				Name: "gpt-4o",
			},
			provider: nil,
			want:     "gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := core.FormatModelWithProvider(tt.model, tt.provider, testStyle)

			// Strip ANSI codes to compare plain text
			plainResult := stripANSI(result)
			require.Equal(t, tt.want, plainResult, "formatted model display should match expected")
		})
	}
}

// stripANSI removes ANSI color codes from a string for testing purposes
func stripANSI(s string) string {
	// Simple ANSI stripper for testing
	// In a real scenario, you might want to use a library like github.com/acarl005/stripansi
	result := ""
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}
		result += string(r)
	}
	return result
}
