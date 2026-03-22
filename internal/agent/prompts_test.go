package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestIsReadOnlyAgent(t *testing.T) {
	tests := []struct {
		name        string
		allowedTools []string
		expected    bool
	}{
		{
			name:        "nil allowed tools returns false",
			allowedTools: nil,
			expected:    false,
		},
		{
			name:        "empty allowed tools returns false",
			allowedTools: []string{},
			expected:    false,
		},
		{
			name:        "read-only tools returns true",
			allowedTools: []string{"glob", "grep", "ls", "view"},
			expected:    true,
		},
		{
			name:        "read-only tools with sourcegraph returns true",
			allowedTools: []string{"sourcegraph", "view", "grep"},
			expected:    true,
		},
		{
			name:        "mixed tools returns false",
			allowedTools: []string{"glob", "bash", "view"},
			expected:    false,
		},
		{
			name:        "write tools returns false",
			allowedTools: []string{"edit", "write"},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := config.Agent{
				AllowedTools: tt.allowedTools,
			}
			result := isReadOnlyAgent(agent)
			assert.Equal(t, tt.expected, result)
		})
	}
}