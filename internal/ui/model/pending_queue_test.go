package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
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
	require.True(t, m.chat.HasLiveSpinner(), "the assistant message is spinning")

	m.sendMessage("second")

	require.Nil(t, m.chat.MessageItem(chat.PendingUserID))
}

// The transcript falls silent while a tool runs: the assistant message
// stops spinning once it carries tool calls, and the tool call stops as
// soon as the model has finished writing it, well before the tool returns.
// A prompt sent in that window is still queued behind the running turn, so
// it belongs in the queue pill rather than echoed into the transcript with
// a second spinner stacked under it.
func TestPromptSentDuringAToolRunIsNotEchoed(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	m.appendSessionMessage(userMessage("u1", "s1", "first"))
	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: true,
	})
	m.appendSessionMessage(message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
		Parts: []message.ContentPart{message.ToolCall{
			ID: "tc1", Name: "bash", Input: "{}", Finished: true,
		}},
	})
	require.False(t, m.chat.HasLiveSpinner(), "a tool mid-run shows no spinner")

	m.sendMessage("second")

	require.Nil(t, m.chat.MessageItem(chat.PendingUserID), "the queue owns it")
	require.False(t, hasPending(m), "no second spinner underneath the turn")
}

// A run this client never sent (a sub-agent report, a channel event, a
// session opened mid-run) has no pending turn to vouch for it. While its
// tool runs the transcript is silent too, so the busy probe has to be what
// keeps a prompt sent then in the queue pill instead of echoed with a
// spinner as though it had started a turn of its own.
func TestPromptSentDuringAnUnsentRunIsNotEchoed(t *testing.T) {
	m := newPendingUI(t)
	m.appendSessionMessage(userMessage("u1", "s1", "report"))
	m.appendSessionMessage(message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
		Parts: []message.ContentPart{message.ToolCall{
			ID: "tc1", Name: "bash", Input: "{}", Finished: true,
		}},
	})
	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: true,
	})
	require.False(t, m.chat.HasLiveSpinner(), "a tool mid-run shows no spinner")

	m.sendMessage("keep going")

	require.Nil(t, m.chat.MessageItem(chat.PendingUserID), "the queue owns it")
	require.False(t, hasPending(m), "no spinner for a queued prompt")
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
	require.True(t, m.chat.HasLiveSpinner(), "the assistant message is spinning")

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

// runComplete delivers the agent's terminal event for sessionID.
func runComplete(m *UI, sessionID string, cancelled bool) {
	_, cmd := m.Update(pubsub.Event[notify.RunComplete]{
		Payload: notify.RunComplete{SessionID: sessionID, Cancelled: cancelled},
	})
	runCmds(m, cmd)
	// Local admission may return after completion. Retire only once that
	// reservation is released and the session probe positively says idle.
	if activity := m.sessionActivities[sessionID]; activity != nil && activity.pending > 0 {
		runCmds(m, m.submissionFinished(sessionID, activity.gen, nil, false))
	}
}

// The clock stops with the run, even after an assistant message has taken
// the spinner over.
func TestRunCompleteStopsTheClockAfterHandoff(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")
	m.appendSessionMessage(message.Message{ID: "a1", Role: message.Assistant, SessionID: "s1"})
	require.False(t, hasPending(m), "the assistant message carries the spinner")

	runComplete(m, "s1", false)

	require.Empty(t, common.Elapsed("s1"))
}

// A turn that ends before producing an assistant message retires its
// spinner and its echo.
func TestRunCompleteRetiresPendingSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")

	runComplete(m, "s1", false)

	require.Empty(t, transcriptIDs(m))
	require.Empty(t, common.Elapsed("s1"))
}

// Busy probes say nothing about when a turn ends, so an idle one landing
// before the run registers must not retire the spinner the send placed.
func TestIdleProbeKeepsSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")

	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: false,
	})

	require.True(t, hasPending(m))
}

// Another session's run ending must not retire this session's spinner.
func TestOtherSessionRunCompleteKeepsSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")

	runComplete(m, "other", false)

	require.True(t, hasPending(m))
	require.NotEmpty(t, common.Elapsed("s1"))
}

// A cancel before any busy probe has seen the run still ends the turn,
// because the agent reports a cancelled run like any other.
func TestEarlyCancelRetiresPendingSpinner(t *testing.T) {
	pinTTLs(t)
	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	warmCaches(m, false)
	m.session = &session.Session{ID: "s1"}
	t.Cleanup(func() { common.StopTurn("s1") })

	m.sendMessage("hello")
	m.cancelAgent()
	m.cancelAgent()
	require.Equal(t, 1, ws.cancelCalls, "the second escape cancels")

	runComplete(m, "s1", true)

	require.False(t, hasPending(m), "a cancelled turn must not leave a spinner")
	require.Empty(t, common.Elapsed("s1"))
}

// An assistant message left unfinished by a killed run still reports
// spinning, but frozen. It is not a turn in flight, so a prompt sent into
// that session starts its own.
func TestGhostSpinnerDoesNotSwallowTheTurn(t *testing.T) {
	m := newPendingUI(t)
	m.setSessionMessages([]message.Message{
		userMessage("u1", "s1", "earlier"),
		{
			ID: "a1", Role: message.Assistant, SessionID: "s1",
			Parts: []message.ContentPart{message.ReasoningContent{Thinking: "hmm"}},
		},
	})

	m.sendMessage("hello")

	require.True(t, hasPending(m))
	require.NotNil(t, m.chat.MessageItem(chat.PendingUserID))
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
