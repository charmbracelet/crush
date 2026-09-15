package agent

import (
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func stallResult() *fantasy.AgentResult {
	return &fantasy.AgentResult{Steps: []fantasy.StepResult{
		stepWith(fantasy.FinishReasonToolCalls,
			fantasy.ToolResultContent{
				ToolCallID: "tc-v", ToolName: "view",
				Result: fantasy.ToolResultOutputContentText{Text: "same output"},
			},
		),
		// The loop detector forces this finish reason — the edge does
		// not require a clean stop.
		stepWith(fantasy.FinishReasonUnknown, fantasy.TextContent{Text: "..."}),
	}}
}

func TestStallEdge(t *testing.T) {
	t.Parallel()

	t.Run("stalled run queues an escalation retry", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		a.ambiguityClarification = true
		a.interactive = true
		a.tools = csync.NewSliceFrom([]fantasy.AgentTool{
			&fakeTool{name: tools.TodosToolName},
			&fakeTool{name: tools.QuestionToolName},
		})
		queued := a.runEdges(t.Context(), SessionAgentCall{SessionID: sessionID, RunID: "r", RunStamp: 42},
			edgeInput{result: stallResult(), stalled: true})
		require.True(t, queued)
		q, _ := a.messageQueue.Get(sessionID)
		require.Len(t, q, 1)
		require.Equal(t, "r", q[0].RunID)
		require.Equal(t, 1, q[0].RepairAttempts)
		// The retry clone keeps the turn's stamp so the scope gate
		// does not re-arm mid-turn.
		require.Equal(t, uint64(42), q[0].RunStamp)
		require.Contains(t, q[0].Prompt, "question")
	})

	t.Run("headless stall lands a blocker report, not a retry", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		a.ambiguityClarification = true
		asst := &message.Message{Role: message.Assistant}
		queued := a.runEdges(t.Context(), SessionAgentCall{SessionID: sessionID, NonInteractive: true},
			edgeInput{result: stallResult(), currentAssistant: asst, stalled: true})
		require.False(t, queued)
		q, _ := a.messageQueue.Get(sessionID)
		require.Empty(t, q)
		require.Contains(t, asst.Content().Text, "Stopped:")
		require.Contains(t, asst.Content().Text, "What's blocking:")
		require.Contains(t, asst.Content().Text, `"view"`)
	})

	t.Run("flag off leaves a silent stop", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		queued := a.runEdges(t.Context(), SessionAgentCall{SessionID: sessionID},
			edgeInput{result: stallResult(), stalled: true})
		require.False(t, queued)
	})

	t.Run("non-stalled run does not fire the edge", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		a.ambiguityClarification = true
		queued := a.runEdges(t.Context(), SessionAgentCall{SessionID: sessionID},
			edgeInput{result: stallResult()})
		require.False(t, queued)
	})

	t.Run("budget exhausted surfaces the terminal note", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		a.ambiguityClarification = true
		a.interactive = true
		a.tools = csync.NewSliceFrom([]fantasy.AgentTool{
			&fakeTool{name: tools.TodosToolName},
			&fakeTool{name: tools.QuestionToolName},
		})
		asst := &message.Message{Role: message.Assistant}
		queued := a.runEdges(t.Context(), SessionAgentCall{
			SessionID: sessionID, RepairAttempts: maxRepairAttempts,
		}, edgeInput{result: stallResult(), currentAssistant: asst, stalled: true})
		require.False(t, queued)
		require.Contains(t, asst.Content().Text, "repair attempt")
		// The structured blocker report survives exhaustion too —
		// interactive runs get the same what's-blocking detail
		// headless runs get from resolve.
		require.Contains(t, asst.Content().Text, "What's blocking:")
	})

	t.Run("one escalation per blocker via the shared budget", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		a.ambiguityClarification = true
		a.interactive = true
		a.tools = csync.NewSliceFrom([]fantasy.AgentTool{
			&fakeTool{name: tools.TodosToolName},
			&fakeTool{name: tools.QuestionToolName},
		})
		asst := &message.Message{Role: message.Assistant}
		for attempt := 0; attempt < maxRepairAttempts; attempt++ {
			queued := a.runEdges(t.Context(), SessionAgentCall{
				SessionID: sessionID, RepairAttempts: attempt,
			}, edgeInput{result: stallResult(), currentAssistant: asst, stalled: true})
			require.True(t, queued, "attempt %d should still have budget", attempt)
		}
		q, _ := a.messageQueue.Get(sessionID)
		require.Len(t, q, maxRepairAttempts)
		// Retries prepend — the newest (most-consumed budget) is first.
		require.Equal(t, maxRepairAttempts, q[0].RepairAttempts)
	})
}
