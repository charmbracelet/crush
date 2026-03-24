package proto

import (
	"encoding/json"
	"errors"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
)

// Workspace represents a running app.App workspace with its associated
// resources and state.
type Workspace struct {
	ID      string         `json:"id"`
	Path    string         `json:"path"`
	YOLO    bool           `json:"yolo,omitempty"`
	Debug   bool           `json:"debug,omitempty"`
	DataDir string         `json:"data_dir,omitempty"`
	Version string         `json:"version,omitempty"`
	Config  *config.Config `json:"config,omitempty"`
	Env     []string       `json:"env,omitempty"`
}

// Error represents an error response.
type Error struct {
	Message string `json:"message"`
}

// SkillInfo describes a visible skill exposed to a frontend.
type SkillInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Label       string `json:"label"`
	Source      string `json:"source"`
}

// AgentInfo represents information about the agent.
type AgentInfo struct {
	IsBusy   bool                 `json:"is_busy"`
	IsReady  bool                 `json:"is_ready"`
	Model    catwalk.Model        `json:"model"`
	ModelCfg config.SelectedModel `json:"model_cfg"`
}

// IsZero checks if the AgentInfo is zero-valued.
func (a AgentInfo) IsZero() bool {
	return !a.IsBusy && !a.IsReady && a.Model.ID == ""
}

// AgentMessage represents a message sent to the agent.
type AgentMessage struct {
	SessionID   string       `json:"session_id"`
	Prompt      string       `json:"prompt"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// AgentSession represents a session with its busy status.
type AgentSession struct {
	Session
	IsBusy bool `json:"is_busy"`
}

// IsZero checks if the AgentSession is zero-valued.
func (a AgentSession) IsZero() bool {
	return a == AgentSession{}
}

// PermissionAction represents an action taken on a permission request.
type PermissionAction string

const (
	PermissionAllow           PermissionAction = "allow"
	PermissionAllowForSession PermissionAction = "allow_session"
	PermissionDeny            PermissionAction = "deny"
)

// MarshalText implements the [encoding.TextMarshaler] interface.
func (p PermissionAction) MarshalText() ([]byte, error) {
	return []byte(p), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (p *PermissionAction) UnmarshalText(text []byte) error {
	*p = PermissionAction(text)
	return nil
}

// PermissionGrant represents a permission grant request.
type PermissionGrant struct {
	Permission PermissionRequest `json:"permission"`
	Action     PermissionAction  `json:"action"`
}

// PermissionSkipRequest represents a request to skip permission prompts.
type PermissionSkipRequest struct {
	Skip bool `json:"skip"`
}

// LSPEventType represents the type of LSP event.
type LSPEventType string

const (
	LSPEventStateChanged       LSPEventType = "state_changed"
	LSPEventDiagnosticsChanged LSPEventType = "diagnostics_changed"
)

// MarshalText implements the [encoding.TextMarshaler] interface.
func (e LSPEventType) MarshalText() ([]byte, error) {
	return []byte(e), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (e *LSPEventType) UnmarshalText(data []byte) error {
	*e = LSPEventType(data)
	return nil
}

// LSPEvent represents an event in the LSP system.
type LSPEvent struct {
	Type            LSPEventType    `json:"type"`
	Name            string          `json:"name"`
	State           lsp.ServerState `json:"state"`
	Error           error           `json:"error,omitempty"`
	DiagnosticCount int             `json:"diagnostic_count,omitempty"`
}

// MarshalJSON implements the [json.Marshaler] interface.
func (e LSPEvent) MarshalJSON() ([]byte, error) {
	type Alias LSPEvent
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
func (e *LSPEvent) UnmarshalJSON(data []byte) error {
	type Alias LSPEvent
	aux := &struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Alias: (Alias)(*e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*e = LSPEvent(aux.Alias)
	if aux.Error != "" {
		e.Error = errors.New(aux.Error)
	}
	return nil
}

// LSPClientInfo holds information about an LSP client's state.
type LSPClientInfo struct {
	Name            string          `json:"name"`
	State           lsp.ServerState `json:"state"`
	Error           error           `json:"error,omitempty"`
	DiagnosticCount int             `json:"diagnostic_count,omitempty"`
	ConnectedAt     time.Time       `json:"connected_at"`
}

// MarshalJSON implements the [json.Marshaler] interface.
func (i LSPClientInfo) MarshalJSON() ([]byte, error) {
	type Alias LSPClientInfo
	return json.Marshal(&struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Error: func() string {
			if i.Error != nil {
				return i.Error.Error()
			}
			return ""
		}(),
		Alias: (Alias)(i),
	})
}

// UnmarshalJSON implements the [json.Unmarshaler] interface.
func (i *LSPClientInfo) UnmarshalJSON(data []byte) error {
	type Alias LSPClientInfo
	aux := &struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Alias: (Alias)(*i),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*i = LSPClientInfo(aux.Alias)
	if aux.Error != "" {
		i.Error = errors.New(aux.Error)
	}
	return nil
}
