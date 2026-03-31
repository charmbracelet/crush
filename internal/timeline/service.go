package timeline

import (
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
)

type EventType string

const (
	EventModeChanged          EventType = "mode_changed"
	EventToolStarted          EventType = "tool_started"
	EventToolProgress         EventType = "tool_progress"
	EventToolFinished         EventType = "tool_finished"
	EventChildSessionStarted  EventType = "child_session_started"
	EventChildSessionFinished EventType = "child_session_finished"
)

type Event struct {
	SessionID         string         `json:"session_id"`
	Type              EventType      `json:"type"`
	Timestamp         int64          `json:"timestamp"`
	Title             string         `json:"title,omitempty"`
	ToolCallID        string         `json:"tool_call_id,omitempty"`
	ToolName          string         `json:"tool_name,omitempty"`
	Status            string         `json:"status,omitempty"`
	Content           string         `json:"content,omitempty"`
	ChildSessionID    string         `json:"child_session_id,omitempty"`
	CollaborationMode string         `json:"collaboration_mode,omitempty"`
	PermissionMode    string         `json:"permission_mode,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type Service interface {
	pubsub.Subscriber[Event]
	Publish(Event)
	ListBySession(sessionID string) []Event
	Shutdown()
}

type service struct {
	*pubsub.Broker[Event]
	mu       sync.RWMutex
	events   map[string][]Event
	maxItems int
}

func NewService() Service {
	return &service{
		Broker:   pubsub.NewBroker[Event](),
		events:   make(map[string][]Event),
		maxItems: 256,
	}
}

func (s *service) Publish(event Event) {
	if event.SessionID == "" || event.Type == "" {
		return
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}

	s.mu.Lock()
	s.events[event.SessionID] = append(s.events[event.SessionID], event)
	if overflow := len(s.events[event.SessionID]) - s.maxItems; overflow > 0 {
		s.events[event.SessionID] = append([]Event(nil), s.events[event.SessionID][overflow:]...)
	}
	s.mu.Unlock()

	s.Broker.Publish(pubsub.CreatedEvent, event)
}

func (s *service) ListBySession(sessionID string) []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.events[sessionID]
	if len(items) == 0 {
		return nil
	}
	out := make([]Event, len(items))
	copy(out, items)
	return out
}
