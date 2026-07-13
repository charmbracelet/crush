package tools

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestGoalStatusToolAcceptsTerminalStates(t *testing.T) {
	t.Parallel()

	tool := NewGoalStatusTool()
	for _, status := range []string{"complete", "blocked"} {
		response, err := tool.Run(t.Context(), fantasy.ToolCall{
			Name:  GoalStatusToolName,
			Input: `{"status":"` + status + `","summary":"verified outcome"}`,
		})
		require.NoError(t, err)
		require.False(t, response.IsError)
		require.Contains(t, response.Content, "Goal "+status)
	}
}

func TestGoalStatusToolRejectsNonTerminalState(t *testing.T) {
	t.Parallel()

	response, err := NewGoalStatusTool().Run(t.Context(), fantasy.ToolCall{
		Name:  GoalStatusToolName,
		Input: `{"status":"in_progress","summary":"still working"}`,
	})
	require.Error(t, err)
	require.Empty(t, response.Content)
}
