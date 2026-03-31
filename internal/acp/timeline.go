package acp

import "github.com/charmbracelet/crush/internal/timeline"

func timelineEventPayload(event timeline.Event) *TimelineEvent {
	return &TimelineEvent{
		Type:              string(event.Type),
		Timestamp:         event.Timestamp,
		Title:             event.Title,
		ToolCallID:        event.ToolCallID,
		ToolName:          event.ToolName,
		Status:            event.Status,
		Content:           event.Content,
		ChildSessionID:    event.ChildSessionID,
		CollaborationMode: event.CollaborationMode,
		PermissionMode:    event.PermissionMode,
		Metadata:          event.Metadata,
	}
}
