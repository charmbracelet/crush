package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

func TestChannelContextIsScopedToTurn(t *testing.T) {
	t.Parallel()
	channelContext := WithChannel(context.Background(), "signal")
	require.Equal(t, "signal", ChannelFromContext(channelContext))
	require.Empty(t, ChannelFromContext(context.Background()))
}

func TestFilterToolsForChannel(t *testing.T) {
	t.Parallel()
	channelTool := &channelTestTool{name: "send", mcpName: "signal"}
	plainTool := &fakeTool{name: "plain"}
	states := map[string]mcp.ClientInfo{"signal": {Channel: true}}

	local := filterToolsForChannel([]fantasy.AgentTool{channelTool, plainTool}, "", states)
	require.Len(t, local, 1)
	require.Equal(t, "plain", local[0].Info().Name)

	matching := filterToolsForChannel([]fantasy.AgentTool{channelTool, plainTool}, "signal", states)
	require.Len(t, matching, 2)
}

type channelTestTool struct {
	name    string
	mcpName string
}

func (t *channelTestTool) MCP() string {
	return t.mcpName
}

func (t *channelTestTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: t.name}
}

func (*channelTestTool) Run(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return fantasy.NewTextResponse("ok"), nil
}

func (*channelTestTool) ProviderOptions() fantasy.ProviderOptions {
	return nil
}

func (*channelTestTool) SetProviderOptions(fantasy.ProviderOptions) {}
