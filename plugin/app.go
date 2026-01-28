package plugin

import (
	"log/slog"

	"github.com/charmbracelet/crush/internal/permission"
)

// App provides access to application services for plugins.
// It is passed to tool factories during initialization.
type App struct {
	workingDir      string
	extensionConfig map[string]map[string]any
	permissions     permission.Service
	logger          *slog.Logger
	cleanupFuncs    []func() error
}

// AppOption configures an App instance.
type AppOption func(*App)

// NewApp creates a new App instance for plugins.
func NewApp(opts ...AppOption) *App {
	app := &App{
		extensionConfig: make(map[string]map[string]any),
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

// WithExtensionConfig sets the extension configuration.
func WithExtensionConfig(cfg map[string]map[string]any) AppOption {
	return func(a *App) {
		a.extensionConfig = cfg
	}
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

// WorkingDir returns the current working directory.
func (a *App) WorkingDir() string {
	return a.workingDir
}

// ExtensionConfig returns config for a specific extension.
func (a *App) ExtensionConfig(name string) map[string]any {
	if a.extensionConfig == nil {
		return nil
	}
	return a.extensionConfig[name]
}

// Permissions returns the permission service.
func (a *App) Permissions() permission.Service {
	return a.permissions
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
