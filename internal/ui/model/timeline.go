package model

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/timeline"
)

func (m *UI) handleTimelineEvent(event pubsub.Event[timeline.Event]) tea.Cmd {
	if event.Type == pubsub.DeletedEvent || m.session == nil {
		return nil
	}
	if event.Payload.SessionID != m.session.ID {
		return nil
	}
	m.timelineEvents = append(m.timelineEvents, event.Payload)
	if len(m.timelineEvents) > 128 {
		m.timelineEvents = append([]timeline.Event(nil), m.timelineEvents[len(m.timelineEvents)-128:]...)
	}
	if event.Payload.Type != timeline.EventModeChanged {
		return nil
	}
	if event.Payload.CollaborationMode != "" {
		m.session.CollaborationMode = session.NormalizeCollaborationMode(event.Payload.CollaborationMode)
	}
	if event.Payload.PermissionMode != "" {
		m.session.PermissionMode = session.NormalizePermissionMode(event.Payload.PermissionMode)
	}
	m.refreshEditorPlaceholder()
	m.renderPills()
	return nil
}
