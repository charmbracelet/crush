package reducer

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestReduce(t *testing.T) {
	t.Run("all completed", func(t *testing.T) {
		result := Reduce([]TaskResult{
			{ID: "a", Description: "Fetch", Status: message.ToolResultSubtaskStatusCompleted, ChildSessionID: "s1", FilesTouched: []string{"/tmp/a.go"}, PatchPlan: []string{"collect baseline"}, TestResults: []string{"fetch smoke passed"}, Followups: []string{"Need API throttling review?"}},
			{ID: "b", Description: "Analyze", Status: message.ToolResultSubtaskStatusCompleted, ChildSessionID: "s2", Artifacts: []string{"shell:bg-1"}, FilesTouched: []string{"/tmp/b.go"}, PatchPlan: []string{"apply optimization"}, TestResults: []string{"analysis tests passed"}, Followups: []string{"Should we add benchmark CI?"}},
		})

		require.Equal(t, "Completed 2/2 subtasks.", result.Summary)
		require.Equal(t, "high", result.Confidence)
		require.Len(t, result.Artifacts, 3)
		require.Equal(t, []string{"/tmp/a.go", "/tmp/b.go"}, result.FilesTouched)
		require.Equal(t, []string{"collect baseline", "apply optimization"}, result.PatchPlan)
		require.Equal(t, []string{"fetch smoke passed", "analysis tests passed"}, result.TestResults)
		require.Equal(t, []string{"Need API throttling review?", "Should we add benchmark CI?"}, result.FollowupQuestions)
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
