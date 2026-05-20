package crush

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/db"
)

// runOptions collects optional parameters for the prompt-running methods.
type runOptions struct {
	agentID    string
	largeModel string
	smallModel string
	hideSpinner bool
}

// RunOption configures how a prompt is executed.
type RunOption func(*runOptions)

// WithAgentID selects the agent to use for the run.
// If not set, the default "coder" agent is used.
func WithAgentID(id string) RunOption {
	return func(o *runOptions) { o.agentID = id }
}

// WithLargeModel overrides the large model for this run.
func WithLargeModel(model string) RunOption {
	return func(o *runOptions) { o.largeModel = model }
}

// WithSmallModel overrides the small model for this run.
func WithSmallModel(model string) RunOption {
	return func(o *runOptions) { o.smallModel = model }
}

// WithHideSpinner disables the progress spinner.
func WithHideSpinner() RunOption {
	return func(o *runOptions) { o.hideSpinner = true }
}

func resolveRunOptions(opts []RunOption) runOptions {
	var o runOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// App is the public, programmatic API surface for Crush.
// It wraps the internal orchestrator and exposes only stable fields and
// methods. Unlike the previous type alias, this struct explicitly chooses
// what is public, preventing internal implementation details from leaking
// into the external API.
type App struct {
	Sessions    SessionService
	Messages    MessageService
	History     HistoryService
	Permissions PermissionService
	FileTracker FileTrackerService

	internal *app.App
}

// NewApp initializes a new application instance from a database connection
// and configuration store.
func NewApp(ctx context.Context, conn *sql.DB, store *ConfigStore, skillsMgr *SkillsManager) (*App, error) {
	a, err := app.New(ctx, conn, store, skillsMgr)
	if err != nil {
		return nil, err
	}
	return wrapApp(a), nil
}

// NewAppWithConfig loads the configuration from default paths, opens the
// SQLite database (using the shared connection pool), and creates a new
// App instance. The returned App owns the database connection; call
// Shutdown() when finished.
func NewAppWithConfig(ctx context.Context, workingDir, dataDir string, debug bool, skillsMgr *SkillsManager) (*App, error) {
	store, err := Load(workingDir, dataDir, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Ensure the data directory exists before opening the database.
	if err := os.MkdirAll(store.Config().Options.DataDirectory, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	conn, err := db.Connect(ctx, store.Config().Options.DataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	a, err := NewApp(ctx, conn, store, skillsMgr)
	if err != nil {
		db.Release(store.Config().Options.DataDirectory)
		return nil, err
	}

	return a, nil
}

func wrapApp(a *app.App) *App {
	return &App{
		Sessions:    a.Sessions,
		Messages:    a.Messages,
		History:     a.History,
		Permissions: a.Permissions,
		FileTracker: a.FileTracker,
		internal:    a,
	}
}

// Config returns the pure-data configuration.
func (a *App) Config() *Config {
	return a.internal.Config()
}

// Store returns the config store.
func (a *App) Store() *ConfigStore {
	return a.internal.Store()
}

// Shutdown performs a graceful shutdown of the application.
func (a *App) Shutdown() {
	a.internal.Shutdown()
}

// RunPrompt runs a single prompt in non-interactive mode with sensible
// defaults, writing output to os.Stdout. For more control, use
// RunPromptWithOptions or [App.RunPromptInSession].
func (a *App) RunPrompt(ctx context.Context, prompt string, opts ...RunOption) error {
	o := resolveRunOptions(opts)
	return a.internal.RunNonInteractive(ctx, os.Stdout, prompt, o.largeModel, o.smallModel, o.hideSpinner, "", false, o.agentID)
}

// RunPromptInSession runs a prompt in an existing session (or a new one if
// sessionID is empty), streaming output to the given writer.
func (a *App) RunPromptInSession(ctx context.Context, output io.Writer, sessionID, prompt string, opts ...RunOption) error {
	o := resolveRunOptions(opts)
	return a.internal.RunNonInteractive(ctx, output, prompt, o.largeModel, o.smallModel, o.hideSpinner, sessionID, false, o.agentID)
}

// RunPromptAndCreateSession creates a new named session and runs a prompt
// in it, returning the session ID so callers can retrieve messages later via
// [App.Sessions] and [App.Messages].
func (a *App) RunPromptAndCreateSession(ctx context.Context, output io.Writer, title, prompt string, opts ...RunOption) (string, error) {
	o := resolveRunOptions(opts)
	sess, err := a.Sessions.Create(ctx, title)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	return sess.ID, a.internal.RunNonInteractive(ctx, output, prompt, o.largeModel, o.smallModel, o.hideSpinner, sess.ID, false, o.agentID)
}

// RunPrompt runs a single prompt in non-interactive mode with sensible defaults.
// Output is written to os.Stdout, models come from config, the spinner is shown,
// and a new session is created.
// Deprecated: Use (*App).RunPrompt instead.
func RunPrompt(app *App, ctx context.Context, prompt string) error {
	return app.RunPrompt(ctx, prompt)
}

// RunPromptAndCreateSession creates a new named session and runs a prompt in it,
// returning the session ID so callers can retrieve messages later via
// [App.Sessions] and [App.Messages].
// Deprecated: Use (*App).RunPromptAndCreateSession instead.
func RunPromptAndCreateSession(app *App, ctx context.Context, output io.Writer, title, prompt string) (string, error) {
	return app.RunPromptAndCreateSession(ctx, output, title, prompt)
}

// RunPromptInSession runs a prompt in an existing session (or a new one if
// sessionID is empty), streaming output to the given writer.
// Deprecated: Use (*App).RunPromptInSession instead.
func RunPromptInSession(app *App, ctx context.Context, output io.Writer, sessionID, prompt string) error {
	return app.RunPromptInSession(ctx, output, sessionID, prompt)
}
