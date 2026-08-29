package tools

import (
	"slices"
	"strings"

	"charm.land/fantasy"
)

// ToolDescriptor is the stable, execution-free description of a host tool.
type ToolDescriptor struct {
	Name        string
	Description string
}

// ToolRegistry lists tool metadata without exposing execution details.
type ToolRegistry interface {
	List() []ToolDescriptor
}

// Catalog owns a deterministic set of model-facing host tools.
type Catalog struct {
	tools []fantasy.AgentTool
}

// NewCatalog builds a catalog sorted by public tool name.
func NewCatalog(agentTools []fantasy.AgentTool) *Catalog {
	catalog := &Catalog{tools: slices.Clone(agentTools)}
	slices.SortFunc(catalog.tools, func(a, b fantasy.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})
	return catalog
}

// List returns execution-free descriptors in deterministic order.
func (c *Catalog) List() []ToolDescriptor {
	result := make([]ToolDescriptor, 0, len(c.tools))
	for _, tool := range c.tools {
		info := tool.Info()
		result = append(result, ToolDescriptor{Name: info.Name, Description: info.Description})
	}
	return result
}

// Tools returns a copy of the executable tool set.
func (c *Catalog) Tools() []fantasy.AgentTool {
	return slices.Clone(c.tools)
}
