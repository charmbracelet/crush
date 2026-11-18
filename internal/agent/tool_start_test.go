package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestReportToolStartsBeforeRun(t *testing.T) {
	t.Parallel()

	var events []string
	inner := fantasy.NewAgentTool("probe", "probe", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		events = append(events, "run")
		return fantasy.NewTextResponse("ok"), nil
	})
	wrapped := reportToolStarts([]fantasy.AgentTool{inner}, func(id string) {
		events = append(events, "start "+id)
	})

	require.Equal(t, "probe", wrapped[0].Info().Name)
	_, err := wrapped[0].Run(t.Context(), fantasy.ToolCall{ID: "c1", Name: "probe", Input: "{}"})
	require.NoError(t, err)
	require.Equal(t, []string{"start c1", "run"}, events)
}
