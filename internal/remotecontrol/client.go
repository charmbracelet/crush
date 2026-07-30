package remotecontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultPongWait       = 60 * time.Second
	defaultPingPeriod     = 50 * time.Second
	defaultWriteWait      = 10 * time.Second
	defaultMaxMessageSize = int64(1 << 20)
	defaultLoginTimeout   = 15 * time.Second
)

// Handlers are invoked from the read loop; they must not block long.
type Handlers struct {
	OnPrompt          func(sessionID, prompt string)
	OnCancel          func(sessionID string)
	OnToolResponse    func(sessionID, requestID string, approved bool)
	OnRequestSnapshot func(sessionID string)
}

// Client is a multi-session CLI WebSocket client for the crush-remote relay.
type Client struct {
	cfg      Config
	handlers Handlers

	mu        sync.RWMutex
	ws        *websocket.Conn
	writeMu   sync.Mutex
	closeCh   chan struct{}
	closeOnce sync.Once
	connected bool

	// sessions maps session id → last advertised info (for reconnect).
	sessions map[string]SessionInfo

	pongWait       time.Duration
	pingPeriod     time.Duration
	writeWait      time.Duration
	maxMessageSize int64
	loginTimeout   time.Duration
	httpClient     *http.Client
}

// NewClient builds a client. Call SetHandlers before Connect.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:            cfg,
		sessions:       make(map[string]SessionInfo),
		closeCh:        make(chan struct{}),
		pongWait:       defaultPongWait,
		pingPeriod:     defaultPingPeriod,
		writeWait:      defaultWriteWait,
		maxMessageSize: defaultMaxMessageSize,
		loginTimeout:   defaultLoginTimeout,
		httpClient:     &http.Client{Timeout: defaultLoginTimeout},
	}
}

// SetHandlers installs inbound event callbacks.
func (c *Client) SetHandlers(h Handlers) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers = h
}

// IsConnected reports whether the WebSocket is up.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.ws != nil
}

// Connect authenticates and opens the CLI WebSocket. It does not register
// sessions; call RegisterSession after Connect.
func (c *Client) Connect(ctx context.Context) error {
	if err := c.cfg.Validate(); err != nil {
		return err
	}

	token, err := c.login(ctx)
	if err != nil {
		return err
	}

	wsURL, err := cliWebSocketURL(c.cfg.RelayURL, token)
	if err != nil {
		return err
	}

	dialer := websocket.Dialer{HandshakeTimeout: c.loginTimeout}
	ws, resp, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return fmt.Errorf("remote control websocket dial: %w", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	ws.SetReadLimit(c.maxMessageSize)
	if err := ws.SetReadDeadline(time.Now().Add(c.pongWait)); err != nil {
		_ = ws.Close()
		return fmt.Errorf("remote control set read deadline: %w", err)
	}
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(c.pongWait))
	})

	c.mu.Lock()
	// Reset close coordination if this is a reconnect after Close.
	select {
	case <-c.closeCh:
		c.closeCh = make(chan struct{})
		c.closeOnce = sync.Once{}
	default:
	}
	c.ws = ws
	c.connected = true
	c.mu.Unlock()

	go c.readLoop()
	go c.pingLoop(ws)
	// Do not watch ctx here: callers often pass a short dial timeout.
	// Lifetime is owned by Close() (bridge cancel).

	return nil
}

// RegisterSession advertises a session and remembers it for reconnect.
func (c *Client) RegisterSession(info SessionInfo) error {
	if info.ID == "" {
		return fmt.Errorf("session id is required")
	}
	c.mu.Lock()
	c.sessions[info.ID] = info
	c.mu.Unlock()

	payload, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return c.SendEvent(TypeRegisterSession, info.ID, payload)
}

// UnregisterSession removes a session from the phone list.
func (c *Client) UnregisterSession(sessionID string) error {
	c.mu.Lock()
	delete(c.sessions, sessionID)
	c.mu.Unlock()
	return c.SendEvent(TypeUnregisterSession, sessionID, nil)
}

// RegisteredSessions returns a snapshot of locally tracked session ids.
func (c *Client) RegisteredSessions() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.sessions))
	for id := range c.sessions {
		out = append(out, id)
	}
	return out
}

// HasSession reports whether sessionID is currently registered.
func (c *Client) HasSession(sessionID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.sessions[sessionID]
	return ok
}

// SessionCount is the number of registered sessions.
func (c *Client) SessionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.sessions)
}

// SendEvent writes a typed frame for sessionID.
func (c *Client) SendEvent(evtType, sessionID string, payload json.RawMessage) error {
	c.mu.RLock()
	ws := c.ws
	c.mu.RUnlock()
	if ws == nil {
		return fmt.Errorf("remote control not connected")
	}

	msg := EventMessage{
		Type:            evtType,
		SessionID:       sessionID,
		Payload:         payload,
		Timestamp:       time.Now().Unix(),
		ProtocolVersion: ProtocolVersion,
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := ws.SetWriteDeadline(time.Now().Add(c.writeWait)); err != nil {
		return err
	}
	return ws.WriteJSON(msg)
}

// SendStreamChunk sends a live text chunk for a session.
func (c *Client) SendStreamChunk(sessionID string, chunk StreamChunkPayload) error {
	payload, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	return c.SendEvent(TypeStreamChunk, sessionID, payload)
}

// SendSnapshot sends a full transcript.
func (c *Client) SendSnapshot(sessionID string, snap SessionSnapshotPayload) error {
	payload, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return c.SendEvent(TypeSessionSnapshot, sessionID, payload)
}

// SendToolRequest forwards a permission prompt to the phone.
func (c *Client) SendToolRequest(sessionID string, req ToolRequestPayload) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return c.SendEvent(TypeToolRequest, sessionID, payload)
}

// SendSessionState pushes busy/title updates.
func (c *Client) SendSessionState(sessionID string, state SessionStatePayload) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return c.SendEvent(TypeSessionState, sessionID, payload)
}

// SendError reports an error for a session to the phone.
func (c *Client) SendError(sessionID string, errp ErrorPayload) error {
	payload, err := json.Marshal(errp)
	if err != nil {
		return err
	}
	return c.SendEvent(TypeError, sessionID, payload)
}

// Close tears down the WebSocket.
func (c *Client) Close() error {
	c.mu.Lock()
	ws := c.ws
	c.ws = nil
	c.connected = false
	c.mu.Unlock()

	c.closeOnce.Do(func() { close(c.closeCh) })
	if ws != nil {
		return ws.Close()
	}
	return nil
}

func (c *Client) login(ctx context.Context) (string, error) {
	httpBase, err := httpBaseURL(c.cfg.RelayURL)
	if err != nil {
		return "", err
	}

	body, err := json.Marshal(map[string]string{
		"username": c.cfg.Username,
		"password": c.cfg.Password,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, httpBase+"/api/login", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("remote control login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("remote control login: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("remote control login failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var lResp struct {
		Success bool   `json:"success"`
		Token   string `json:"token"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &lResp); err != nil {
		return "", fmt.Errorf("remote control login decode: %w", err)
	}
	if !lResp.Success || lResp.Token == "" {
		msg := lResp.Message
		if msg == "" {
			msg = "authentication failed"
		}
		return "", fmt.Errorf("remote control login failed: %s", msg)
	}
	return lResp.Token, nil
}

func (c *Client) pingLoop(ws *websocket.Conn) {
	ticker := time.NewTicker(c.pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-c.closeCh:
			return
		case <-ticker.C:
			c.writeMu.Lock()
			err := ws.SetWriteDeadline(time.Now().Add(c.writeWait))
			if err == nil {
				err = ws.WriteMessage(websocket.PingMessage, nil)
			}
			c.writeMu.Unlock()
			if err != nil {
				slog.Debug("Remote control ping failed", "err", err)
				_ = c.Close()
				return
			}
		}
	}
}

func (c *Client) readLoop() {
	defer func() {
		c.mu.Lock()
		c.connected = false
		c.ws = nil
		c.mu.Unlock()
		c.closeOnce.Do(func() { close(c.closeCh) })
	}()

	for {
		c.mu.RLock()
		ws := c.ws
		c.mu.RUnlock()
		if ws == nil {
			return
		}

		var msg EventMessage
		if err := ws.ReadJSON(&msg); err != nil {
			slog.Debug("Remote control websocket closed", "err", err)
			return
		}
		c.dispatch(msg)
	}
}

func (c *Client) dispatch(msg EventMessage) {
	c.mu.RLock()
	h := c.handlers
	c.mu.RUnlock()

	switch msg.Type {
	case TypeSendPrompt:
		var p SendPromptPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil || p.Prompt == "" {
			return
		}
		if h.OnPrompt != nil {
			h.OnPrompt(msg.SessionID, p.Prompt)
		}
	case TypeCancelTask:
		if h.OnCancel != nil {
			h.OnCancel(msg.SessionID)
		}
	case TypeToolResponse:
		var t ToolResponsePayload
		if err := json.Unmarshal(msg.Payload, &t); err != nil || t.RequestID == "" {
			return
		}
		if h.OnToolResponse != nil {
			h.OnToolResponse(msg.SessionID, t.RequestID, t.Approved)
		}
	case TypeRequestSnapshot:
		if h.OnRequestSnapshot != nil {
			h.OnRequestSnapshot(msg.SessionID)
		}
	case TypePing:
		_ = c.SendEvent(TypePong, msg.SessionID, nil)
	}
}

func httpBaseURL(relayURL string) (string, error) {
	u, err := url.Parse(relayURL)
	if err != nil {
		return "", fmt.Errorf("invalid relay URL: %w", err)
	}
	switch u.Scheme {
	case "wss":
		u.Scheme = "https"
	case "ws":
		u.Scheme = "http"
	default:
		return "", fmt.Errorf("relay URL must use ws:// or wss://")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func cliWebSocketURL(relayURL, token string) (string, error) {
	u, err := url.Parse(relayURL)
	if err != nil {
		return "", fmt.Errorf("invalid relay URL: %w", err)
	}
	base := strings.TrimRight(u.String(), "/")
	return fmt.Sprintf("%s/ws/cli?token=%s", base, url.QueryEscape(token)), nil
}
