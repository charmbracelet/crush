package proto

import (
	"encoding/json"
	"errors"
	"fmt"
)

// SkillDiscoveryState represents the outcome of discovering a single skill file.
type SkillDiscoveryState int

const (
	// SkillStateNormal indicates the skill was parsed and validated successfully.
	SkillStateNormal SkillDiscoveryState = iota
	// SkillStateError indicates discovery encountered a scan/parse/validate error.
	SkillStateError
)

// MarshalText implements the [encoding.TextMarshaler] interface.
func (s SkillDiscoveryState) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
func (s *SkillDiscoveryState) UnmarshalText(data []byte) error {
	switch string(data) {
	case "normal":
		*s = SkillStateNormal
	case "error":
		*s = SkillStateError
	default:
		return fmt.Errorf("unknown skill discovery state: %s", data)
	}
	return nil
}

// String returns the string representation of the SkillDiscoveryState.
func (s SkillDiscoveryState) String() string {
	switch s {
	case SkillStateNormal:
		return "normal"
	case SkillStateError:
		return "error"
	default:
		return "unknown"
	}
}

// SkillState represents the latest discovery status of a skill file.
type SkillState struct {
	Name  string              `json:"name"`
	Path  string              `json:"path"`
	State SkillDiscoveryState `json:"state"`
	Error error               `json:"error,omitempty"`
}

// MarshalJSON implements the [json.Marshaler] interface.
func (s SkillState) MarshalJSON() ([]byte, error) {
	type Alias SkillState
	return json.Marshal(&struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Error: func() string {
			if s.Error != nil {
				return s.Error.Error()
			}
			return ""
		}(),
		Alias: Alias(s),
	})
}

// UnmarshalJSON implements the [json.Unmarshaler] interface.
func (s *SkillState) UnmarshalJSON(data []byte) error {
	type Alias SkillState
	aux := &struct {
		Error string `json:"error,omitempty"`
		Alias
	}{
		Alias: Alias(*s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*s = SkillState(aux.Alias)
	if aux.Error != "" {
		s.Error = errors.New(aux.Error)
	}
	return nil
}

// SkillEvent represents an event in the skill discovery system.
type SkillEvent struct {
	States []SkillState `json:"states"`
}

// SkillsEvent is the wire representation of skills.Event.
type SkillsEvent struct {
	States []SkillState `json:"states"`
}
