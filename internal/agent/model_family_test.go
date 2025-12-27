package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectModelFamily(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		model    string
		expected ModelFamily
	}{
		// Anthropic models
		{"claude-3-5-sonnet", "claude-3-5-sonnet-20241022", ModelFamilyAnthropic},
		{"claude-3-opus", "claude-3-opus-20240229", ModelFamilyAnthropic},
		{"claude-2", "claude-2.1", ModelFamilyAnthropic},
		{"claude-instant", "claude-instant-1.2", ModelFamilyAnthropic},

		// OpenAI models
		{"gpt-4", "gpt-4-turbo", ModelFamilyOpenAI},
		{"gpt-4o", "gpt-4o", ModelFamilyOpenAI},
		{"gpt-3.5-turbo", "gpt-3.5-turbo", ModelFamilyOpenAI},
		{"o1-preview", "o1-preview", ModelFamilyOpenAI},
		{"o1-mini", "o1-mini", ModelFamilyOpenAI},
		{"chatgpt", "chatgpt-4o-latest", ModelFamilyOpenAI},

		// Google models
		{"gemini-pro", "gemini-pro", ModelFamilyGoogle},
		{"gemini-1.5-pro", "gemini-1.5-pro-latest", ModelFamilyGoogle},
		{"gemini-1.5-flash", "gemini-1.5-flash-002", ModelFamilyGoogle},

		// Default/unknown models
		{"llama", "llama-3-70b", ModelFamilyDefault},
		{"mistral", "mistral-large", ModelFamilyDefault},
		{"unknown", "some-unknown-model", ModelFamilyDefault},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := DetectModelFamily(tt.model)
			require.Equal(t, tt.expected, result, "model: %s", tt.model)
		})
	}
}
