package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
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
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/subagents"
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

// SetCurrentSession reports the active session to herdr so the pane
// can persist a resumable reference. Multi-client presence tracking
// is irrelevant in single-client local mode, but herdr still needs
// to know which session is live to support agent resume.
func (w *AppWorkspace) SetCurrentSession(ctx context.Context, sessionID string) error {
	w.app.ReportCurrentSession(sessionID)
	return nil
}

// -- Messages --

func (w *AppWorkspace) ListMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	// Drain any debounced updates so the caller observes the latest
	// in-memory state. message.Service buffers streaming deltas and a
	// cold List would otherwise miss them at session-switch time.
	if err := w.app.Messages.FlushAll(ctx); err != nil {
		return nil, err
	}
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

func (w *AppWorkspace) AgentRunShellCommand(ctx context.Context, sessionID, command string, termWidth int, onProgress func(string), isFirstMessage bool) (proto.ShellCommandResponse, error) {
	var persist shell.PersistFunc
	if sessionID != "" {
		persist = func(cmd, output string, exitCode int) error {
			return shell.PersistOutput(ctx, w.app.Messages, sessionID, cmd, output, exitCode)
		}
	}

	opts := shell.RunOptions{
		Command:   command,
		Cwd:       w.store.WorkingDir(),
		TermWidth: termWidth,
	}

	var result shell.CaptureResult
	var err error

	if onProgress != nil {
		result, err = shell.RunAndCaptureStream(ctx, opts, onProgress)
	} else {
		result, err = shell.RunAndPersist(ctx, opts, persist)
	}

	if err != nil && onProgress == nil {
		return proto.ShellCommandResponse{}, err
	}

	// Persist if we used the streaming path (persist wasn't called by RunAndPersist).
	if onProgress != nil && persist != nil {
		if persistErr := persist(command, result.Output, result.ExitCode); persistErr != nil {
			slog.Error("Failed to persist shell command output", "error", persistErr, "command", command)
		}
	}

	// Generate a title from the shell command if it was the first message.
	if isFirstMessage && w.app.AgentCoordinator != nil {
		titleCtx := context.WithoutCancel(ctx)
		w.app.AgentCoordinator.GenerateTitle(titleCtx, sessionID, "$ "+command)
	}

	return proto.ShellCommandResponse{
		Output:   result.Output,
		ExitCode: result.ExitCode,
	}, nil
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

func (w *AppWorkspace) InitCoderAgentNonInteractive(ctx context.Context) error {
	return w.app.InitCoderAgentNonInteractive(ctx)
}

func (w *AppWorkspace) GetDefaultSmallModel(providerID string) config.SelectedModel {
	return w.app.GetDefaultSmallModel(providerID)
}

// -- Permissions --

func (w *AppWorkspace) PermissionGrant(perm permission.PermissionRequest) bool {
	return w.app.Permissions.Grant(perm)
}

func (w *AppWorkspace) PermissionGrantPersistent(perm permission.PermissionRequest) bool {
	return w.app.Permissions.GrantPersistent(perm)
}

func (w *AppWorkspace) PermissionDeny(perm permission.PermissionRequest) bool {
	return w.app.Permissions.Deny(perm)
}

func (w *AppWorkspace) PermissionSkipRequests() bool {
	return w.app.Permissions.SkipRequests()
}

func (w *AppWorkspace) PermissionSetSkipRequests(skip bool) {
	w.app.Permissions.SetSkipRequests(skip)
}

// -- Questions --

func (w *AppWorkspace) QuestionAnswer(responses []question.Answer) bool {
	return w.app.Questions.Answer(responses)
}

func (w *AppWorkspace) QuestionCancel() bool {
	return w.app.Questions.Cancel()
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
	if err := w.store.SetProviderAPIKey(scope, providerID, apiKey); err != nil {
		return err
	}
	w.store.SignalAuthComplete(providerID)
	return nil
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

func (w *AppWorkspace) ListSkills(_ context.Context) ([]skills.CatalogEntry, error) {
	mgr := w.app.Skills
	return skills.Catalog(mgr.ActiveSkills(), mgr.ResolvedPaths(), mgr.WorkingDir()), nil
}

func (w *AppWorkspace) ReadSkill(_ context.Context, skillID string) ([]byte, skills.SkillReadResult, error) {
	mgr := w.app.Skills
	return skills.ReadContent(mgr.ActiveSkills(), mgr.ResolvedPaths(), mgr.WorkingDir(), skillID)
}

// ActiveSubagents returns the workspace's post-filter list of active subagents
// projected to the frontend-facing SubagentInfo shape. Returns nil when the
// workspace has no Subagents manager configured.
func (w *AppWorkspace) ActiveSubagents() []SubagentInfo {
	mgr := w.app.Subagents
	if mgr == nil {
		return nil
	}
	active := mgr.ActiveSubagents()
	result := make([]SubagentInfo, len(active))
	for i, sa := range active {
		result[i] = SubagentInfo{Name: sa.Name, Description: sa.Description}
	}
	return result
}

// RunningSubagents returns info about all subagent sessions currently running
// under the given parentSessionID, enriched with token counts from the session
// service where available. Returns nil when SubagentRuntime is nil.
func (w *AppWorkspace) RunningSubagents(parentSessionID string) []RunningSubagentInfo {
	if w.app.SubagentRuntime == nil {
		return nil
	}
	entries := w.app.SubagentRuntime.List(parentSessionID)
	if len(entries) == 0 {
		return nil
	}
	result := make([]RunningSubagentInfo, len(entries))
	for i, e := range entries {
		info := RunningSubagentInfo{
			ChildSessionID:  e.ChildSessionID,
			ParentSessionID: e.ParentSessionID,
			Name:            e.Name,
			Color:           e.Color,
			Model:           e.Model,
			Status:          e.Status,
			StartedAt:       e.StartedAt,
		}
		if w.app.Sessions != nil {
			if sess, err := w.app.Sessions.Get(context.Background(), e.ChildSessionID); err == nil {
				info.PromptTokens = sess.PromptTokens
				info.CompletionTokens = sess.CompletionTokens
			}
		}
		result[i] = info
	}
	return result
}

// SubscribeSubagentRuntime returns a channel of RuntimeEvents from the
// SubagentRuntime. Returns a closed channel when SubagentRuntime is nil.
func (w *AppWorkspace) SubscribeSubagentRuntime(ctx context.Context) <-chan pubsub.Event[subagents.RuntimeEvent] {
	if w.app.SubagentRuntime == nil {
		ch := make(chan pubsub.Event[subagents.RuntimeEvent])
		close(ch)
		return ch
	}
	return w.app.SubagentRuntime.Subscribe(ctx)
}

// CancelSubagent cancels the subagent session with the given childSessionID.
// It is a no-op when AgentCoordinator is nil.
func (w *AppWorkspace) CancelSubagent(childSessionID string) {
	if w.app.AgentCoordinator == nil {
		return
	}
	w.app.AgentCoordinator.Cancel(childSessionID)
}

// AllSubagents returns all discovered subagent definitions projected to the
// frontend-facing SubagentDefInfo shape, with scope detection relative to the
// workspace working directory. Returns nil when the Subagents manager is nil.
func (w *AppWorkspace) AllSubagents() []SubagentDefInfo {
	mgr := w.app.Subagents
	if mgr == nil {
		return nil
	}
	all := mgr.AllSubagents()
	cfg := w.store.Config()
	workingDir := w.store.WorkingDir()

	var disabledSubagents []string
	if cfg.Options != nil {
		disabledSubagents = cfg.Options.DisabledSubagents
	}
	disabledSet := make(map[string]bool, len(disabledSubagents))
	for _, name := range disabledSubagents {
		disabledSet[name] = true
	}

	result := make([]SubagentDefInfo, len(all))
	for i, s := range all {
		scope := "user"
		if s.FilePath == "" {
			scope = "builtin"
		} else if workingDir != "" && (strings.HasPrefix(s.FilePath, workingDir+"/") || s.FilePath == workingDir) {
			scope = "project"
		}
		result[i] = SubagentDefInfo{
			Name:        s.Name,
			Description: s.Description,
			Color:       s.ResolvedColor(),
			FilePath:    s.FilePath,
			Scope:       scope,
			Disabled:    disabledSet[s.Name],
		}
	}
	return result
}

// DeleteUserSubagent removes a user-scoped subagent by name. It returns an
// error if the subagent is not found or is not user-scoped. On success it
// deletes the file from disk and reloads the Subagents manager.
func (w *AppWorkspace) DeleteUserSubagent(name string) error {
	var target *SubagentDefInfo
	for _, info := range w.AllSubagents() {
		if info.Name == name {
			cp := info
			target = &cp
			break
		}
	}
	if target == nil {
		return fmt.Errorf("subagent %q not found", name)
	}
	if target.Scope != "user" {
		return fmt.Errorf("subagent %q is not user-scoped and cannot be deleted", name)
	}
	if err := os.Remove(target.FilePath); err != nil {
		return err
	}
	cfg := w.store.Config()
	var subagentsPaths, disabledSubagents []string
	if cfg.Options != nil {
		subagentsPaths = cfg.Options.SubagentsPaths
		disabledSubagents = cfg.Options.DisabledSubagents
	}
	all, active, states := subagents.DiscoverFromConfig(subagents.DiscoveryConfig{
		SubagentsPaths:    subagentsPaths,
		DisabledSubagents: disabledSubagents,
		// Match startup discovery (cmd/root.go, backend.go): validate model
		// ids so a subagent with an invalid model stays rejected after reload.
		IsKnownModelID: cfg.IsKnownModelID,
	})
	w.app.Subagents.Reload(all, active, states)
	return nil
}

// SessionTokens returns the prompt and completion token counts for the given
// session. It delegates to the session service and propagates any error.
func (w *AppWorkspace) SessionTokens(ctx context.Context, sessionID string) (prompt, completion int64, err error) {
	sess, err := w.app.Sessions.Get(ctx, sessionID)
	if err != nil {
		return 0, 0, err
	}
	return sess.PromptTokens, sess.CompletionTokens, nil
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
		w.store.RemoveDockerMCPInMemory()
		return fmt.Errorf("failed to start docker MCP: %w", errors.Join(err, disableErr))
	}

	if err := w.store.PersistDockerMCPConfig(mcpConfig); err != nil {
		disableErr := mcptools.DisableSingle(w.store, config.DockerMCPName)
		w.store.RemoveDockerMCPInMemory()
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
