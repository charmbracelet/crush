// Package plugin provides the extension system for Crush.
//
// Extensions register themselves via init() functions using RegisterTool,
// RegisterHook, etc. The xcrush build tool imports extension packages,
// triggering their registration.
package plugin

import (
	"context"
	"sync"

	"charm.land/fantasy"
)

// Tool is the interface for agent tools.
type Tool = fantasy.AgentTool

// ToolFactory creates a tool instance during app initialization.
// The context is the application startup context.
// The App parameter provides access to application services.
type ToolFactory func(ctx context.Context, app *App) (Tool, error)

var (
	toolsMu    sync.RWMutex
	toolsOrder []string
	tools      = make(map[string]ToolFactory)
)

// RegisterTool registers a tool factory.
// Name must be unique and lowercase with no spaces.
// The factory is called during application initialization to create the tool.
func RegisterTool(name string, factory ToolFactory) {
	toolsMu.Lock()
	defer toolsMu.Unlock()

	if _, exists := tools[name]; exists {
		panic("plugin: RegisterTool called twice for tool " + name)
	}

	tools[name] = factory
	toolsOrder = append(toolsOrder, name)
}

// RegisteredTools returns a copy of registered tool names in registration order.
func RegisteredTools() []string {
	toolsMu.RLock()
	defer toolsMu.RUnlock()

	result := make([]string, len(toolsOrder))
	copy(result, toolsOrder)
	return result
}

// GetToolFactory returns the factory for a registered tool.
func GetToolFactory(name string) (ToolFactory, bool) {
	toolsMu.RLock()
	defer toolsMu.RUnlock()

	factory, ok := tools[name]
	return factory, ok
}

// ResetTools clears all registered tools. Used for testing.
func ResetTools() {
	toolsMu.Lock()
	defer toolsMu.Unlock()

	tools = make(map[string]ToolFactory)
	toolsOrder = nil
}
