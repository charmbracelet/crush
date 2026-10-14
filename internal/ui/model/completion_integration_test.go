package model

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

func TestCompletionKeepsQueuedContinuationInterval(t *testing.T) {
	m := newPendingUI(t)
	ws := m.com.Workspace.(*countingWorkspace)
	m.sendMessage("A")
	runCmds(m, m.submissionFinished("s1", m.submissionGen, nil, false))
	ws.agentBusy = true
	_, cmd := m.Update(runCompleteMsg(notify.RunComplete{Sequence: 2, Error: "B failed"}))
	runCmds(m, cmd)
	require.True(t, hasPending(m), "A's continuation still owns the interval")
	require.NotEmpty(t, common.Elapsed("s1"))
	ws.agentBusy = false
	runCmds(m, m.handleAgentNotification(notify.Notification{Type: notify.TypeAgentFinished, SessionID: "s1", FinishState: notify.FinishIdleUnsuccessful}))
	require.False(t, hasPending(m))
	require.Empty(t, common.Elapsed("s1"))
	require.Equal(t, tea.ProgramStateError, m.turnOutcome)
}

func TestCompletionOutcomeUsesLogicalFinishOrder(t *testing.T) {
	for _, queuedFails := range []bool{false, true} {
		m := newProgramStatusUI(t)
		first := notify.RunComplete{Sequence: 1}
		last := notify.RunComplete{Sequence: 2}
		want := tea.ProgramStateDone
		if queuedFails {
			last.Error = "queued failure"
			want = tea.ProgramStateError
		} else {
			first.Error = "initial failure"
		}
		// Coordinator buffering delivers A after B without changing either result.
		m.trackTurnOutcome(runCompleteMsg(last))
		m.trackTurnOutcome(runCompleteMsg(first))
		require.Equal(t, want, m.turnOutcome)
		m.trackTurnOutcome(tea.FocusMsg{})
		m.trackTurnOutcome(runCompleteMsg(first))
		require.Empty(t, m.turnOutcome, "focus must not revive an older result")
	}
}

func TestFailedBusyProbeKeepsPendingInterval(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("prompt")
	m.Update(runCompleteMsg(notify.RunComplete{}))
	m.applyBusyState(busyStateMsg{gen: m.busyFetchGen, ready: true, forSession: "s1", sessionBusyErr: errors.New("offline")})
	require.True(t, hasPending(m))
	require.NotEmpty(t, common.Elapsed("s1"))
}

func TestForeignIdleDoesNotStopCurrentInterval(t *testing.T) {
	m := newPendingUI(t)
	m.sendMessage("prompt")
	common.StartTurn("s2")
	t.Cleanup(func() { common.StopTurn("s2") })
	_, cmd := m.Update(pubsub.Event[notify.Notification]{Payload: notify.Notification{Type: notify.TypeAgentFinished, SessionID: "s2", FinishState: notify.FinishIdleSuccess}})
	runCmds(m, cmd)
	require.NotEmpty(t, common.Elapsed("s1"))
	require.Empty(t, common.Elapsed("s2"))
}

func TestSubmissionErrorDoesNotReplaceAuthoritativeQueuedOutcome(t *testing.T) {
	for _, completed := range []bool{false, true} {
		m := newPendingUI(t)
		m.sendMessage("A")
		_, cmd := m.Update(runCompleteMsg(notify.RunComplete{Sequence: 2}))
		runCmds(m, cmd)
		_, cmd = m.Update(agentRunSubmittedMsg{sessionID: "s1", gen: m.submissionGen, err: errors.New("A failed"), completed: completed})
		runCmds(m, cmd)
		if completed {
			require.Equal(t, tea.ProgramStateDone, m.turnOutcome, "A's synchronous return must not replace B's later success")
		} else {
			require.Equal(t, tea.ProgramStateError, m.turnOutcome, "an admission error has no completion of its own")
		}
	}
}

func TestFinishedNotificationAndDelayedProbePreserveFocusLifetime(t *testing.T) {
	m := newProgramStatusUI(t)
	m.trackTurnOutcome(finished())
	runCmds(m, m.handleAgentNotification(notify.Notification{Type: notify.TypeAgentFinished, SessionID: "s1", FinishState: notify.FinishIdleSuccess}))
	require.Equal(t, tea.ProgramStateDone, m.turnOutcome)
	m.trackTurnOutcome(tea.FocusMsg{})
	runCmds(m, m.handleAgentNotification(notify.Notification{Type: notify.TypeAgentFinished, SessionID: "s1", FinishState: notify.FinishIdleSuccess}))
	require.Empty(t, m.turnOutcome)
}

func TestCompletionSequenceAcrossRecovery(t *testing.T) {
	for _, recreated := range []bool{false, true} {
		m := newPendingUI(t)
		m.trackTurnOutcome(runCompleteMsg(notify.RunComplete{Sequence: 100, Error: "old failure"}))
		// Recovery is delivered after all old-stream events, before new events.
		m.handleConnectionEvent(workspace.ConnectionEvent{State: workspace.ConnectionRecovered, WorkspaceRecreated: recreated})
		m.trackTurnOutcome(runCompleteMsg(notify.RunComplete{Sequence: 1}))
		if recreated {
			require.Equal(t, tea.ProgramStateDone, m.turnOutcome)
			require.Equal(t, uint64(1), m.lastCompletionSequence)
		} else {
			require.Equal(t, tea.ProgramStateError, m.turnOutcome)
			require.Equal(t, uint64(100), m.lastCompletionSequence)
		}
	}
}
