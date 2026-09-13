package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"
)

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

	// Per-session update channels.
	sessMu     sync.Mutex
	sessionChs map[acp.SessionId]chan acp.SessionUpdate
}

// NewClient spawns an external ACP agent process and returns a Client ready
// to drive it. Call Start before using other methods.
func NewClient(agentPath string, args []string, opts ...ClientOption) (*Client, error) {
	c := &Client{
		cwd:        ".",
		sessionChs: make(map[acp.SessionId]chan acp.SessionUpdate),
	}
	for _, opt := range opts {
		opt(c)
	}

	cmd := exec.Command(agentPath, args...)
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
	resp, err := c.conn.NewSession(ctx, req)
	if err != nil {
		return acp.NewSessionResponse{}, fmt.Errorf("new session: %w", err)
	}
	c.sessMu.Lock()
	ch := make(chan acp.SessionUpdate, 64)
	c.sessionChs[resp.SessionId] = ch
	c.sessMu.Unlock()
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
	return nil
}

// Cancel cancels an ongoing prompt turn for the given session.
func (c *Client) Cancel(ctx context.Context, req acp.CancelNotification) error {
	return c.conn.Cancel(ctx, req)
}

// Prompt sends a prompt to the agent and waits for the response.
func (c *Client) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
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

func (cc *crushClient) RequestPermission(_ context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	cc.client.log.Info("ACP client: permission requested", "sessionId", params.SessionId, "toolCall", params.ToolCall.Title)
	if cc.client.autoAllow {
		cc.client.log.Info("ACP client: auto-allowing permission")
		var chosen acp.PermissionOptionId
		for _, opt := range params.Options {
			if opt.Kind == acp.PermissionOptionKindAllowOnce || opt.Kind == acp.PermissionOptionKindAllowAlways {
				chosen = opt.OptionId
				break
			}
		}
		return acp.RequestPermissionResponse{
			Outcome: acp.NewRequestPermissionOutcomeSelected(chosen),
		}, nil
	}
	// Default: allow once on first option.
	if len(params.Options) > 0 {
		return acp.RequestPermissionResponse{
			Outcome: acp.NewRequestPermissionOutcomeSelected(params.Options[0].OptionId),
		}, nil
	}
	return acp.RequestPermissionResponse{
		Outcome: acp.NewRequestPermissionOutcomeCancelled(),
	}, nil
}

func (cc *crushClient) ReadTextFile(_ context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	data, err := os.ReadFile(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("read file: %w", err)
	}
	content := string(data)
	if params.Line != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		start := 0
		if params.Line != nil {
			start = *params.Line - 1
			if start < 0 {
				start = 0
			}
		}
		end := len(lines)
		if params.Limit != nil {
			end = start + *params.Limit
			if end > len(lines) {
				end = len(lines)
			}
		}
		if start > len(lines) {
			start = len(lines)
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

func (cc *crushClient) CreateTerminal(_ context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, fmt.Errorf("terminals not supported by Crush ACP client")
}

func (cc *crushClient) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, fmt.Errorf("terminals not supported by Crush ACP client")
}

func (cc *crushClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, fmt.Errorf("terminals not supported by Crush ACP client")
}

func (cc *crushClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, fmt.Errorf("terminals not supported by Crush ACP client")
}

func (cc *crushClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, fmt.Errorf("terminals not supported by Crush ACP client")
}

// parseToolCall extracts raw input from a tool call update as a map.
func parseToolCall(update acp.SessionUpdate) (map[string]any, bool) {
	if update.ToolCall == nil || update.ToolCall.RawInput == nil {
		return nil, false
	}
	b, err := json.Marshal(update.ToolCall.RawInput)
	if err != nil {
		return nil, false
	}
	var input map[string]any
	if err := json.Unmarshal(b, &input); err != nil {
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
