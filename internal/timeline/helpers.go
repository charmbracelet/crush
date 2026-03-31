package timeline

import (
	"strings"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/toolruntime"
)

func ModeChangedEvent(sessionID string, transition session.ModeTransition) Event {
	return Event{
		SessionID:         sessionID,
		Type:              EventModeChanged,
		CollaborationMode: string(transition.Current.CollaborationMode),
		PermissionMode:    string(transition.Current.PermissionMode),
		Metadata: map[string]any{
			"previous_collaboration_mode": string(transition.Previous.CollaborationMode),
			"previous_permission_mode":    string(transition.Previous.PermissionMode),
		},
	}
}

func ChildSessionStartedEvent(parentSessionID, childSessionID, title string) Event {
	return Event{
		SessionID:      parentSessionID,
		Type:           EventChildSessionStarted,
		ChildSessionID: childSessionID,
		Title:          title,
	}
}

func ChildSessionFinishedEvent(parentSessionID, childSessionID, title, status, content string) Event {
	return Event{
		SessionID:      parentSessionID,
		Type:           EventChildSessionFinished,
		ChildSessionID: childSessionID,
		Title:          title,
		Status:         status,
		Content:        strings.TrimSpace(content),
	}
}

func ToolEventsFromRuntime(previous *toolruntime.State, current toolruntime.State) []Event {
	if current.SessionID == "" || current.ToolCallID == "" || current.ToolName == "" {
		return nil
	}

	snapshot := strings.TrimSpace(current.SnapshotText)
	previousSnapshot := ""
	if previous != nil {
		previousSnapshot = strings.TrimSpace(previous.SnapshotText)
	}

	isRunning := current.Status == toolruntime.StatusRunning || current.Status == toolruntime.StatusBackgroundRunning
	wasRunning := previous != nil && (previous.Status == toolruntime.StatusRunning || previous.Status == toolruntime.StatusBackgroundRunning)

	events := make([]Event, 0, 2)
	if isRunning && !wasRunning {
		events = append(events, Event{
			SessionID:  current.SessionID,
			Type:       EventToolStarted,
			ToolCallID: current.ToolCallID,
			ToolName:   current.ToolName,
			Title:      current.ToolName,
			Status:     string(current.Status),
		})
	}
	if isRunning && snapshot != "" && snapshot != previousSnapshot {
		events = append(events, Event{
			SessionID:  current.SessionID,
			Type:       EventToolProgress,
			ToolCallID: current.ToolCallID,
			ToolName:   current.ToolName,
			Title:      current.ToolName,
			Status:     string(current.Status),
			Content:    snapshot,
		})
	}
	if current.Status == toolruntime.StatusCompleted || current.Status == toolruntime.StatusFailed || current.Status == toolruntime.StatusCanceled {
		events = append(events, Event{
			SessionID:  current.SessionID,
			Type:       EventToolFinished,
			ToolCallID: current.ToolCallID,
			ToolName:   current.ToolName,
			Title:      current.ToolName,
			Status:     string(current.Status),
			Content:    snapshot,
		})
	}
	return events
}
