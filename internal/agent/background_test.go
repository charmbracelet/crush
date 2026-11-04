package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/require"
)

const (
	// bgProviderID is the provider configured by newGateTestCoordinator.
	bgProviderID = "mock"
	// bgTimeout is a safety net for channel waits, never a synchronization
	// mechanism.
	bgTimeout = 5 * time.Second
)

// bgHarness is a coordinator wired with a recording mock main agent, so
// report delivery can be observed without a real model.
type bgHarness struct {
	coord    *coordinator
	parentID string
	mainRuns chan SessionAgentCall
}

func newBGHarness(t *testing.T, interactive bool) *bgHarness {
	t.Helper()

	coord := newGateTestCoordinator(t, interactive)
	coord.runtime = subagents.NewRuntime()
	t.Cleanup(coord.runtime.Shutdown)
	coord.subagentCancels = csync.NewMap[string, context.CancelFunc]()

	mainRuns := make(chan SessionAgentCall, 16)
	main := newMockAgent(bgProviderID, 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		mainRuns <- call
		return agentResultWithText("ok"), nil
	})
	coord.agentMu.Lock()
	coord.mainAgent = main
	coord.agents[config.AgentCoder] = main
	coord.agentMu.Unlock()

	// Registered after the runtime cleanup so it runs first and every
	// background run has finished before the runtime shuts down.
	t.Cleanup(coord.CancelAll)

	h := &bgHarness{coord: coord, mainRuns: mainRuns}
	h.parentID = h.newParent(t)
	return h
}

func (h *bgHarness) newParent(t *testing.T) string {
	t.Helper()
	sess, err := h.coord.sessions.Create(t.Context(), "Parent")
	require.NoError(t, err)
	return sess.ID
}

func (h *bgHarness) params(parentID, typ, callID string, agent SessionAgent) subAgentParams {
	return subAgentParams{
		Agent:          agent,
		SessionID:      parentID,
		AgentMessageID: "msg-" + callID,
		ToolCallID:     callID,
		Prompt:         "initial prompt",
		SessionTitle:   typ + " Agent Session",
		AgentName:      typ,
		AgentColor:     "blue",
		AgentModel:     "mock-model",
	}
}

func (h *bgHarness) childSessionID(p subAgentParams) string {
	return h.coord.sessions.CreateAgentToolSessionID(p.AgentMessageID, p.ToolCallID)
}

// start runs startBackground and fails if it does not return promptly.
func (h *bgHarness) start(t *testing.T, p subAgentParams) (fantasy.ToolResponse, AgentBackgroundMetadata) {
	t.Helper()

	type result struct {
		resp fantasy.ToolResponse
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := h.coord.startBackground(t.Context(), p)
		done <- result{resp, err}
	}()
	res := receive(t, done)
	require.NoError(t, res.err)
	require.False(t, res.resp.IsError, res.resp.Content)

	var meta AgentBackgroundMetadata
	require.NoError(t, json.Unmarshal([]byte(res.resp.Metadata), &meta))
	return res.resp, meta
}

// receive waits for a value on ch, failing the test after bgTimeout.
func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(bgTimeout):
		t.Fatal("timed out waiting for a value")
	}
	var zero T
	return zero
}

// markActive mimics sessionAgent.Run signalling that the run is active.
func markActive(call SessionAgentCall) {
	if call.onActive != nil {
		call.onActive()
	}
}

// blockUntil parks the run until release is closed or ctx is cancelled.
func blockUntil(ctx context.Context, release <-chan struct{}, text string) (*fantasy.AgentResult, error) {
	select {
	case <-release:
		return agentResultWithText(text), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// blockingChild is a child whose run is active and parked until release is
// closed or its context is cancelled. started receives one value per run.
func blockingChild(started chan<- struct{}, release <-chan struct{}) *mockSessionAgent {
	return newMockAgent(bgProviderID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		markActive(call)
		started <- struct{}{}
		return blockUntil(ctx, release, "released")
	})
}

// instantChild is a child that finishes at once with the given text.
func instantChild(text string) *mockSessionAgent {
	return newMockAgent(bgProviderID, 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		markActive(call)
		return agentResultWithText(text), nil
	})
}

// listReports returns the subagent report parts stored on a session. It
// returns nil on error so it is safe inside require.Eventually.
func listReports(ctx context.Context, svc message.Service, sessionID string) []message.SubagentReport {
	msgs, err := svc.List(ctx, sessionID)
	if err != nil {
		return nil
	}
	var reports []message.SubagentReport
	for _, m := range msgs {
		if r, ok := m.SubagentReport(); ok {
			reports = append(reports, r)
		}
	}
	return reports
}

func (h *bgHarness) awaitReports(t *testing.T, sessionID string, n int) []message.SubagentReport {
	t.Helper()
	var reports []message.SubagentReport
	require.Eventually(t, func() bool {
		reports = listReports(t.Context(), h.coord.messages, sessionID)
		return len(reports) >= n
	}, bgTimeout, 5*time.Millisecond)
	return reports
}

func TestStartBackground_NamesCountPerParentAndType(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	started := make(chan struct{}, 8)
	release := make(chan struct{})
	child := blockingChild(started, release)

	otherParent := h.newParent(t)

	_, first := h.start(t, h.params(h.parentID, "task", "call-1", child))
	_, second := h.start(t, h.params(h.parentID, "task", "call-2", child))
	_, otherSession := h.start(t, h.params(otherParent, "task", "call-3", child))
	_, otherType := h.start(t, h.params(h.parentID, "code-reviewer", "call-4", child))

	require.Equal(t, "task-1", first.Name)
	require.Equal(t, "task-2", second.Name)
	require.Equal(t, "task-1", otherSession.Name)
	require.Equal(t, "code-reviewer-1", otherType.Name)
}

func TestStartBackground_ReturnsImmediatelyWithNameAndMetadata(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	p := h.params(h.parentID, "task", "call-1", blockingChild(started, release))

	// The child parks until released, so a return here proves startBackground
	// did not wait for it.
	resp, meta := h.start(t, p)

	require.Contains(t, resp.Content, `Started in the background as "task-1"`)
	require.Equal(t, AgentBackgroundMetadata{Name: "task-1", Background: true}, meta)

	childSession, err := h.coord.sessions.Get(t.Context(), h.childSessionID(p))
	require.NoError(t, err)
	require.Equal(t, h.parentID, childSession.ParentSessionID)
	require.Equal(t, []string{"task-1"}, h.coord.background.names(h.parentID))

	receive(t, started)
}

func TestDeliverReport_StartsParentTurnWhenCompleted(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	h.start(t, h.params(h.parentID, "task", "call-1", instantChild("all good")))

	call := receive(t, h.mainRuns)
	require.Equal(t, h.parentID, call.SessionID)
	require.NotNil(t, call.Report)
	require.Equal(t, message.SubagentReport{Name: "task-1", Status: "completed", Output: "all good"}, *call.Report)
	require.Equal(t, call.Report.PromptText(), call.Prompt)

	// The parent turn stores the report itself, so the coordinator must not
	// write a second copy.
	require.Empty(t, listReports(t.Context(), h.coord.messages, h.parentID))
}

func TestCancelAll_WritesStoppedReportWithoutParentTurn(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	h.start(t, h.params(h.parentID, "task", "call-1", blockingChild(started, release)))
	receive(t, started)

	h.coord.CancelAll()

	reports := listReports(t.Context(), h.coord.messages, h.parentID)
	require.Equal(t, []message.SubagentReport{
		{Name: "task-1", Status: "cancelled", Output: "Stopped: Crush exited"},
	}, reports)
	require.Empty(t, h.mainRuns, "shutdown must not start a parent turn")
}

func TestStartBackground_RefusedWhileShuttingDown(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	h.coord.CancelAll()

	resp, err := h.coord.startBackground(t.Context(), h.params(h.parentID, "task", "call-1", instantChild("late")))
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "Crush is shutting down")
	require.Empty(t, h.coord.background.names(h.parentID))
}

func TestDispatchSubAgent_BackgroundOnlyWhenInteractive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		interactive bool
		background  bool
		wantStarted bool
	}{
		{name: "headless ignores the flag and blocks", interactive: false, background: true},
		{name: "interactive without the flag blocks", interactive: true, background: false},
		{name: "interactive with the flag starts in the background", interactive: true, background: true, wantStarted: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newBGHarness(t, tt.interactive)
			p := h.params(h.parentID, "task", "call-1", instantChild("child output"))

			resp, err := h.coord.dispatchSubAgent(t.Context(), p, tt.background)
			require.NoError(t, err)
			require.False(t, resp.IsError, resp.Content)

			if !tt.wantStarted {
				require.Equal(t, "child output", resp.Content)
				require.Empty(t, h.coord.background.names(h.parentID))
				require.Empty(t, h.mainRuns)
				return
			}
			require.Contains(t, resp.Content, `Started in the background as "task-1"`)
			receive(t, h.mainRuns)
		})
	}
}

func TestRollUpSessionCost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		billed        float64
		childCost     float64
		wantParent    float64
		wantWatermark float64
	}{
		{name: "adds only the unbilled delta", billed: 0.5, childCost: 1.25, wantParent: 0.75, wantWatermark: 1.25},
		{name: "bills everything when nothing was billed", billed: 0, childCost: 1.25, wantParent: 1.25, wantWatermark: 1.25},
		{name: "never adds a negative amount", billed: 2, childCost: 1.25, wantParent: 0, wantWatermark: 1.25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := testEnv(t)
			coord := &coordinator{sessions: env.sessions}

			parent, err := env.sessions.Create(t.Context(), "Parent")
			require.NoError(t, err)
			child, err := env.sessions.CreateTaskSession(t.Context(), "tool-1", parent.ID, "Child")
			require.NoError(t, err)
			require.NoError(t, setSessionCost(t.Context(), env.sessions, child.ID, tt.childCost))

			watermark, err := coord.rollUpSessionCost(t.Context(), child.ID, parent.ID, tt.billed)
			require.NoError(t, err)
			require.InDelta(t, tt.wantWatermark, watermark, 1e-9)

			updated, err := env.sessions.Get(t.Context(), parent.ID)
			require.NoError(t, err)
			require.InDelta(t, tt.wantParent, updated.Cost, 1e-9)
		})
	}
}

// setSessionCost sets a session's absolute cost, like a child accumulating
// spend over its turns.
func setSessionCost(ctx context.Context, svc session.Service, sessionID string, cost float64) error {
	sess, err := svc.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	sess.Cost = cost
	_, err = svc.Save(ctx, sess)
	return err
}
