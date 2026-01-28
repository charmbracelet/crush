// Package ping provides a simple "ping" tool for testing the Crush plugin system.
//
// When the agent calls ping(), the tool responds with "pong".
// This serves as a proof-of-concept for the plugin architecture.
package ping

import (
	"context"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/plugin"
)

const (
	// ToolName is the name of the ping tool.
	ToolName = "ping"

	// Description is the tool description shown to the LLM.
	Description = `A simple test tool that responds with "pong" when called.

<usage>
Call this tool to verify the plugin system is working correctly.
No parameters are required.
</usage>

<example>
ping() -> "pong"
</example>
`
)

// PingParams defines the parameters for the ping tool (none required).
type PingParams struct{}

func init() {
	plugin.RegisterTool(ToolName, func(ctx context.Context, app *plugin.App) (plugin.Tool, error) {
		return NewPingTool(), nil
	})
}

// NewPingTool creates a new ping tool instance.
func NewPingTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ToolName,
		Description,
		func(ctx context.Context, params PingParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("pong"), nil
		},
	)
}
