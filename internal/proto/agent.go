package proto

import (
	"encoding/json"
	"errors"
)

type AgentEventType string

const (
	AgentEventTypeError     AgentEventType = "error"
	AgentEventTypeResponse  AgentEventType = "response"
	AgentEventTypeSummarize AgentEventType = "summarize"
)

func (t AgentEventType) MarshalText() ([]byte, error) {
	return []byte(t), nil
}

func (t *AgentEventType) UnmarshalText(text []byte) error {
	*t = AgentEventType(text)
	return nil
}

type AgentEvent struct {
	Type    AgentEventType `json:"type"`
	Message Message        `json:"message"`
	Error   error          `json:"error,omitempty"`

	// When summarizing
	SessionID string `json:"session_id,omitempty"`
	Progress  string `json:"progress,omitempty"`
	Done      bool   `json:"done,omitempty"`
}

// MarshalJSON implements the [json.Marshaler] interface.
func (e AgentEvent) MarshalJSON() ([]byte, error) {
	type Alias AgentEvent
	return json.Marshal(&struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Error: func() string {
			if e.Error != nil {
				return e.Error.Error()
			}
			return ""
		}(),
		Alias: (Alias)(e),
	})
}

// UnmarshalJSON implements the [json.Unmarshaler] interface.
func (e *AgentEvent) UnmarshalJSON(data []byte) error {
	type Alias AgentEvent
	aux := &struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Alias: (Alias)(*e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*e = AgentEvent(aux.Alias)
	if aux.Error != "" {
		e.Error = errors.New(aux.Error)
	}
	return nil
}
