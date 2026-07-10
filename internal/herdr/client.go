// Package herdr provides native integration with the herdr terminal
// multiplexer. When Crush runs inside a herdr-managed pane it reports
// agent state (idle, working, blocked) and session identity over
// herdr's Unix socket API so herdr can display accurate status without
// screen scraping.
//
// The client consumes a small, herdr-specific event vocabulary rather
// than accepting raw proto or domain types. Callers translate their
// events into herdr.Event before forwarding. This keeps the client
// decoupled from both the proto and internal domain layers.
package herdr

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"
)

// State values matching herdr's PaneAgentState enum.
const (
	stateIdle    = "idle"
	stateWorking = "working"
	stateBlocked = "blocked"
)

// Event is the herdr-specific event vocabulary. Each type maps to a
// distinct state transition in the agent lifecycle. Callers translate
// from proto or domain types into these before calling HandleEvent.
type Event interface {
	herdrEvent()
}

// AssistantMessage indicates the agent produced output. Transitions
// to working if not already active.
type AssistantMessage struct {
	SessionID string
}

func (AssistantMessage) herdrEvent() {}

// RunComplete indicates the agent finished a turn. Transitions to idle.
type RunComplete struct {
	SessionID string
}

func (RunComplete) herdrEvent() {}

// PermissionRequested indicates the agent is waiting for user approval.
// Transitions to blocked.
type PermissionRequested struct{}

func (PermissionRequested) herdrEvent() {}

// PermissionResolved indicates a permission decision was made.
// Transitions back to working if a run is active, idle otherwise.
type PermissionResolved struct{}

func (PermissionResolved) herdrEvent() {}

// Summarizing indicates the agent is compacting context. Transitions
// to working if not already active.
type Summarizing struct{}

func (Summarizing) herdrEvent() {}

// sender abstracts the transport layer for reporting state to herdr.
// Production uses a Unix socket; tests use a recorder.
type sender interface {
	send(req reportRequest) error
	close()
}

// Client reports Crush agent state to a running herdr instance.
type Client struct {
	socketPath string
	paneID     string

	mu        sync.Mutex
	sessionID string
	state     string
	runActive bool
	seq       uint64

	snd sender
}

// defaultClient is the process-wide herdr client. Initialized once
// via Init(). All integration sites share this single instance so
// only one Unix socket connection exists per process.
var (
	defaultClient *Client
	initOnce      sync.Once
)

// Init returns the process-wide herdr Client, creating it on first
// call from environment variables. Returns nil when Crush is not
// running inside a herdr pane. Safe to call from any goroutine.
func Init() *Client {
	initOnce.Do(func() {
		defaultClient = newFromEnv()
	})
	return defaultClient
}

func newFromEnv() *Client {
	if os.Getenv("HERDR_ENV") != "1" {
		return nil
	}
	// A test binary inherits the launching shell's HERDR_* env, so
	// without this it would attach to the developer's live pane and
	// release its agent on teardown. Skip herdr entirely under test.
	if flag.Lookup("test.v") != nil {
		slog.Debug("Herdr integration disabled: running under go test")
		return nil
	}
	socketPath := os.Getenv("HERDR_SOCKET_PATH")
	paneID := os.Getenv("HERDR_PANE_ID")
	if socketPath == "" || paneID == "" {
		slog.Debug(
			"Herdr integration disabled: incomplete environment",
			"has_socket", socketPath != "",
			"has_pane_id", paneID != "",
		)
		return nil
	}
	c := &Client{
		socketPath: socketPath,
		paneID:     paneID,
		state:      stateIdle,
		seq:        uint64(time.Now().UnixNano()),
		snd:        newUnixSender(socketPath),
	}
	c.registerInitial()
	return c
}

// registerInitial sends an initial idle-state report to herdr so the
// pane knows about the agent immediately, not just after the first
// event. Called once during client creation. Bypasses the dedup
// check since the initial state must always be reported regardless
// of redundancy.
//
// herdr remembers the highest seq it has seen per source for the
// lifetime of a pane and silently drops any report with a seq that
// is not strictly greater. Because crush seeds seq from the wall
// clock at startup (see newFromEnv), a restarted crush in the same
// pane always reports above the previous run's high-water mark, so
// the first report is accepted instead of being rejected as stale.
func (c *Client) registerInitial() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snd.send(c.newRequestLocked("pane.report_agent", "init", stateIdle))
}

// Close releases the agent's authority on the pane and shuts down
// the background writer. Safe to call on a nil client.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.releaseAgent()
	c.snd.close()
}

// releaseAgent sends a pane.release_agent request to herdr so the
// pane is freed for a new agent to claim authority. This is the
// clean-shutdown protocol per herdr's socket API. Sends directly
// on the socket to ensure delivery even if the write loop is busy.
func (c *Client) releaseAgent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	req := c.newRequestLocked("pane.release_agent", "release", "")
	if err := dialSend(c.socketPath, req); err != nil {
		slog.Debug("Herdr release_agent failed", "error", err)
	}
}

// HandleEvent processes a single herdr event and reports state changes.
// Safe to call from any goroutine.
func (c *Client) HandleEvent(ev Event) {
	if c == nil {
		return
	}
	switch e := ev.(type) {
	case AssistantMessage:
		c.onAssistantMessage(e.SessionID)
	case RunComplete:
		c.onRunComplete(e.SessionID)
	case PermissionRequested:
		c.onPermissionRequest()
	case PermissionResolved:
		c.onPermissionResolved()
	case Summarizing:
		c.onSummarizing()
	}
}

// SetSessionID sets the session ID for reporting. Call this when the
// session is created or resolved, before events start flowing.
func (c *Client) SetSessionID(id string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionID = id
}

func (c *Client) onAssistantMessage(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if sessionID != "" {
		c.sessionID = sessionID
	}
	if !c.runActive {
		c.runActive = true
		c.reportLocked(stateWorking)
	}
}

func (c *Client) onRunComplete(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runActive = false
	if sessionID != "" {
		c.sessionID = sessionID
	}
	c.reportLocked(stateIdle)
}

func (c *Client) onPermissionRequest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	// A permission request implies a run is active, even if no
	// assistant message has arrived yet (e.g. tool calls that fire
	// before any text output).
	if !c.runActive {
		c.runActive = true
	}
	c.reportLocked(stateBlocked)
}

func (c *Client) onPermissionResolved() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.runActive {
		c.reportLocked(stateWorking)
	} else {
		c.reportLocked(stateIdle)
	}
}

func (c *Client) onSummarizing() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.runActive {
		c.runActive = true
	}
	c.reportLocked(stateWorking)
}

// newRequestLocked builds a seq-stamped JSON-RPC request to herdr.
// Must be called with c.mu held. Every request increments c.seq so
// herdr accepts it as strictly newer than the last (see
// registerInitial for why monotonic seq matters). State is empty for
// requests that carry no agent state, such as pane.release_agent.
func (c *Client) newRequestLocked(method, idPrefix, state string) reportRequest {
	c.seq++
	return reportRequest{
		ID:     fmt.Sprintf("crush:%s:%d", idPrefix, time.Now().UnixNano()),
		Method: method,
		Params: reportParams{
			PaneID:         c.paneID,
			Source:         "crush",
			Agent:          "crush",
			State:          state,
			Seq:            c.seq,
			AgentSessionID: c.sessionID,
		},
	}
}

// reportLocked sends a pane.report_agent request to herdr. Must be
// called with c.mu held. Skips redundant reports when the state has
// not changed.
func (c *Client) reportLocked(state string) {
	if state == c.state {
		return
	}
	c.state = state
	c.snd.send(c.newRequestLocked("pane.report_agent", "report", state))
}

// reportRequest is the JSON-RPC envelope sent to herdr.
type reportRequest struct {
	ID     string       `json:"id"`
	Method string       `json:"method"`
	Params reportParams `json:"params"`
}

// reportParams carries the agent state payload.
type reportParams struct {
	PaneID         string `json:"pane_id"`
	Source         string `json:"source"`
	Agent          string `json:"agent"`
	State          string `json:"state"`
	Seq            uint64 `json:"seq"`
	AgentSessionID string `json:"agent_session_id"`
}

// unixSender sends JSON-RPC requests over a Unix domain socket using
// a single background writer goroutine and a buffered channel. This
// serializes writes and avoids spawning unbounded goroutines under
// high event throughput. Each report opens a short-lived connection.
type unixSender struct {
	socketPath string
	ch         chan reportRequest
	cancel     context.CancelFunc
}

func newUnixSender(socketPath string) *unixSender {
	ctx, cancel := context.WithCancel(context.Background())
	s := &unixSender{
		socketPath: socketPath,
		ch:         make(chan reportRequest, 16),
		cancel:     cancel,
	}
	go s.writeLoop(ctx)
	return s
}

func (s *unixSender) send(req reportRequest) error {
	select {
	case s.ch <- req:
	default:
		// Drop if the buffer is full. State reports are
		// best-effort; blocking the agent is worse than
		// missing a transition.
	}
	return nil
}

func (s *unixSender) close() {
	s.cancel()
}

func (s *unixSender) writeLoop(ctx context.Context) {
	for {
		select {
		case req, ok := <-s.ch:
			if !ok {
				return
			}
			if err := dialSend(s.socketPath, req); err != nil {
				slog.Debug("Herdr report failed", "error", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// dialSend opens a short-lived Unix socket connection to herdr,
// sends a single JSON-RPC request, and drains the response.
func dialSend(socketPath string, req reportRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = conn.Write(data)
	if err != nil {
		return err
	}

	// Drain the response to complete the request cycle.
	_, _ = io.Copy(io.Discard, conn)
	return nil
}
