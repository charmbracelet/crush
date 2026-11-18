package model

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/x/ansi"
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

// created and updated deliver a message event through Update, as the app
// does, so the turn timer is synced the way it is at runtime.
func created(m *UI, msg message.Message) {
	m.Update(pubsub.Event[message.Message]{Type: pubsub.CreatedEvent, Payload: msg})
}

func updated(m *UI, msg message.Message) {
	m.Update(pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: msg})
}

// hasPending reports whether the placeholder spinner is showing, as opposed
// to nothing or the "Running" line.
func hasPending(m *UI) bool {
	item, ok := m.chat.MessageItem(chat.PendingAssistantID).(*chat.PendingAssistantItem)
	return ok && item.Live()
}

// hasTurnTimer reports whether the "Running" line is showing.
func hasTurnTimer(m *UI) bool {
	item, ok := m.chat.MessageItem(chat.PendingAssistantID).(*chat.PendingAssistantItem)
	return ok && !item.Live()
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

	created(m, userMessage("u1", "s1", "hello"))

	require.Equal(t, []string{"u1", chat.PendingAssistantID}, transcriptIDs(m))
}

// The load and the event can arrive in either order; the prompt shows once.
func TestLoadRacingTheStoredPromptShowsItOnce(t *testing.T) {
	stored := userMessage("u1", "s1", "hello")

	t.Run("load first", func(t *testing.T) {
		m := newPendingUI(t)
		m.sendMessage("hello")

		m.setSessionMessages([]message.Message{stored})
		require.Equal(t, []string{"u1", chat.PendingAssistantID}, transcriptIDs(m))

		created(m, stored)
		require.Equal(t, []string{"u1", chat.PendingAssistantID}, transcriptIDs(m))
	})

	t.Run("event first", func(t *testing.T) {
		m := newPendingUI(t)
		m.sendMessage("hello")

		created(m, stored)
		m.setSessionMessages([]message.Message{stored})
		require.Equal(t, []string{"u1", chat.PendingAssistantID}, transcriptIDs(m))
	})
}

// A load that lands before the new prompt is stored, showing only the
// history that was already on screen, must leave the echo in place.
func TestLoadOfOlderHistoryKeepsTheEcho(t *testing.T) {
	m := newPendingUI(t)
	older := []message.Message{userMessage("u0", "s1", "earlier")}
	m.setSessionMessages(older)
	m.sendMessage("hello")

	m.setSessionMessages(older)

	require.Equal(t, []string{"u0", chat.PendingUserID, chat.PendingAssistantID}, transcriptIDs(m))
}

// A prompt queued behind a running turn belongs in the queue pill, not in
// the transcript, so it is not echoed.
func TestQueuedPromptIsNotEchoed(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	created(m, userMessage("u1", "s1", "first"))
	created(m, message.Message{ID: "a1", Role: message.Assistant, SessionID: "s1"})

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
	created(m, userMessage("u1", "s1", "first"))
	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: true,
	})
	created(m, message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
		Parts: []message.ContentPart{message.ToolCall{
			ID: "tc1", Name: "bash", Input: "{}", Finished: true,
		}},
	})

	m.sendMessage("second")

	require.Nil(t, m.chat.MessageItem(chat.PendingUserID), "the queue owns it")
	require.False(t, hasPending(m), "no second spinner underneath the turn")
	require.True(t, hasTurnTimer(m), "the turn time stays on screen while the tool runs")
}

// A run this client never sent (a subagent report, a channel event, a
// session opened mid-run) has no pending turn to vouch for it. While its
// tool runs the transcript is silent too, so the busy probe has to be what
// keeps a prompt sent then in the queue pill instead of echoed with a
// spinner as though it had started a turn of its own.
func TestPromptSentDuringAnUnsentRunIsNotEchoed(t *testing.T) {
	m := newPendingUI(t)
	created(m, userMessage("u1", "s1", "report"))
	created(m, message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
		Parts: []message.ContentPart{message.ToolCall{
			ID: "tc1", Name: "bash", Input: "{}", Finished: true,
		}},
	})
	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: true,
	})

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

	created(m, message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
	})

	require.False(t, hasPending(m))
}

// An assistant message belonging to another session must not retire this
// session's spinner.
func TestOtherSessionAssistantLeavesPendingSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")

	m.Update(pubsub.Event[message.Message]{
		Type:    pubsub.CreatedEvent,
		Payload: message.Message{ID: "a1", Role: message.Assistant, SessionID: "other"},
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
	created(m, message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
	})

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
	m.Update(pubsub.Event[notify.RunComplete]{
		Payload: notify.RunComplete{SessionID: sessionID, Cancelled: cancelled},
	})
}

// The clock stops with the run, even after an assistant message has taken
// the spinner over.
func TestRunCompleteStopsTheClockAfterHandoff(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("hello")
	created(m, message.Message{ID: "a1", Role: message.Assistant, SessionID: "s1"})
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

// A ghost spinner left in an earlier turn by a killed run is not a turn in
// flight. Counting it would treat every new prompt in a long session as
// queued, holding back its echo and spinner until the agent publishes them.
func TestGhostSpinnerInAnEarlierTurnDoesNotQueueThePrompt(t *testing.T) {
	m := newPendingUI(t)
	m.setSessionMessages([]message.Message{
		userMessage("u0", "s1", "earlier"),
		{ID: "a0", Role: message.Assistant, SessionID: "s1"},
		userMessage("u1", "s1", "later"),
	})
	m.animClock.frozen = false

	m.sendMessage("hello")

	require.NotNil(t, m.chat.MessageItem(chat.PendingUserID), "the prompt is echoed")
	require.True(t, hasPending(m), "the spinner shows straight away")
}

// The busy probe taken mid-turn goes stale the moment the run ends. A prompt
// sent before the next probe lands starts a turn rather than queueing.
func TestEndedRunClearsTheBusyProbe(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	created(m, userMessage("u1", "s1", "first"))
	m.applyBusyState(busyStateMsg{
		gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: true,
	})
	runComplete(m, "s1", false)

	m.sendMessage("second")

	require.NotNil(t, m.chat.MessageItem(chat.PendingUserID), "the prompt is echoed")
	require.True(t, hasPending(m), "the spinner shows straight away")
}

// A probe sent mid-turn can land after the run ends. Its busy answer is
// stale, so it must not put the next prompt back in the queue.
func TestProbeInFlightAtRunEndIsDiscarded(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	created(m, userMessage("u1", "s1", "first"))
	inFlight := m.busyFetchGen
	runComplete(m, "s1", false)
	m.applyBusyState(busyStateMsg{
		gen: inFlight, ready: true, forSession: "s1", sessionBusy: true,
	})

	m.sendMessage("second")

	require.NotNil(t, m.chat.MessageItem(chat.PendingUserID), "the prompt is echoed")
	require.True(t, hasPending(m), "the spinner shows straight away")
}

// Once the assistant message stops spinning, the turn's elapsed time stays
// at the bottom of the transcript until the turn ends, hidden only while a
// spinning assistant message shows it itself.
func TestTurnTimerOutlivesTheSpinner(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	created(m, userMessage("u1", "s1", "first"))
	created(m, message.Message{ID: "a1", Role: message.Assistant, SessionID: "s1"})
	require.False(t, hasTurnTimer(m), "the spinning message shows the time")

	updated(m, message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
		Parts: []message.ContentPart{message.ToolCall{ID: "tc1", Name: "bash", Input: "{}", Finished: true}},
	})
	require.True(t, hasTurnTimer(m))
	ids := transcriptIDs(m)
	require.Equal(t, chat.PendingAssistantID, ids[len(ids)-1], "the timer sits under the tool call")

	m.endPendingTurn("s1")
	require.False(t, hasTurnTimer(m), "the timer goes with the turn")
}

func TestTurnTimerReadsRunning(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	created(m, message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
		Parts: []message.ContentPart{message.ToolCall{ID: "tc1", Name: "bash", Input: "{}", Finished: true}},
	})
	require.True(t, hasTurnTimer(m))
	out := ansi.Strip(m.chat.MessageItem(chat.PendingAssistantID).Render(80))
	require.Regexp(t, `Running \d+s`, out)
}

// While tool calls are still being written, the turn time rides on the last
// one's spinner instead of a separate line; once the arguments finish it
// moves back to the "Running" line under the calls.
func TestTurnTimerRidesOnTheLastStreamingToolCall(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	created(m, userMessage("u1", "s1", "first"))
	created(m, message.Message{ID: "a1", Role: message.Assistant, SessionID: "s1"})

	streaming := message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
		Parts: []message.ContentPart{
			message.ToolCall{ID: "tc1", Name: "bash"},
			message.ToolCall{ID: "tc2", Name: "bash"},
		},
	}
	updated(m, streaming)
	require.False(t, hasTurnTimer(m), "the streaming call carries the time")
	require.Equal(t, "tc2", m.pending.carrier.ID(), "only the last streaming call shows it")

	streaming.Parts = []message.ContentPart{
		message.ToolCall{ID: "tc1", Name: "bash", Input: "{}", Finished: true},
		message.ToolCall{ID: "tc2", Name: "bash", Input: "{}", Finished: true},
	}
	updated(m, streaming)
	require.True(t, hasTurnTimer(m), "finished arguments hand the time back to the Running line")
	require.Nil(t, m.pending.carrier)
}

// The bottom spinner means live tokens are streaming: it spins while answer
// text arrives and settles to the static "Running" line once a tool runs.
func TestTurnTimerSpinsWhileTextStreams(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	created(m, userMessage("u1", "s1", "first"))
	created(m, message.Message{ID: "a1", Role: message.Assistant, SessionID: "s1"})

	text := message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
		Parts: []message.ContentPart{message.TextContent{Text: "partial answer"}},
	}
	updated(m, text)
	require.True(t, hasPending(m), "streaming text keeps the bottom spinner spinning")

	text.Parts = append(text.Parts, message.ToolCall{ID: "tc1", Name: "bash", Input: "{}", Finished: true})
	updated(m, text)
	require.True(t, hasTurnTimer(m), "once a tool runs it settles to the Running line")
}

// A working subagent already shows a spinner under its tree, so it carries
// the turn time itself instead of a second spinner or Running line below it.
func TestTurnTimerRidesOnARunningSubagent(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	created(m, userMessage("u1", "s1", "first"))
	created(m, message.Message{ID: "a1", Role: message.Assistant, SessionID: "s1"})

	updated(m, message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
		Parts: []message.ContentPart{message.ToolCall{
			ID: "ag1", Name: "agentic_fetch", Input: `{"prompt":"look it up"}`, Finished: true,
		}},
	})
	require.False(t, hasTurnTimer(m), "the subagent's spinner shows the time")
	require.False(t, hasPending(m), "and no second spinner sits under it")
	require.Equal(t, "ag1", m.pending.carrier.ID())

	created(m, message.Message{
		ID: "r1", Role: message.Tool, SessionID: "s1",
		Parts: []message.ContentPart{message.ToolResult{ToolCallID: "ag1", Name: "agentic_fetch", Content: "done"}},
	})
	require.True(t, hasTurnTimer(m), "once it finishes the Running line takes over")
	require.Nil(t, m.pending.carrier)
}

// A turn already running when the session is opened, or started by another
// client, gets the same turn timer, counted from its prompt, and an idle
// probe retires it if no RunComplete arrives.
func TestRunningTurnFromElsewhereIsTimed(t *testing.T) {
	m := newPendingUI(t)
	m.lastUserMessageTime = time.Now().Add(-90 * time.Second).Unix()
	created(m, userMessage("u1", "s1", "from another client"))
	m.lastUserMessageTime = time.Now().Add(-90 * time.Second).Unix()
	created(m, message.Message{
		ID: "a1", Role: message.Assistant, SessionID: "s1",
		Parts: []message.ContentPart{message.ToolCall{ID: "tc1", Name: "bash", Input: "{}", Finished: true}},
	})
	require.False(t, hasTurnTimer(m), "nothing to time until the probe says the session is busy")

	m.applyBusyState(busyStateMsg{gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: true})
	m.Update(neutralMsg{})
	require.True(t, hasTurnTimer(m), "the adopted turn shows the Running line")
	require.Contains(t, common.Elapsed("s1"), "1m", "it counts from the prompt, not from when it was seen")

	m.applyBusyState(busyStateMsg{gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: false})
	m.Update(neutralMsg{})
	require.False(t, hasTurnTimer(m), "an idle probe retires a turn this client only adopted")
}

// A turn this client sent is not touched by an idle probe that lags behind it.
func TestIdleProbeLeavesASentTurn(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("first")
	m.applyBusyState(busyStateMsg{gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusy: false})
	m.Update(neutralMsg{})
	require.True(t, hasPending(m))
}
