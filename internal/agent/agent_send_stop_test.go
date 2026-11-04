package agent

import (
	"context"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestStopBackground_CancelsRunningAgentAndWritesReportWithoutTurn(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	started := make(chan struct{}, 1)
	h.start(t, h.params(h.parentID, "task", "call-1", blockingChild(started, make(chan struct{}))))
	receive(t, started)

	out := h.coord.stopBackground(h.parentID, "task-1")
	require.Contains(t, out, "Stopping task-1")

	reports := h.awaitReports(t, h.parentID, 1)
	require.Len(t, reports, 1)
	require.Equal(t, "task-1", reports[0].Name)
	require.Equal(t, "cancelled", reports[0].Status)
	require.Empty(t, h.mainRuns, "a stop must not start a parent turn")
}

func TestStopBackground_FinishedAgentIsAlreadyFinished(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	h.start(t, h.params(h.parentID, "task", "call-1", instantChild("done")))
	// The parent turn starts only after the agent stopped running.
	receive(t, h.mainRuns)

	out := h.coord.stopBackground(h.parentID, "task-1")
	require.Contains(t, out, "already finished")
}

func TestStopBackground_UnknownNameListsNames(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	h.start(t, h.params(h.parentID, "task", "call-1", blockingChild(make(chan struct{}, 1), make(chan struct{}))))

	out := h.coord.stopBackground(h.parentID, "ghost")
	require.Contains(t, out, "ghost")
	require.Contains(t, out, "task-1")
}

func TestSendToBackground_SteersRunningAgentWithQueueOnlyCall(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	type steer struct {
		call   SessionAgentCall
		active bool
	}
	var (
		activated atomic.Bool
		turns     atomic.Int32
	)
	first := make(chan SessionAgentCall, 4)
	steered := make(chan steer, 4)
	release := make(chan struct{})
	child := newMockAgent(bgProviderID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		if call.QueueOnly {
			steered <- steer{call: call, active: activated.Load()}
			return nil, nil
		}
		turns.Add(1)
		first <- call
		activated.Store(true)
		markActive(call)
		return blockUntil(ctx, release, "done")
	})
	p := h.params(h.parentID, "task", "call-1", child)
	h.start(t, p)

	out, err := h.coord.sendToBackground(t.Context(), h.parentID, "task-1", "also check the tests")
	require.NoError(t, err)
	require.Contains(t, out, "Delivered")

	got := receive(t, steered)
	require.True(t, got.active, "the steering call must wait until the run is active")
	require.True(t, got.call.QueueOnly)
	require.Equal(t, "also check the tests", got.call.Prompt)
	require.Equal(t, h.childSessionID(p), got.call.SessionID)
	require.Equal(t, receive(t, first).MaxOutputTokens, got.call.MaxOutputTokens)

	close(release)
	receive(t, h.mainRuns)

	require.EqualValues(t, 1, turns.Load(), "steering must not start another turn")
	children, err := h.coord.sessions.ListChildSessions(t.Context(), h.parentID)
	require.NoError(t, err)
	require.Len(t, children, 1)
}

func TestSendToBackground_ReopensFinishedAgentOnSameSession(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	turns := make(chan SessionAgentCall, 4)
	var turnCount atomic.Int32
	child := newMockAgent(bgProviderID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		markActive(call)
		turns <- call
		// The child session's cost is cumulative across its turns.
		if turnCount.Add(1) == 1 {
			if err := setSessionCost(ctx, h.coord.sessions, call.SessionID, 1.0); err != nil {
				return nil, err
			}
			return agentResultWithText("first answer"), nil
		}
		if err := setSessionCost(ctx, h.coord.sessions, call.SessionID, 1.5); err != nil {
			return nil, err
		}
		return agentResultWithText("second answer"), nil
	})
	p := h.params(h.parentID, "task", "call-1", child)
	h.start(t, p)

	firstReport := receive(t, h.mainRuns).Report
	require.NotNil(t, firstReport)
	require.Equal(t, "first answer", firstReport.Output)
	require.Equal(t, h.childSessionID(p), receive(t, turns).SessionID)

	out, err := h.coord.sendToBackground(t.Context(), h.parentID, "task-1", "now do the follow-up")
	require.NoError(t, err)
	require.Contains(t, out, "Reopened")

	reopened := receive(t, turns)
	require.False(t, reopened.QueueOnly)
	require.Equal(t, "now do the follow-up", reopened.Prompt)
	require.Equal(t, h.childSessionID(p), reopened.SessionID)

	secondReport := receive(t, h.mainRuns).Report
	require.NotNil(t, secondReport)
	require.Equal(t, "task-1", secondReport.Name)
	require.Equal(t, "second answer", secondReport.Output)

	parent, err := h.coord.sessions.Get(t.Context(), h.parentID)
	require.NoError(t, err)
	require.InDelta(t, 1.5, parent.Cost, 1e-9, "a reopened turn bills only the cost it added")

	children, err := h.coord.sessions.ListChildSessions(t.Context(), h.parentID)
	require.NoError(t, err)
	require.Len(t, children, 1)
}

func TestSendToBackground_ReopensWhenSteerFindsSessionIdle(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	var turnCount atomic.Int32
	turns := make(chan SessionAgentCall, 4)
	queueAttempts := make(chan struct{}, 4)
	release := make(chan struct{})
	child := newMockAgent(bgProviderID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		if call.QueueOnly {
			queueAttempts <- struct{}{}
			return nil, ErrSessionIdle
		}
		markActive(call)
		turns <- call
		if turnCount.Add(1) == 1 {
			return blockUntil(ctx, release, "first answer")
		}
		return agentResultWithText("second answer"), nil
	})
	p := h.params(h.parentID, "task", "call-1", child)
	h.start(t, p)
	receive(t, turns)

	type sendResult struct {
		out string
		err error
	}
	sent := make(chan sendResult, 1)
	go func() {
		out, err := h.coord.sendToBackground(t.Context(), h.parentID, "task-1", "late message")
		sent <- sendResult{out, err}
	}()

	// The agent still counts as running but reports idle, so send must hold
	// until the turn ends and then reopen it.
	receive(t, queueAttempts)
	close(release)

	res := receive(t, sent)
	require.NoError(t, res.err)
	require.Contains(t, res.out, "Reopened")

	reopened := receive(t, turns)
	require.Equal(t, "late message", reopened.Prompt)
	require.Equal(t, h.childSessionID(p), reopened.SessionID)

	receive(t, h.mainRuns)
	receive(t, h.mainRuns)
}

func buildToolNames(t *testing.T, coord *coordinator, agent config.Agent, isSubAgent bool) []string {
	t.Helper()
	got, err := coord.buildTools(t.Context(), agent, isSubAgent, "")
	require.NoError(t, err)
	names := make([]string, 0, len(got))
	for _, tool := range got {
		names = append(names, tool.Info().Name)
	}
	return names
}

func TestBuildTools_BackgroundToolsExposure(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()
	coder := cfg.Config().Agents[config.AgentCoder]

	interactive := newBuildToolsCoordinator(cfg, env)
	interactive.interactive = true
	headless := newBuildToolsCoordinator(cfg, env)

	t.Run("interactive top-level agent gets both", func(t *testing.T) {
		t.Parallel()
		names := buildToolNames(t, interactive, coder, false)
		require.Contains(t, names, AgentSendToolName)
		require.Contains(t, names, AgentStopToolName)
	})

	t.Run("headless agent gets neither", func(t *testing.T) {
		t.Parallel()
		names := buildToolNames(t, headless, coder, false)
		require.NotContains(t, names, AgentSendToolName)
		require.NotContains(t, names, AgentStopToolName)
	})

	t.Run("subagents get neither", func(t *testing.T) {
		t.Parallel()
		names := buildToolNames(t, interactive, coder, true)
		require.NotContains(t, names, AgentSendToolName)
		require.NotContains(t, names, AgentStopToolName)
	})

	t.Run("agent without the dispatcher gets neither", func(t *testing.T) {
		t.Parallel()
		noDispatcher := config.Agent{ID: "limited", Name: "Limited", AllowedTools: []string{"view"}}
		names := buildToolNames(t, interactive, noDispatcher, false)
		require.NotContains(t, names, AgentSendToolName)
		require.NotContains(t, names, AgentStopToolName)
	})
}

func TestAgentTool_BackgroundParameterOnlyWhenInteractive(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()
	coder := cfg.Config().Agents[config.AgentCoder]

	t.Run("interactive schema has a boolean background property", func(t *testing.T) {
		t.Parallel()
		coord := newBuildToolsCoordinator(cfg, env)
		coord.interactive = true
		tool, err := coord.agentTool(t.Context(), coder)
		require.NoError(t, err)

		params := tool.Info().Parameters
		require.Contains(t, params, "background")
		background, ok := params["background"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "boolean", background["type"])
	})

	t.Run("headless schema has no background property", func(t *testing.T) {
		t.Parallel()
		coord := newBuildToolsCoordinator(cfg, env)
		tool, err := coord.agentTool(t.Context(), coder)
		require.NoError(t, err)

		require.NotContains(t, tool.Info().Parameters, "background")
	})
}
