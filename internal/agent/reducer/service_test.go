package reducer

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestReduce(t *testing.T) {
	t.Run("all completed", func(t *testing.T) {
		result := Reduce([]TaskResult{
			{ID: "a", Description: "Fetch", Status: message.ToolResultSubtaskStatusCompleted, ChildSessionID: "s1"},
			{ID: "b", Description: "Analyze", Status: message.ToolResultSubtaskStatusCompleted, ChildSessionID: "s2"},
		})

		require.Equal(t, "Completed 2/2 subtasks.", result.Summary)
		require.Equal(t, "high", result.Confidence)
		require.Len(t, result.Artifacts, 2)
		require.Empty(t, result.Risks)
		require.Contains(t, result.NextActions, "Review child session outputs and integrate accepted results.")
	})

	t.Run("failures and cancellations", func(t *testing.T) {
		result := Reduce([]TaskResult{
			{ID: "a", Description: "Root", Status: message.ToolResultSubtaskStatusFailed, Content: "boom"},
			{ID: "b", Description: "Child", Status: message.ToolResultSubtaskStatusCanceled, Content: "blocked"},
		})

		require.Equal(t, "Completed 0/2 subtasks (1 failed, 1 canceled).", result.Summary)
		require.Equal(t, "low", result.Confidence)
		require.Len(t, result.Risks, 2)
		require.Contains(t, result.NextActions, "Address failed or canceled subtasks before finalizing.")
	})
}
