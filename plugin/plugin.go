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

// ToolRegistration holds the factory and optional config schema for a tool.
type ToolRegistration struct {
	Factory      ToolFactory
	ConfigSchema any // Optional: struct type for config validation (e.g., &PingConfig{})
}

var (
	toolsMu       sync.RWMutex
	toolsOrder    []string
	tools         = make(map[string]ToolRegistration)
	configSchemas = make(map[string]any)
)

// RegisterTool registers a tool factory.
// Name must be unique and lowercase with no spaces.
// The factory is called during application initialization to create the tool.
func RegisterTool(name string, factory ToolFactory) {
	RegisterToolWithConfig(name, factory, nil)
}

// RegisterToolWithConfig registers a tool factory with an optional config schema.
// The configSchema should be a pointer to a struct that defines the expected
// configuration structure. If provided, the config will be validated against
// this schema when loading.
//
// Example:
//
//	type MyConfig struct {
//	    APIKey string `json:"api_key" jsonschema:"required,description=API key for service"`
//	    Timeout int   `json:"timeout,omitempty" jsonschema:"default=30"`
//	}
//
//	RegisterToolWithConfig("mytool", myFactory, &MyConfig{})
func RegisterToolWithConfig(name string, factory ToolFactory, configSchema any) {
	toolsMu.Lock()
	defer toolsMu.Unlock()

	if _, exists := tools[name]; exists {
		panic("plugin: RegisterTool called twice for tool " + name)
	}

	tools[name] = ToolRegistration{
		Factory:      factory,
		ConfigSchema: configSchema,
	}
	toolsOrder = append(toolsOrder, name)

	if configSchema != nil {
		configSchemas[name] = configSchema
	}
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

	reg, ok := tools[name]
	if !ok {
		return nil, false
	}
	return reg.Factory, true
}

// GetToolRegistration returns the full registration for a tool.
func GetToolRegistration(name string) (ToolRegistration, bool) {
	toolsMu.RLock()
	defer toolsMu.RUnlock()

	reg, ok := tools[name]
	return reg, ok
}

// GetConfigSchema returns the config schema for a tool, if registered.
func GetConfigSchema(name string) (any, bool) {
	toolsMu.RLock()
	defer toolsMu.RUnlock()

	schema, ok := configSchemas[name]
	return schema, ok
}

// ResetTools clears all registered tools. Used for testing.
func ResetTools() {
	toolsMu.Lock()
	defer toolsMu.Unlock()

	tools = make(map[string]ToolRegistration)
	configSchemas = make(map[string]any)
	toolsOrder = nil
}
