package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldExposeDispatcher(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		allowed    []string
		isSubAgent bool
		want       bool
	}{
		{
			name:       "top_level_with_agent_in_allowed",
			allowed:    []string{"bash", AgentToolName},
			isSubAgent: false,
			want:       true,
		},
		{
			name:       "top_level_without_agent_in_allowed",
			allowed:    []string{"bash", "grep"},
			isSubAgent: false,
			want:       false,
		},
		{
			name:       "subagent_with_agent_in_allowed_still_excluded",
			allowed:    []string{"bash", AgentToolName},
			isSubAgent: true,
			want:       false,
		},
		{
			name:       "subagent_without_agent_excluded",
			allowed:    []string{"bash"},
			isSubAgent: true,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, shouldExposeDispatcher(tt.allowed, tt.isSubAgent))
		})
	}
}
