package crush

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	// ErrAgentIDRequired is returned when an agent is missing an ID.
	ErrAgentIDRequired = errors.New("agent id is required")
	// ErrAgentModelInvalid is returned when an agent model is not "large" or "small".
	ErrAgentModelInvalid = errors.New("agent model must be large or small")
)

// AgentOption configures an Agent during construction.
type AgentOption func(*Agent)

// NewAgent creates a new Agent with the given ID and options.
// ID is required and must be non-empty. Model defaults to SelectedModelTypeLarge.
func NewAgent(id string, opts ...AgentOption) (Agent, error) {
	if strings.TrimSpace(id) == "" {
		return Agent{}, fmt.Errorf("%w: id cannot be empty", ErrAgentIDRequired)
	}
	a := Agent{
		ID:    id,
		Model: SelectedModelTypeLarge,
	}
	for _, opt := range opts {
		opt(&a)
	}
	if err := ValidateAgent(a); err != nil {
		return Agent{}, err
	}
	return a, nil
}

// WithAgentName sets the agent name.
func WithAgentName(name string) AgentOption {
	return func(a *Agent) { a.Name = name }
}

// WithAgentDescription sets the agent description.
func WithAgentDescription(desc string) AgentOption {
	return func(a *Agent) { a.Description = desc }
}

// WithAgentModel sets the model type (large or small).
func WithAgentModel(model SelectedModelType) AgentOption {
	return func(a *Agent) { a.Model = model }
}

// WithAgentAllowedTools sets the exact list of allowed tool names.
// If not called, all tools are available (nil).
func WithAgentAllowedTools(tools ...string) AgentOption {
	return func(a *Agent) { a.AllowedTools = tools }
}

// WithAgentAllowedMCPs adds or replaces an MCP server entry.
// If tools is empty (no variadic args), the slice is nil, meaning all tools
// from that MCP are available. To disable all MCPs, use WithAgentDisabledMCPs.
func WithAgentAllowedMCPs(mcp string, tools ...string) AgentOption {
	return func(a *Agent) {
		if a.DisableMCP {
			a.DisableMCP = false
		}
		if a.AllowedMCP == nil {
			a.AllowedMCP = make(map[string][]string)
		}
		a.AllowedMCP[mcp] = tools
	}
}

// WithAgentAllowedMCP is a deprecated alias for WithAgentAllowedMCPs.
// Deprecated: Use WithAgentAllowedMCPs instead.
func WithAgentAllowedMCP(mcp string, tools ...string) AgentOption {
	return WithAgentAllowedMCPs(mcp, tools...)
}

// WithAgentAllowAllMCPTools explicitly enables all tools from an MCP server.
// This is the self-documenting equivalent of calling WithAgentAllowedMCPs(mcp)
// with no tool arguments.
func WithAgentAllowAllMCPTools(mcp string) AgentOption {
	return WithAgentAllowedMCPs(mcp)
}

// WithAgentDisabledMCPs disables all MCPs for the agent.
func WithAgentDisabledMCPs() AgentOption {
	return func(a *Agent) {
		a.DisableMCP = true
		a.AllowedMCP = make(map[string][]string)
	}
}

// WithAgentDisableMCPs is a deprecated alias for WithAgentDisabledMCPs.
// Deprecated: Use WithAgentDisabledMCPs instead.
func WithAgentDisableMCPs() AgentOption {
	return WithAgentDisabledMCPs()
}

// WithAgentContextPaths overrides the context paths for the agent.
func WithAgentContextPaths(paths ...string) AgentOption {
	return func(a *Agent) { a.ContextPaths = paths }
}

// WithAgentDisabled sets whether the agent is disabled.
func WithAgentDisabled(disabled bool) AgentOption {
	return func(a *Agent) { a.Disabled = disabled }
}

// ValidateAgent checks the agent configuration for errors.
func ValidateAgent(a Agent) error {
	if strings.TrimSpace(a.ID) == "" {
		return ErrAgentIDRequired
	}
	if a.Model != SelectedModelTypeLarge && a.Model != SelectedModelTypeSmall {
		return fmt.Errorf("%w: got %q", ErrAgentModelInvalid, a.Model)
	}
	return nil
}

// IsAgentValid reports whether the agent configuration is valid.
func IsAgentValid(a Agent) bool {
	return ValidateAgent(a) == nil
}

// AgentIsValid is a deprecated alias for IsAgentValid.
// Deprecated: Use IsAgentValid instead.
func AgentIsValid(a *Agent) bool {
	if a == nil {
		return false
	}
	return IsAgentValid(*a)
}

// CloneAgent returns a deep copy of the agent.
func CloneAgent(a Agent) Agent {
	cloned := Agent{
		ID:           a.ID,
		Name:         a.Name,
		Description:  a.Description,
		Disabled:     a.Disabled,
		DisableMCP:   a.DisableMCP,
		Model:        a.Model,
		AllowedTools: slices.Clone(a.AllowedTools),
		ContextPaths: slices.Clone(a.ContextPaths),
	}
	if a.AllowedMCP != nil {
		cloned.AllowedMCP = make(map[string][]string, len(a.AllowedMCP))
		for k, v := range a.AllowedMCP {
			cloned.AllowedMCP[k] = slices.Clone(v)
		}
	}
	return cloned
}
