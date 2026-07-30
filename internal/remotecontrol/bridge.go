package remotecontrol

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/workspace"
)

// Bridge attaches remote-control to a live workspace: register sessions,
// forward phone prompts into AgentRun, stream/snapshot messages, and dual-path
// tool approvals.
type Bridge struct {
	cfg Config
	ws  workspace.Workspace

	mu     sync.Mutex
	client *Client
	cancel context.CancelFunc

	// enabled session ids (per-session RC flag).
	enabled map[string]struct{}
}

// NewBridge creates a bridge that is not yet connected.
func NewBridge(cfg Config, ws workspace.Workspace) *Bridge {
	return &Bridge{
		cfg:     cfg,
		ws:      ws,
		enabled: make(map[string]struct{}),
	}
}

// IsEnabled reports whether sessionID has remote control on.
func (b *Bridge) IsEnabled(sessionID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.enabled[sessionID]
	return ok
}

// Connected reports whether the relay WebSocket is up.
func (b *Bridge) Connected() bool {
	b.mu.Lock()
	c := b.client
	b.mu.Unlock()
	return c != nil && c.IsConnected()
}

// EnabledCount is how many sessions are shared.
func (b *Bridge) EnabledCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.enabled)
}

// Enable turns remote control on for sessionID, connecting if needed.
func (b *Bridge) Enable(ctx context.Context, sess session.Session) error {
	if sess.ID == "" {
		return fmt.Errorf("no active session")
	}
	if err := b.ensureConnected(ctx); err != nil {
		return err
	}

	info := b.sessionInfo(sess)
	b.mu.Lock()
	b.enabled[sess.ID] = struct{}{}
	client := b.client
	b.mu.Unlock()

	if err := client.RegisterSession(info); err != nil {
		return fmt.Errorf("register session: %w", err)
	}
	// Best-effort initial snapshot so a phone that already selected this id
	// can refresh; primary path is request_snapshot on select.
	go b.pushSnapshot(sess.ID)
	return nil
}

// Disable turns remote control off for sessionID. Closes the socket when none remain.
func (b *Bridge) Disable(sessionID string) error {
	b.mu.Lock()
	delete(b.enabled, sessionID)
	client := b.client
	remaining := len(b.enabled)
	b.mu.Unlock()

	if client == nil {
		return nil
	}
	_ = client.UnregisterSession(sessionID)
	if remaining == 0 {
		return b.Close()
	}
	return nil
}

// Toggle enables or disables RC for the session. Returns the new enabled state.
func (b *Bridge) Toggle(ctx context.Context, sess session.Session) (bool, error) {
	if b.IsEnabled(sess.ID) {
		if err := b.Disable(sess.ID); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := b.Enable(ctx, sess); err != nil {
		return false, err
	}
	return true, nil
}

// Close disconnects and clears enabled sessions.
func (b *Bridge) Close() error {
	b.mu.Lock()
	cancel := b.cancel
	client := b.client
	b.cancel = nil
	b.client = nil
	b.enabled = make(map[string]struct{})
	b.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if client != nil {
		return client.Close()
	}
	return nil
}

// ForwardPermission sends a tool approval request to the phone when the
// session is remote-enabled. Safe to call for every permission event.
func (b *Bridge) ForwardPermission(req permission.PermissionRequest) {
	if !b.IsEnabled(req.SessionID) || !b.Connected() {
		return
	}
	b.mu.Lock()
	client := b.client
	b.mu.Unlock()
	if client == nil {
		return
	}
	_ = client.SendToolRequest(req.SessionID, ToolRequestPayload{
		RequestID:   req.ID,
		ToolName:    req.ToolName,
		Description: req.Description,
		Command:     fmt.Sprint(req.Params),
		Action:      req.Action,
		Path:        req.Path,
	})
}

// PublishMessage sends a stream_chunk for an updated message when RC is on.
func (b *Bridge) PublishMessage(msg message.Message) {
	if msg.SessionID == "" || !b.IsEnabled(msg.SessionID) || !b.Connected() {
		return
	}
	b.mu.Lock()
	client := b.client
	b.mu.Unlock()
	if client == nil {
		return
	}
	content := msg.Content().Text
	if content == "" && len(msg.ToolCalls()) == 0 && len(msg.ToolResults()) == 0 {
		return
	}
	if content == "" {
		// Surface tool activity briefly so the phone is not silent.
		for _, tc := range msg.ToolCalls() {
			content += fmt.Sprintf("[tool] %s\n", tc.Name)
		}
	}
	_ = client.SendStreamChunk(msg.SessionID, StreamChunkPayload{
		MessageID: msg.ID,
		Role:      string(msg.Role),
		Content:   content,
		Done:      msg.IsFinished(),
	})
}

// RefreshSession re-advertises session metadata (title/busy).
func (b *Bridge) RefreshSession(sess session.Session, busy bool) {
	if !b.IsEnabled(sess.ID) || !b.Connected() {
		return
	}
	b.mu.Lock()
	client := b.client
	b.mu.Unlock()
	if client == nil {
		return
	}
	info := b.sessionInfo(sess)
	info.Busy = busy
	_ = client.RegisterSession(info)
	_ = client.SendSessionState(sess.ID, SessionStatePayload{
		Busy:  busy,
		Title: sess.Title,
	})
}

func (b *Bridge) ensureConnected(ctx context.Context) error {
	b.mu.Lock()
	if b.client != nil && b.client.IsConnected() {
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()

	client := NewClient(b.cfg)
	client.SetHandlers(Handlers{
		OnPrompt:          b.onPrompt,
		OnCancel:          b.onCancel,
		OnToolResponse:    b.onToolResponse,
		OnRequestSnapshot: b.onRequestSnapshot,
	})

	// Bound only dial/login. Socket lifetime is owned by b.cancel → Close.
	dialCtx := ctx
	var dialCancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		dialCtx, dialCancel = context.WithTimeout(ctx, 20*time.Second)
		defer dialCancel()
	}
	if err := client.Connect(dialCtx); err != nil {
		return err
	}

	lifeCtx, cancel := context.WithCancel(context.Background())
	go func() {
		<-lifeCtx.Done()
		_ = client.Close()
	}()

	b.mu.Lock()
	// If another goroutine won the race, drop this connection.
	if b.client != nil && b.client.IsConnected() {
		b.mu.Unlock()
		cancel()
		_ = client.Close()
		return nil
	}
	if b.cancel != nil {
		b.cancel()
	}
	b.client = client
	b.cancel = cancel
	b.mu.Unlock()
	return nil
}

func (b *Bridge) sessionInfo(sess session.Session) SessionInfo {
	modelName := ""
	if cfg := b.ws.Config(); cfg != nil {
		if agentCfg, ok := cfg.Agents["coder"]; ok {
			if m, ok := cfg.Models[agentCfg.Model]; ok {
				modelName = m.Provider + "/" + m.Model
			}
		}
	}
	return SessionInfo{
		ID:        sess.ID,
		Title:     sess.Title,
		Cwd:       b.ws.WorkingDir(),
		Busy:      b.ws.AgentIsSessionBusy(sess.ID),
		Model:     modelName,
		UpdatedAt: sess.UpdatedAt,
	}
}

func (b *Bridge) onPrompt(sessionID, prompt string) {
	if !b.IsEnabled(sessionID) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := b.ws.AgentRun(ctx, sessionID, prompt); err != nil {
			slog.Warn("Remote control prompt failed", "session", sessionID, "err", err)
			b.mu.Lock()
			client := b.client
			b.mu.Unlock()
			if client != nil {
				_ = client.SendError(sessionID, ErrorPayload{
					Message: err.Error(),
					Code:    "prompt_failed",
				})
			}
		}
	}()
}

func (b *Bridge) onCancel(sessionID string) {
	if !b.IsEnabled(sessionID) {
		return
	}
	b.ws.AgentCancel(sessionID)
}

func (b *Bridge) onToolResponse(sessionID, requestID string, approved bool) {
	if !b.IsEnabled(sessionID) || requestID == "" {
		return
	}
	perm := permission.PermissionRequest{
		ID:        requestID,
		SessionID: sessionID,
	}
	if approved {
		b.ws.PermissionGrant(perm)
	} else {
		b.ws.PermissionDeny(perm)
	}
}

func (b *Bridge) onRequestSnapshot(sessionID string) {
	if !b.IsEnabled(sessionID) {
		return
	}
	go b.pushSnapshot(sessionID)
}

func (b *Bridge) pushSnapshot(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	msgs, err := b.ws.ListMessages(ctx, sessionID)
	if err != nil {
		slog.Warn("Remote control snapshot failed", "session", sessionID, "err", err)
		return
	}
	snap := SessionSnapshotPayload{Messages: make([]SnapshotMessage, 0, len(msgs))}
	for _, m := range msgs {
		content := m.Content().Text
		if content == "" {
			continue
		}
		snap.Messages = append(snap.Messages, SnapshotMessage{
			ID:        m.ID,
			Role:      string(m.Role),
			Content:   content,
			CreatedAt: m.CreatedAt,
		})
	}
	b.mu.Lock()
	client := b.client
	b.mu.Unlock()
	if client == nil {
		return
	}
	if err := client.SendSnapshot(sessionID, snap); err != nil {
		slog.Debug("Remote control send snapshot failed", "err", err)
	}
}
