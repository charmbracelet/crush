// Package acp provides ACP (Agent Client Protocol) server support for Crush.
package acp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync/atomic"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	acp "github.com/coder/acp-go-sdk"
)

// Server is the ACP server adapter for Crush.
type Server struct {
	conn        *acp.AgentSideConnection
	permb       *permissionBridge
	permService permission.Service
	eventBridge *eventBridge
}

// Agent is the Crush-side interface the SDK calls into.
type Agent interface {
	Initialize(ctx context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error)
	NewSession(ctx context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error)
	ListSessions(ctx context.Context, req acp.ListSessionsRequest) (acp.ListSessionsResponse, error)
	CloseSession(ctx context.Context, req acp.CloseSessionRequest) (acp.CloseSessionResponse, error)
	ResumeSession(ctx context.Context, req acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error)
	Cancel(ctx context.Context, req acp.CancelNotification) error
	Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error)
	SetSessionMode(ctx context.Context, req acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error)
	SetSessionConfigOption(ctx context.Context, req acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error)
	Logout(ctx context.Context, req acp.LogoutRequest) (acp.LogoutResponse, error)
	DeleteSession(ctx context.Context, req acp.DeleteSessionRequest) (acp.DeleteSessionResponse, error)
	// ValidateCwd reports whether a session cwd names this workspace.
	ValidateCwd(cwd string) error
	// SessionExists reports whether the store knows a session.
	SessionExists(ctx context.Context, sessionID acp.SessionId) bool
	// MarkSessionActive records that a session was created, loaded or resumed
	// on this connection, which is what makes it active for the session-scoped
	// requests (error.md:56).
	MarkSessionActive(sessionID acp.SessionId)
	// SessionState reports the mode and config-option payload for a session.
	SessionState(sessionID acp.SessionId) (*acp.SessionModeState, []acp.SessionConfigOption)
	// SetSessionRoots records the additional roots a setup request supplied.
	SetSessionRoots(sessionID acp.SessionId, dirs []string) error
	// ConnectMCPServers starts the MCP servers a setup request supplied. The
	// dispatcher owns session/load, which carries the same field session/new
	// and session/resume do (session-setup.md:458).
	ConnectMCPServers(ctx context.Context, sessionID string, servers []acp.McpServer) error
}

// NewServer creates a new ACP server that connects to the given peer.
func NewServer(a *app.App, log *slog.Logger, stdin io.Reader, stdout io.Writer) *Server {
	ca := newCrushAgent(a, log)
	d := &dispatcher{agent: ca, app: a}
	conn := acp.NewAgentSideConnection(d, stdout, stdin)
	d.conn = conn
	ca.conn = conn

	// Create permission bridge and wrap the real service.
	permb := newPermissionBridge(conn, log)
	permService := newACPPermissionService(a.Permissions, permb)

	// Install the client file system so view/write tools delegate to the
	// connected client when it advertises fs support (file-system.md). The
	// adapter reads the capabilities at call time, since Initialize arrives
	// after the tools are constructed.
	tools.SetClientFS(&clientFSAdapter{agent: ca})

	// Create event bridge for streaming notifications.
	eventBridge := newEventBridge(conn, a, log)
	ca.bridge = eventBridge

	return &Server{
		conn:        conn,
		permb:       permb,
		permService: permService,
		eventBridge: eventBridge,
	}
}

// PermissionService returns the wrapped permission service for use by tools.
func (s *Server) PermissionService() permission.Service {
	return s.permService
}

// StartEventBridge starts the event bridge in a goroutine. Call this before Start.
func (s *Server) StartEventBridge(ctx context.Context) {
	go s.eventBridge.Start(ctx)
}

// absPath resolves a path from a replayed tool call into its absolute form,
// for the same reason the live bridge does: the spec requires every path on the
// wire to be absolute (overview.md:160).
func (d *dispatcher) absPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	local := ""
	if d.app != nil {
		if store := d.app.Store(); store != nil {
			local = store.WorkingDir()
		}
	}
	abs, err := filepath.Abs(filepathext.SmartJoin(local, path))
	if err != nil {
		return path
	}
	return abs
}

// dispatcher forwards SDK Agent method calls to our Agent interface.
type dispatcher struct {
	agent Agent
	app   *app.App
	conn  *acp.AgentSideConnection
	// initialized records that session/initialize completed on this connection.
	// Every other request is rejected with -32600 until it is set (error.md:62-64).
	initialized atomic.Bool
}

// requireInitialized rejects a request that arrived before initialization
// completed. The spec makes this a SHOULD, and the reason it matters is that an
// early request otherwise looks like a working session to the client.
func (d *dispatcher) requireInitialized(method string) error {
	if d.initialized.Load() {
		return nil
	}
	return acp.NewInvalidRequest(map[string]any{"method": method})
}

// Authenticate answers -32601. Crush advertises no auth methods, so the spec
// says clients must not call this; answering "method not found" matches the
// guidance for a capability-gated method the Agent never advertised, whereas a
// generic error would surface as -32603 internal error.
func (d *dispatcher) Authenticate(_ context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, acp.NewMethodNotFound("authenticate")
}

func (d *dispatcher) Logout(ctx context.Context, req acp.LogoutRequest) (acp.LogoutResponse, error) {
	if err := d.requireInitialized("logout"); err != nil {
		return acp.LogoutResponse{}, err
	}
	return d.agent.Logout(ctx, req)
}

func (d *dispatcher) Initialize(ctx context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error) {
	resp, err := d.agent.Initialize(ctx, req)
	if err != nil {
		return acp.InitializeResponse{}, err
	}
	d.initialized.Store(true)
	return resp, nil
}

func (d *dispatcher) NewSession(ctx context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	if err := d.requireInitialized("session/new"); err != nil {
		return acp.NewSessionResponse{}, err
	}
	return d.agent.NewSession(ctx, req)
}

func (d *dispatcher) ListSessions(ctx context.Context, req acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	if err := d.requireInitialized("session/list"); err != nil {
		return acp.ListSessionsResponse{}, err
	}
	return d.agent.ListSessions(ctx, req)
}

func (d *dispatcher) CloseSession(ctx context.Context, req acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	if err := d.requireInitialized("session/close"); err != nil {
		return acp.CloseSessionResponse{}, err
	}
	return d.agent.CloseSession(ctx, req)
}

func (d *dispatcher) ResumeSession(ctx context.Context, req acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	if err := d.requireInitialized("session/resume"); err != nil {
		return acp.ResumeSessionResponse{}, err
	}
	return d.agent.ResumeSession(ctx, req)
}

// LoadSession implements acp.AgentLoader. It replays the session's stored
// message history as session/update notifications before returning, satisfying
// the spec requirement that session/load MUST stream the entire conversation:
// user/agent text chunks, agent thought chunks, and tool calls with their
// terminal status and result content.
func (d *dispatcher) LoadSession(ctx context.Context, req acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	if err := d.requireInitialized("session/load"); err != nil {
		return acp.LoadSessionResponse{}, err
	}
	if err := d.agent.ValidateCwd(req.Cwd); err != nil {
		return acp.LoadSessionResponse{}, err
	}
	if !d.agent.SessionExists(ctx, req.SessionId) {
		return acp.LoadSessionResponse{}, acp.NewRequestError(
			acpErrorResourceNotFound,
			"Resource not found",
			map[string]any{"sessionId": string(req.SessionId)},
		)
	}
	// Loading is what makes the session active on this connection
	// (error.md:56), and it must be recorded before the replay streams so a
	// later session-scoped request passes the active check.
	d.agent.MarkSessionActive(req.SessionId)
	if err := d.agent.SetSessionRoots(req.SessionId, req.AdditionalDirectories); err != nil {
		return acp.LoadSessionResponse{}, err
	}
	sessionID := string(req.SessionId)
	// Reconnect the MCP servers the client supplied for this load. The spec
	// makes stdio a baseline (session-setup.md:458) and session/load carries
	// the same field as session/new.
	if err := d.agent.ConnectMCPServers(ctx, sessionID, req.McpServers); err != nil {
		return acp.LoadSessionResponse{}, err
	}
	msgs, err := d.app.Messages.List(ctx, sessionID)
	if err != nil {
		return acp.LoadSessionResponse{}, fmt.Errorf("list messages: %w", err)
	}

	// started tracks tool calls whose tool_call start update was already sent,
	// mapped to their raw input JSON so file-modifying results can carry diff
	// content even when the call is missing from history.
	started := map[string]string{}
	send := func(update acp.SessionUpdate) error {
		note := acp.SessionNotification{SessionId: req.SessionId, Update: update}
		if len(req.Meta) > 0 {
			note.Meta = req.Meta
		}
		return d.conn.SessionUpdate(ctx, note)
	}
	emitToolResult := func(tr message.ToolResult) error {
		id := acp.ToolCallId(tr.ToolCallID)
		input, seen := started[tr.ToolCallID]
		if !seen {
			started[tr.ToolCallID] = ""
			if err := send(acp.StartToolCall(id, toolTitle(tr.Name, ""), acp.WithStartName(tr.Name), acp.WithStartKind(toolKindFor(tr.Name)))); err != nil {
				return err
			}
		}
		// A todos result carries the full plan state; replay it (S4).
		if plan := planUpdateFor(tr); plan != nil {
			if err := send(acp.SessionUpdate{Plan: plan}); err != nil {
				return err
			}
		}
		status := acp.ToolCallStatusCompleted
		text := tr.Content
		if tr.IsError {
			status = acp.ToolCallStatusFailed
			text = "Error: " + text
		}
		var contents []acp.ToolCallContent
		if text != "" {
			contents = append(contents, acp.ToolContent(acp.TextBlock(text)))
		}
		if diff := diffContentFor(d, tr.Name, input, tr); diff != nil {
			contents = append(contents, *diff)
		}
		opts := []acp.ToolCallUpdateOpt{acp.WithUpdateStatus(status), acp.WithUpdateName(tr.Name)}
		if len(contents) > 0 {
			opts = append(opts,
				acp.WithUpdateContent(contents),
				acp.WithUpdateRawOutput(tr.Content),
			)
		}
		return send(acp.UpdateToolCall(id, opts...))
	}

	for _, m := range msgs {
		for _, part := range m.Parts {
			var err error
			switch p := part.(type) {
			case message.TextContent:
				if p.Text == "" {
					continue
				}
				switch m.Role {
				case message.User:
					err = send(withMessageID(acp.UpdateUserMessageText(p.Text), m.ID))
				case message.Assistant:
					err = send(withMessageID(acp.UpdateAgentMessageText(p.Text), m.ID))
				}
			case message.ReasoningContent:
				if m.Role == message.Assistant && p.Thinking != "" {
					err = send(withMessageID(acp.UpdateAgentThoughtText(p.Thinking), m.ID))
				}
			case message.ToolCall:
				if m.Role != message.Assistant || p.ID == "" {
					continue
				}
				started[p.ID] = p.Input
				status := acp.ToolCallStatusInProgress
				if p.Finished {
					status = acp.ToolCallStatusCompleted
				}
				err = send(acp.StartToolCall(acp.ToolCallId(p.ID), toolTitle(p.Name, p.Input),
					acp.WithStartName(p.Name),
					acp.WithStartKind(toolKindFor(p.Name)),
					acp.WithStartStatus(status),
					acp.WithStartRawInput(toolRawInput(p.Input)),
				))
			case message.ToolResult:
				if m.Role != message.Tool || p.ToolCallID == "" {
					continue
				}
				err = emitToolResult(p)
			}
			if err != nil {
				return acp.LoadSessionResponse{}, fmt.Errorf("stream loaded message: %w", err)
			}
		}
	}
	// Return the same setup payload session/new and session/resume carry, so a
	// client that loads a session can render its mode and config controls
	// without a second round-trip (session-setup.md:200).
	modes, configOptions := d.agent.SessionState(req.SessionId)
	return acp.LoadSessionResponse{Modes: modes, ConfigOptions: configOptions}, nil
}

// DeleteSession forwards the session/delete request to the agent.
func (d *dispatcher) DeleteSession(ctx context.Context, req acp.DeleteSessionRequest) (acp.DeleteSessionResponse, error) {
	if err := d.requireInitialized("session/delete"); err != nil {
		return acp.DeleteSessionResponse{}, err
	}
	return d.agent.DeleteSession(ctx, req)
}

// Cancel is a notification, so it has no error channel to report -32600
// through; the SDK's own dispatch handles a notification for an unknown method.
// A cancel that arrives before initialization simply finds nothing to cancel.
func (d *dispatcher) Cancel(ctx context.Context, req acp.CancelNotification) error {
	return d.agent.Cancel(ctx, req)
}

func (d *dispatcher) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	if err := d.requireInitialized("session/prompt"); err != nil {
		return acp.PromptResponse{}, err
	}
	return d.agent.Prompt(ctx, req)
}

func (d *dispatcher) SetSessionMode(ctx context.Context, req acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	if err := d.requireInitialized("session/set_mode"); err != nil {
		return acp.SetSessionModeResponse{}, err
	}
	return d.agent.SetSessionMode(ctx, req)
}

func (d *dispatcher) SetSessionConfigOption(ctx context.Context, req acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	if err := d.requireInitialized("session/set_config_option"); err != nil {
		return acp.SetSessionConfigOptionResponse{}, err
	}
	return d.agent.SetSessionConfigOption(ctx, req)
}

// SetLogger configures structured logging on the connection.
func (s *Server) SetLogger(l *slog.Logger) {
	s.conn.SetLogger(l)
}

// Start runs the ACP server loop. Blocks until the client disconnects.
func (s *Server) Start(ctx context.Context) error {
	done := s.conn.Done()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return errors.New("client disconnected")
	}
}
