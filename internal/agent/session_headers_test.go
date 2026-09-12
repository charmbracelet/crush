package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/assert"
)

func TestSessionHeaders(t *testing.T) {
	t.Parallel()

	const sessionID = "0199c0a5-1111-7000-8000-000000000000"
	hash := session.HashID(sessionID)

	tests := []struct {
		name       string
		providerID string
		opencode   bool
	}{
		{"opencode go", string(catwalk.InferenceProviderOpenCodeGo), true},
		{"opencode zen", string(catwalk.InferenceProviderOpenCodeZen), true},
		{"anthropic", string(catwalk.InferenceProviderAnthropic), false},
		{"openai", string(catwalk.InferenceProviderOpenAI), false},
		{"custom provider", "my-custom-provider", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			headers := sessionHeaders(sessionID, tt.providerID)
			assert.Equal(t, hash, headers["x-session-id"])
			assert.Equal(t, hash, headers["x-session-affinity"])
			if tt.opencode {
				assert.Equal(t, hash, headers["x-opencode-session"])
			} else {
				assert.NotContains(t, headers, "x-opencode-session")
			}
		})
	}
}
