package chat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentRenderLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		subagentType string
		want         string
	}{
		{"empty_returns_agent", "", "Agent"},
		{"task_returns_agent", "task", "Agent"},
		{"named_subagent_prefixed", "code-reviewer", "Agent: code-reviewer"},
		{"another_named", "tester", "Agent: tester"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, agentRenderLabel(tt.subagentType))
		})
	}
}
