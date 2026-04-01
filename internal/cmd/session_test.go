package cmd

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestConvertPartsIncludesToolResultMetadataAndSubtaskResult(t *testing.T) {
	t.Parallel()

	parts := convertParts([]message.ContentPart{
		message.ToolResult{
			ToolCallID: "call-1",
			Name:       "agent",
			Content:    "done",
			Metadata:   `{"foo":"bar"}`,
		}.WithSubtaskResult(message.ToolResultSubtaskResult{
			ChildSessionID:   "child-1",
			ParentToolCallID: "call-1",
			Status:           message.ToolResultSubtaskStatusCompleted,
		}).WithReducer(message.ToolResultReducer{
			Summary:     "Execution finished",
			Artifacts:   []string{"dist/app"},
			Risks:       []string{"network flakiness"},
			NextActions: []string{"monitor"},
			Confidence:  "high",
		}),
	})

	require.Len(t, parts, 1)
	require.Equal(t, "tool_result", parts[0].Type)
	require.Equal(t, "call-1", parts[0].ToolCallID)
	require.NotEmpty(t, parts[0].Metadata)
	require.NotNil(t, parts[0].SubtaskResult)
	require.Equal(t, "child-1", parts[0].SubtaskResult.ChildSessionID)
	require.Equal(t, "call-1", parts[0].SubtaskResult.ParentToolCallID)
	require.Equal(t, "completed", parts[0].SubtaskResult.Status)
	require.NotNil(t, parts[0].Reducer)
	require.Equal(t, "Execution finished", parts[0].Reducer.Summary)
	require.Equal(t, []string{"dist/app"}, parts[0].Reducer.Artifacts)
	require.Equal(t, []string{"network flakiness"}, parts[0].Reducer.Risks)
	require.Equal(t, []string{"monitor"}, parts[0].Reducer.NextActions)
	require.Equal(t, "high", parts[0].Reducer.Confidence)
}
