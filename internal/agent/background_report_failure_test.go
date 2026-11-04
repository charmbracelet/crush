package agent

import (
	"context"
	"errors"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// settle waits for every background run, including its report delivery.
func (h *bgHarness) settle() {
	h.coord.backgroundWg.Wait()
}

func TestDeliverReport_PersistsReportWhenDeliveryFailsBeforeTheTurn(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	h.start(t, h.params(h.parentID, "task", "call-1", blockingChild(started, release)))
	receive(t, started)

	// The parent turn cannot start: its agent is gone by the time the child
	// finishes, so the run fails before any message is created.
	h.coord.agentMu.Lock()
	h.coord.mainAgentName = "no-such-agent"
	h.coord.agentMu.Unlock()
	close(release)
	h.settle()

	require.Equal(t, []message.SubagentReport{
		{Name: "task-1", Status: "completed", Output: "released"},
	}, listReports(t.Context(), h.coord.messages, h.parentID))
	require.Empty(t, h.mainRuns, "the fallback must not start a turn")
}

func TestDeliverReport_DoesNotDuplicateReportWhenTurnFailsAfterItWasStored(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)

	failing := newMockAgent(bgProviderID, 4096, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		// The real agent stores the report message before it calls the model.
		_, err := h.coord.messages.Create(ctx, call.SessionID, message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{*call.Report},
		})
		require.NoError(t, err)
		return nil, errors.New("model failed")
	})
	h.coord.agentMu.Lock()
	h.coord.mainAgent = failing
	h.coord.agents["coder"] = failing
	h.coord.agentMu.Unlock()

	h.start(t, h.params(h.parentID, "task", "call-1", instantChild("all good")))
	h.settle()

	require.Equal(t, []message.SubagentReport{
		{Name: "task-1", Status: "completed", Output: "all good"},
	}, listReports(t.Context(), h.coord.messages, h.parentID))
}
