package tools

import (
	"context"

	"charm.land/fantasy"
)

type RegistrySearchOptions struct {
	Limit           int
	IncludeDeferred bool
	ExposedOnly     bool
}

type RegistryEntry struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Required    []string       `json:"required,omitempty"`
	Source      string         `json:"source,omitempty"`
	Metadata    ToolMetadata   `json:"metadata,omitempty"`
	Exposed     bool           `json:"exposed"`
}

type Registry interface {
	Search(query string, opts RegistrySearchOptions) []RegistryEntry
	Resolve(name string) (RegistryEntry, bool)
	Invoke(ctx context.Context, name string, args map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error)
}
