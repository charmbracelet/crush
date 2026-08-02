package model

// Memoized workspace state.
//
// In client/server mode every workspace probe (busy checks, permission mode,
// queued prompts) is a synchronous HTTP round-trip, and the Update goroutine
// is the render loop — blocking it freezes typing. The UI therefore never
// probes the workspace synchronously from Update or View:
//
//   - Reads (isAgentBusy, yoloModeCached, promptQueue) always return the
//     memoized value, stale or not.
//   - State edges (message created, agent finished/errored, prompt
//     submitted, cancel, session switch, yolo toggle) invalidate or
//     write through the caches and dispatch an off-thread refresh cmd.
//   - A TTL backstop at the end of Update re-dispatches a refresh whenever
//     the memoized state has gone stale, so unrelated churn (typing,
//     resize storms, spinner ticks) only ever schedules async work.
//
// Fresh values arrive as busyStateMsg / promptQueueMsg and are applied on
// the Update goroutine, per the UI guidelines (no IO in Update, no model
// mutation inside commands).

import (
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
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
	agentBusy bool
	yolo      bool
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
			st.agentBusy = ws.AgentIsBusy()
		}
		st.yolo = ws.PermissionSkipRequests()
		return st
	}
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
