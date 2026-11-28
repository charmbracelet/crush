package agent

import (
	"context"

	"charm.land/fantasy"
)

// startReportingTool tells onStart when fantasy begins running a call.
// That moment is later than when the call first streamed in: fantasy
// runs calls only once the whole response has arrived, and calls that
// cannot run in parallel wait for the ones ahead of them.
type startReportingTool struct {
	fantasy.AgentTool
	onStart func(callID string)
}

func (t *startReportingTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	t.onStart(call.ID)
	return t.AgentTool.Run(ctx, call)
}

// reportToolStarts wraps each tool so onStart hears when its calls begin
// running.
func reportToolStarts(agentTools []fantasy.AgentTool, onStart func(callID string)) []fantasy.AgentTool {
	out := make([]fantasy.AgentTool, len(agentTools))
	for i, tool := range agentTools {
		out[i] = &startReportingTool{AgentTool: tool, onStart: onStart}
	}
	return out
}
