package proto

import "fmt"

// MCPState represents the current state of an MCP client
type MCPState int

const (
	MCPStateDisabled MCPState = iota
	MCPStateStarting
	MCPStateConnected
	MCPStateError
)

func (s MCPState) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *MCPState) UnmarshalText(data []byte) error {
	switch string(data) {
	case "disabled":
		*s = MCPStateDisabled
	case "starting":
		*s = MCPStateStarting
	case "connected":
		*s = MCPStateConnected
	case "error":
		*s = MCPStateError
	default:
		return fmt.Errorf("unknown mcp state: %s", data)
	}
	return nil
}

func (s MCPState) String() string {
	switch s {
	case MCPStateDisabled:
		return "disabled"
	case MCPStateStarting:
		return "starting"
	case MCPStateConnected:
		return "connected"
	case MCPStateError:
		return "error"
	default:
		return "unknown"
	}
}

// MCPEventType represents the type of MCP event
type MCPEventType string

const (
	MCPEventStateChanged MCPEventType = "state_changed"
)

func (t MCPEventType) MarshalText() ([]byte, error) {
	return []byte(t), nil
}

func (t *MCPEventType) UnmarshalText(data []byte) error {
	*t = MCPEventType(data)
	return nil
}

// MCPEvent represents an event in the MCP system
type MCPEvent struct {
	Type      MCPEventType `json:"type"`
	Name      string       `json:"name"`
	State     MCPState     `json:"state"`
	Error     error        `json:"error,omitempty"`
	ToolCount int          `json:"tool_count,omitempty"`
}
