package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/questions"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// ClientWorkspace implements the Workspace interface by delegating all
// operations to a remote server via the client SDK. It caches the
// proto.Workspace returned at creation time and refreshes it after
// config-mutating operations.
type ClientWorkspace struct {
	client *client.Client

	mu     sync.RWMutex
	ws     proto.Workspace
	skills *skills.Manager
}

// NewClientWorkspace creates a new ClientWorkspace that proxies all
// operations through the given client SDK. The ws parameter is the
// proto.Workspace snapshot returned by the server at creation time. The
// snapshot's Skills field seeds a process-local skills.Manager so the
// TUI sees discovery state before the first SSE event arrives. The
// manager is constructed with WithGlobalMirror because the client
// process represents exactly one workspace and the TUI reads
// skills.GetLatestStates directly at construction time.
func NewClientWorkspace(c *client.Client, ws proto.Workspace) *ClientWorkspace {
	if ws.Config != nil {
		ws.Config.SetupAgents()
	}
	states := protoToSkillStates(ws.Skills)
	mgr := skills.NewManager(nil, nil, states, skills.WithGlobalMirror())
	return &ClientWorkspace{
		client: c,
		ws:     ws,
		skills: mgr,
	}
}

// refreshWorkspace re-fetches the workspace from the server, updating
// the cached snapshot. Called after config-mutating operations.
func (w *ClientWorkspace) refreshWorkspace() {
	updated, err := w.client.GetWorkspace(context.Background(), w.workspaceID())
	if err != nil {
		slog.Error("Failed to refresh workspace", "error", err)
		return
	}
	if updated.Config != nil {
		updated.Config.SetupAgents()
	}
	w.mu.Lock()
	w.ws = *updated
	w.mu.Unlock()
}

// cached returns a snapshot of the cached workspace.
func (w *ClientWorkspace) cached() proto.Workspace {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.ws
}

// workspaceID returns the cached workspace ID.
func (w *ClientWorkspace) workspaceID() string {
	return w.cached().ID
}

// -- Sessions --

func (w *ClientWorkspace) CreateSession(ctx context.Context, title string) (session.Session, error) {
	sess, err := w.client.CreateSession(ctx, w.workspaceID(), title)
	if err != nil {
		return session.Session{}, err
	}
	return protoToSession(*sess), nil
}

func (w *ClientWorkspace) GetSession(ctx context.Context, sessionID string) (session.Session, error) {
	sess, err := w.client.GetSession(ctx, w.workspaceID(), sessionID)
	if err != nil {
		return session.Session{}, err
	}
	return protoToSession(*sess), nil
}

func (w *ClientWorkspace) ListSessions(ctx context.Context) ([]session.Session, error) {
	protoSessions, err := w.client.ListSessions(ctx, w.workspaceID())
	if err != nil {
		return nil, err
	}
	sessions := make([]session.Session, len(protoSessions))
	for i, s := range protoSessions {
		sessions[i] = protoToSession(s)
	}
	return sessions, nil
}

func (w *ClientWorkspace) SaveSession(ctx context.Context, sess session.Session) (session.Session, error) {
	saved, err := w.client.SaveSession(ctx, w.workspaceID(), sessionToProto(sess))
	if err != nil {
		return session.Session{}, err
	}
	return protoToSession(*saved), nil
}

func (w *ClientWorkspace) DeleteSession(ctx context.Context, sessionID string) error {
	return w.client.DeleteSession(ctx, w.workspaceID(), sessionID)
}

func (w *ClientWorkspace) CreateAgentToolSessionID(messageID, toolCallID string) string {
	return fmt.Sprintf("%s$$%s", messageID, toolCallID)
}

func (w *ClientWorkspace) ParseAgentToolSessionID(sessionID string) (string, string, bool) {
	parts := strings.Split(sessionID, "$$")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// SetCurrentSession reports the session this client is currently
// viewing to the server. Empty sessionID clears the entry. Errors
// are propagated to the caller; the TUI logs and ignores them since
// the presence record is a hint, not correctness-critical state.
func (w *ClientWorkspace) SetCurrentSession(ctx context.Context, sessionID string) error {
	return w.client.SetCurrentSession(ctx, w.workspaceID(), sessionID)
}

// -- Messages --

func (w *ClientWorkspace) ListMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	msgs, err := w.client.ListMessages(ctx, w.workspaceID(), sessionID)
	if err != nil {
		return nil, err
	}
	return protoToMessages(msgs), nil
}

func (w *ClientWorkspace) ListUserMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	msgs, err := w.client.ListUserMessages(ctx, w.workspaceID(), sessionID)
	if err != nil {
		return nil, err
	}
	return protoToMessages(msgs), nil
}

func (w *ClientWorkspace) ListAllUserMessages(ctx context.Context) ([]message.Message, error) {
	msgs, err := w.client.ListAllUserMessages(ctx, w.workspaceID())
	if err != nil {
		return nil, err
	}
	return protoToMessages(msgs), nil
}

// -- Agent --

func (w *ClientWorkspace) AgentRun(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) error {
	// The interactive TUI does not consume notify.RunComplete for
	// completion detection (it observes message events directly),
	// so passing an empty RunID is correct here: it skips the
	// correlator stamping path without functional consequences.
	return w.client.SendMessage(ctx, w.workspaceID(), sessionID, "", prompt, attachments...)
}

func (w *ClientWorkspace) AgentCancel(sessionID string) {
	_ = w.client.CancelAgentSession(context.Background(), w.workspaceID(), sessionID)
}

func (w *ClientWorkspace) AgentIsBusy() bool {
	info, err := w.client.GetAgentInfo(context.Background(), w.workspaceID())
	if err != nil {
		return false
	}
	return info.IsBusy
}

func (w *ClientWorkspace) AgentIsSessionBusy(sessionID string) bool {
	info, err := w.client.GetAgentSessionInfo(context.Background(), w.workspaceID(), sessionID)
	if err != nil {
		return false
	}
	return info.IsBusy
}

func (w *ClientWorkspace) AgentModel() AgentModel {
	info, err := w.client.GetAgentInfo(context.Background(), w.workspaceID())
	if err != nil {
		return AgentModel{}
	}
	return AgentModel{
		CatwalkCfg: info.Model,
		ModelCfg:   info.ModelCfg,
	}
}

func (w *ClientWorkspace) AgentIsReady() bool {
	info, err := w.client.GetAgentInfo(context.Background(), w.workspaceID())
	if err != nil {
		return false
	}
	return info.IsReady
}

func (w *ClientWorkspace) AgentQueuedPrompts(sessionID string) int {
	count, err := w.client.GetAgentSessionQueuedPrompts(context.Background(), w.workspaceID(), sessionID)
	if err != nil {
		return 0
	}
	return count
}

func (w *ClientWorkspace) AgentQueuedPromptsList(sessionID string) []string {
	prompts, err := w.client.GetAgentSessionQueuedPromptsList(context.Background(), w.workspaceID(), sessionID)
	if err != nil {
		return nil
	}
	return prompts
}

func (w *ClientWorkspace) AgentClearQueue(sessionID string) {
	_ = w.client.ClearAgentSessionQueuedPrompts(context.Background(), w.workspaceID(), sessionID)
}

func (w *ClientWorkspace) AgentSummarize(ctx context.Context, sessionID string) error {
	return w.client.AgentSummarizeSession(ctx, w.workspaceID(), sessionID)
}

func (w *ClientWorkspace) UpdateAgentModel(ctx context.Context) error {
	return w.client.UpdateAgent(ctx, w.workspaceID())
}

func (w *ClientWorkspace) InitCoderAgent(ctx context.Context) error {
	return w.client.InitiateAgentProcessing(ctx, w.workspaceID())
}

func (w *ClientWorkspace) GetDefaultSmallModel(providerID string) config.SelectedModel {
	model, err := w.client.GetDefaultSmallModel(context.Background(), w.workspaceID(), providerID)
	if err != nil {
		return config.SelectedModel{}
	}
	return *model
}

// -- Permissions --

func (w *ClientWorkspace) PermissionGrant(perm permission.PermissionRequest) bool {
	resolved, _ := w.client.GrantPermission(context.Background(), w.workspaceID(), proto.PermissionGrant{
		Permission: proto.PermissionRequest{
			ID:          perm.ID,
			SessionID:   perm.SessionID,
			ToolCallID:  perm.ToolCallID,
			ToolName:    perm.ToolName,
			Description: perm.Description,
			Action:      perm.Action,
			Path:        perm.Path,
			Params:      perm.Params,
		},
		Action: proto.PermissionAllow,
	})
	return resolved
}

func (w *ClientWorkspace) PermissionGrantPersistent(perm permission.PermissionRequest) bool {
	resolved, _ := w.client.GrantPermission(context.Background(), w.workspaceID(), proto.PermissionGrant{
		Permission: proto.PermissionRequest{
			ID:          perm.ID,
			SessionID:   perm.SessionID,
			ToolCallID:  perm.ToolCallID,
			ToolName:    perm.ToolName,
			Description: perm.Description,
			Action:      perm.Action,
			Path:        perm.Path,
			Params:      perm.Params,
		},
		Action: proto.PermissionAllowForSession,
	})
	return resolved
}

func (w *ClientWorkspace) PermissionDeny(perm permission.PermissionRequest) bool {
	resolved, _ := w.client.GrantPermission(context.Background(), w.workspaceID(), proto.PermissionGrant{
		Permission: proto.PermissionRequest{
			ID:          perm.ID,
			SessionID:   perm.SessionID,
			ToolCallID:  perm.ToolCallID,
			ToolName:    perm.ToolName,
			Description: perm.Description,
			Action:      perm.Action,
			Path:        perm.Path,
			Params:      perm.Params,
		},
		Action: proto.PermissionDeny,
	})
	return resolved
}

func (w *ClientWorkspace) PermissionSkipRequests() bool {
	skip, err := w.client.GetPermissionsSkipRequests(context.Background(), w.workspaceID())
	if err != nil {
		return false
	}
	return skip
}

func (w *ClientWorkspace) PermissionSetSkipRequests(skip bool) {
	_ = w.client.SetPermissionsSkipRequests(context.Background(), w.workspaceID(), skip)
}

func (w *ClientWorkspace) QuestionsAnswer(res questions.QuestionsResponse) {
	// TODO Implement for ClientWorkspace.
}

// -- FileTracker --

func (w *ClientWorkspace) FileTrackerRecordRead(ctx context.Context, sessionID, path string) {
	_ = w.client.FileTrackerRecordRead(ctx, w.workspaceID(), sessionID, path)
}

func (w *ClientWorkspace) FileTrackerLastReadTime(ctx context.Context, sessionID, path string) time.Time {
	t, err := w.client.FileTrackerLastReadTime(ctx, w.workspaceID(), sessionID, path)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (w *ClientWorkspace) FileTrackerListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	return w.client.FileTrackerListReadFiles(ctx, w.workspaceID(), sessionID)
}

// -- History --

func (w *ClientWorkspace) ListSessionHistory(ctx context.Context, sessionID string) ([]history.File, error) {
	files, err := w.client.ListSessionHistoryFiles(ctx, w.workspaceID(), sessionID)
	if err != nil {
		return nil, err
	}
	return protoToFiles(files), nil
}

// -- LSP --

func (w *ClientWorkspace) LSPStart(ctx context.Context, path string) {
	_ = w.client.LSPStart(ctx, w.workspaceID(), path)
}

func (w *ClientWorkspace) LSPStopAll(ctx context.Context) {
	_ = w.client.LSPStopAll(ctx, w.workspaceID())
}

func (w *ClientWorkspace) LSPGetStates() map[string]LSPClientInfo {
	states, err := w.client.GetLSPs(context.Background(), w.workspaceID())
	if err != nil {
		return nil
	}
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

func (w *ClientWorkspace) LSPGetDiagnosticCounts(name string) lsp.DiagnosticCounts {
	diags, err := w.client.GetLSPDiagnostics(context.Background(), w.workspaceID(), name)
	if err != nil {
		return lsp.DiagnosticCounts{}
	}
	var counts lsp.DiagnosticCounts
	for _, fileDiags := range diags {
		for _, d := range fileDiags {
			switch d.Severity {
			case protocol.SeverityError:
				counts.Error++
			case protocol.SeverityWarning:
				counts.Warning++
			case protocol.SeverityInformation:
				counts.Information++
			case protocol.SeverityHint:
				counts.Hint++
			}
		}
	}
	return counts
}

// -- Config (read-only) --

func (w *ClientWorkspace) Config() *config.Config {
	return w.cached().Config
}

func (w *ClientWorkspace) WorkingDir() string {
	return w.cached().Path
}

func (w *ClientWorkspace) Resolver() config.VariableResolver {
	return config.IdentityResolver()
}

// -- Config mutations --

func (w *ClientWorkspace) UpdatePreferredModel(scope config.Scope, modelType config.SelectedModelType, model config.SelectedModel) error {
	err := w.client.UpdatePreferredModel(context.Background(), w.workspaceID(), scope, modelType, model)
	if err == nil {
		w.refreshWorkspace()
	}
	return err
}

func (w *ClientWorkspace) SetCompactMode(scope config.Scope, enabled bool) error {
	err := w.client.SetCompactMode(context.Background(), w.workspaceID(), scope, enabled)
	if err == nil {
		w.refreshWorkspace()
	}
	return err
}

func (w *ClientWorkspace) SetProviderAPIKey(scope config.Scope, providerID string, apiKey any) error {
	err := w.client.SetProviderAPIKey(context.Background(), w.workspaceID(), scope, providerID, apiKey)
	if err == nil {
		w.refreshWorkspace()
	}
	return err
}

func (w *ClientWorkspace) SetConfigField(scope config.Scope, key string, value any) error {
	err := w.client.SetConfigField(context.Background(), w.workspaceID(), scope, key, value)
	if err == nil {
		w.refreshWorkspace()
	}
	return err
}

func (w *ClientWorkspace) RemoveConfigField(scope config.Scope, key string) error {
	err := w.client.RemoveConfigField(context.Background(), w.workspaceID(), scope, key)
	if err == nil {
		w.refreshWorkspace()
	}
	return err
}

func (w *ClientWorkspace) ImportCopilot() (*oauth.Token, bool) {
	token, ok, err := w.client.ImportCopilot(context.Background(), w.workspaceID())
	if err != nil {
		return nil, false
	}
	if ok {
		w.refreshWorkspace()
	}
	return token, ok
}

func (w *ClientWorkspace) RefreshOAuthToken(ctx context.Context, scope config.Scope, providerID string) error {
	err := w.client.RefreshOAuthToken(ctx, w.workspaceID(), scope, providerID)
	if err == nil {
		w.refreshWorkspace()
	}
	return err
}

// -- Project lifecycle --

func (w *ClientWorkspace) ProjectNeedsInitialization() (bool, error) {
	return w.client.ProjectNeedsInitialization(context.Background(), w.workspaceID())
}

func (w *ClientWorkspace) MarkProjectInitialized() error {
	return w.client.MarkProjectInitialized(context.Background(), w.workspaceID())
}

func (w *ClientWorkspace) InitializePrompt() (string, error) {
	return w.client.GetInitializePrompt(context.Background(), w.workspaceID())
}

func (w *ClientWorkspace) ListSkills(ctx context.Context) ([]skills.CatalogEntry, error) {
	entries, err := w.client.ListSkills(ctx, w.workspaceID())
	if err != nil {
		return nil, err
	}
	result := make([]skills.CatalogEntry, len(entries))
	for i, entry := range entries {
		result[i] = skills.CatalogEntry{
			ID:            entry.ID,
			Name:          entry.Name,
			Description:   entry.Description,
			Label:         entry.Label,
			Source:        skills.SourceType(entry.Source),
			UserInvocable: entry.UserInvocable,
		}
	}
	return result, nil
}

func (w *ClientWorkspace) ReadSkill(ctx context.Context, skillID string) ([]byte, skills.SkillReadResult, error) {
	resp, err := w.client.ReadSkill(ctx, w.workspaceID(), skillID)
	if err != nil {
		return nil, skills.SkillReadResult{}, err
	}
	return resp.Content, skills.SkillReadResult{
		Name:        resp.Result.Name,
		Description: resp.Result.Description,
		Source:      skills.SourceType(resp.Result.Source),
		Builtin:     resp.Result.Builtin,
	}, nil
}

// -- MCP operations --

func (w *ClientWorkspace) MCPGetStates() map[string]mcp.ClientInfo {
	states, err := w.client.MCPGetStates(context.Background(), w.workspaceID())
	if err != nil {
		return nil
	}
	result := make(map[string]mcp.ClientInfo, len(states))
	for k, v := range states {
		result[k] = mcp.ClientInfo{
			Name:  v.Name,
			State: mcp.State(v.State),
			Error: v.Error,
			Counts: mcp.Counts{
				Tools:     v.ToolCount,
				Prompts:   v.PromptCount,
				Resources: v.ResourceCount,
			},
			ConnectedAt: v.ConnectedAt,
		}
	}
	return result
}

func (w *ClientWorkspace) MCPRefreshPrompts(ctx context.Context, name string) {
	_ = w.client.MCPRefreshPrompts(ctx, w.workspaceID(), name)
}

func (w *ClientWorkspace) MCPRefreshResources(ctx context.Context, name string) {
	_ = w.client.MCPRefreshResources(ctx, w.workspaceID(), name)
}

func (w *ClientWorkspace) RefreshMCPTools(ctx context.Context, name string) {
	_ = w.client.RefreshMCPTools(ctx, w.workspaceID(), name)
}

func (w *ClientWorkspace) ReadMCPResource(ctx context.Context, name, uri string) ([]MCPResourceContents, error) {
	contents, err := w.client.ReadMCPResource(ctx, w.workspaceID(), name, uri)
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

func (w *ClientWorkspace) GetMCPPrompt(clientID, promptID string, args map[string]string) (string, error) {
	return w.client.GetMCPPrompt(context.Background(), w.workspaceID(), clientID, promptID, args)
}

func (w *ClientWorkspace) EnableDockerMCP(ctx context.Context) error {
	return w.client.EnableDockerMCP(ctx, w.workspaceID())
}

func (w *ClientWorkspace) DisableDockerMCP() error {
	return w.client.DisableDockerMCP(context.Background(), w.workspaceID())
}

// -- Lifecycle --

func (w *ClientWorkspace) Subscribe(program *tea.Program) {
	defer log.RecoverPanic("ClientWorkspace.Subscribe", func() {
		slog.Info("TUI subscription panic: attempting graceful shutdown")
		program.Quit()
	})

	evc, err := w.client.SubscribeEvents(context.Background(), w.workspaceID())
	if err != nil {
		slog.Error("Failed to subscribe to events", "error", err)
		return
	}

	w.consumeEvents(evc, program.Send)
}

// consumeEvents drives the workspace event loop. It is split out from
// Subscribe so tests can drive it without a real *tea.Program.
// ConfigChanged events trigger a workspace refresh; all other events
// are translated into domain types and forwarded to send.
func (w *ClientWorkspace) consumeEvents(evc <-chan any, send func(tea.Msg)) {
	for ev := range evc {
		if _, ok := ev.(pubsub.Event[proto.ConfigChanged]); ok {
			w.refreshWorkspace()
			continue
		}
		translated := w.translateEvent(ev)
		if translated != nil && send != nil {
			send(translated)
		}
	}
}

func (w *ClientWorkspace) Shutdown() {
	_ = w.client.DeleteWorkspace(context.Background(), w.workspaceID())
}

// translateEvent converts proto-typed SSE events into the domain types
// that the TUI's Update() method expects. Skills events also update the
// process-local skills.Manager so callers reading
// skills.GetLatestStates see fresh data.
func (w *ClientWorkspace) translateEvent(ev any) tea.Msg {
	switch e := ev.(type) {
	case pubsub.Event[proto.LSPEvent]:
		return pubsub.Event[LSPEvent]{
			Type: e.Type,
			Payload: LSPEvent{
				Type:            LSPEventType(e.Payload.Type),
				Name:            e.Payload.Name,
				State:           e.Payload.State,
				Error:           e.Payload.Error,
				DiagnosticCount: e.Payload.DiagnosticCount,
			},
		}
	case pubsub.Event[proto.MCPEvent]:
		return pubsub.Event[mcp.Event]{
			Type: e.Type,
			Payload: mcp.Event{
				Type:  protoToMCPEventType(e.Payload.Type),
				Name:  e.Payload.Name,
				State: mcp.State(e.Payload.State),
				Error: e.Payload.Error,
				Counts: mcp.Counts{
					Tools:     e.Payload.ToolCount,
					Prompts:   e.Payload.PromptCount,
					Resources: e.Payload.ResourceCount,
				},
			},
		}
	case pubsub.Event[proto.PermissionRequest]:
		return pubsub.Event[permission.PermissionRequest]{
			Type: e.Type,
			Payload: permission.PermissionRequest{
				ID:          e.Payload.ID,
				SessionID:   e.Payload.SessionID,
				ToolCallID:  e.Payload.ToolCallID,
				ToolName:    e.Payload.ToolName,
				Description: e.Payload.Description,
				Action:      e.Payload.Action,
				Path:        e.Payload.Path,
				Params:      e.Payload.Params,
			},
		}
	case pubsub.Event[proto.PermissionNotification]:
		return pubsub.Event[permission.PermissionNotification]{
			Type: e.Type,
			Payload: permission.PermissionNotification{
				ToolCallID: e.Payload.ToolCallID,
				Granted:    e.Payload.Granted,
				Denied:     e.Payload.Denied,
			},
		}
	case pubsub.Event[proto.Message]:
		return pubsub.Event[message.Message]{
			Type:    e.Type,
			Payload: protoToMessage(e.Payload),
		}
	case pubsub.Event[proto.Session]:
		return pubsub.Event[session.Session]{
			Type:    e.Type,
			Payload: protoToSession(e.Payload),
		}
	case pubsub.Event[proto.File]:
		return pubsub.Event[history.File]{
			Type:    e.Type,
			Payload: protoToFile(e.Payload),
		}
	case pubsub.Event[proto.AgentEvent]:
		n := notify.Notification{
			SessionID:    e.Payload.SessionID,
			SessionTitle: e.Payload.SessionTitle,
			RunID:        e.Payload.RunID,
			Type:         notify.Type(e.Payload.Type),
		}
		if e.Payload.Error != nil {
			n.Message = e.Payload.Error.Error()
		}
		return pubsub.Event[notify.Notification]{
			Type:    e.Type,
			Payload: n,
		}
	case pubsub.Event[proto.RunComplete]:
		// Translate the wire-level proto.RunComplete back into the
		// agent's domain notify.RunComplete. Without this case the
		// default branch below warns on every run completion in the
		// server-mode TUI, even though the TUI itself doesn't act
		// on RunComplete — converting silently keeps the workspace
		// event bridge symmetric with the server-side wrapEvent.
		return pubsub.Event[notify.RunComplete]{
			Type: e.Type,
			Payload: notify.RunComplete{
				SessionID: e.Payload.SessionID,
				RunID:     e.Payload.RunID,
				MessageID: e.Payload.MessageID,
				Text:      e.Payload.Text,
				Error:     e.Payload.Error,
				Cancelled: e.Payload.Cancelled,
			},
		}
	case pubsub.Event[proto.SkillsEvent]:
		states := protoToSkillStates(e.Payload.States)
		if w.skills != nil {
			w.skills.SetLatestStates(states)
		}
		return pubsub.Event[skills.Event]{
			Type:    e.Type,
			Payload: skills.Event{States: states},
		}
	default:
		slog.Warn("Unknown event type in translateEvent", "type", fmt.Sprintf("%T", ev))
		return nil
	}
}

func protoToMCPEventType(t proto.MCPEventType) mcp.EventType {
	switch t {
	case proto.MCPEventStateChanged:
		return mcp.EventStateChanged
	case proto.MCPEventToolsListChanged:
		return mcp.EventToolsListChanged
	case proto.MCPEventPromptsListChanged:
		return mcp.EventPromptsListChanged
	case proto.MCPEventResourcesListChanged:
		return mcp.EventResourcesListChanged
	default:
		return mcp.EventStateChanged
	}
}

// protoToSession converts a wire-level proto.Session into the domain
// session.Session. Fields that exist only on the wire (computed-on-read
// signals like IsBusy, and any future presence counters) are
// intentionally dropped here: session.Session models persisted state,
// not transient runtime signals. UI features that need those signals
// should either extend session.Session or read them from the proto
// payload directly before this conversion runs.
func protoToSession(s proto.Session) session.Session {
	return session.Session{
		ID:               s.ID,
		ParentSessionID:  s.ParentSessionID,
		Title:            s.Title,
		SummaryMessageID: s.SummaryMessageID,
		MessageCount:     s.MessageCount,
		PromptTokens:     s.PromptTokens,
		CompletionTokens: s.CompletionTokens,
		Cost:             s.Cost,
		Todos:            protoToTodos(s.Todos),
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

func protoToTodos(todos []proto.Todo) []session.Todo {
	if len(todos) == 0 {
		return nil
	}
	out := make([]session.Todo, len(todos))
	for i, t := range todos {
		out[i] = session.Todo{
			Content:    t.Content,
			Status:     session.TodoStatus(t.Status),
			ActiveForm: t.ActiveForm,
		}
	}
	return out
}

func protoToFile(f proto.File) history.File {
	return history.File{
		ID:        f.ID,
		SessionID: f.SessionID,
		Path:      f.Path,
		Content:   f.Content,
		Version:   f.Version,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
	}
}

func protoToMessage(m proto.Message) message.Message {
	msg := message.Message{
		ID:        m.ID,
		SessionID: m.SessionID,
		Role:      message.MessageRole(m.Role),
		Model:     m.Model,
		Provider:  m.Provider,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}

	for _, p := range m.Parts {
		switch v := p.(type) {
		case proto.TextContent:
			msg.Parts = append(msg.Parts, message.TextContent{Text: v.Text})
		case proto.ReasoningContent:
			msg.Parts = append(msg.Parts, message.ReasoningContent{
				Thinking:   v.Thinking,
				Signature:  v.Signature,
				StartedAt:  v.StartedAt,
				FinishedAt: v.FinishedAt,
			})
		case proto.ToolCall:
			msg.Parts = append(msg.Parts, message.ToolCall{
				ID:       v.ID,
				Name:     v.Name,
				Input:    v.Input,
				Finished: v.Finished,
			})
		case proto.ToolResult:
			msg.Parts = append(msg.Parts, message.ToolResult{
				ToolCallID: v.ToolCallID,
				Name:       v.Name,
				Content:    v.Content,
				Data:       v.Data,
				MIMEType:   v.MIMEType,
				Metadata:   v.Metadata,
				IsError:    v.IsError,
			})
		case proto.Finish:
			msg.Parts = append(msg.Parts, message.Finish{
				Reason:  message.FinishReason(v.Reason),
				Time:    v.Time,
				Message: v.Message,
				Details: v.Details,
			})
		case proto.ImageURLContent:
			msg.Parts = append(msg.Parts, message.ImageURLContent{URL: v.URL, Detail: v.Detail})
		case proto.BinaryContent:
			msg.Parts = append(msg.Parts, message.BinaryContent{Path: v.Path, MIMEType: v.MIMEType, Data: v.Data})
		}
	}

	return msg
}

func protoToMessages(msgs []proto.Message) []message.Message {
	out := make([]message.Message, len(msgs))
	for i, m := range msgs {
		out[i] = protoToMessage(m)
	}
	return out
}

func protoToFiles(files []proto.File) []history.File {
	out := make([]history.File, len(files))
	for i, f := range files {
		out[i] = protoToFile(f)
	}
	return out
}

func sessionToProto(s session.Session) proto.Session {
	return proto.Session{
		ID:               s.ID,
		ParentSessionID:  s.ParentSessionID,
		Title:            s.Title,
		SummaryMessageID: s.SummaryMessageID,
		MessageCount:     s.MessageCount,
		PromptTokens:     s.PromptTokens,
		CompletionTokens: s.CompletionTokens,
		Cost:             s.Cost,
		Todos:            todosToProto(s.Todos),
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

// protoToSkillStates reconstructs internal skill state slices from
// their wire representation. Non-empty Error strings are turned into
// synthetic error values; the TUI never type-asserts on Err.
func protoToSkillStates(in []proto.SkillState) []*skills.SkillState {
	if len(in) == 0 {
		return nil
	}
	out := make([]*skills.SkillState, len(in))
	for i, s := range in {
		state := &skills.SkillState{
			Name:  s.Name,
			Path:  s.Path,
			State: skills.DiscoveryState(s.State),
		}
		if s.Error != "" {
			state.Err = errors.New(s.Error)
		}
		out[i] = state
	}
	return out
}

func todosToProto(todos []session.Todo) []proto.Todo {
	if len(todos) == 0 {
		return nil
	}
	out := make([]proto.Todo, len(todos))
	for i, t := range todos {
		out[i] = proto.Todo{
			Content:    t.Content,
			Status:     string(t.Status),
			ActiveForm: t.ActiveForm,
		}
	}
	return out
}
