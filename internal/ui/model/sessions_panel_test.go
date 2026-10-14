package model

import (
	"context"
	"errors"
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/workspace"
)

// makeSessions returns sessions s1..sn titled "title 1".."title n", s1
// created last, so both ListSessions and the panel order them s1..sn.
func makeSessions(n int) []session.Session {
	out := make([]session.Session, n)
	for i := range out {
		out[i] = session.Session{
			ID:        fmt.Sprintf("s%d", i+1),
			Title:     fmt.Sprintf("title %d", i+1),
			CreatedAt: int64(n - i),
		}
	}
	return out
}

// newPanelUI is newBusyUI with warm caches, so only the refreshes a test
// triggers are dispatched, and with jump keys bound, so the panel rows carry
// their numbers (no default keys: see TestSessionsPanelNumbersFollowJumpKeys).
func newPanelUI(ws *countingWorkspace) *UI {
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.keyMap.Chat.JumpSession.SetKeys("alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9")
	return m
}

// refreshPanel runs one session list refresh to completion through Update,
// as if the throttle interval had passed.
func refreshPanel(m *UI) {
	m.sessionListFetchedAt = time.Time{}
	runCmds(m, m.dispatchSessionListRefresh())
}

// panelRows returns the session rows of the rendered panel, ANSI stripped.
func panelRows(m *UI) []string {
	var rows []string
	for line := range strings.SplitSeq(ansi.Strip(m.sessionsInfo(60, sessionsPanelMax+1, true)), "\n") {
		if line = strings.TrimRight(line, " "); strings.Contains(line, "title ") {
			rows = append(rows, line)
		}
	}
	return rows
}

func TestSessionsPanelListsTopNine(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(12)}
	m := newPanelUI(ws) // Viewing s1, the first session.
	refreshPanel(m)

	rows := panelRows(m)
	require.Len(t, rows, 9)
	for i, row := range rows {
		require.Equal(t, fmt.Sprintf("%d ○ title %d", i+1, i+1), row)
	}
	require.Equal(t, 9, ws.statusCalls, "only the panel's sessions are probed")
}

func TestSessionsPanelPinsCurrentBeyondNine(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(12)}
	m := newPanelUI(ws)
	m.session = &session.Session{ID: "s12"}
	refreshPanel(m)

	rows := panelRows(m)
	require.Len(t, rows, 10)
	require.Equal(t, "9 ○ title 9", rows[8])
	require.Equal(t, "· ○ title 12", rows[9], "the viewed session is pinned as a tenth row")
	require.Equal(t, 10, ws.statusCalls, "the pinned session is probed too")
	require.Equal(t, makeSessions(12), m.sessionList, "pinning must not overwrite the list")
}

// TestSessionsPanelNumbersFollowJumpKeys: the row numbers name the
// chat.jump_session keys, so with none bound (the default) no row is
// numbered, and with keys bound only the rows a key reaches are.
func TestSessionsPanelNumbersFollowJumpKeys(t *testing.T) {
	t.Parallel()

	m := newBusyUI(&countingWorkspace{ready: true, sessions: makeSessions(3)})
	warmCaches(m, false)
	refreshPanel(m)
	require.Equal(t, []string{"○ title 1", "○ title 2", "○ title 3"}, panelRows(m))

	m = newBusyUI(&countingWorkspace{ready: true, sessions: makeSessions(3)})
	warmCaches(m, false)
	m.keyMap.Chat.JumpSession.SetKeys("alt+1", "alt+2")
	refreshPanel(m)
	require.Equal(t, []string{"1 ○ title 1", "2 ○ title 2", "· ○ title 3"}, panelRows(m))
}

// TestMessageAndLoadEventsDriveActivity: a new message marks a listed
// session working (not an unlisted sub-agent session), and opening a
// session clears its unseen result.
func TestMessageAndLoadEventsDriveActivity(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(2)}
	m := newPanelUI(ws)
	refreshPanel(m)

	for _, id := range []string{"child", "s2"} {
		m.Update(pubsub.Event[message.Message]{
			Type:    pubsub.CreatedEvent,
			Payload: message.Message{ID: "m-" + id, SessionID: id, Role: message.User},
		})
		require.Equal(t, id == "s2", m.sessionListInFlight, "only listed sessions trigger a refresh")
	}
	require.Equal(t, activityWorking, m.activity.Get("s2"))
	require.Equal(t, activityIdle, m.activity.Get("child"), "only listed sessions are tracked")

	m.Update(pubsub.Event[notify.RunComplete]{Type: pubsub.CreatedEvent, Payload: notify.RunComplete{SessionID: "s2"}})
	require.Equal(t, activityDoneUnseen, m.activity.Get("s2"))
	m.Update(loadSessionMsg{session: &session.Session{ID: "s2"}})
	require.Equal(t, activityIdle, m.activity.Get("s2"), "opening a session marks its result seen")
}

// TestStaleSessionProbesSkipped: a probe taken before a RunComplete still
// sees the session busy; reconciling it would turn the done mark back into
// working. The list is kept, the probes are skipped and a fetch follows.
func TestStaleSessionProbesSkipped(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{
		ready:         true,
		sessions:      makeSessions(2),
		sessionStatus: map[string]workspace.SessionStatus{"s2": {Busy: true}},
	}
	m := newPanelUI(ws)
	stale := m.dispatchSessionListRefresh()

	m.Update(pubsub.Event[notify.RunComplete]{Type: pubsub.CreatedEvent, Payload: notify.RunComplete{SessionID: "s2"}})
	m.sessionListFetchedAt = time.Time{} // Skip the throttle wait.
	_, cmd := m.Update(stale())
	require.Equal(t, activityDoneUnseen, m.activity.Get("s2"), "stale probes must not be reconciled")
	require.Len(t, m.sessionList, 2, "the list itself is applied")
	require.True(t, m.sessionListInFlight, "a fresh fetch must follow")

	ws.sessionStatus = nil // The run has ended by the time the re-fetch probes.
	runCmds(m, cmd)
	require.Equal(t, activityDoneUnseen, m.activity.Get("s2"))
}

// TestLateMessageAfterRunCompleteKeepsFailedMark: error and cancel paths
// create messages after the run's RunComplete; only a user message starts a
// run, so a late tool message must not erase ✗.
func TestLateMessageAfterRunCompleteKeepsFailedMark(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(2)}
	m := newPanelUI(ws)
	refreshPanel(m)

	m.Update(pubsub.Event[notify.RunComplete]{Type: pubsub.CreatedEvent, Payload: notify.RunComplete{SessionID: "s2", Error: "boom"}})
	_, cmd := m.Update(pubsub.Event[message.Message]{
		Type:    pubsub.CreatedEvent,
		Payload: message.Message{ID: "late", SessionID: "s2", Role: message.Tool},
	})
	runCmds(m, cmd) // The probe reports idle.
	require.Equal(t, activityFailedUnseen, m.activity.Get("s2"))
	require.Contains(t, panelRows(m), "2 ✗ title 2 failed")
}

// TestSessionListAppliedUnderEventStream: a steady event stream must not
// starve the panel. Events during a fetch queue one more fetch instead of
// discarding the result.
func TestSessionListAppliedUnderEventStream(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{
		ready:         true,
		sessions:      makeSessions(3),
		sessionStatus: map[string]workspace.SessionStatus{"s3": {Busy: true, PendingPrompt: true}},
	}
	m := newPanelUI(ws)
	m.sessionListFetchedAt = time.Time{}
	inFlight := m.dispatchSessionListRefresh()
	for range 3 {
		m.Update(pubsub.Event[session.Session]{Type: pubsub.UpdatedEvent, Payload: ws.sessions[0]})
	}
	require.True(t, m.sessionListQueued)

	_, next := m.Update(inFlight())
	require.Len(t, m.sessionList, 3, "the result must be applied despite the events")
	require.Equal(t, activityWaiting, m.activity.Get("s3"), "no mark changed, so the probes apply too")
	require.True(t, m.sessionListInFlight, "the queued refresh must be dispatched")
	require.False(t, m.sessionListQueued)

	// Not run: it would sleep out the throttle.
	require.NotNil(t, next)
	require.True(t, m.sessionListFetchedAt.After(time.Now().Add(sessionListMinInterval/2)),
		"the queued fetch waits out the minimum interval")
	require.Equal(t, 1, ws.listCalls, "three events cost one extra fetch, not three")
}

// TestSessionsPanelOrderStableAcrossUpdates: ListSessions orders by last
// update, which changes every agent step; the panel numbers must not.
func TestSessionsPanelOrderStableAcrossUpdates(t *testing.T) {
	t.Parallel()

	all := makeSessions(3)
	all[1].CreatedAt = all[2].CreatedAt // Same second: the ID decides.
	ws := &countingWorkspace{ready: true, sessions: slices.Clone(all)}
	m := newPanelUI(ws)
	refreshPanel(m)
	want := []string{"1 ○ title 1", "2 ○ title 2", "3 ○ title 3"}
	require.Equal(t, want, panelRows(m))

	ws.sessions = []session.Session{all[2], all[0], all[1]} // s3 just updated.
	refreshPanel(m)
	require.Equal(t, want, panelRows(m))
}

// TestPromptEventsRefreshSessionList: ▲ comes from the status probe, so
// permission and question events must fetch it rather than wait for the TTL.
func TestPromptEventsRefreshSessionList(t *testing.T) {
	t.Parallel()

	for name, ev := range map[string]any{
		"permission request":      pubsub.Event[permission.PermissionRequest]{Payload: permission.PermissionRequest{SessionID: "s2"}},
		"permission notification": pubsub.Event[permission.PermissionNotification]{Payload: permission.PermissionNotification{SessionID: "s2"}},
		"question request":        pubsub.Event[question.Request]{Payload: question.Request{SessionID: "s2"}},
		"question notification":   pubsub.Event[question.Notification]{Payload: question.Notification{SessionID: "s2"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ws := &countingWorkspace{ready: true, sessions: makeSessions(2), cfg: &config.Config{Options: &config.Options{TUI: &config.TUIOptions{}}}}
			m := newPanelUI(ws)
			m.Update(ev)
			require.True(t, m.sessionListInFlight)
		})
	}
}

func TestSessionsPanelDropsDeleted(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{
		ready:         true,
		sessions:      makeSessions(3),
		sessionStatus: map[string]workspace.SessionStatus{"s2": {Busy: true}},
	}
	m := newPanelUI(ws)
	refreshPanel(m)
	require.Equal(t, activityWorking, m.activity.Get("s2"))
	require.Contains(t, panelRows(m), "2 ● title 2 running")

	ws.sessions = []session.Session{ws.sessions[0], ws.sessions[2]}
	refreshPanel(m)
	require.Equal(t, []string{"1 ○ title 1", "2 ○ title 3"}, panelRows(m))
	require.Equal(t, activityIdle, m.activity.Get("s2"), "a deleted session must be forgotten")
}

func TestSessionListRefreshIsOffThread(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(2)}
	m := newPanelUI(ws)

	m.Update(pubsub.Event[session.Session]{
		Type:    pubsub.CreatedEvent,
		Payload: session.Session{ID: "title-of-s1", ParentSessionID: "s1"},
	})
	require.False(t, m.sessionListInFlight, "child sessions are not listed, so no refresh")

	_, cmd := m.Update(pubsub.Event[session.Session]{
		Type:    pubsub.CreatedEvent,
		Payload: session.Session{ID: "s2"},
	})
	require.Zero(t, ws.listCalls, "Update must not list sessions itself")
	require.Zero(t, ws.syncProbes())

	runCmds(m, cmd)
	require.Equal(t, 1, ws.listCalls, "the session event schedules exactly one fetch")
	require.True(t, ws.listHadDeadline, "a hung server must not keep the fetch in flight forever")
	require.Len(t, m.sessionList, 2)
}

func TestBackgroundRunCompleteMarksDone(t *testing.T) {
	t.Parallel()

	// Agent not ready: no model info, since the stub has no providers.
	ws := &countingWorkspace{
		sessions: makeSessions(4),
		cfg:      &config.Config{Options: &config.Options{}},
	}
	m := newPanelUI(ws)
	m.agentReady = false
	m.layout.sidebar = image.Rect(0, 0, 40, 60)
	refreshPanel(m)

	for _, rc := range []notify.RunComplete{
		{SessionID: "s1"}, // The viewed session gets no unseen mark.
		{SessionID: "s2"},
		{SessionID: "s3", Error: "context canceled", Cancelled: true},
		{SessionID: "s4", Error: "boom"},
	} {
		m.sessionListFetchedAt = time.Time{} // Skip the throttle wait.
		_, cmd := m.Update(pubsub.Event[notify.RunComplete]{Type: pubsub.CreatedEvent, Payload: rc})
		// The follow-up probe reports idle and must keep the mark.
		runCmds(m, cmd)
	}

	m.updateSidebarScrollState()
	content := ansi.Strip(m.sidebarContent)
	require.Contains(t, content, "1 ○ title 1")
	require.Contains(t, content, "2 ✓ title 2")
	require.Contains(t, content, "3 ✓ title 3", "a cancelled run is not a failure")
	require.Contains(t, content, "4 ✗ title 4")
}

// childSession is a sub-agent session of parent, with the agent tool's
// "<message>$$<tool call>" ID.
func childSession(parent, toolCall string) session.Session {
	return session.Session{ID: "m-" + parent + "$$" + toolCall, ParentSessionID: parent, Title: "agent " + toolCall}
}

// sendMsg delivers a message event through Update and runs its commands.
func sendMsg(m *UI, typ pubsub.EventType, msg message.Message) {
	_, cmd := m.Update(pubsub.Event[message.Message]{Type: typ, Payload: msg})
	runCmds(m, cmd)
}

// startChild delivers the first message of a sub-agent run.
func startChild(m *UI, child session.Session) {
	sendMsg(m, pubsub.CreatedEvent, message.Message{ID: "u-" + child.ID, SessionID: child.ID, Role: message.User})
}

// finished is an assistant message of session id that ended with reason.
func finished(id string, reason message.FinishReason) message.Message {
	return message.Message{ID: "a-" + id, SessionID: id, Role: message.Assistant, Parts: []message.ContentPart{message.Finish{Reason: reason}}}
}

// childRows returns the sub-agent rows of the rendered panel, ANSI stripped.
func childRows(m *UI) []string {
	var rows []string
	for line := range strings.SplitSeq(ansi.Strip(m.sessionsInfo(60, sessionsPanelMax+1, true)), "\n") {
		if line = strings.TrimRight(line, " "); strings.Contains(line, "└") {
			rows = append(rows, line)
		}
	}
	return rows
}

func TestChildRunShownUnderParent(t *testing.T) {
	t.Parallel()

	child := childSession("s2", "tc1")
	ws := &countingWorkspace{
		sessions: makeSessions(3),
		children: []session.Session{child},
		cfg:      &config.Config{Options: &config.Options{}},
	}
	m := newPanelUI(ws)
	m.agentReady = false
	m.layout.sidebar = image.Rect(0, 0, 40, 60)
	refreshPanel(m)
	ws.resetCounters()

	_, cmd := m.Update(pubsub.Event[message.Message]{
		Type:    pubsub.CreatedEvent,
		Payload: message.Message{ID: "u1", SessionID: child.ID, Role: message.User},
	})
	require.Zero(t, ws.syncProbes(), "the parent is resolved off-thread")
	runCmds(m, cmd)
	require.Equal(t, 2, ws.getCalls, "the child, then its parent (a root)")

	m.updateSidebarScrollState()
	lines := strings.Split(ansi.Strip(m.sidebarContent), "\n")
	i := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "2 ○ title 2") })
	require.GreaterOrEqual(t, i, 0)
	require.Equal(t, "  └ ● agent tc1 running", strings.TrimRight(lines[i+1], " "), "the child row sits right under its parent")
	require.Contains(t, lines[i+2], "3 ○ title 3")

	for range 3 { // More activity of the same child.
		sendMsg(m, pubsub.CreatedEvent, message.Message{ID: "t1", SessionID: child.ID, Role: message.Tool})
	}
	require.Equal(t, 2, ws.getCalls, "a child is resolved once")
	require.Len(t, childRows(m), 1)
}

func TestChildRunClearedOnFinish(t *testing.T) {
	t.Parallel()

	child := childSession("s2", "tc1")
	ws := &countingWorkspace{ready: true, sessions: makeSessions(2), children: []session.Session{child}}
	m := newPanelUI(ws)
	refreshPanel(m)
	startChild(m, child)
	require.Equal(t, []string{"  └ ● agent tc1 running"}, childRows(m))

	sendMsg(m, pubsub.UpdatedEvent, finished(child.ID, message.FinishReasonToolUse))
	require.Equal(t, []string{"  └ ● agent tc1 running"}, childRows(m), "a tool-use step does not end the run")

	sendMsg(m, pubsub.UpdatedEvent, finished(child.ID, message.FinishReasonEndTurn))
	require.Empty(t, childRows(m))
	require.Empty(t, m.childRuns, "only running children are kept")
}

func TestChildRunsCappedAtThree(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(2)}
	for i := range 5 {
		ws.children = append(ws.children, childSession("s2", fmt.Sprintf("tc%d", i+1)))
	}
	m := newPanelUI(ws)
	refreshPanel(m)
	for _, c := range ws.children {
		startChild(m, c)
	}

	require.Equal(t, []string{
		"  └ ● agent tc1 running",
		"  └ ● agent tc2 running",
		"  └ ● agent tc3 running",
		"  └ +2 more",
	}, childRows(m))
	require.Equal(t, []string{"1 ○ title 1", "2 ○ title 2"}, panelRows(m), "child rows are not numbered")
}

func TestChildRunClearedOnParentRunComplete(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{
		ready:    true,
		sessions: makeSessions(2),
		children: []session.Session{childSession("s2", "tc1"), childSession("s2", "tc2")},
	}
	m := newPanelUI(ws)
	refreshPanel(m)
	for _, c := range ws.children {
		startChild(m, c)
	}
	require.Len(t, childRows(m), 2)

	m.sessionListFetchedAt = time.Time{}
	_, cmd := m.Update(pubsub.Event[notify.RunComplete]{Type: pubsub.UpdatedEvent, Payload: notify.RunComplete{SessionID: "s2"}})
	runCmds(m, cmd)
	require.Empty(t, childRows(m))
	require.Equal(t, []string{"1 ○ title 1", "2 ✓ title 2 done"}, panelRows(m))
}

// TestChildRunsDroppedWhenParentProbedIdle: a missed run end (a dropped
// event stream) must not leave a row running forever. The list refresh's
// status probe shows the parent idle.
func TestChildRunsDroppedWhenParentProbedIdle(t *testing.T) {
	t.Parallel()

	child := childSession("s2", "tc1")
	ws := &countingWorkspace{
		ready:         true,
		sessions:      makeSessions(2),
		children:      []session.Session{child},
		sessionStatus: map[string]workspace.SessionStatus{"s2": {Busy: true}},
	}
	m := newPanelUI(ws)
	refreshPanel(m)
	startChild(m, child)
	refreshPanel(m)
	require.Len(t, childRows(m), 1, "a busy parent keeps its sub-agents")

	ws.sessionStatus = nil
	refreshPanel(m)
	require.Empty(t, childRows(m))
}

// TestChildRunResolvedOnceWhilePending: messages that land before the
// lookup returns must not look the child up again.
func TestChildRunResolvedOnceWhilePending(t *testing.T) {
	t.Parallel()

	child := childSession("s2", "tc1")
	ws := &countingWorkspace{ready: true, sessions: makeSessions(2), children: []session.Session{child}}
	m := newPanelUI(ws)
	refreshPanel(m)
	ws.resetCounters()

	_, first := m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: message.Message{ID: "u1", SessionID: child.ID, Role: message.User}})
	_, second := m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: message.Message{ID: "a1", SessionID: child.ID, Role: message.Assistant}})
	require.NotNil(t, first)
	runCmds(m, second)
	require.Zero(t, ws.getCalls, "the second message must not look the child up")
	runCmds(m, first)
	require.Equal(t, 2, ws.getCalls)
	require.Equal(t, []string{"  └ ● agent tc1 running"}, childRows(m))
}

// TestChildRunShowsGeneratedTitle: a sub-agent starts with a placeholder
// title; the generated one arrives seconds later as a session update.
func TestChildRunShowsGeneratedTitle(t *testing.T) {
	t.Parallel()

	child := childSession("s2", "tc1")
	child.Title = "New Agent Session"
	ws := &countingWorkspace{ready: true, sessions: makeSessions(2), children: []session.Session{child}}
	m := newPanelUI(ws)
	refreshPanel(m)
	startChild(m, child)
	require.Equal(t, []string{"  └ ● New Agent Session running"}, childRows(m))

	renamed := child
	renamed.Title = "Explore auth flow"
	_, cmd := m.Update(pubsub.Event[session.Session]{Type: pubsub.UpdatedEvent, Payload: renamed})
	runCmds(m, cmd)
	require.Equal(t, []string{"  └ ● Explore auth flow running"}, childRows(m))
	require.False(t, m.sessionListInFlight, "a child's update must not fetch the list")
}

// TestChildRunTitleBeforeLookup: the rename can land before the lookup,
// which read the record earlier; the newer title wins.
func TestChildRunTitleBeforeLookup(t *testing.T) {
	t.Parallel()

	child := childSession("s2", "tc1")
	child.Title = "New Agent Session"
	ws := &countingWorkspace{ready: true, sessions: makeSessions(2), children: []session.Session{child}}
	m := newPanelUI(ws)
	refreshPanel(m)
	_, lookup := m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: message.Message{ID: "u1", SessionID: child.ID, Role: message.User}})
	stale := lookup() // Reads the placeholder.

	renamed := child
	renamed.Title = "Explore auth flow"
	m.Update(pubsub.Event[session.Session]{Type: pubsub.UpdatedEvent, Payload: renamed})
	_, cmd := m.Update(stale)
	runCmds(m, cmd)
	require.Equal(t, []string{"  └ ● Explore auth flow running"}, childRows(m))
}

// TestChildRunsPrunedWhenParentUnlisted: a run under a deleted session can
// never show, so the next list drops it.
func TestChildRunsPrunedWhenParentUnlisted(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{
		ready:         true,
		sessions:      makeSessions(2),
		children:      []session.Session{childSession("s2", "tc1")},
		sessionStatus: map[string]workspace.SessionStatus{"s2": {Busy: true}},
	}
	m := newPanelUI(ws)
	refreshPanel(m)
	startChild(m, ws.children[0])
	require.Len(t, childRows(m), 1)

	ws.sessions = ws.sessions[:1] // s2 deleted: not listed, not probed.
	refreshPanel(m)
	require.Empty(t, m.childRuns)
}

// TestUnresolvedChildRunExpires: a child whose lookup failed stays hidden
// and is forgotten after childRunMaxUnresolved lists; its next message
// looks it up again.
func TestUnresolvedChildRunExpires(t *testing.T) {
	t.Parallel()

	child := childSession("s2", "tc1")
	ws := &countingWorkspace{ready: true, sessions: makeSessions(2)} // The lookup fails.
	m := newPanelUI(ws)
	refreshPanel(m)
	startChild(m, child)
	require.Len(t, m.childRuns, 1)
	require.Empty(t, childRows(m))

	refreshPanel(m)
	require.Len(t, m.childRuns, 1, "kept within the bound: no lookup storm")
	refreshPanel(m)
	require.Empty(t, m.childRuns)

	ws.children = []session.Session{child} // The server is back.
	sendMsg(m, pubsub.CreatedEvent, message.Message{ID: "t1", SessionID: child.ID, Role: message.Tool})
	require.Equal(t, []string{"  └ ● agent tc1 running"}, childRows(m))
}

// TestFailedProbeKeepsActivity: a failed status probe must not turn a
// working session's ● into ○.
func TestFailedProbeKeepsActivity(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{
		ready:         true,
		sessions:      makeSessions(2),
		sessionStatus: map[string]workspace.SessionStatus{"s2": {Busy: true}},
	}
	m := newPanelUI(ws)
	refreshPanel(m)
	require.Contains(t, panelRows(m), "2 ● title 2 running")

	ws.statusFails = true
	refreshPanel(m)
	require.Contains(t, panelRows(m), "2 ● title 2 running")
}

// TestUnresolvedChildRunExpiresWhileListFails: the bound counts list
// landings, failed ones too, so a hidden run cannot live forever while
// ListSessions keeps failing.
func TestUnresolvedChildRunExpiresWhileListFails(t *testing.T) {
	t.Parallel()

	resolved := childSession("s2", "tc2")
	ws := &countingWorkspace{ready: true, sessions: makeSessions(2), children: []session.Session{resolved}} // tc1's lookup fails.
	m := newPanelUI(ws)
	refreshPanel(m)
	startChild(m, childSession("s2", "tc1"))
	startChild(m, resolved)
	require.Len(t, m.childRuns, 2)

	ws.listErr = errors.New("server down")
	refreshPanel(m)
	require.Len(t, m.childRuns, 2)
	refreshPanel(m)
	require.Len(t, m.childRuns, 1, "the unresolved run is gone")
	require.Equal(t, []string{"  └ ● agent tc2 running"}, childRows(m), "a failed list says nothing about resolved runs")
}

// TestChildRunsKeptWhenParentProbeFails: a failed probe (timeout, server
// error) reads as the zero status; it is unknown, not idle.
func TestChildRunsKeptWhenParentProbeFails(t *testing.T) {
	t.Parallel()

	child := childSession("s2", "tc1")
	ws := &countingWorkspace{
		ready:         true,
		sessions:      makeSessions(2),
		children:      []session.Session{child},
		sessionStatus: map[string]workspace.SessionStatus{"s2": {Busy: true}},
	}
	m := newPanelUI(ws)
	refreshPanel(m)
	startChild(m, child)

	ws.statusFails = true
	refreshPanel(m)
	require.Equal(t, []string{"  └ ● agent tc1 running"}, childRows(m))
}

// TestChildRunShownOnLanding: the landing screen (no session open) shows
// the panel too, so sub-agents are tracked there.
func TestChildRunShownOnLanding(t *testing.T) {
	t.Parallel()

	child := childSession("s2", "tc1")
	ws := &countingWorkspace{ready: true, sessions: makeSessions(2), children: []session.Session{child}}
	m := newPanelUI(ws)
	m.session = nil
	m.state = uiLanding
	refreshPanel(m)
	startChild(m, child)
	require.Equal(t, []string{"  └ ● agent tc1 running"}, childRows(m))
}

// TestChildRunCompleteLeavesActivityAlone: a sub-agent's run publishes its
// own RunComplete. It ends the child's row, but the child is not a listed
// session, so it must neither create a mark (which bumps the version and
// discards an in-flight probe) nor fetch.
func TestChildRunCompleteLeavesActivityAlone(t *testing.T) {
	t.Parallel()

	child := childSession("s2", "tc1")
	ws := &countingWorkspace{ready: true, sessions: makeSessions(2), children: []session.Session{child}}
	m := newPanelUI(ws)
	refreshPanel(m)
	startChild(m, child)
	require.Len(t, childRows(m), 1)
	version := m.activity.version

	m.Update(pubsub.Event[notify.RunComplete]{Type: pubsub.UpdatedEvent, Payload: notify.RunComplete{SessionID: child.ID}})
	require.Empty(t, childRows(m), "the child's run is over")
	require.Equal(t, version, m.activity.version, "a child run must not bump the activity version")
	require.Equal(t, activityIdle, m.activity.Get(child.ID))
	require.False(t, m.sessionListInFlight, "a child run must not fetch the list")
}

// TestSessionsPanelMemoInvalidates: the memoized panel must re-render when
// only the list, only a mark, or only the viewed session changes.
func TestSessionsPanelMemoInvalidates(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(2)}
	m := newPanelUI(ws)
	refreshPanel(m)
	require.Equal(t, []string{"1 ○ title 1", "2 ○ title 2"}, panelRows(m))

	renamed := makeSessions(2)
	renamed[1].Title = "title renamed"
	ws.sessions = renamed
	refreshPanel(m)
	require.Equal(t, []string{"1 ○ title 1", "2 ○ title renamed"}, panelRows(m), "list change")

	m.activity.Finished("s2", false, "s1")
	require.Equal(t, []string{"1 ○ title 1", "2 ✓ title renamed done"}, panelRows(m), "mark change")

	before := m.sessionsInfo(60, sessionsPanelMax+1, true)
	m.session = &session.Session{ID: "s2"}
	require.NotEqual(t, before, m.sessionsInfo(60, sessionsPanelMax+1, true), "viewed session change")
}

// TestSessionsPanelShowsStatusWord: every non-idle state has a word next to
// its glyph, so the state does not rest on color alone; idle stays bare.
func TestSessionsPanelShowsStatusWord(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(5)}
	m := newPanelUI(ws)
	refreshPanel(m)
	m.activity.Working("s2")
	m.activity.Waiting("s3")
	m.activity.Finished("s4", true, "s1")
	m.activity.Finished("s5", false, "s1")

	require.Equal(t, []string{
		"1 ○ title 1",
		"2 ● title 2 running",
		"3 ▲ title 3 waiting",
		"4 ✗ title 4 failed",
		"5 ✓ title 5 done",
	}, panelRows(m))
}

// TestSessionsPanelNarrowKeepsStatusWord: a long title is cut first, never
// the status word, for sessions and sub-agent rows alike.
func TestSessionsPanelNarrowKeepsStatusWord(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{ready: true, sessions: makeSessions(2)}
	ws.sessions[1].Title = "a very long session title indeed"
	m := newPanelUI(ws)
	refreshPanel(m)
	m.activity.Waiting("s2")
	m.childRuns = map[string]*childRun{"c1": {id: "c1", parentID: "s2", title: "a very long sub-agent title"}}
	m.childRunsVersion++

	const width = 24
	out := ansi.Strip(m.sessionsInfo(width, sessionsPanelMax+1, true))
	var rows []string
	for line := range strings.SplitSeq(out, "\n") {
		if line = strings.TrimRight(line, " "); strings.Contains(line, "▲") || strings.Contains(line, "└") {
			rows = append(rows, line)
		}
	}
	require.Len(t, rows, 2)
	require.True(t, strings.HasSuffix(rows[0], " waiting"), rows[0])
	require.True(t, strings.HasSuffix(rows[1], " running"), rows[1])
	for _, r := range rows {
		require.LessOrEqual(t, ansi.StringWidth(r), width, r)
		require.Contains(t, r, "…", "the title gives way")
	}
}

// BenchmarkSessionsInfo measures the per-frame cost of the sidebar's
// Sessions section (the sidebar re-renders it on every Draw).
func BenchmarkSessionsInfo(b *testing.B) {
	for _, n := range []int{50, 500} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			ws := &countingWorkspace{
				ready:    true,
				sessions: makeSessions(n),
				sessionStatus: map[string]workspace.SessionStatus{
					"s2": {Busy: true}, "s4": {Busy: true}, "s6": {Busy: true},
				},
			}
			m := newPanelUI(ws)
			refreshPanel(m)
			b.ReportAllocs()
			for b.Loop() {
				m.sessionsInfo(38, len(m.panelSessions()), true)
			}
		})
	}
}

// jumpWorkspace records what a session jump asks the workspace for. The
// loads run in the returned cmd, so the test executes it to see them.
type jumpWorkspace struct {
	*countingWorkspace

	loaded []string
	ran    int
}

func (w *jumpWorkspace) GetSession(_ context.Context, id string) (session.Session, error) {
	w.loaded = append(w.loaded, id)
	return w.countingWorkspace.GetSession(context.Background(), id)
}

func (w *jumpWorkspace) ListSessionHistory(context.Context, string) ([]history.File, error) {
	return nil, nil
}

func (w *jumpWorkspace) FileTrackerListReadFiles(context.Context, string) ([]string, error) {
	return nil, nil
}

func (w *jumpWorkspace) SetCurrentSession(context.Context, string) error { return nil }

func (w *jumpWorkspace) AgentRun(context.Context, string, string, ...message.Attachment) error {
	w.ran++
	return nil
}

// newJumpUI is a UI on s1 with 12 sessions in the panel (9 numbered).
func newJumpUI() (*UI, *jumpWorkspace) {
	ws := &jumpWorkspace{countingWorkspace: &countingWorkspace{ready: true, sessions: makeSessions(12)}}
	m := newPanelUI(ws.countingWorkspace)
	m.com.Workspace = ws
	m.sessionList = ws.sessions
	// No default keys any more; bind the former ones to test the action.
	m.keyMap.Chat.JumpSession.SetKeys("alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9")
	return m, ws
}

func altKey(n int) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune('0' + n), Mod: tea.ModAlt}
}

// runLeaves executes a command tree and drops the messages.
func runLeaves(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if b, ok := cmd().(tea.BatchMsg); ok {
		for _, c := range b {
			runLeaves(c)
		}
	}
}

func TestAltNumberLoadsNthSession(t *testing.T) {
	t.Parallel()

	for n := 2; n <= sessionsPanelMax; n++ { // 1 is the viewed session.
		for _, focus := range []uiFocusState{uiFocusEditor, uiFocusMain} {
			m, ws := newJumpUI()
			m.focus = focus
			runLeaves(m.handleKeyPressMsg(altKey(n)))
			require.Equal(t, []string{m.panelSessions()[n-1].ID}, ws.loaded, "alt+%d focus %d", n, focus)
		}
	}
}

func TestAltNumberKeepsDraft(t *testing.T) {
	t.Parallel()

	m, ws := newJumpUI()
	m.textarea.SetValue("abc")
	runLeaves(m.handleKeyPressMsg(altKey(2)))
	require.Equal(t, []string{"s2"}, ws.loaded)
	require.Equal(t, "abc", m.textarea.Value())
	require.Zero(t, ws.ran, "nothing is sent")
}

func TestAltNumberNoop(t *testing.T) {
	t.Parallel()

	m, ws := newJumpUI()
	m.textarea.SetValue("abc")
	runLeaves(m.handleKeyPressMsg(altKey(1))) // The viewed session.
	m.sessionList = m.sessionList[:3]
	runLeaves(m.handleKeyPressMsg(altKey(4))) // Past the end.
	require.Empty(t, ws.loaded)
	require.Equal(t, "abc", m.textarea.Value())

	// The pinned current session beyond the ninth is not numbered.
	m, ws = newJumpUI()
	m.session = &session.Session{ID: "s11"}
	require.Len(t, m.panelSessions(), sessionsPanelMax+1)
	runLeaves(m.jumpToSession(10))
	require.Empty(t, ws.loaded)
}

func TestAltNumberGoesToDialog(t *testing.T) {
	t.Parallel()

	m, ws := newJumpUI()
	m.dialog.OpenDialog(&actionDialog{}) // Swallows keys.
	runLeaves(m.handleKeyPressMsg(altKey(2)))
	require.Empty(t, ws.loaded)
}

func TestAltNumberFollowsOverride(t *testing.T) {
	t.Parallel()

	m, ws := newJumpUI()
	ctrl := func(n int) tea.KeyPressMsg { return tea.KeyPressMsg{Code: rune('0' + n), Mod: tea.ModCtrl} }
	require.Empty(t, applyKeyOverrides(&m.keyMap, map[string][]string{
		"chat.jump_session": {"ctrl+1", "ctrl+2", "ctrl+3"},
	}))
	runLeaves(m.handleKeyPressMsg(altKey(2)))
	runLeaves(m.handleKeyPressMsg(ctrl(3)))
	require.Equal(t, []string{"s3"}, ws.loaded, "alt is rebound away, ctrl+3 is the third session")
}
