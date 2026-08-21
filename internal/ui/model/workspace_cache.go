package model

// Memoized workspace state.
//
// In client/server mode every workspace probe (busy checks, permission mode,
// queued prompts, agent readiness/model, LSP state) is a synchronous HTTP
// round-trip, and the Update goroutine is the render loop — blocking it
// freezes typing. The UI therefore never probes the workspace synchronously
// from Update or View. (The constructor is the one carve-out: New seeds the
// yolo and ready/model caches synchronously so the first frame has values to
// render; Init then refreshes them off-thread.)
//
//   - Reads (isAgentBusy, yoloModeCached, promptQueue, selectedLargeModel,
//     lspInfo) always return the memoized value, stale or not.
//   - State edges (message created, agent finished/errored, prompt
//     submitted, cancel, session switch, yolo toggle, model change, LSP
//     events) invalidate or write through the caches and dispatch an
//     off-thread refresh cmd.
//   - A TTL backstop at the end of Update re-dispatches a refresh whenever
//     the memoized state has gone stale, so unrelated churn (typing,
//     resize storms, spinner ticks) only ever schedules async work.
//
// Fresh values arrive as busyStateMsg / promptQueueMsg / lspStatesMsg and
// are applied on the Update goroutine, per the UI guidelines (no IO in
// Update, no model mutation inside commands).

import (
	"bytes"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

// busyCacheTTL bounds how long the memoized busy/permission state may go
// without a re-probe being scheduled. Package var so tests can pin it.
var busyCacheTTL = 500 * time.Millisecond

// promptQueueTTL is the backstop refresh interval for the queued-prompt
// state; the queue is otherwise refreshed on event edges.
var promptQueueTTL = 2 * time.Second

// ttlCache memoizes one boolean workspace probe result.
type ttlCache struct {
	val bool
	at  time.Time
}

// fresh reports whether the cached value is within its TTL.
func (c *ttlCache) fresh(ttl time.Duration) bool {
	return !c.at.IsZero() && time.Since(c.at) < ttl
}

// set writes a known-good value through the cache.
func (c *ttlCache) set(val bool) {
	c.val = val
	c.at = time.Now()
}

// invalidate marks the value stale so the next Update-tail backstop
// re-probes; the last value keeps being served in the meantime.
func (c *ttlCache) invalidate() {
	c.at = time.Time{}
}

// busyStateMsg delivers the result of an off-thread busy/permission probe.
type busyStateMsg struct {
	// gen is the busy generation captured when the probe was dispatched.
	// A result whose generation no longer matches m.busyFetchGen started
	// before a newer state transition (optimistic send, invalidation,
	// session switch, ...) and is discarded, then re-fetched, so the
	// authoritative refresh is never lost to an older in-flight request.
	gen       uint64
	ready     bool
	agentBusy bool
	yolo      bool
	// model is the coordinator's selected model, fetched by the same probe
	// so the sidebar/landing model info renders from memoized state. Zero
	// (and ignored) when ready is false.
	model workspace.AgentModel
}

// promptQueueMsg delivers the queued prompts fetched off-thread.
type promptQueueMsg struct {
	// forSession is the session the fetch was scoped to; a result that
	// raced a session switch is discarded and re-fetched.
	forSession string
	// gen is the queue generation captured at dispatch; like
	// busyStateMsg.gen it guards against a stale in-flight result
	// overwriting newer optimistic or invalidated queue state.
	gen     uint64
	prompts []string
}

// agentRunSubmittedMsg reports that AgentRun accepted a prompt (it either
// started a run or was enqueued behind one), so busy and queue state should
// be re-fetched.
type agentRunSubmittedMsg struct{}

// queuedMessagePoppedMsg delivers an off-thread queued-message pop result.
type queuedMessagePoppedMsg struct {
	forSession string
	message    agent.QueuedMessage
	found      bool
	err        error
}

// promptQueueClearedMsg reports that an off-thread queue drain finished for
// the session, so the memoized queue can be emptied and re-fetched.
type promptQueueClearedMsg struct {
	forSession string
	// messages is what the drain removed, oldest to newest. They live
	// nowhere else anymore, so only a discard may drop them.
	messages []agent.QueuedMessage
	// restore reports whether the drained prompts belong back in the
	// input field or are a deliberate discard (commands dialog).
	restore bool
	err     error
}

// agentModelChangedMsg reports that the coordinator's model was updated
// (model selection, thinking toggle, reasoning effort), so the memoized
// ready/model state should be re-fetched without waiting for the TTL.
type agentModelChangedMsg struct{}

// agentModelChangedCmd is sequenced after cmds that call UpdateAgentModel so
// the refresh probes the coordinator only once the update has completed.
// Callers should reach for updateAgentModelCmd rather than sequencing this
// by hand.
func agentModelChangedCmd() tea.Msg { return agentModelChangedMsg{} }

// currentSessionID returns the active session's ID, or "" when none.
func (m *UI) currentSessionID() string {
	if m.session == nil {
		return ""
	}
	return m.session.ID
}

// invalidateBusyCaches marks all memoized workspace probe state stale and
// bumps the busy generation so any in-flight probe result is discarded when
// it lands. Called by handlers for events that change agent or permission
// state.
func (m *UI) invalidateBusyCaches() {
	m.agentBusyCache.invalidate()
	m.yoloCache.invalidate()
	m.busyFetchGen++
}

// invalidatePromptQueue bumps the prompt-queue generation so any in-flight
// queue fetch result is discarded when it lands (and re-fetched) instead of
// overwriting newer optimistic or cleared queue state.
func (m *UI) invalidatePromptQueue() {
	m.promptQueueGen++
}

// dispatchBusyRefresh returns a command that probes the workspace busy and
// permission state off the Update goroutine, delivering a busyStateMsg. It
// returns nil while a probe is already in flight. The closure captures only
// locals (never m) so it is safe off-thread; state is applied by
// applyBusyState on the Update goroutine.
func (m *UI) dispatchBusyRefresh() tea.Cmd {
	if m.busyFetchInFlight || m.com == nil || m.com.Workspace == nil {
		return nil
	}
	m.busyFetchInFlight = true
	ws := m.com.Workspace
	gen := m.busyFetchGen
	return func() tea.Msg {
		st := busyStateMsg{gen: gen}
		if ws.AgentIsReady() {
			st.ready = true
			st.agentBusy = ws.AgentIsBusy()
			st.model = ws.AgentModel()
		}
		st.yolo = ws.PermissionSkipRequests()
		return st
	}
}

// updateAgentModelCmd sequences a coordinator model rebuild
// (UpdateAgentModel) with the invalidation of the memoized ready/model
// state. Callers wrap their pre-work in pre; the memoized model must only
// be re-probed after the rebuild lands (a synchronous HTTP round-trip in
// client/server mode), so the message drives the refresh instead of each
// call site remembering to.
func (m *UI) updateAgentModelCmd(pre tea.Cmd) tea.Cmd {
	return tea.Sequence(pre, agentModelChangedCmd)
}

// applyBusyState stores an off-thread probe result and reacts to busy
// edges (todo spinner, pills). Runs on the Update goroutine.
func (m *UI) applyBusyState(msg busyStateMsg) []tea.Cmd {
	m.busyFetchInFlight = false
	if msg.gen != m.busyFetchGen {
		// This probe started before a newer state transition (optimistic
		// send, invalidation, session switch, ...). Discard its result and
		// re-dispatch so the required authoritative refresh is not lost
		// merely because this older request was in flight.
		if cmd := m.dispatchBusyRefresh(); cmd != nil {
			return []tea.Cmd{cmd}
		}
		return nil
	}
	prevBusy := m.isAgentBusy()
	prevYolo := m.yoloModeCached()
	m.agentBusyCache.set(msg.agentBusy)
	m.yoloCache.set(msg.yolo)
	m.agentReady = msg.ready
	m.agentModel = msg.model
	if prevYolo != msg.yolo {
		// A remote/async toggle changed yolo mode: update the editor
		// prompt function so the prompt icon/style tracks the new mode.
		// The cache is written above and the placeholder is refreshed by
		// the Update tail.
		m.setEditorPrompt(msg.yolo)
	}

	var cmds []tea.Cmd
	busy := m.isAgentBusy()
	if m.hasSession() && hasInProgressTodo(m.session.Todos) && busy && !m.todoIsSpinning {
		m.todoIsSpinning = true
		cmds = append(cmds, m.todoSpinner.Tick)
	}
	if m.todoIsSpinning && !busy {
		m.todoIsSpinning = false
	}
	if prevBusy != busy {
		m.renderPills()
	}
	return cmds
}

// dispatchPromptQueueRefresh returns a command that fetches the queued
// prompts off the Update goroutine, delivering a promptQueueMsg. It returns
// nil while a fetch is already in flight. With no active session the queue
// is simply cleared.
func (m *UI) dispatchPromptQueueRefresh() tea.Cmd {
	if m.promptQueueInFlight || m.com == nil || m.com.Workspace == nil {
		return nil
	}
	if !m.hasSession() {
		m.promptQueueItems = nil
		m.promptQueueCheckedAt = time.Now()
		// Bump the generation so any in-flight fetch scoped to the
		// now-departed session is discarded rather than repopulating the
		// queue.
		m.invalidatePromptQueue()
		if m.promptQueue != 0 {
			m.promptQueue = 0
			m.updateLayoutAndSize()
		}
		return nil
	}
	m.promptQueueInFlight = true
	ws := m.com.Workspace
	sessionID := m.session.ID
	gen := m.promptQueueGen
	return func() tea.Msg {
		msg := promptQueueMsg{forSession: sessionID, gen: gen}
		if ws.AgentIsReady() {
			msg.prompts = ws.AgentQueuedPromptsList(sessionID)
		}
		return msg
	}
}

// applyPromptQueue stores an off-thread queue fetch and re-layouts when the
// count changed. Runs on the Update goroutine.
func (m *UI) applyPromptQueue(msg promptQueueMsg) []tea.Cmd {
	m.promptQueueInFlight = false
	if msg.forSession != m.currentSessionID() || msg.gen != m.promptQueueGen {
		// The fetch raced a session switch or a newer queue transition
		// (submit, clear, invalidation). Discard the stale result and
		// re-fetch so newer state is not clobbered and the authoritative
		// refresh is not lost to this older in-flight request.
		if cmd := m.dispatchPromptQueueRefresh(); cmd != nil {
			return []tea.Cmd{cmd}
		}
		return nil
	}
	m.promptQueueCheckedAt = time.Now()
	itemsChanged := !slices.Equal(m.promptQueueItems, msg.prompts)
	countChanged := len(msg.prompts) != m.promptQueue
	m.promptQueueItems = msg.prompts
	m.promptQueue = len(msg.prompts)
	if countChanged {
		m.updateLayoutAndSize()
	} else if itemsChanged {
		m.renderPills()
	}
	return nil
}

// noQueuedMessages is the banner shown when a pop or a restore drain finds
// nothing to bring back.
const noQueuedMessages = "No queued messages."

// popQueuedMessage removes the newest queued message off the Update
// goroutine. Single-flight: the pop is destructive at the agent layer.
func (m *UI) popQueuedMessage() tea.Cmd {
	if m.queuedPopInFlight {
		return nil
	}
	if m.promptQueue == 0 {
		return util.ReportInfo(noQueuedMessages)
	}
	if !m.hasSession() || m.com == nil || m.com.Workspace == nil {
		return nil
	}
	m.queuedPopInFlight = true
	ws := m.com.Workspace
	sessionID := m.session.ID
	return func() tea.Msg {
		queued, found, err := ws.AgentPopQueuedMessage(sessionID)
		return queuedMessagePoppedMsg{
			forSession: sessionID,
			message:    queued,
			found:      found,
			err:        err,
		}
	}
}

// applyQueuedMessagePop restores a popped message and supersedes stale queue
// reads. It runs on the Update goroutine.
func (m *UI) applyQueuedMessagePop(msg queuedMessagePoppedMsg) []tea.Cmd {
	m.queuedPopInFlight = false
	if msg.err != nil {
		// The pop may have succeeded before the error (decode failure,
		// dropped connection): the queue state is unknown, so re-fetch.
		slog.Error("Failed to pop queued message",
			"session_id", msg.forSession, "error", msg.err)
		m.invalidatePromptQueue()
		cmds := []tea.Cmd{util.ReportError(fmt.Errorf("%w (it may have left the queue anyway)", msg.err))}
		if cmd := m.dispatchPromptQueueRefresh(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return cmds
	}
	if !msg.found {
		if msg.forSession != m.currentSessionID() {
			// Nothing was popped, for a session the user has left: the
			// result says nothing about the queue now on screen.
			return nil
		}
		// The queue drained while the pop was in flight: supersede the
		// wrong count, re-fetch, and explain the empty editor.
		m.invalidatePromptQueue()
		cmds := []tea.Cmd{util.ReportInfo(noQueuedMessages)}
		if cmd := m.dispatchPromptQueueRefresh(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return cmds
	}
	if msg.forSession != m.currentSessionID() {
		// The pop raced a session switch: the message is already out of
		// its queue and cannot go back, so park it for that session.
		slog.Warn("Parking popped queued message after session switch",
			"for_session", msg.forSession, "current_session", m.currentSessionID())
		m.parkQueuedMessages(msg.forSession, []agent.QueuedMessage{msg.message})
		return []tea.Cmd{util.ReportWarn(parkedQueuedMessagesBanner(1))}
	}

	cmds := m.restoreQueuedMessages([]agent.QueuedMessage{msg.message})

	m.invalidatePromptQueue()
	if len(m.promptQueueItems) > 0 {
		m.promptQueueItems = m.promptQueueItems[:len(m.promptQueueItems)-1]
	}
	m.promptQueue = len(m.promptQueueItems)
	m.promptQueueCheckedAt = time.Now()
	m.updateLayoutAndSize()

	if cmd := m.dispatchPromptQueueRefresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// clearQueuedMessages drains the session's queued prompts off the Update
// goroutine; restore says whether they return to the editor or are dropped.
func (m *UI) clearQueuedMessages(restore bool) tea.Cmd {
	if m.queueClearInFlight {
		return nil
	}
	if !m.hasSession() || m.com == nil || m.com.Workspace == nil {
		return nil
	}
	m.queueClearInFlight = true
	ws := m.com.Workspace
	sessionID := m.session.ID
	return func() tea.Msg {
		messages, err := ws.AgentClearQueue(sessionID)
		return promptQueueClearedMsg{
			forSession: sessionID,
			messages:   messages,
			restore:    restore,
			err:        err,
		}
	}
}

// applyPromptQueueCleared empties the memoized queue once a drain lands,
// restoring the drained prompts to the editor when the drain was a restore.
func (m *UI) applyPromptQueueCleared(msg promptQueueClearedMsg) []tea.Cmd {
	m.queueClearInFlight = false
	if msg.err != nil {
		// The drain may have succeeded before the error, so the queue
		// state is unknown: re-fetch rather than serve a stale count.
		slog.Error("Failed to clear queued messages",
			"session_id", msg.forSession, "error", msg.err)
		m.invalidatePromptQueue()
		cmds := []tea.Cmd{util.ReportError(fmt.Errorf("%w (the queue may have been emptied anyway)", msg.err))}
		if cmd := m.dispatchPromptQueueRefresh(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return cmds
	}
	if msg.forSession != m.currentSessionID() {
		// The drain raced a session switch, so it says nothing about the
		// queue now on screen; that session's own reads stay authoritative.
		if msg.restore && len(msg.messages) > 0 {
			// Already out of their session's queue and written for it, so
			// park them for its next load; a discard has nothing to park.
			slog.Warn("Parking drained queued messages after session switch",
				"for_session", msg.forSession, "current_session", m.currentSessionID(),
				"messages", len(msg.messages))
			m.parkQueuedMessages(msg.forSession, msg.messages)
			return []tea.Cmd{util.ReportWarn(parkedQueuedMessagesBanner(len(msg.messages)))}
		}
		return nil
	}
	// Bump the generation so a fetch started before the drain cannot land
	// and repopulate the pill it just emptied.
	m.invalidatePromptQueue()
	hadQueue := m.promptQueue > 0 || len(m.promptQueueItems) > 0
	m.promptQueueItems = nil
	m.promptQueue = 0
	m.promptQueueCheckedAt = time.Now()
	if hadQueue {
		m.updateLayoutAndSize()
	}

	var cmds []tea.Cmd
	switch {
	case !msg.restore:
		cmds = append(cmds, util.ReportInfo("Queued messages cleared."))
	case len(msg.messages) == 0:
		// The count that let the press through was stale: the queue
		// emptied while the drain was in flight.
		cmds = append(cmds, util.ReportInfo(noQueuedMessages))
	default:
		cmds = append(m.restoreQueuedMessages(msg.messages),
			util.ReportInfo("Queued messages moved to the input field."))
	}
	if cmd := m.dispatchPromptQueueRefresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// restoreQueuedMessages puts queue-removed prompts back into the editor,
// ahead of the draft. They cannot be re-queued, so nothing here drops text.
func (m *UI) restoreQueuedMessages(queued []agent.QueuedMessage) []tea.Cmd {
	prevHeight := m.textarea.Height()
	parts := make([]string, 0, len(queued)+1)
	for _, restored := range queued {
		if restored.Prompt != "" {
			parts = append(parts, restored.Prompt)
		}
	}
	// No prompt text (attachments-only): leave the buffer alone, since
	// rewriting it would disengage bang mode with no "!" to materialize.
	if len(parts) > 0 {
		draft := m.textarea.Value()
		// Bang mode hides the leading "!" from the value: materialize it so
		// the prepended prompt is not folded into the shell command.
		if m.bangMode && draft != "" {
			draft = "!" + draft
		}
		if draft != "" {
			parts = append(parts, draft)
		}
		m.textarea.SetValue(strings.Join(parts, "\n\n"))
		m.syncBangModeFromTextarea()
		m.textarea.MoveToEnd()
		m.promptHistory.index = -1
		m.promptHistory.draft = m.textarea.Value()
	}
	// Skip attachments already held byte for byte; dedupe by bytes, not
	// name — paste_<n>.txt names are only unique within one editor.
	held := m.attachments.List()
	for _, restored := range queued {
		for _, attachment := range restored.Attachments {
			if slices.ContainsFunc(held, func(have message.Attachment) bool {
				return sameAttachment(have, attachment)
			}) {
				continue
			}
			m.attachments.Update(attachment)
		}
	}
	return []tea.Cmd{m.updateTextareaWithPrevHeight(nil, prevHeight)}
}

// sameAttachment reports whether a and b are the same file with the same bytes.
func sameAttachment(a, b message.Attachment) bool {
	return a.FilePath == b.FilePath &&
		a.FileName == b.FileName &&
		a.MimeType == b.MimeType &&
		bytes.Equal(a.Content, b.Content)
}

// parkQueuedMessages parks messages removed from a session the user has
// left, keyed by that session, for restoreParkedQueuedMessages. Parks append.
func (m *UI) parkQueuedMessages(sessionID string, queued []agent.QueuedMessage) {
	if m.queuedRestoreOrphans == nil {
		m.queuedRestoreOrphans = make(map[string][]agent.QueuedMessage, 1)
	}
	m.queuedRestoreOrphans[sessionID] = append(m.queuedRestoreOrphans[sessionID], queued...)
}

func queuedMessagesLabel(n int) string {
	if n == 1 {
		return "the queued message"
	}
	return fmt.Sprintf("%d queued messages", n)
}

func parkedQueuedMessagesBanner(n int) string {
	return fmt.Sprintf("Session changed: %s will be restored when you return to that session.",
		queuedMessagesLabel(n))
}

// restoreParkedQueuedMessages restores the messages parked for the session
// now loading, in the order they were parked.
func (m *UI) restoreParkedQueuedMessages() tea.Cmd {
	sessionID := m.currentSessionID()
	queued, ok := m.queuedRestoreOrphans[sessionID]
	if !ok {
		return nil
	}
	delete(m.queuedRestoreOrphans, sessionID)
	cmds := m.restoreQueuedMessages(queued)
	warn := fmt.Sprintf("Restored %s you removed before switching sessions.",
		queuedMessagesLabel(len(queued)))
	return tea.Batch(append(cmds, util.ReportWarn(warn))...)
}

// staleWorkspaceRefreshCmds is the TTL backstop, called at the tail of
// Update: when any memoized workspace state has outlived its TTL (and no
// event edge refreshed it), schedule an off-thread re-probe. It never does
// IO itself — a couple of time comparisons per message at most.
func (m *UI) staleWorkspaceRefreshCmds() []tea.Cmd {
	if m.com == nil || m.com.Workspace == nil {
		return nil
	}
	var cmds []tea.Cmd
	if !m.agentBusyCache.fresh(busyCacheTTL) || !m.yoloCache.fresh(busyCacheTTL) {
		if cmd := m.dispatchBusyRefresh(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.hasSession() && time.Since(m.promptQueueCheckedAt) >= promptQueueTTL {
		if cmd := m.dispatchPromptQueueRefresh(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if time.Since(m.lspCheckedAt) >= lspStatesTTL {
		if cmd := m.dispatchLSPRefresh(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

// toggleYoloMode flips permission auto-approval and writes the new value
// through the yolo cache (no re-probe needed) and the editor prompt. Shared
// by the direct keybinding and the commands-dialog action so both stay
// write-through. Returns the new mode.
func (m *UI) toggleYoloMode() bool {
	yolo := !m.com.Workspace.PermissionSkipRequests()
	m.com.Workspace.PermissionSetSkipRequests(yolo)
	m.yoloCache.set(yolo)
	// Supersede any in-flight busy/yolo probe: its result carries the old
	// generation and would otherwise overwrite the value we just wrote.
	// Bump the generation (rather than invalidateBusyCaches, which would
	// clear the fresh value) so applyBusyState's guard discards and
	// re-dispatches the stale probe.
	m.busyFetchGen++
	m.setEditorPrompt(yolo)
	return yolo
}

// yoloModeCached reports the memoized permission-skip ("yolo") mode. Toggles
// write through the cache; the Update-tail backstop keeps it bounded-stale
// otherwise.
func (m *UI) yoloModeCached() bool {
	return m.yoloCache.val
}
