package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

const reportChannel = "signal"

// startOnChannel dispatches p in the background from a turn that originated
// on channel, which is empty for a local turn.
func startOnChannel(t *testing.T, h *bgHarness, channel string, p subAgentParams) {
	t.Helper()
	ctx := t.Context()
	if channel != "" {
		ctx = WithChannel(ctx, channel)
	}
	resp, err := h.coord.startBackground(ctx, p)
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
}

func (h *bgHarness) sessionChannel(t *testing.T, sessionID string) string {
	t.Helper()
	sess, err := h.coord.sessions.Get(t.Context(), sessionID)
	require.NoError(t, err)
	return sess.Channel
}

func TestDeliverReport_KeepsDispatchChannel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		channel     string
		wantChannel string
	}{
		{name: "idle parent runs the report turn on the channel", channel: reportChannel, wantChannel: reportChannel},
		{name: "local dispatch stays unbound", channel: "", wantChannel: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newBGHarness(t, true)
			if tt.channel != "" {
				_, err := h.coord.sessions.SetChannel(t.Context(), h.parentID, tt.channel)
				require.NoError(t, err)
			}

			startOnChannel(t, h, tt.channel, h.params(h.parentID, "task", "call-1", instantChild("all good")))

			call := receive(t, h.mainRuns)
			require.Equal(t, tt.wantChannel, call.Channel)
			require.Equal(t, tt.wantChannel, h.sessionChannel(t, h.parentID))
		})
	}
}

func TestDeliverReport_BusyParentKeepsChannelBinding(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)
	_, err := h.coord.sessions.SetChannel(t.Context(), h.parentID, reportChannel)
	require.NoError(t, err)

	// A busy parent accepts the report into its queue and returns without
	// running a turn.
	queued := make(chan SessionAgentCall, 1)
	busy := newMockAgent(bgProviderID, 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		queued <- call
		return nil, nil
	})
	h.coord.agentMu.Lock()
	h.coord.mainAgent = busy
	h.coord.agents[config.AgentCoder] = busy
	h.coord.agentMu.Unlock()

	startOnChannel(t, h, reportChannel, h.params(h.parentID, "task", "call-1", instantChild("all good")))

	call := receive(t, queued)
	require.Equal(t, reportChannel, call.Channel)
	require.Equal(t, reportChannel, h.sessionChannel(t, h.parentID))
}

func TestDeliverReport_KeepsChannelForReopenedTurn(t *testing.T) {
	t.Parallel()
	h := newBGHarness(t, true)
	_, err := h.coord.sessions.SetChannel(t.Context(), h.parentID, reportChannel)
	require.NoError(t, err)

	startOnChannel(t, h, reportChannel, h.params(h.parentID, "task", "call-1", instantChild("first")))
	receive(t, h.mainRuns)

	_, err = h.coord.sendToBackground(WithChannel(t.Context(), reportChannel), h.parentID, "task-1", "again")
	require.NoError(t, err)

	call := receive(t, h.mainRuns)
	require.Equal(t, reportChannel, call.Channel)
	require.Equal(t, reportChannel, h.sessionChannel(t, h.parentID))
}
