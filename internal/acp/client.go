package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	acp "github.com/coder/acp-go-sdk"
)

// ElicitationHandler answers an elicitation/create request from an external
// Agent. The handler owns all user interaction and validation.
type ElicitationHandler func(context.Context, acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error)

// PermissionHandler answers a session/request_permission request from an
// external Agent. Canceling the context means the turn was cancelled.
type PermissionHandler func(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error)

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithCWD sets the working directory for new sessions.
func WithCWD(dir string) ClientOption {
	return func(c *Client) { c.cwd = dir }
}

// WithAutoAllowPermissions causes permission requests to be approved automatically.
func WithAutoAllowPermissions() ClientOption {
	return func(c *Client) { c.autoAllow = true }
}

// WithSystemPrompt sets an append-only system prompt for new sessions.
func WithSystemPrompt(text string) ClientOption {
	return func(c *Client) { c.systemPrompt = text }
}

// WithElicitationHandler installs the client-side elicitation UI handler.
func WithElicitationHandler(handler ElicitationHandler) ClientOption {
	return func(c *Client) { c.elicitationHandler = handler }
}

// WithPermissionHandler installs the client-side permission UI handler.
func WithPermissionHandler(handler PermissionHandler) ClientOption {
	return func(c *Client) { c.permissionHandler = handler }
}

type permissionWaiter struct {
	cancel     context.CancelFunc
	cancelOnce sync.Once
	cancelled  chan struct{}
}

// Client drives an external ACP agent over stdio.
type Client struct {
	conn      *acp.ClientSideConnection
	cmd       *exec.Cmd
	pipeIn    io.WriteCloser // agent stdin pipe (our writer)
	pipeOut   io.ReadCloser  // agent stdout pipe (our reader)
	log       *slog.Logger
	cwd       string
	autoAllow bool
	mu        sync.Mutex

	systemPrompt        string
	elicitationHandler  ElicitationHandler
	permissionHandler   PermissionHandler
	terminals           *terminalRegistry
	terminalSeq         atomic.Uint64
	elicitations        map[acp.ElicitationId]struct{}
	permissionMu        sync.Mutex
	permissionWaiters   map[acp.SessionId]map[*permissionWaiter]struct{}
	permissionCancelled map[acp.SessionId]struct{}

	// Per-session update channels.
	sessMu     sync.Mutex
	sessionChs map[acp.SessionId]chan acp.SessionUpdate
}

// NewClient prepares an external ACP agent process and returns a Client ready
// to drive it. The context bounds the subprocess, so both the caller and its
// cancellation propagate to the agent. Call Start before using other methods.
func NewClient(ctx context.Context, agentPath string, args []string, opts ...ClientOption) (*Client, error) {
	c := &Client{
		cwd:                 ".",
		sessionChs:          make(map[acp.SessionId]chan acp.SessionUpdate),
		terminals:           newTerminalRegistry(),
		elicitations:        make(map[acp.ElicitationId]struct{}),
		permissionWaiters:   make(map[acp.SessionId]map[*permissionWaiter]struct{}),
		permissionCancelled: make(map[acp.SessionId]struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}

	cmd := exec.CommandContext(ctx, agentPath, args...)
	cmd.Stderr = os.Stderr

	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	impl := &crushClient{client: c}
	c.conn = acp.NewClientSideConnection(impl, in, out)
	c.pipeIn = in
	c.pipeOut = out
	c.cmd = cmd

	return c, nil
}

// Start launches the agent subprocess and begins the JSON-RPC message loop.
// Blocks until the connection closes or ctx is cancelled.
func (c *Client) Start(ctx context.Context) error {
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start agent: %w", err)
	}
	c.log.Info("ACP client: agent process started", "pid", c.cmd.Process.Pid)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := c.cmd.Wait(); err != nil {
			c.log.Warn("ACP client: agent exited with error", "err", err)
		}
	}()

	// The SDK drains its input pipe internally; we just wait for Done().
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-c.conn.Done()
	}()

	select {
	case <-ctx.Done():
		_ = c.pipeIn.Close()
		wg.Wait()
		return ctx.Err()
	case <-done:
		wg.Wait()
		return fmt.Errorf("agent process exited")
	}
}

// Close shuts down the client gracefully.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sessMu.Lock()
	for id, ch := range c.sessionChs {
		close(ch)
		delete(c.sessionChs, id)
	}
	c.sessMu.Unlock()
	c.cancelPermissionRequests("")

	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Signal(os.Interrupt); err != nil {
			c.log.Warn("ACP client: failed to send interrupt", "err", err)
		}
	}
	return nil
}

// Initialize sends the ACP initialize request and returns the agent's response.
func (c *Client) Initialize(ctx context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error) {
	resp, err := c.conn.Initialize(ctx, req)
	if err != nil {
		return acp.InitializeResponse{}, fmt.Errorf("initialize: %w", err)
	}
	c.log.Info("ACP client: initialized", "version", resp.ProtocolVersion, "agent", resp.AgentInfo.Name)
	return resp, nil
}

// NewSession creates a new session with the agent.
func (c *Client) NewSession(ctx context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	c.clearPermissionCancel("")
	if req.SystemPrompt == nil && c.systemPrompt != "" {
		req.SystemPrompt = &acp.SystemPrompt{
			Mode:    acp.SystemPromptModeAppend,
			Content: []acp.ContentBlock{acp.TextBlock(c.systemPrompt)},
		}
	}
	resp, err := c.conn.NewSession(ctx, req)
	if err != nil {
		return acp.NewSessionResponse{}, fmt.Errorf("new session: %w", err)
	}
	c.sessMu.Lock()
	ch := make(chan acp.SessionUpdate, 64)
	c.sessionChs[resp.SessionId] = ch
	c.sessMu.Unlock()
	c.clearPermissionCancel(resp.SessionId)
	c.log.Info("ACP client: new session", "sessionId", resp.SessionId)
	return resp, nil
}

// ListSessions lists active sessions on the agent.
func (c *Client) ListSessions(ctx context.Context, req acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return c.conn.ListSessions(ctx, req)
}

// CloseSession closes an existing session.
func (c *Client) CloseSession(ctx context.Context, req acp.CloseSessionRequest) error {
	_, err := c.conn.CloseSession(ctx, req)
	if err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	c.sessMu.Lock()
	if ch, ok := c.sessionChs[req.SessionId]; ok {
		close(ch)
		delete(c.sessionChs, req.SessionId)
	}
	c.sessMu.Unlock()
	c.cancelPermissionRequests(req.SessionId)
	c.clearPermissionCancel(req.SessionId)
	return nil
}

// Cancel cancels an ongoing prompt turn for the given session.
func (c *Client) Cancel(ctx context.Context, req acp.CancelNotification) error {
	c.cancelPermissionRequests(req.SessionId)
	return c.conn.Cancel(ctx, req)
}

// Prompt sends a prompt to the agent and waits for the response.
func (c *Client) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	c.clearPermissionCancel(req.SessionId)
	resp, err := c.conn.Prompt(ctx, req)
	if err != nil {
		return acp.PromptResponse{}, fmt.Errorf("prompt: %w", err)
	}
	c.log.Info("ACP client: prompt done", "sessionId", req.SessionId, "stopReason", resp.StopReason)
	return resp, nil
}

// Subscribe returns a channel that receives session updates for the given session.
// The caller is responsible for draining the channel. The channel is closed when
// the session is closed or the client disconnects.
func (c *Client) Subscribe(sessionID acp.SessionId) (<-chan acp.SessionUpdate, error) {
	c.sessMu.Lock()
	defer c.sessMu.Unlock()
	ch, ok := c.sessionChs[sessionID]
	if !ok {
		return nil, fmt.Errorf("no session %s", sessionID)
	}
	return ch, nil
}

// --- acp.Client implementation (callbacks from agent) ---

type crushClient struct {
	client *Client
}

func (c *Client) connDone() <-chan struct{} {
	if c.conn == nil {
		return nil
	}
	return c.conn.Done()
}

func (cc *crushClient) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	cc.client.sessMu.Lock()
	ch, ok := cc.client.sessionChs[params.SessionId]
	cc.client.sessMu.Unlock()
	if !ok {
		return nil
	}
	select {
	case ch <- params.Update:
	default:
		// Drop updates if the buffer is full.
	}
	return nil
}

func (cc *crushClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return cc.client.requestPermission(ctx, params)
}

func (c *Client) requestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	c.log.Info("ACP client: permission requested", "sessionId", params.SessionId, "toolCall", params.ToolCall.Title)
	c.permissionMu.Lock()
	_, cancelled := c.permissionCancelled[params.SessionId]
	c.permissionMu.Unlock()
	if cancelled {
		return cancelledPermission(), nil
	}
	if c.permissionHandler == nil {
		return c.immediatePermission(params), nil
	}

	handlerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	waiter := &permissionWaiter{cancel: cancel, cancelled: make(chan struct{})}
	type result struct {
		resp acp.RequestPermissionResponse
		err  error
	}
	answered := make(chan result, 1)

	c.permissionMu.Lock()
	if c.permissionWaiters[params.SessionId] == nil {
		c.permissionWaiters[params.SessionId] = make(map[*permissionWaiter]struct{})
	}
	c.permissionWaiters[params.SessionId][waiter] = struct{}{}
	c.permissionMu.Unlock()
	defer func() {
		c.permissionMu.Lock()
		delete(c.permissionWaiters[params.SessionId], waiter)
		if len(c.permissionWaiters[params.SessionId]) == 0 {
			delete(c.permissionWaiters, params.SessionId)
		}
		c.permissionMu.Unlock()
	}()

	go func() {
		resp, err := c.permissionHandler(handlerCtx, params)
		answered <- result{resp: resp, err: err}
	}()

	done := c.connDone()
	select {
	case res := <-answered:
		select {
		case <-waiter.cancelled:
			return cancelledPermission(), nil
		default:
		}
		if handlerCtx.Err() != nil {
			return cancelledPermission(), nil
		}
		return res.resp, res.err
	case <-waiter.cancelled:
		return cancelledPermission(), nil
	case <-ctx.Done():
		return cancelledPermission(), nil
	case <-done:
		return cancelledPermission(), nil
	}
}

func (c *Client) immediatePermission(params acp.RequestPermissionRequest) acp.RequestPermissionResponse {
	if c.autoAllow {
		c.log.Info("ACP client: auto-allowing permission")
		var chosen acp.PermissionOptionId
		for _, opt := range params.Options {
			if opt.Kind == acp.PermissionOptionKindAllowOnce || opt.Kind == acp.PermissionOptionKindAllowAlways {
				chosen = opt.OptionId
				break
			}
		}
		return acp.RequestPermissionResponse{
			Outcome: acp.NewRequestPermissionOutcomeSelected(chosen),
		}
	}
	// Default: allow once on first option.
	if len(params.Options) > 0 {
		return acp.RequestPermissionResponse{
			Outcome: acp.NewRequestPermissionOutcomeSelected(params.Options[0].OptionId),
		}
	}
	return cancelledPermission()
}

func cancelledPermission() acp.RequestPermissionResponse {
	return acp.RequestPermissionResponse{
		Outcome: acp.NewRequestPermissionOutcomeCancelled(),
	}
}

func (c *Client) cancelPermissionRequests(sessionID acp.SessionId) {
	c.permissionMu.Lock()
	if sessionID == "" {
		for id, waiters := range c.permissionWaiters {
			c.cancelWaitersLocked(waiters)
			delete(c.permissionWaiters, id)
		}
		c.permissionCancelled = make(map[acp.SessionId]struct{})
	} else {
		c.cancelWaitersLocked(c.permissionWaiters[sessionID])
		delete(c.permissionWaiters, sessionID)
		c.permissionCancelled[sessionID] = struct{}{}
	}
	c.permissionMu.Unlock()
}

func (c *Client) cancelWaitersLocked(waiters map[*permissionWaiter]struct{}) {
	for waiter := range waiters {
		waiter.cancelOnce.Do(func() {
			close(waiter.cancelled)
			waiter.cancel()
		})
	}
}

func (c *Client) clearPermissionCancel(sessionID acp.SessionId) {
	c.permissionMu.Lock()
	if sessionID == "" {
		c.permissionCancelled = make(map[acp.SessionId]struct{})
	} else {
		delete(c.permissionCancelled, sessionID)
	}
	c.permissionMu.Unlock()
}

func (cc *crushClient) ReadTextFile(_ context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	data, err := os.ReadFile(params.Path)
	if err != nil {
		// A missing file is a referenced resource that does not exist, which
		// the spec answers with -32002 rather than a generic failure
		// (error.md:38,60).
		if errors.Is(err, fs.ErrNotExist) {
			return acp.ReadTextFileResponse{}, acp.NewRequestError(
				acpErrorResourceNotFound,
				"Resource not found",
				map[string]any{"path": params.Path},
			)
		}
		return acp.ReadTextFileResponse{}, fmt.Errorf("read file: %w", err)
	}
	content := string(data)
	if params.Line != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		start := int64(0)
		if params.Line != nil {
			start = *params.Line - 1
			if start < 0 {
				start = 0
			}
		}
		end := int64(len(lines))
		if params.Limit != nil {
			end = start + *params.Limit
			if end > int64(len(lines)) {
				end = int64(len(lines))
			}
		}
		if start > int64(len(lines)) {
			start = int64(len(lines))
		}
		content = strings.Join(lines[start:end], "\n")
	}
	return acp.ReadTextFileResponse{Content: content}, nil
}

func (cc *crushClient) WriteTextFile(_ context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	dir := params.Path
	if i := strings.LastIndex(dir, "/"); i >= 0 {
		dir = dir[:i]
	}
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(params.Path, []byte(params.Content), 0o644); err != nil {
		return acp.WriteTextFileResponse{}, fmt.Errorf("write file: %w", err)
	}
	return acp.WriteTextFileResponse{}, nil
}

func (cc *crushClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	cwd := cc.client.cwd
	if params.Cwd != nil {
		cwd = *params.Cwd
	}
	if !filepath.IsAbs(cwd) {
		return acp.CreateTerminalResponse{}, acp.NewInvalidParams(map[string]any{"cwd": cwd})
	}
	if !withinRoot(cwd, cc.client.cwd) {
		return acp.CreateTerminalResponse{}, acp.NewInvalidParams(map[string]any{"cwd": cwd})
	}
	id := acp.TerminalId(fmt.Sprintf("term-%d", cc.client.terminalSeq.Add(1)))
	if _, err := cc.client.terminals.create(ctx, id, params.Command, params.Args, params.Env, cwd, params.OutputByteLimit); err != nil {
		return acp.CreateTerminalResponse{}, err
	}
	return acp.CreateTerminalResponse{TerminalId: id}, nil
}

func terminalNotFound(id acp.TerminalId) error {
	return acp.NewRequestError(-32002, "Terminal not found", map[string]any{"terminalId": string(id)})
}

func withinRoot(target, root string) bool {
	if root == "" {
		return true
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (cc *crushClient) KillTerminal(_ context.Context, params acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	if !cc.client.terminals.kill(params.TerminalId) {
		return acp.KillTerminalResponse{}, terminalNotFound(params.TerminalId)
	}
	return acp.KillTerminalResponse{}, nil
}

func (cc *crushClient) TerminalOutput(_ context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	output, truncated, status, ok := cc.client.terminals.output(params.TerminalId)
	if !ok {
		return acp.TerminalOutputResponse{}, terminalNotFound(params.TerminalId)
	}
	return acp.TerminalOutputResponse{Output: output, Truncated: truncated, ExitStatus: status}, nil
}

func (cc *crushClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	code, signal, ok := cc.client.terminals.wait(ctx, params.TerminalId)
	if !ok {
		return acp.WaitForTerminalExitResponse{}, terminalNotFound(params.TerminalId)
	}
	return acp.WaitForTerminalExitResponse{ExitCode: code, Signal: signal}, nil
}

func (cc *crushClient) ReleaseTerminal(_ context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	if !cc.client.terminals.release(params.TerminalId) {
		return acp.ReleaseTerminalResponse{}, terminalNotFound(params.TerminalId)
	}
	return acp.ReleaseTerminalResponse{}, nil
}

// CreateElicitation dispatches to the installed CLI handler.
func (cc *crushClient) CreateElicitation(ctx context.Context, req acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	if cc.client.elicitationHandler == nil {
		return acp.CreateElicitationResponse{}, acp.NewMethodNotFound("elicitation/create")
	}
	if req.Url != nil {
		cc.client.sessMu.Lock()
		if _, exists := cc.client.elicitations[req.Url.ElicitationId]; exists {
			cc.client.sessMu.Unlock()
			return acp.CreateElicitationResponse{}, acp.NewInvalidParams(map[string]any{"elicitationId": string(req.Url.ElicitationId)})
		}
		cc.client.elicitations[req.Url.ElicitationId] = struct{}{}
		cc.client.sessMu.Unlock()
	}
	return cc.client.elicitationHandler(ctx, req)
}

// CompleteElicitation acknowledges the end of a URL-based elicitation.
func (cc *crushClient) CompleteElicitation(_ context.Context, notification acp.CompleteElicitationNotification) error {
	return cc.client.completeElicitation(notification)
}

func (c *Client) completeElicitation(notification acp.CompleteElicitationNotification) error {
	c.sessMu.Lock()
	delete(c.elicitations, notification.ElicitationId)
	c.sessMu.Unlock()
	return nil
}

// parseToolCall extracts raw input from a tool call update as a map.
func parseToolCall(update acp.SessionUpdate) (map[string]any, bool) {
	if update.ToolCall == nil || len(update.ToolCall.RawInput) == 0 {
		return nil, false
	}
	var input map[string]any
	if err := json.Unmarshal(update.ToolCall.RawInput, &input); err != nil {
		return nil, false
	}
	return input, true
}

// drain reads all available updates from a channel until it is closed.
func drain(ch <-chan acp.SessionUpdate) []acp.SessionUpdate {
	var updates []acp.SessionUpdate
	for u := range ch {
		updates = append(updates, u)
	}
	return updates
}
