package plugin

import (
	"context"
	"log/slog"
	"sync"

	"github.com/charmbracelet/crush/internal/message"
)

var (
	plugins          []Plugin
	initializedHooks []Hooks
	customTools      map[string]ToolDefinition
	mu               sync.RWMutex
)

// Register adds a plugin to the registry.
// Plugins should call this in their init() function.
// Registered plugins will be initialized when Init is called.
func Register(p Plugin) {
	mu.Lock()
	defer mu.Unlock()
	plugins = append(plugins, p)
	slog.Debug("Plugin registered", "name", p.Name())
}

// Init initializes all registered plugins.
// This should be called early in the application lifecycle,
// after core services (config, sessions, messages) are available.
// Plugins are initialized in registration order.
// If a plugin fails to initialize, it is logged and skipped.
func Init(ctx context.Context, input PluginInput) error {
	mu.Lock()
	initializedHooks = nil
	customTools = make(map[string]ToolDefinition)

	// Snapshot plugins that were manually registered (via Register) before we clear.
	// These need to be re-initialized after Close since they don't have config to recreate them.
	manuallyRegistered := append([]Plugin(nil), plugins...)

	// Close all existing plugins before clearing to prevent process leaks.
	for _, p := range plugins {
		if err := p.Close(ctx); err != nil {
			slog.Debug("Failed to close plugin during init", "name", p.Name(), "error", err)
		}
	}
	plugins = nil
	mu.Unlock()

	configuredPlugins, err := newConfiguredPlugins(input)
	if err != nil {
		return err
	}
	// Register configured plugins so they appear in ListPlugins() and are
	// properly closed by Close()/Reset(). This is safe because Reset() clears
	// the plugins slice before tests that call Init() multiple times.
	for _, p := range configuredPlugins {
		Register(p)
	}
	toolSources := make(map[string]string)

	// Initialize manually registered plugins first (they were closed above and need re-init).
	for _, p := range manuallyRegistered {
		slog.Info("Initializing plugin", "name", p.Name())
		h, err := p.Init(ctx, input)
		if err != nil {
			slog.Error("Failed to initialize plugin", "name", p.Name(), "error", err)
			continue
		}

		mu.Lock()
		initializedHooks = append(initializedHooks, h)
		for name, tool := range h.Tools {
			source := "plugin:" + p.Name()
			if existingSource, exists := toolSources[name]; exists {
				slog.Warn("Custom tool registration collision", "tool", name, "existing_source", existingSource, "overriding_source", source)
			}
			customTools[name] = tool
			toolSources[name] = source
		}
		mu.Unlock()

		slog.Info("Plugin initialized", "name", p.Name(), "tools", len(h.Tools))
	}

	// Then initialize newly configured plugins (fresh instances, not previously closed).
	for _, p := range configuredPlugins {
		slog.Info("Initializing plugin", "name", p.Name())
		h, err := p.Init(ctx, input)
		if err != nil {
			slog.Error("Failed to initialize plugin", "name", p.Name(), "error", err)
			continue
		}

		mu.Lock()
		initializedHooks = append(initializedHooks, h)
		for name, tool := range h.Tools {
			source := "plugin:" + p.Name()
			if existingSource, exists := toolSources[name]; exists {
				slog.Warn("Custom tool registration collision", "tool", name, "existing_source", existingSource, "overriding_source", source)
			}
			customTools[name] = tool
			toolSources[name] = source
		}
		mu.Unlock()

		slog.Info("Plugin initialized", "name", p.Name(), "tools", len(h.Tools))
	}

	localTools, err := DiscoverLocalTools(input.WorkingDir)
	if err != nil {
		slog.Error("Failed to discover local tools", "error", err)
		return err
	}
	mu.Lock()
	for name, tool := range localTools {
		if existingSource, exists := toolSources[name]; exists {
			slog.Warn("Custom tool registration collision", "tool", name, "existing_source", existingSource, "overriding_source", "local")
		}
		customTools[name] = tool
		toolSources[name] = "local"
	}
	mu.Unlock()
	if len(localTools) > 0 {
		slog.Info("Local tools loaded", "count", len(localTools))
	}

	return nil
}

// GetHooks returns a copy of the merged hooks.
// This is useful for checking if specific hooks are registered.
func GetHooks() Hooks {
	mu.RLock()
	defer mu.RUnlock()
	hooks := Hooks{Tools: make(map[string]ToolDefinition, len(customTools))}
	for name, tool := range customTools {
		hooks.Tools[name] = tool
	}
	for _, hook := range initializedHooks {
		if hook.ToolBeforeExecute != nil {
			hooks.ToolBeforeExecute = TriggerToolBeforeExecute
		}
		if hook.ToolAfterExecute != nil {
			hooks.ToolAfterExecute = TriggerToolAfterExecute
		}
		if hook.ChatBeforeRequest != nil {
			hooks.ChatBeforeRequest = TriggerChatBeforeRequest
		}
		if hook.ChatAfterResponse != nil {
			hooks.ChatAfterResponse = TriggerChatAfterResponse
		}
		if hook.ChatMessagesTransform != nil {
			hooks.ChatMessagesTransform = func(ctx context.Context, input ChatMessagesTransformInput, output *ChatMessagesTransformOutput) error {
				transformed, err := TriggerChatMessagesTransform(ctx, input, *output)
				if err != nil {
					return err
				}
				*output = transformed
				return nil
			}
		}
		if hook.ChatSystemTransform != nil {
			hooks.ChatSystemTransform = func(ctx context.Context, input ChatSystemTransformInput, output *ChatSystemTransformOutput) error {
				transformed, err := TriggerChatSystemTransform(ctx, input, *output)
				if err != nil {
					return err
				}
				*output = transformed
				return nil
			}
		}
		if hook.SessionCompacting != nil {
			hooks.SessionCompacting = func(ctx context.Context, input SessionCompactingInput, output *SessionCompactingOutput) error {
				transformed, err := TriggerSessionCompacting(ctx, input, *output)
				if err != nil {
					return err
				}
				*output = transformed
				return nil
			}
		}
		if hook.PermissionAsk != nil {
			hooks.PermissionAsk = TriggerPermissionAsk
		}
		if hook.ShellEnv != nil {
			hooks.ShellEnv = TriggerShellEnv
		}
		if hook.MessageCreated != nil {
			hooks.MessageCreated = TriggerMessageCreated
		}
	}
	return hooks
}

// TriggerToolBeforeExecute executes the ToolBeforeExecute hook if registered.
// Returns nil, nil if no hook is registered.
func TriggerToolBeforeExecute(ctx context.Context, input ToolBeforeExecuteInput) (*ToolBeforeExecuteOutput, error) {
	mu.RLock()
	hooks := append([]Hooks(nil), initializedHooks...)
	mu.RUnlock()

	currentArgs := input.Args
	changed := false
	for _, hook := range hooks {
		if hook.ToolBeforeExecute == nil {
			continue
		}
		out, err := hook.ToolBeforeExecute(ctx, ToolBeforeExecuteInput{
			Tool:      input.Tool,
			SessionID: input.SessionID,
			CallID:    input.CallID,
			Args:      currentArgs,
		})
		if err != nil {
			return nil, err
		}
		if out == nil {
			continue
		}
		if out.Args != nil {
			currentArgs = out.Args
			changed = true
		}
		if out.Skip {
			return &ToolBeforeExecuteOutput{Args: currentArgs, Skip: true, PreResult: out.PreResult}, nil
		}
	}
	if !changed {
		return nil, nil
	}
	return &ToolBeforeExecuteOutput{Args: currentArgs}, nil
}

// TriggerToolAfterExecute executes the ToolAfterExecute hook if registered.
// Returns nil, nil if no hook is registered.
func TriggerToolAfterExecute(ctx context.Context, input ToolAfterExecuteInput) (*ToolAfterExecuteOutput, error) {
	mu.RLock()
	hooks := append([]Hooks(nil), initializedHooks...)
	mu.RUnlock()

	currentResult := input.Result
	currentMetadata := input.Metadata
	changed := false
	for _, hook := range hooks {
		if hook.ToolAfterExecute == nil {
			continue
		}
		out, err := hook.ToolAfterExecute(ctx, ToolAfterExecuteInput{
			Tool:      input.Tool,
			SessionID: input.SessionID,
			CallID:    input.CallID,
			Args:      input.Args,
			Result:    currentResult,
			Metadata:  currentMetadata,
		})
		if err != nil {
			return nil, err
		}
		if out == nil {
			continue
		}
		if out.ResultChanged {
			currentResult = out.Result
			changed = true
		}
		if out.Metadata != nil {
			currentMetadata = out.Metadata
			changed = true
		}
	}
	if !changed {
		return nil, nil
	}
	return &ToolAfterExecuteOutput{Result: currentResult, ResultChanged: true, Metadata: currentMetadata}, nil
}

// TriggerChatBeforeRequest executes the ChatBeforeRequest hook if registered.
func TriggerChatBeforeRequest(ctx context.Context, input ChatBeforeRequestInput) error {
	mu.RLock()
	hooks := append([]Hooks(nil), initializedHooks...)
	mu.RUnlock()

	for _, hook := range hooks {
		if hook.ChatBeforeRequest == nil {
			continue
		}
		if err := hook.ChatBeforeRequest(ctx, input); err != nil {
			return err
		}
	}
	return nil
}

// TriggerChatAfterResponse executes the ChatAfterResponse hook if registered.
func TriggerChatAfterResponse(ctx context.Context, input ChatAfterResponseInput) error {
	mu.RLock()
	hooks := append([]Hooks(nil), initializedHooks...)
	mu.RUnlock()

	for _, hook := range hooks {
		if hook.ChatAfterResponse == nil {
			continue
		}
		if err := hook.ChatAfterResponse(ctx, input); err != nil {
			return err
		}
	}
	return nil
}

// TriggerChatMessagesTransform executes all registered ChatMessagesTransform hooks in order.
func TriggerChatMessagesTransform(ctx context.Context, input ChatMessagesTransformInput, output ChatMessagesTransformOutput) (ChatMessagesTransformOutput, error) {
	mu.RLock()
	hooks := append([]Hooks(nil), initializedHooks...)
	mu.RUnlock()

	for _, hook := range hooks {
		if hook.ChatMessagesTransform == nil {
			continue
		}
		if err := hook.ChatMessagesTransform(ctx, input, &output); err != nil {
			return output, err
		}
	}
	return output, nil
}

// TriggerChatSystemTransform executes all registered ChatSystemTransform hooks in order.
func TriggerChatSystemTransform(ctx context.Context, input ChatSystemTransformInput, output ChatSystemTransformOutput) (ChatSystemTransformOutput, error) {
	mu.RLock()
	hooks := append([]Hooks(nil), initializedHooks...)
	mu.RUnlock()

	for _, hook := range hooks {
		if hook.ChatSystemTransform == nil {
			continue
		}
		if err := hook.ChatSystemTransform(ctx, input, &output); err != nil {
			return output, err
		}
	}
	return output, nil
}

// TriggerSessionCompacting executes all registered SessionCompacting hooks in order.
func TriggerSessionCompacting(ctx context.Context, input SessionCompactingInput, output SessionCompactingOutput) (SessionCompactingOutput, error) {
	mu.RLock()
	hooks := append([]Hooks(nil), initializedHooks...)
	mu.RUnlock()

	for _, hook := range hooks {
		if hook.SessionCompacting == nil {
			continue
		}
		if err := hook.SessionCompacting(ctx, input, &output); err != nil {
			return output, err
		}
	}
	return output, nil
}

// TriggerPermissionAsk executes the PermissionAsk hook if registered.
// Returns the action from the hook, or PermissionAsk if no hook is registered.
func TriggerPermissionAsk(input PermissionAskInput) PermissionAskOutput {
	mu.RLock()
	hooks := append([]Hooks(nil), initializedHooks...)
	mu.RUnlock()

	decision := PermissionAskOutput{Action: PermissionAsk}
	for _, hook := range hooks {
		if hook.PermissionAsk == nil {
			continue
		}
		out := hook.PermissionAsk(input)
		if out.Action != PermissionAsk {
			decision = out
		}
	}
	return decision
}

// TriggerShellEnv executes the ShellEnv hook if registered.
// Returns an empty map if no hook is registered.
func TriggerShellEnv(ctx context.Context, input ShellEnvInput) map[string]string {
	mu.RLock()
	hooks := append([]Hooks(nil), initializedHooks...)
	mu.RUnlock()

	env := make(map[string]string)
	for _, hook := range hooks {
		if hook.ShellEnv == nil {
			continue
		}
		current := hook.ShellEnv(ctx, input)
		if len(current) == 0 {
			continue
		}
		for key, value := range current {
			env[key] = value
		}
	}
	return env
}

// TriggerMessageCreated executes the MessageCreated hook if registered.
func TriggerMessageCreated(ctx context.Context, msg message.Message) error {
	mu.RLock()
	hooks := append([]Hooks(nil), initializedHooks...)
	mu.RUnlock()

	for _, hook := range hooks {
		if hook.MessageCreated == nil {
			continue
		}
		if err := hook.MessageCreated(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

// GetCustomTools returns all custom tools registered by plugins.
func GetCustomTools() map[string]ToolDefinition {
	mu.RLock()
	defer mu.RUnlock()

	// Return a copy to prevent modification.
	result := make(map[string]ToolDefinition, len(customTools))
	for k, v := range customTools {
		result[k] = v
	}
	return result
}

// ListPlugins returns the names of all registered and configured plugins.
func ListPlugins() []string {
	mu.RLock()
	defer mu.RUnlock()

	seen := make(map[string]struct{})
	for _, p := range plugins {
		seen[p.Name()] = struct{}{}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}

// Reset clears all registered plugins and hooks.
// This is primarily intended for testing.
func Reset() {
	mu.Lock()
	defer mu.Unlock()

	// Close all plugins before clearing to release resources (e.g. persistent processes).
	for _, p := range plugins {
		if err := p.Close(context.Background()); err != nil {
			slog.Debug("Failed to close plugin during reset", "name", p.Name(), "error", err)
		}
	}

	plugins = nil
	initializedHooks = nil
	customTools = make(map[string]ToolDefinition)
}

// Close shuts down all registered plugins that hold resources (e.g. persistent processes).
func Close(ctx context.Context) {
	mu.RLock()
	registeredPlugins := append([]Plugin(nil), plugins...)
	mu.RUnlock()

	for _, p := range registeredPlugins {
		if err := p.Close(ctx); err != nil {
			slog.Error("Failed to close plugin", "name", p.Name(), "error", err)
		}
	}
}
