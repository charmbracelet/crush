package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent"
	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

// AppWorkspace implements the Workspace interface by delegating
// directly to an in-process [app.App] instance. This is the default
// mode when the client/server architecture is not enabled.
type AppWorkspace struct {
	app   *app.App
	store *config.ConfigStore
}

// NewAppWorkspace creates a new AppWorkspace wrapping the given app
// and config store.
func NewAppWorkspace(a *app.App, store *config.ConfigStore) *AppWorkspace {
	return &AppWorkspace{
		app:   a,
		store: store,
	}
}

// -- Sessions --

func (w *AppWorkspace) CreateSession(ctx context.Context, title string) (session.Session, error) {
	return w.app.Sessions.Create(ctx, title)
}

func (w *AppWorkspace) GetSession(ctx context.Context, sessionID string) (session.Session, error) {
	return w.app.Sessions.Get(ctx, sessionID)
}

func (w *AppWorkspace) ListSessions(ctx context.Context) ([]session.Session, error) {
	return w.app.Sessions.List(ctx)
}

func (w *AppWorkspace) SaveSession(ctx context.Context, sess session.Session) (session.Session, error) {
	return w.app.Sessions.Save(ctx, sess)
}

func (w *AppWorkspace) DeleteSession(ctx context.Context, sessionID string) error {
	return w.app.Sessions.Delete(ctx, sessionID)
}

func (w *AppWorkspace) CreateAgentToolSessionID(messageID, toolCallID string) string {
	return w.app.Sessions.CreateAgentToolSessionID(messageID, toolCallID)
}

func (w *AppWorkspace) ParseAgentToolSessionID(sessionID string) (string, string, bool) {
	return w.app.Sessions.ParseAgentToolSessionID(sessionID)
}

// -- Messages --

func (w *AppWorkspace) ListMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	return w.app.Messages.List(ctx, sessionID)
}

func (w *AppWorkspace) ListUserMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	return w.app.Messages.ListUserMessages(ctx, sessionID)
}

func (w *AppWorkspace) ListAllUserMessages(ctx context.Context) ([]message.Message, error) {
	return w.app.Messages.ListAllUserMessages(ctx)
}

// -- Agent --

func (w *AppWorkspace) AgentRun(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) error {
	if w.app.AgentCoordinator == nil {
		return errors.New("agent coordinator not initialized")
	}
	_, err := w.app.AgentCoordinator.Run(ctx, sessionID, prompt, attachments...)
	return err
}

func (w *AppWorkspace) AgentCancel(sessionID string) {
	if w.app.AgentCoordinator != nil {
		w.app.AgentCoordinator.Cancel(sessionID)
	}
}

func (w *AppWorkspace) AgentIsBusy() bool {
	if w.app.AgentCoordinator == nil {
		return false
	}
	return w.app.AgentCoordinator.IsBusy()
}

func (w *AppWorkspace) AgentIsSessionBusy(sessionID string) bool {
	if w.app.AgentCoordinator == nil {
		return false
	}
	return w.app.AgentCoordinator.IsSessionBusy(sessionID)
}

func (w *AppWorkspace) AgentModel() AgentModel {
	if w.app.AgentCoordinator == nil {
		return AgentModel{}
	}
	m := w.app.AgentCoordinator.Model()
	return AgentModel{
		CatwalkCfg: m.CatwalkCfg,
		ModelCfg:   m.ModelCfg,
	}
}

func (w *AppWorkspace) AgentIsReady() bool {
	return w.app.AgentCoordinator != nil
}

func (w *AppWorkspace) AgentQueuedPrompts(sessionID string) int {
	if w.app.AgentCoordinator == nil {
		return 0
	}
	return w.app.AgentCoordinator.QueuedPrompts(sessionID)
}

func (w *AppWorkspace) AgentQueuedPromptsList(sessionID string) []string {
	if w.app.AgentCoordinator == nil {
		return nil
	}
	return w.app.AgentCoordinator.QueuedPromptsList(sessionID)
}

func (w *AppWorkspace) AgentClearQueue(sessionID string) {
	if w.app.AgentCoordinator != nil {
		w.app.AgentCoordinator.ClearQueue(sessionID)
	}
}

func (w *AppWorkspace) AgentSummarize(ctx context.Context, sessionID string) error {
	if w.app.AgentCoordinator == nil {
		return errors.New("agent coordinator not initialized")
	}
	return w.app.AgentCoordinator.Summarize(ctx, sessionID)
}

func (w *AppWorkspace) UpdateAgentModel(ctx context.Context) error {
	return w.app.UpdateAgentModel(ctx)
}

func (w *AppWorkspace) InitCoderAgent(ctx context.Context) error {
	return w.app.InitCoderAgent(ctx)
}

func (w *AppWorkspace) GetDefaultSmallModel(providerID string) config.SelectedModel {
	return w.app.GetDefaultSmallModel(providerID)
}

// -- Permissions --

func (w *AppWorkspace) PermissionGrant(perm permission.PermissionRequest) {
	w.app.Permissions.Grant(perm)
}

func (w *AppWorkspace) PermissionGrantPersistent(perm permission.PermissionRequest) {
	w.app.Permissions.GrantPersistent(perm)
}

func (w *AppWorkspace) PermissionDeny(perm permission.PermissionRequest) {
	w.app.Permissions.Deny(perm)
}

func (w *AppWorkspace) PermissionSkipRequests() bool {
	return w.app.Permissions.SkipRequests()
}

func (w *AppWorkspace) PermissionSetSkipRequests(skip bool) {
	w.app.Permissions.SetSkipRequests(skip)
}

// -- FileTracker --

func (w *AppWorkspace) FileTrackerRecordRead(ctx context.Context, sessionID, path string) {
	w.app.FileTracker.RecordRead(ctx, sessionID, path)
}

func (w *AppWorkspace) FileTrackerLastReadTime(ctx context.Context, sessionID, path string) time.Time {
	return w.app.FileTracker.LastReadTime(ctx, sessionID, path)
}

func (w *AppWorkspace) FileTrackerListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	return w.app.FileTracker.ListReadFiles(ctx, sessionID)
}

// -- History --

func (w *AppWorkspace) ListSessionHistory(ctx context.Context, sessionID string) ([]history.File, error) {
	return w.app.History.ListBySession(ctx, sessionID)
}

// -- Undo / Redo --

// UndoLastMessage rolls the session back by one user message. It:
//  1. Determines the target message (the previous user message when a revert
//     is already active, or the last user message otherwise).
//  2. Restores all session files to their state before that message.
//  3. Sets the revert marker on the session so the UI can hide the undone
//     messages and offer a redo.
func (w *AppWorkspace) UndoLastMessage(ctx context.Context, sessionID string) error {
	sess, err := w.app.Sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}

	var targetMsg message.Message
	if sess.RevertMessageID == "" {
		// No active revert — undo the very last user message.
		msgs, err := w.app.Messages.ListUserMessages(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("listing user messages: %w", err)
		}
		if len(msgs) == 0 {
			return errors.New("nothing to undo")
		}
		targetMsg = msgs[0] // ListUserMessages returns DESC.
	} else {
		// Already in revert — step back one more user message.
		prev, err := w.app.Messages.FindPreviousUserMessage(ctx, sessionID, sess.RevertMessageID)
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("nothing more to undo")
		}
		if err != nil {
			return fmt.Errorf("finding previous user message: %w", err)
		}
		targetMsg = prev
	}

	if err := w.app.History.RestoreToTimestamp(ctx, sessionID, targetMsg.CreatedAt); err != nil {
		return fmt.Errorf("restoring files: %w", err)
	}
	return w.app.Sessions.SetRevert(ctx, sessionID, targetMsg.ID)
}

// RedoMessage moves the revert marker forward by one user message, or clears
// it entirely when the marker is already at the last user message.
func (w *AppWorkspace) RedoMessage(ctx context.Context, sessionID string) error {
	sess, err := w.app.Sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	if sess.RevertMessageID == "" {
		return errors.New("nothing to redo")
	}

	next, err := w.app.Messages.FindNextUserMessage(ctx, sessionID, sess.RevertMessageID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("finding next user message: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		// No later user message — redo to head state (unrevert).
		if err := w.app.History.RestoreToLatest(ctx, sessionID); err != nil {
			return fmt.Errorf("restoring files to latest: %w", err)
		}
		return w.app.Sessions.ClearRevert(ctx, sessionID)
	}

	// Redo to next user message boundary.
	if err := w.app.History.RestoreToTimestamp(ctx, sessionID, next.CreatedAt); err != nil {
		return fmt.Errorf("restoring files: %w", err)
	}
	return w.app.Sessions.SetRevert(ctx, sessionID, next.ID)
}

// CleanupRevert permanently discards the messages and file-version records
// that were hidden by the revert marker, then clears the marker. Call this
// before sending a new prompt so the undone history is gone for good.
func (w *AppWorkspace) CleanupRevert(ctx context.Context, sessionID string) error {
	sess, err := w.app.Sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	if sess.RevertMessageID == "" {
		return nil // Nothing to clean up.
	}

	// Use the first undone message (the one after the revert marker) as the
	// deletion boundary rather than the revert message itself. This avoids
	// deleting records that merely share the same second-resolution timestamp
	// as the kept revert message.
	nextMsg, err := w.app.Messages.FindNextUserMessage(ctx, sessionID, sess.RevertMessageID)
	if errors.Is(err, sql.ErrNoRows) {
		// No next message — nothing to delete; just clear the marker.
		return w.app.Sessions.ClearRevert(ctx, sessionID)
	}
	if err != nil {
		return fmt.Errorf("finding first undone message: %w", err)
	}

	if err := w.app.Messages.DeleteMessagesAfterTimestamp(ctx, sessionID, nextMsg.CreatedAt); err != nil {
		return fmt.Errorf("deleting messages: %w", err)
	}
	if err := w.app.History.CleanupAfterTimestamp(ctx, sessionID, nextMsg.CreatedAt); err != nil {
		return fmt.Errorf("cleaning up file versions: %w", err)
	}
	return w.app.Sessions.ClearRevert(ctx, sessionID)
}

// -- LSP --

func (w *AppWorkspace) LSPStart(ctx context.Context, path string) {
	w.app.LSPManager.Start(ctx, path)
}

func (w *AppWorkspace) LSPStopAll(ctx context.Context) {
	w.app.LSPManager.StopAll(ctx)
}

func (w *AppWorkspace) LSPGetStates() map[string]LSPClientInfo {
	states := app.GetLSPStates()
	result := make(map[string]LSPClientInfo, len(states))
	for k, v := range states {
		result[k] = LSPClientInfo{
			Name:            v.Name,
			State:           v.State,
			Error:           v.Error,
			DiagnosticCount: v.DiagnosticCount,
			ConnectedAt:     v.ConnectedAt,
		}
	}
	return result
}

func (w *AppWorkspace) LSPGetDiagnosticCounts(name string) lsp.DiagnosticCounts {
	state, ok := app.GetLSPState(name)
	if !ok || state.Client == nil {
		return lsp.DiagnosticCounts{}
	}
	return state.Client.GetDiagnosticCounts()
}

// -- Config (read-only) --

func (w *AppWorkspace) Config() *config.Config {
	return w.store.Config()
}

func (w *AppWorkspace) WorkingDir() string {
	return w.store.WorkingDir()
}

func (w *AppWorkspace) Resolver() config.VariableResolver {
	return w.store.Resolver()
}

// -- Config mutations --

func (w *AppWorkspace) UpdatePreferredModel(scope config.Scope, modelType config.SelectedModelType, model config.SelectedModel) error {
	return w.store.UpdatePreferredModel(scope, modelType, model)
}

func (w *AppWorkspace) SetCompactMode(scope config.Scope, enabled bool) error {
	return w.store.SetCompactMode(scope, enabled)
}

func (w *AppWorkspace) SetProviderAPIKey(scope config.Scope, providerID string, apiKey any) error {
	return w.store.SetProviderAPIKey(scope, providerID, apiKey)
}

func (w *AppWorkspace) SetConfigField(scope config.Scope, key string, value any) error {
	return w.store.SetConfigField(scope, key, value)
}

func (w *AppWorkspace) RemoveConfigField(scope config.Scope, key string) error {
	return w.store.RemoveConfigField(scope, key)
}

func (w *AppWorkspace) ImportCopilot() (*oauth.Token, bool) {
	return w.store.ImportCopilot()
}

func (w *AppWorkspace) RefreshOAuthToken(ctx context.Context, scope config.Scope, providerID string) error {
	return w.store.RefreshOAuthToken(ctx, scope, providerID)
}

// -- Project lifecycle --

func (w *AppWorkspace) ProjectNeedsInitialization() (bool, error) {
	return config.ProjectNeedsInitialization(w.store)
}

func (w *AppWorkspace) MarkProjectInitialized() error {
	return config.MarkProjectInitialized(w.store)
}

func (w *AppWorkspace) InitializePrompt() (string, error) {
	return agent.InitializePrompt(w.store)
}

// -- MCP operations --

func (w *AppWorkspace) MCPGetStates() map[string]mcptools.ClientInfo {
	return mcptools.GetStates()
}

func (w *AppWorkspace) MCPRefreshPrompts(ctx context.Context, name string) {
	mcptools.RefreshPrompts(ctx, name)
}

func (w *AppWorkspace) MCPRefreshResources(ctx context.Context, name string) {
	mcptools.RefreshResources(ctx, name)
}

func (w *AppWorkspace) RefreshMCPTools(ctx context.Context, name string) {
	mcptools.RefreshTools(ctx, w.store, name)
}

func (w *AppWorkspace) ReadMCPResource(ctx context.Context, name, uri string) ([]MCPResourceContents, error) {
	contents, err := mcptools.ReadResource(ctx, w.store, name, uri)
	if err != nil {
		return nil, err
	}
	result := make([]MCPResourceContents, len(contents))
	for i, c := range contents {
		result[i] = MCPResourceContents{
			URI:      c.URI,
			MIMEType: c.MIMEType,
			Text:     c.Text,
			Blob:     c.Blob,
		}
	}
	return result, nil
}

func (w *AppWorkspace) GetMCPPrompt(clientID, promptID string, args map[string]string) (string, error) {
	return commands.GetMCPPrompt(w.store, clientID, promptID, args)
}

func (w *AppWorkspace) EnableDockerMCP(ctx context.Context) error {
	mcpConfig, err := w.store.PrepareDockerMCPConfig()
	if err != nil {
		return err
	}

	if err := mcptools.InitializeSingle(ctx, config.DockerMCPName, w.store); err != nil {
		disableErr := mcptools.DisableSingle(w.store, config.DockerMCPName)
		delete(w.store.Config().MCP, config.DockerMCPName)
		return fmt.Errorf("failed to start docker MCP: %w", errors.Join(err, disableErr))
	}

	if err := w.store.PersistDockerMCPConfig(mcpConfig); err != nil {
		disableErr := mcptools.DisableSingle(w.store, config.DockerMCPName)
		delete(w.store.Config().MCP, config.DockerMCPName)
		return fmt.Errorf("docker MCP started but failed to persist configuration: %w", errors.Join(err, disableErr))
	}

	return nil
}

func (w *AppWorkspace) DisableDockerMCP() error {
	if err := mcptools.DisableSingle(w.store, config.DockerMCPName); err != nil {
		return fmt.Errorf("failed to disable docker MCP: %w", err)
	}
	return w.store.DisableDockerMCP()
}

// -- Lifecycle --

func (w *AppWorkspace) Subscribe(program *tea.Program) {
	w.app.Subscribe(program)
}

func (w *AppWorkspace) Shutdown() {
	w.app.Shutdown()
}

// App returns the underlying app.App instance.
func (w *AppWorkspace) App() *app.App {
	return w.app
}

// Store returns the underlying config store.
func (w *AppWorkspace) Store() *config.ConfigStore {
	return w.store
}

// Compile-time check that AppWorkspace implements Workspace.
var _ Workspace = (*AppWorkspace)(nil)
