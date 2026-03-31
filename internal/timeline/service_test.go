package timeline

import (
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/toolruntime"
	"github.com/stretchr/testify/require"
)

func TestToolEventsFromRuntime(t *testing.T) {
	t.Parallel()

	events := ToolEventsFromRuntime(nil, toolruntime.State{
		SessionID:    "sess-1",
		ToolCallID:   "tool-1",
		ToolName:     "bash",
		Status:       toolruntime.StatusRunning,
		SnapshotText: "hello",
	})
	require.Len(t, events, 2)
	require.Equal(t, EventToolStarted, events[0].Type)
	require.Equal(t, EventToolProgress, events[1].Type)

	events = ToolEventsFromRuntime(&toolruntime.State{
		SessionID:    "sess-1",
		ToolCallID:   "tool-1",
		ToolName:     "bash",
		Status:       toolruntime.StatusRunning,
		SnapshotText: "hello",
	}, toolruntime.State{
		SessionID:    "sess-1",
		ToolCallID:   "tool-1",
		ToolName:     "bash",
		Status:       toolruntime.StatusCompleted,
		SnapshotText: "done",
	})
	require.Len(t, events, 1)
	require.Equal(t, EventToolFinished, events[0].Type)
}

func TestModeChangedEvent(t *testing.T) {
	t.Parallel()

	event := ModeChangedEvent("sess-1", session.ModeTransition{
		Previous: session.ModeState{CollaborationMode: session.CollaborationModeDefault, PermissionMode: session.PermissionModeAuto},
		Current:  session.ModeState{CollaborationMode: session.CollaborationModePlan, PermissionMode: session.PermissionModeYolo},
	})

	require.Equal(t, EventModeChanged, event.Type)
	require.Equal(t, "plan", event.CollaborationMode)
	require.Equal(t, "yolo", event.PermissionMode)
	require.Equal(t, "auto", event.Metadata["previous_permission_mode"])
}
