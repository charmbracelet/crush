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
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// State values matching herdr's PaneAgentState enum.
const (
	stateIdle    = "idle"
	stateWorking = "working"
	stateBlocked = "blocked"
)

// blockReason identifies a cause that keeps the agent waiting on the
// user. The reported state is derived from the set of active blocks
// plus the run/summarize flags, so overlapping blocks only clear once
// every one of them is resolved.
type blockReason string

const (
	blockPermission blockReason = "permission"
	blockQuestion   blockReason = "question"
	blockAuth       blockReason = "auth"
)

// blockKey identifies one outstanding block. The id correlates a
// block with the event that resolves it: the tool call id for
// permissions, the batch id for questions. Reasons that carry no
// correlation id (auth) use the empty string. Keying on the id keeps
// a resolution for one request from clearing a block raised by
// another, and makes a re-delivered resolution idempotent.
type blockKey struct {
	reason blockReason
	id     string
}

// subSessionSeparator separates the parent message id from the tool
// call id in agent-tool sub-session ids
// ("<messageID>$$<toolCallID>", see session.CreateAgentToolSessionID).
const subSessionSeparator = "$$"

// Event is the herdr-specific event vocabulary. Each type maps to a
// distinct state transition in the agent lifecycle. Callers translate
// from proto or domain types into these before calling HandleEvent.
type Event interface {
	herdrEvent()
}

// RunStarted indicates the user submitted a prompt. Transitions to
// working immediately, before the first assistant output arrives.
type RunStarted struct {
	SessionID string
}

func (RunStarted) herdrEvent() {}

// AssistantMessage indicates the agent produced output. Transitions
// to working if not already active.
type AssistantMessage struct {
	SessionID string
	// Model is the id of the model that produced the message, when
	// known. Kept fresh on every assistant message so the pane's
	// model token self-heals after a mid-session model switch.
	Model string
	// Finished marks the terminal snapshot of an assistant message
	// (a finish part is set, e.g. stop or canceled). A finished
	// message is output, not ongoing activity, so it must not arm
	// runActive: the bridge consumes the messages broker and the
	// runCompletions broker on separate goroutines, and a finished
	// snapshot processed after its turn's RunComplete would
	// otherwise re-arm runActive and strand the pane in working.
	Finished bool
}

func (AssistantMessage) herdrEvent() {}

// RunComplete indicates the agent finished a turn. Transitions to idle.
type RunComplete struct {
	SessionID string
}

func (RunComplete) herdrEvent() {}

// PermissionRequested indicates the agent is waiting for user approval.
// Transitions to blocked.
type PermissionRequested struct {
	// ToolCallID identifies the tool call awaiting approval. It
	// pairs the block with the PermissionResolved that ends it.
	ToolCallID string
	// ToolName is the tool asking for approval ("bash", "edit").
	ToolName string
	// Description is the request's human-readable detail (the
	// command, the file). Free-form and possibly multi-line; the
	// client reduces it to a single capped line.
	Description string
}

func (PermissionRequested) herdrEvent() {}

// PermissionResolved indicates a permission decision was made.
// Transitions back to working if a run is active, idle otherwise.
type PermissionResolved struct {
	// ToolCallID identifies the tool call whose approval was
	// decided. Only that tool call's block is cleared.
	ToolCallID string
}

func (PermissionResolved) herdrEvent() {}

// QuestionAsked indicates the question tool is waiting for the user
// to answer. Transitions to blocked.
type QuestionAsked struct {
	// BatchID identifies the question batch. It pairs the block
	// with the QuestionResolved that ends it.
	BatchID string
	// Text is the blocked message, derived from the first
	// question's text and truncated to herdr's text-field cap by
	// Translate.
	Text string
}

func (QuestionAsked) herdrEvent() {}

// QuestionResolved indicates a pending question was answered or
// cancelled. Transitions back to working if a run is active, idle
// otherwise.
type QuestionResolved struct {
	// BatchID identifies the resolved question batch. Only that
	// batch's block is cleared.
	BatchID string
}

func (QuestionResolved) herdrEvent() {}

// AuthRequired indicates the agent hit an authentication error and is
// waiting for the user to re-authenticate. Transitions to blocked
// until the run that needed auth completes: re-auth resolution
// publishes no event, so the block's lifetime is tied to the run to
// avoid leaking a permanent blocked state.
type AuthRequired struct {
	// ProviderID names the provider needing re-authentication, when
	// known. Client/server mode does not carry it on the wire, so
	// it may be empty.
	ProviderID string
}

func (AuthRequired) herdrEvent() {}

// SummarizeStarted indicates context compaction began. Transitions to
// working until a matching SummarizeFinished arrives. Compaction never
// publishes a RunComplete, so it must not ride on runActive: that is
// what left the pane stuck in working after a /compact.
type SummarizeStarted struct {
	SessionID string
}

func (SummarizeStarted) herdrEvent() {}

// SummarizeFinished indicates context compaction ended, whether by
// success, error, or user cancel. Transitions back to working if a
// run is still active (auto-compaction mid-turn), idle otherwise.
type SummarizeFinished struct {
	SessionID string
}

func (SummarizeFinished) herdrEvent() {}

// SessionUpdated carries a session's current title. Not a state
// transition: it refreshes the pane's presentation metadata when the
// current session's title changes (auto-titling, rename). Events for
// any session other than the authoritative current one are ignored.
type SessionUpdated struct {
	SessionID string
	Title     string
}

func (SessionUpdated) herdrEvent() {}

// sender abstracts the transport layer for reporting state to herdr.
// Production uses a Unix socket; tests use a recorder. Every call is
// made with Client.mu held, so implementations never see concurrent
// use and need not be thread-safe themselves.
type sender interface {
	send(req reportRequest) error
	close()
}

// Client reports Crush agent state to a running herdr instance.
type Client struct {
	socketPath string
	paneID     string

	mu sync.Mutex
	// sessionID is the current top-level session, sent along with
	// every state report.
	sessionID string
	// state and message hold the last reported pair; reports are
	// deduplicated on both so a changed blocked reason still reaches
	// herdr.
	state   string
	message string
	// runActive tracks an in-flight agent turn, summarizing tracks
	// context compaction. Blocks record the outstanding reasons the
	// agent is waiting on the user, keyed by reason and request id;
	// blockOrder keeps them oldest-first so the newest block's
	// message can be picked for the report.
	runActive   bool
	summarizing bool
	blocks      map[blockKey]string
	blockOrder  []blockKey
	seq         uint64
	// pres holds the complete desired pane presentation (session
	// title, session id, model); reportedPres holds the last set
	// sent. herdr treats presentation fields as replace-all per
	// source, so every metadata report carries the complete set and
	// identical sets are deduplicated.
	pres         presentation
	reportedPres presentation

	snd sender
}

// presentation is the pane metadata crush reports to herdr. All
// fields are strings so the zero value is a valid, comparable
// "nothing to show" state.
type presentation struct {
	title   string
	session string
	model   string
}

// defaultClient is the process-wide herdr client. Initialized once
// via Init(). All integration sites share this single instance so
// only one Unix socket connection exists per process.
var (
	defaultClient *Client
	initOnce      sync.Once
	// disabled, when set, makes Init return nil forever after. Set
	// via Disable before the first Init call.
	disabled atomic.Bool
)

// Disable permanently disables the herdr integration for this
// process: Init returns nil and no socket connection is ever made.
// The crush server hosts workspaces on behalf of clients and must
// never claim the pane of the terminal that launched it, so it calls
// this at startup. Call before the first Init; an already-created
// client is not closed.
func Disable() {
	disabled.Store(true)
}

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
	if disabled.Load() {
		slog.Debug("Herdr integration disabled for this process")
		return nil
	}
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
		blocks:     make(map[blockKey]string),
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
	c.snd.send(c.newRequestLocked("pane.report_agent", "init", stateIdle, ""))
}

// Close releases the agent's authority on the pane and shuts down
// the background writer. Safe to call on a nil client.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.releaseAgent()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snd.close()
}

// releaseAgent sends a pane.release_agent request to herdr so the
// pane is freed for a new agent to claim authority. This is the
// clean-shutdown protocol per herdr's socket API. Sends directly
// on the socket to ensure delivery even if the write loop is busy.
func (c *Client) releaseAgent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	req := c.newRequestLocked("pane.release_agent", "release", "", "")
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
	case RunStarted:
		c.onRunStarted(e.SessionID)
	case AssistantMessage:
		c.onAssistantMessage(e.SessionID, e.Model, e.Finished)
	case RunComplete:
		c.onRunComplete(e.SessionID)
	case PermissionRequested:
		c.onPermissionRequest(e.ToolCallID, e.ToolName, e.Description)
	case PermissionResolved:
		c.onPermissionResolved(e.ToolCallID)
	case QuestionAsked:
		c.onQuestionAsked(e.BatchID, e.Text)
	case QuestionResolved:
		c.onQuestionResolved(e.BatchID)
	case AuthRequired:
		c.onAuthRequired(e.ProviderID)
	case SummarizeStarted:
		c.onSummarizeStarted(e.SessionID)
	case SummarizeFinished:
		c.onSummarizeFinished(e.SessionID)
	case SessionUpdated:
		c.onSessionUpdated(e.SessionID, e.Title)
	}
}

// SetSession sets the session ID for reporting. This is the
// authoritative current session: lifecycle events for any other
// top-level session are ignored afterwards. Call this when the
// session is created or resolved, before events start flowing. The
// title refreshes the pane's presentation metadata immediately: no
// session event fires on a plain switch, so the title must be pushed
// here. An empty id (landing screen) clears both the title and the
// session token.
//
// A switch to a different session also resets the lifecycle flags.
// They describe the session being left, whose terminal events
// acceptLifecycleLocked drops from this point on, so nothing else
// could ever clear them: switching sessions mid-run would strand the
// pane in working forever. The cost is that a run still going in the
// session we left reports idle until it emits its next assistant
// message, which is transient and self-healing. Blocks survive the
// switch: they are not session-scoped, and a dialog raised by any
// session still waits on the user.
func (c *Client) SetSession(id, title string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if id != c.sessionID {
		c.runActive = false
		c.summarizing = false
	}
	c.sessionID = id
	c.pres.title = title
	c.pres.session = id
	c.reportMetadataLocked()
	c.recomputeLocked()
}

// ReportModel records the active model id for the pane's model
// token. Called at startup from the loaded config and refreshed from
// assistant messages afterwards. An empty model is ignored: crush
// always has a configured model, so clearing the token is never
// meaningful.
func (c *Client) ReportModel(model string) {
	if c == nil || model == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pres.model = model
	c.reportMetadataLocked()
}

// maxNotificationBodyLength is herdr's 240-character cap on the
// notification.show body field, the one text field that is not
// bound by the generic maxTextFieldLength cap.
const maxNotificationBodyLength = 240

// Notify sends a notification.show request so herdr surfaces a toast
// in its own UI. The request carries no pane id, source, or seq: it
// is a global herdr UI notification, not a pane-scoped report, so it
// skips the state machinery and neither reads nor writes client
// state. It still takes c.mu, because that is what gives sender
// implementations their single-threaded contract. Best-effort and
// fire-and-forget, like state reports. Safe to call on a nil client.
func (c *Client) Notify(title, body string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.snd.send(reportRequest{
		ID:     fmt.Sprintf("crush:notify:%d", time.Now().UnixNano()),
		Method: "notification.show",
		Params: notificationParams{
			Title: truncateText(title),
			Body:  truncateRunes(body, maxNotificationBodyLength),
		},
	})
}

// acceptLifecycleLocked applies the session scoping shared by all
// lifecycle events and reports whether the event should be processed.
// Events from agent-tool sub-sessions are always ignored: a sub-agent
// run must not drive the pane state nor overwrite the reported
// session id. Events from a different top-level session than the
// current one are stale and ignored as well. While no session is
// known, the first scoped event establishes it, for the reported
// session id and the presentation token alike. Must be called with
// c.mu held.
func (c *Client) acceptLifecycleLocked(sessionID string) bool {
	if strings.Contains(sessionID, subSessionSeparator) {
		return false
	}
	if sessionID == "" {
		// The event carries no session; accept it so the state
		// transition still happens, but there is nothing to learn.
		return true
	}
	if c.sessionID == "" {
		// The learned id rides on every state report from here on,
		// so the session token has to move with it: both name the
		// current session, and writing them together is what keeps
		// pane.report_agent and pane.report_metadata from
		// disagreeing. The title is unknown here; it arrives with
		// the SetSession call that follows the session load.
		c.sessionID = sessionID
		c.pres.session = sessionID
		c.reportMetadataLocked()
		return true
	}
	return sessionID == c.sessionID
}

func (c *Client) onRunStarted(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.acceptLifecycleLocked(sessionID) {
		return
	}
	c.runActive = true
	c.recomputeLocked()
}

func (c *Client) onAssistantMessage(sessionID, model string, finished bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.acceptLifecycleLocked(sessionID) {
		return
	}
	// A finished message is the turn's terminal snapshot, not a
	// sign of ongoing work. Arming runActive here could land after
	// the turn's RunComplete (the two travel on separate brokers,
	// so their delivery order is not serialized) and strand the
	// pane in working — the ESC-cancel path is exactly that: the
	// cancelled assistant's finished update races the cancelled
	// RunComplete. Only RunComplete clears the flag, so skipping
	// the arm can never cause a premature idle.
	if !finished {
		c.runActive = true
	}
	if model != "" {
		c.pres.model = model
		c.reportMetadataLocked()
	}
	c.recomputeLocked()
}

func (c *Client) onRunComplete(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.acceptLifecycleLocked(sessionID) {
		return
	}
	c.runActive = false
	// The run that needed re-authentication is over, one way or
	// another: the auth wait happened inside it, or it ended with
	// the 401 that raised the prompt. Successful re-auth publishes
	// no event, so this is the only reliable point to drop the
	// block without leaking a permanent blocked state.
	c.clearBlockLocked(blockKey{reason: blockAuth})
	c.recomputeLocked()
}

func (c *Client) onPermissionRequest(toolCallID, toolName, description string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// A permission request implies a run is active, even if no
	// assistant message has arrived yet (e.g. tool calls that fire
	// before any text output).
	c.runActive = true
	c.setExclusiveBlockLocked(
		blockKey{reason: blockPermission, id: toolCallID},
		permissionBlockMessage(toolName, description),
	)
	c.recomputeLocked()
}

// permissionBlockMessage renders the reason a permission prompt is
// waiting. The tool name leads because it is always short and always
// present; the description carries the specifics (the command, the
// path) and only its first line is usable, since herdr's text fields
// are single-line and capped. Truncation cuts the tail, so the tool
// name survives even for a long description.
func permissionBlockMessage(toolName, description string) string {
	msg := "Permission required"
	if toolName = strings.TrimSpace(toolName); toolName != "" {
		msg = "Permission: " + toolName
	}
	if detail := firstLine(description); detail != "" {
		msg += " - " + detail
	}
	return truncateText(msg)
}

func (c *Client) onPermissionResolved(toolCallID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearBlockLocked(blockKey{reason: blockPermission, id: toolCallID})
	c.recomputeLocked()
}

func (c *Client) onQuestionAsked(batchID, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// A question implies a run is active, even if no assistant
	// message has arrived yet: the tool can only block on user
	// input mid-turn.
	c.runActive = true
	c.setExclusiveBlockLocked(blockKey{reason: blockQuestion, id: batchID}, text)
	c.recomputeLocked()
}

func (c *Client) onQuestionResolved(batchID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearBlockLocked(blockKey{reason: blockQuestion, id: batchID})
	c.recomputeLocked()
}

func (c *Client) onAuthRequired(providerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// An auth prompt implies a run is active: the coordinator only
	// requests re-authentication from within a failed or waiting
	// turn.
	c.runActive = true
	msg := "Re-authentication required"
	if providerID != "" {
		msg = "Re-authentication required: " + providerID
	}
	c.setBlockLocked(blockKey{reason: blockAuth}, truncateText(msg))
	c.recomputeLocked()
}

func (c *Client) onSummarizeStarted(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.acceptLifecycleLocked(sessionID) {
		return
	}
	c.summarizing = true
	c.recomputeLocked()
}

func (c *Client) onSummarizeFinished(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.acceptLifecycleLocked(sessionID) {
		return
	}
	c.summarizing = false
	c.recomputeLocked()
}

// onSessionUpdated refreshes the pane title when the current
// session's title changes. Title events for any other session
// (background auto-titling of task sessions, sub-sessions) must not
// clobber the presentation, and with no current session known there
// is nothing to attach a title to.
func (c *Client) onSessionUpdated(sessionID, title string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if sessionID == "" || strings.Contains(sessionID, subSessionSeparator) {
		return
	}
	if c.sessionID == "" || sessionID != c.sessionID {
		return
	}
	c.pres.title = title
	c.reportMetadataLocked()
}

// setBlockLocked records one outstanding reason the agent is waiting
// on the user. Re-setting a key it already holds keeps that key's
// position in blockOrder: only genuinely new blocks become the
// newest, so a re-delivered request cannot jump ahead of a block
// raised after it. Must be called with c.mu held.
func (c *Client) setBlockLocked(key blockKey, message string) {
	if c.blocks == nil {
		c.blocks = make(map[blockKey]string)
	}
	if _, ok := c.blocks[key]; !ok {
		c.blockOrder = append(c.blockOrder, key)
	}
	c.blocks[key] = message
}

// setExclusiveBlockLocked records a block whose reason can only ever
// have one instance outstanding, dropping any older block that
// shares the reason. Both prompting services serialize their
// requests — the permission service behind a request mutex
// (internal/permission/permission.go), the question service behind a
// single pending slot (internal/question/question.go) — so an older
// block with the same reason belongs to a request that has already
// returned. That includes the requests cancelled by a turn interrupt,
// which publish no resolution at all and would otherwise leave the
// pane blocked forever. Must be called with c.mu held.
func (c *Client) setExclusiveBlockLocked(key blockKey, message string) {
	kept := c.blockOrder[:0]
	for _, k := range c.blockOrder {
		if k.reason == key.reason && k.id != key.id {
			delete(c.blocks, k)
			continue
		}
		kept = append(kept, k)
	}
	c.blockOrder = kept
	c.setBlockLocked(key, message)
}

// clearBlockLocked drops one block. Must be called with c.mu held.
func (c *Client) clearBlockLocked(key blockKey) {
	if _, ok := c.blocks[key]; !ok {
		return
	}
	delete(c.blocks, key)
	for i, k := range c.blockOrder {
		if k == key {
			c.blockOrder = slices.Delete(c.blockOrder, i, i+1)
			break
		}
	}
}

// recomputeLocked derives the reported state from the active blocks
// and the run/summarize flags, then reports it. Must be called with
// c.mu held. Blocks outrank everything: as long as any block is
// active the pane is blocked, carrying the newest block's message.
func (c *Client) recomputeLocked() {
	var state, message string
	switch {
	case len(c.blocks) > 0:
		state = stateBlocked
		message = c.blocks[c.blockOrder[len(c.blockOrder)-1]]
	case c.runActive || c.summarizing:
		state = stateWorking
	default:
		state = stateIdle
	}
	c.reportLocked(state, message)
}

// newRequestLocked builds a seq-stamped JSON-RPC request to herdr.
// Must be called with c.mu held. Every request increments c.seq so
// herdr accepts it as strictly newer than the last (see
// registerInitial for why monotonic seq matters). State and message
// are empty for requests that carry no agent state, such as
// pane.release_agent.
func (c *Client) newRequestLocked(method, idPrefix, state, message string) reportRequest {
	c.seq++
	return reportRequest{
		ID:     fmt.Sprintf("crush:%s:%d", idPrefix, time.Now().UnixNano()),
		Method: method,
		Params: reportParams{
			PaneID:         c.paneID,
			Source:         "crush",
			Agent:          "crush",
			State:          state,
			Message:        message,
			Seq:            c.seq,
			AgentSessionID: c.sessionID,
		},
	}
}

// reportLocked sends a pane.report_agent request to herdr. Must be
// called with c.mu held. Skips redundant reports when neither the
// state nor the message has changed.
func (c *Client) reportLocked(state, message string) {
	if state == c.state && message == c.message {
		return
	}
	c.state = state
	c.message = message
	c.snd.send(c.newRequestLocked("pane.report_agent", "report", state, message))
}

// reportMetadataLocked sends a pane.report_metadata request to
// herdr. Must be called with c.mu held. Skips redundant reports when
// the complete presentation set is unchanged.
func (c *Client) reportMetadataLocked() {
	if c.pres == c.reportedPres {
		return
	}
	c.reportedPres = c.pres
	c.snd.send(c.newMetadataRequestLocked())
}

// newMetadataRequestLocked builds a seq-stamped pane.report_metadata
// request carrying the complete presentation set. Must be called
// with c.mu held. herdr treats presentation fields as replace-all
// per source but merges tokens per key, so the title is always sent
// (omitted when empty, which clears it) and the session token is
// always sent (null when empty, which clears it). The model token is
// omitted until known so it is never cleared accidentally.
func (c *Client) newMetadataRequestLocked() reportRequest {
	c.seq++
	tokens := map[string]*string{"session": nil}
	if c.pres.session != "" {
		session := truncateText(c.pres.session)
		tokens["session"] = &session
	}
	if c.pres.model != "" {
		model := truncateText(c.pres.model)
		tokens["model"] = &model
	}
	return reportRequest{
		ID:     fmt.Sprintf("crush:metadata:%d", time.Now().UnixNano()),
		Method: "pane.report_metadata",
		Params: metadataParams{
			PaneID: c.paneID,
			Source: "crush",
			Title:  truncateText(c.pres.title),
			Tokens: tokens,
			Seq:    c.seq,
		},
	}
}

// reportRequest is the JSON-RPC envelope sent to herdr. Params is
// method-specific: reportParams for agent state, metadataParams for
// pane presentation.
type reportRequest struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params"`
}

// reportParams carries the agent state payload. Message is the
// human-readable reason the agent is blocked (e.g. the question
// text); it is sent on every report so herdr always has a complete
// (state, message) snapshot, and is empty when nothing is pending.
type reportParams struct {
	PaneID         string `json:"pane_id"`
	Source         string `json:"source"`
	Agent          string `json:"agent"`
	State          string `json:"state"`
	Message        string `json:"message"`
	Seq            uint64 `json:"seq"`
	AgentSessionID string `json:"agent_session_id"`
}

// metadataParams carries the pane presentation payload. Title is
// omitted when empty: herdr's presentation fields are replace-all
// per source, so omission clears it. Tokens merge per key; a null
// value clears that token.
type metadataParams struct {
	PaneID string             `json:"pane_id"`
	Source string             `json:"source"`
	Title  string             `json:"title,omitempty"`
	Tokens map[string]*string `json:"tokens"`
	Seq    uint64             `json:"seq"`
}

// notificationParams carries the notification.show payload. Title is
// required and capped at 80 characters; body is optional, capped at
// 240, and omitted when empty so herdr shows a title-only toast.
type notificationParams struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
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
