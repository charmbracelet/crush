package pubsub

import (
	"context"
	"encoding/json"
)

const (
	CreatedEvent EventType = "created"
	UpdatedEvent EventType = "updated"
	DeletedEvent EventType = "deleted"
)

// PayloadType identifies the type of event payload for discriminated
// deserialization over JSON.
type PayloadType = string

const (
	PayloadTypeLSPEvent               PayloadType = "lsp_event"
	PayloadTypeMCPEvent               PayloadType = "mcp_event"
	PayloadTypePermissionRequest      PayloadType = "permission_request"
	PayloadTypePermissionNotification PayloadType = "permission_notification"
	PayloadTypeMessage                PayloadType = "message"
	PayloadTypeSession                PayloadType = "session"
	PayloadTypeFile                   PayloadType = "file"
	PayloadTypeAgentEvent             PayloadType = "agent_event"
)

// Payload wraps a discriminated JSON payload with a type tag.
type Payload struct {
	Type    PayloadType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Subscriber can subscribe to events of type T.
type Subscriber[T any] interface {
	Subscribe(context.Context) <-chan Event[T]
}

type (
	// EventType identifies the type of event.
	EventType string

	// Event represents an event in the lifecycle of a resource.
	Event[T any] struct {
		Type    EventType `json:"type"`
		Payload T         `json:"payload"`
	}

	// Publisher can publish events of type T.
	Publisher[T any] interface {
		Publish(EventType, T)
	}
)
