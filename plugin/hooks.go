package plugin

import (
	"context"
	"sync"
)

// Hook represents a plugin lifecycle hook that processes events.
// Unlike tools, hooks run in the background and observe system events.
type Hook interface {
	// Name returns the hook identifier.
	Name() string
	// Start begins processing events. It should return when ctx is canceled.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the hook.
	Stop() error
}

// HookFactory creates a hook instance during app initialization.
// The context is the application startup context.
// The App parameter provides access to application services.
type HookFactory func(ctx context.Context, app *App) (Hook, error)

// HookRegistration holds the factory and optional config schema for a hook.
type HookRegistration struct {
	Factory      HookFactory
	ConfigSchema any
}

var (
	hooksMu    sync.RWMutex
	hooksOrder []string
	hooks      = make(map[string]HookRegistration)
)

// RegisterHook registers a hook factory.
// Name must be unique and lowercase with no spaces.
// The factory is called during application initialization to create the hook.
func RegisterHook(name string, factory HookFactory) {
	RegisterHookWithConfig(name, factory, nil)
}

// RegisterHookWithConfig registers a hook factory with an optional config schema.
// The configSchema should be a pointer to a struct that defines the expected
// configuration structure.
func RegisterHookWithConfig(name string, factory HookFactory, configSchema any) {
	hooksMu.Lock()
	defer hooksMu.Unlock()

	if _, exists := hooks[name]; exists {
		panic("plugin: RegisterHook called twice for hook " + name)
	}

	hooks[name] = HookRegistration{
		Factory:      factory,
		ConfigSchema: configSchema,
	}
	hooksOrder = append(hooksOrder, name)
}

// RegisteredHooks returns a copy of registered hook names in registration order.
func RegisteredHooks() []string {
	hooksMu.RLock()
	defer hooksMu.RUnlock()

	result := make([]string, len(hooksOrder))
	copy(result, hooksOrder)
	return result
}

// GetHookFactory returns the factory for a registered hook.
func GetHookFactory(name string) (HookFactory, bool) {
	hooksMu.RLock()
	defer hooksMu.RUnlock()

	reg, ok := hooks[name]
	if !ok {
		return nil, false
	}
	return reg.Factory, true
}

// GetHookRegistration returns the full registration for a hook.
func GetHookRegistration(name string) (HookRegistration, bool) {
	hooksMu.RLock()
	defer hooksMu.RUnlock()

	reg, ok := hooks[name]
	return reg, ok
}

// ResetHooks clears all registered hooks. Used for testing.
func ResetHooks() {
	hooksMu.Lock()
	defer hooksMu.Unlock()

	hooks = make(map[string]HookRegistration)
	hooksOrder = nil
}
