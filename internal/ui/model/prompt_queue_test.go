package model

import (
	"context"
	"errors"
	"slices"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestPromptQueue_DuplicateTextMatchesOneToOne(t *testing.T) {
	t.Parallel()

	var queue promptQueue
	queue.SetFetched([]string{"continue"})
	queue.Add("continue")
	require.Equal(t, []string{"continue", "continue"}, queue.Items())

	// The fetch still reporting only the first one claims the pending
	// copy: the known, brief under-count.
	queue.SetFetched([]string{"continue"})
	require.Equal(t, 1, queue.Shown())

	queue.SetFetched([]string{"continue", "continue"})
	require.Equal(t, 2, queue.Shown())
}

func TestPromptQueue_SettledPrunedOnNextFetch(t *testing.T) {
	t.Parallel()

	var queue promptQueue
	id := queue.Add("next")
	queue.Settle(id)
	require.Equal(t, []string{"next"}, queue.Items(), "settled prompts show until the next fetch")
	queue.ClearServer()
	require.Zero(t, queue.Shown())
}

// queueWorkspace is a countingWorkspace whose AgentRun enqueues the prompt
// behind the busy turn, or fails with err.
type queueWorkspace struct {
	*countingWorkspace
	err error
}

func (w *queueWorkspace) AgentRun(_ context.Context, _ string, prompt string, _ ...message.Attachment) error {
	if w.err != nil {
		return w.err
	}
	w.queued = append(w.queued, prompt)
	return nil
}

// newQueueUI returns a UI whose session s1 has a turn in flight.
func newQueueUI(t *testing.T) (*UI, *queueWorkspace) {
	t.Helper()
	pinTTLs(t)
	ws := &queueWorkspace{countingWorkspace: &countingWorkspace{ready: true, agentBusy: true}}
	m := newBusyUI(ws.countingWorkspace)
	m.com.Workspace = ws
	warmCaches(m, true)
	m.probedSession, m.probedSessionBusy = "s1", true
	return m, ws
}

// submit sends prompt and runs the submission, returning the
// agentRunSubmittedMsg without applying it.
func submit(t *testing.T, m *UI, prompt string) agentRunSubmittedMsg {
	t.Helper()
	batch, ok := m.sendMessage(prompt)().(tea.BatchMsg)
	require.True(t, ok)
	var submitted *agentRunSubmittedMsg
	for _, cmd := range batch {
		if cmd == nil {
			continue
		}
		if msg, ok := cmd().(agentRunSubmittedMsg); ok {
			submitted = &msg
		}
	}
	require.NotNil(t, submitted)
	return *submitted
}

// fetch applies a queue fetch of the workspace's current queue.
func fetch(m *UI, ws *queueWorkspace) {
	m.applyPromptQueue(promptQueueMsg{forSession: "s1", gen: m.promptQueueGen, prompts: ws.queued})
}

func TestQueuedPrompt_ShowsOnSubmit(t *testing.T) {
	m, ws := newQueueUI(t)

	batch := m.sendMessage("next")
	require.Equal(t, []string{"next"}, m.promptQueue.Items(), "the pill shows before the agent enqueues")

	// The agent enqueues it; the confirming fetch shows it once.
	for _, cmd := range batch().(tea.BatchMsg) {
		if cmd == nil {
			continue
		}
		if msg, ok := cmd().(agentRunSubmittedMsg); ok {
			_, next := m.Update(msg)
			runCmds(m, next)
		}
	}
	require.Equal(t, []string{"next"}, ws.queued)
	require.Equal(t, []string{"next"}, m.promptQueue.Items())
}

func TestQueuedPrompt_RejectedSubmitLeavesPill(t *testing.T) {
	m, ws := newQueueUI(t)
	ws.err = errors.New("no")

	msg := submit(t, m, "next")
	require.Equal(t, 1, m.promptQueue.Shown())

	_, next := m.Update(msg)
	runCmds(m, next)
	require.Zero(t, m.promptQueue.Shown(), "a rejected prompt leaves the pill once its fetch lands")
}

// TestQueuedPrompt_DequeuedBeforeSubmitSettles covers a fetch that sees the
// prompt, then the turn consuming it, all before AgentRun returns: the
// consumed prompt must not come back.
func TestQueuedPrompt_DequeuedBeforeSubmitSettles(t *testing.T) {
	m, ws := newQueueUI(t)

	msg := submit(t, m, "next")
	fetch(m, ws)
	require.Equal(t, []string{"next"}, m.promptQueue.Items())

	ws.queued = nil
	fetch(m, ws)
	require.Zero(t, m.promptQueue.Shown(), "a consumed prompt must not reappear")

	_, next := m.Update(msg)
	runCmds(m, next)
	require.Zero(t, m.promptQueue.Shown())
}

func TestQueuedPrompt_SessionSwitchDropsPending(t *testing.T) {
	m, ws := newQueueUI(t)

	submit(t, m, "next")
	ws.queued = nil
	_, cmd := m.Update(loadSessionMsg{session: &session.Session{ID: "s2"}})
	require.Zero(t, m.promptQueue.Shown())
	runCmds(m, cmd)
	require.Zero(t, m.promptQueue.Shown())
}

// TestQueuedPrompt_EscWithOnlyPendingCancels pins that esc clears only the
// server queue: prompts still being submitted are not there to clear, so
// esc arms and then cancels the running turn instead.
func TestQueuedPrompt_EscWithOnlyPendingCancels(t *testing.T) {
	m, ws := newQueueUI(t)

	submit(t, m, "next")
	ws.queued = nil
	require.Equal(t, 1, m.promptQueue.Shown())

	m.cancelAgent()
	require.True(t, m.isCanceling, "first esc arms the cancel")
	require.Zero(t, ws.clearQueueCalls)
	m.cancelAgent()
	require.Equal(t, 1, ws.cancelCalls, "second esc cancels the turn")
}

// The esc hint must match what esc does: with only pending prompts it does
// not clear a queue.
func TestQueuedPrompt_EscHelpMatchesQueue(t *testing.T) {
	m, ws := newQueueUI(t)
	m.state = uiChat
	escHelp := func(binds []key.Binding) string {
		for _, b := range binds {
			if b.Help().Key == "esc" {
				return b.Help().Desc
			}
		}
		return ""
	}
	escHelps := func() []string {
		return []string{escHelp(m.ShortHelp()), escHelp(slices.Concat(m.FullHelp()...))}
	}

	submit(t, m, "next")
	ws.queued = nil
	require.NotContains(t, escHelps(), "clear queue", "pending prompts are not in the queue esc clears")

	ws.queued = []string{"next"}
	fetch(m, ws)
	require.Equal(t, []string{"clear queue", "clear queue"}, escHelps())
}

func TestQueuedPrompt_HiddenNeverShows(t *testing.T) {
	m, _ := newQueueUI(t)

	_ = m.sendMessageInternal("continue the plan", true)
	require.Zero(t, m.promptQueue.Shown(), "a hidden prompt stays out of the pill")
}
