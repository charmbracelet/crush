package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/charmbracelet/crush/internal/permission"
)

// SubAgentOptions configures a sub-agent invocation.
type SubAgentOptions struct {
	// Name is an identifier for the sub-agent (for logging/tracing).
	Name string
	// SystemPrompt is the custom system prompt for this sub-agent.
	SystemPrompt string
	// Prompt is the task/query to execute.
	Prompt string
	// AllowedTools lists tools the sub-agent can use (nil = inherit all).
	AllowedTools []string
	// DisallowedTools lists tools the sub-agent cannot use.
	DisallowedTools []string
	// Model specifies which model to use ("inherit", "sonnet", "opus", "haiku").
	Model string
}

// SubAgentRunner executes sub-agents within the current session context.
type SubAgentRunner interface {
	// RunSubAgent executes a sub-agent with the given options.
	// The context should contain session and message IDs from the parent call.
	// Returns the sub-agent's text response or an error.
	RunSubAgent(ctx context.Context, opts SubAgentOptions) (string, error)
}

// App provides access to application services for plugins.
// It is passed to tool factories during initialization.
type App struct {
	workingDir          string
	pluginConfig        map[string]map[string]any
	disabledPlugins     []string
	permissions         permission.Service
	messageSubscriber   MessageSubscriber
	sessionInfoProvider SessionInfoProvider
	promptSubmitter     PromptSubmitter
	subAgentRunner      SubAgentRunner
	logger              *slog.Logger
	cleanupFuncs        []func() error
}

// AppOption configures an App instance.
type AppOption func(*App)

// NewApp creates a new App instance for plugins.
func NewApp(opts ...AppOption) *App {
	app := &App{
		pluginConfig: make(map[string]map[string]any),
	}
	for _, opt := range opts {
		opt(app)
	}
	return app
}

// WithWorkingDir sets the working directory.
func WithWorkingDir(dir string) AppOption {
	return func(a *App) {
		a.workingDir = dir
	}
}

// WithPluginConfig sets the plugin configuration.
func WithPluginConfig(cfg map[string]map[string]any) AppOption {
	return func(a *App) {
		if cfg != nil {
			a.pluginConfig = cfg
		}
	}
}

// WithDisabledPlugins sets the list of disabled plugins.
func WithDisabledPlugins(disabled []string) AppOption {
	return func(a *App) {
		a.disabledPlugins = disabled
	}
}

// WithExtensionConfig is deprecated. Use WithPluginConfig instead.
// Kept for backwards compatibility.
func WithExtensionConfig(cfg map[string]map[string]any) AppOption {
	return WithPluginConfig(cfg)
}

// WithPermissions sets the permission service.
func WithPermissions(p permission.Service) AppOption {
	return func(a *App) {
		a.permissions = p
	}
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) AppOption {
	return func(a *App) {
		a.logger = l
	}
}

// WithMessageSubscriber sets the message subscriber for hooks.
func WithMessageSubscriber(ms MessageSubscriber) AppOption {
	return func(a *App) {
		a.messageSubscriber = ms
	}
}

// WithSessionInfoProvider sets the session info provider for hooks.
func WithSessionInfoProvider(sip SessionInfoProvider) AppOption {
	return func(a *App) {
		a.sessionInfoProvider = sip
	}
}

// WithPromptSubmitter sets the prompt submitter for hooks to send prompts.
func WithPromptSubmitter(ps PromptSubmitter) AppOption {
	return func(a *App) {
		a.promptSubmitter = ps
	}
}

// WithSubAgentRunner sets the sub-agent runner for plugins to execute sub-agents.
func WithSubAgentRunner(sar SubAgentRunner) AppOption {
	return func(a *App) {
		a.subAgentRunner = sar
	}
}

// WorkingDir returns the current working directory.
func (a *App) WorkingDir() string {
	return a.workingDir
}

// IsPluginDisabled returns true if the plugin is disabled by config.
func (a *App) IsPluginDisabled(name string) bool {
	for _, disabled := range a.disabledPlugins {
		if disabled == name {
			return true
		}
	}
	return false
}

// PluginConfig returns the raw config map for a specific plugin.
func (a *App) PluginConfig(name string) map[string]any {
	if a.pluginConfig == nil {
		return nil
	}
	return a.pluginConfig[name]
}

// ExtensionConfig is deprecated. Use PluginConfig instead.
func (a *App) ExtensionConfig(name string) map[string]any {
	return a.PluginConfig(name)
}

// LoadConfig loads and validates plugin configuration into a typed struct.
// The target must be a pointer to a struct. If no config is found, the struct
// is left with its default/zero values.
//
// Example:
//
//	type MyConfig struct {
//	    APIKey  string `json:"api_key"`
//	    Timeout int    `json:"timeout"`
//	}
//
//	var cfg MyConfig
//	if err := app.LoadConfig("myplugin", &cfg); err != nil {
//	    return nil, err
//	}
func (a *App) LoadConfig(name string, target any) error {
	if target == nil {
		return fmt.Errorf("target cannot be nil")
	}

	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer to a struct")
	}

	if rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to a struct, got pointer to %s", rv.Elem().Kind())
	}

	raw := a.PluginConfig(name)
	if raw == nil {
		// No config provided, leave defaults.
		return nil
	}

	// Marshal to JSON and unmarshal into the target struct.
	// This provides automatic type coercion and validation.
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("failed to serialize config for %s: %w", name, err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse config for %s: %w", name, err)
	}

	return nil
}

// Permissions returns the permission service.
func (a *App) Permissions() permission.Service {
	return a.permissions
}

// Messages returns the message subscriber for hooks to observe chat messages.
// Returns nil if no message subscriber is configured.
func (a *App) Messages() MessageSubscriber {
	return a.messageSubscriber
}

// SessionInfo returns the session info provider for hooks to get session metadata.
// Returns nil if no session info provider is configured.
func (a *App) SessionInfo() SessionInfoProvider {
	return a.sessionInfoProvider
}

// PromptSubmitter returns the prompt submitter for hooks to send prompts.
// Returns nil if no prompt submitter is configured.
func (a *App) PromptSubmitter() PromptSubmitter {
	return a.promptSubmitter
}

// SubAgentRunner returns the sub-agent runner for executing sub-agents.
// Returns nil if no sub-agent runner is configured.
func (a *App) SubAgentRunner() SubAgentRunner {
	return a.subAgentRunner
}

// Logger returns a structured logger.
func (a *App) Logger() *slog.Logger {
	if a.logger == nil {
		return slog.Default()
	}
	return a.logger
}

// RegisterCleanup adds a cleanup function called on shutdown.
func (a *App) RegisterCleanup(fn func() error) {
	a.cleanupFuncs = append(a.cleanupFuncs, fn)
}

// Cleanup runs all registered cleanup functions.
func (a *App) Cleanup() error {
	var firstErr error
	for _, fn := range a.cleanupFuncs {
		if err := fn(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
