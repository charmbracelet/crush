package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

// newPendingUI builds a UI on session s1 with caches warmed so only the
// paths under test can move the pending spinner.
func newPendingUI(t *testing.T) *UI {
	t.Helper()
	pinTTLs(t)
	m := newBusyUI(&countingWorkspace{ready: true})
	warmCaches(m, false)
	m.session = &session.Session{ID: "s1"}
	t.Cleanup(func() { common.StopTurn("s1") })
	return m
}

func hasPending(m *UI) bool {
	return m.chat.MessageItem(chat.PendingAssistantID) != nil
}

// transcriptIDs lists the ids of the items in view, top to bottom.
func transcriptIDs(m *UI) []string {
	ids := make([]string, 0, m.chat.list.Len())
	for i := range m.chat.list.Len() {
		if item, ok := m.chat.list.ItemAt(i).(chat.MessageItem); ok {
			ids = append(ids, item.ID())
		}
	}
	return ids
}

// userMessage builds the stored prompt the agent publishes back.
func userMessage(id, sessionID, text string) message.Message {
	return message.Message{
		ID:        id,
		Role:      message.User,
		SessionID: sessionID,
		Parts:     []message.ContentPart{message.TextContent{Text: text}},
	}
}

// The prompt shows up the moment it is sent, above its spinner, rather than
// waiting for the agent to store it and publish it back.
func TestSendEchoesThePromptAboveTheSpinner(t *testing.T) {
	m := newPendingUI(t)

	m.sendMessage("hello")

	require.Equal(t, []string{chat.PendingUserID, chat.PendingAssistantID}, transcriptIDs(m))
}

// The stored prompt takes the echo's place, and the spinner stays at the
// bottom instead of hopping over the arriving message.
func TestStoredPromptReplacesTheEcho(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")

	m.appendSessionMessage(userMessage("u1", "s1", "hello"))

	require.Equal(t, []string{"u1", chat.PendingAssistantID}, transcriptIDs(m))
}

// A prompt queued behind a running turn belongs in the queue pill, not in
// the transcript, so it is not echoed.
func TestQueuedPromptIsNotEchoed(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	m.appendSessionMessage(userMessage("u1", "s1", "first"))
	m.appendSessionMessage(message.Message{ID: "a1", Role: message.Assistant, SessionID: "s1"})
	require.True(t, m.chat.HasSpinningItem(), "the assistant message is spinning")

	m.sendMessage("second")

	require.Nil(t, m.chat.MessageItem(chat.PendingUserID))
}

// A generated continuation is hidden from the transcript, so it gets a
// spinner but no echo.
func TestHiddenPromptIsNotEchoed(t *testing.T) {
	m := newPendingUI(t)

	m.sendMessageInternal("Implement the plan.", true)

	require.True(t, hasPending(m))
	require.Nil(t, m.chat.MessageItem(chat.PendingUserID))
}

// A turn that ends before its prompt was stored takes the echo with it:
// nothing was persisted, so nothing should be left behind.
func TestEndedTurnDropsTheEcho(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")

	m.endPendingTurn("s1")

	require.Empty(t, transcriptIDs(m))
}

// Sending shows the spinner straight away rather than waiting for the
// assistant message, and the clock runs from the prompt.
func TestPendingSpinnerAppearsOnSend(t *testing.T) {
	m := newPendingUI(t)

	m.sendMessage("hello")

	require.True(t, hasPending(m))
	require.NotEmpty(t, common.Elapsed("s1"), "the turn clock runs from the prompt")
}

// The assistant message owns the spinner once it exists, so the
// placeholder steps aside rather than doubling up.
func TestAssistantMessageRetiresPendingSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")
	require.True(t, hasPending(m))

	m.appendSessionMessage(message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
	})

	require.False(t, hasPending(m))
}

// An assistant message belonging to another session must not retire this
// session's spinner.
func TestOtherSessionAssistantLeavesPendingSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")

	m.appendSessionMessage(message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "other",
	})

	require.True(t, hasPending(m))
}

// A prompt sent while a turn is already spinning is queued behind it. The
// running turn owns the spinner, so no second one is stacked underneath and
// the elapsed time keeps counting from the turn in flight.
func TestQueuedPromptDoesNotStackASecondSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	started := common.Elapsed("s1")
	m.appendSessionMessage(message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
	})
	require.True(t, m.chat.HasSpinningItem(), "the assistant message is spinning")

	m.sendMessage("second")

	require.False(t, hasPending(m), "the running turn keeps the only spinner")
	require.NotEmpty(t, started)
	require.NotEmpty(t, common.Elapsed("s1"), "the clock was not restarted")
}

// Sending twice must never leave two placeholders: the second would be
// unreachable through the id map and would spin until a session reload.
func TestPendingSpinnerIsNotDuplicated(t *testing.T) {
	m := newPendingUI(t)

	m.sendMessage("hello")
	m.syncPendingItem()
	m.syncPendingItem()

	var count int
	for i := range m.chat.list.Len() {
		if item, ok := m.chat.list.ItemAt(i).(chat.MessageItem); ok &&
			item.ID() == chat.PendingAssistantID {
			count++
		}
	}
	require.Equal(t, 1, count)
}

// A cancel emits no terminal event, so the idle probe is the only notice
// that the turn is over. It has to stop the clock even once an assistant
// message has taken the spinner over, or the next turn goes on counting
// from the cancelled one.
func TestIdleProbeStopsTheClockAfterHandoff(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")
	m.appendSessionMessage(message.Message{ID: "a1", Role: message.Assistant, SessionID: "s1"})
	require.False(t, hasPending(m), "the assistant message carries the spinner")

	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: true,
	})
	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: false,
	})

	require.Empty(t, common.Elapsed("s1"), "the clock stops with the turn")
}

// A turn that ends without ever producing an assistant message, whether by
// a cancel, a run rejected at submission, or a dropped notification, still
// retires its spinner, because the authoritative busy probe reports the
// session idle.
func TestIdleProbeRetiresPendingSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")
	require.True(t, hasPending(m))

	// The run registers, then ends.
	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: true,
	})
	require.True(t, hasPending(m), "still running")

	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: false,
	})

	require.False(t, hasPending(m))
	require.Empty(t, common.Elapsed("s1"), "the clock stops with the turn")
}

// A prompt is accepted before its run registers as active, so an idle probe
// that lands in that window means "not started yet", not "already over". It
// must not retire the spinner the send just placed.
func TestIdleProbeBeforeRunRegistersKeepsSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")

	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: false,
	})

	require.True(t, hasPending(m), "the run has not registered yet")
}

// A probe scoped to a session the user has since left must not retire the
// spinner belonging to the session they moved to.
func TestIdleProbeForOtherSessionKeepsSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")

	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "other", sessionBusy: false,
	})

	require.True(t, hasPending(m))
}

// Cancelling a turn before it produced anything emits neither a finished
// nor an error notification, so the spinner can only be retired by the busy
// probe. Drive the real escape path to prove it is.
func TestCancelRetiresPendingSpinner(t *testing.T) {
	pinTTLs(t)
	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.session = &session.Session{ID: "s1"}
	t.Cleanup(func() { common.StopTurn("s1") })

	m.sendMessage("hello")
	require.True(t, hasPending(m))

	// The run registers, then escape is pressed twice.
	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: true,
	})
	m.cancelAgent()
	m.cancelAgent()
	require.Equal(t, 1, ws.cancelCalls, "the second escape cancels")

	// No terminal notification follows a cancel; the probe is the backstop.
	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: false,
	})

	require.False(t, hasPending(m), "a cancelled turn must not leave a spinner")
	require.Empty(t, common.Elapsed("s1"))
}

// Loading a session rebuilds the transcript from stored messages, which can
// never contain the placeholder. A session still waiting on a turn gets it
// back, so tabbing away and returning does not lose the spinner.
func TestSessionLoadRestoresPendingSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")
	require.True(t, hasPending(m))

	m.setSessionMessages(nil)

	require.True(t, hasPending(m), "same session: spinner comes back")

	m.session = &session.Session{ID: "other"}
	m.setSessionMessages(nil)

	require.False(t, hasPending(m), "a different session has no pending turn")
}
