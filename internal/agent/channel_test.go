package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
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

// TestSyncSessionChannelLifecycle covers the full lifecycle of the persisted
// session-channel binding: a channel turn binds the session, a push from a
// different channel rebinds it (newest wins), and a local turn clears it —
// the reaping path for sessions the user takes over.
func TestSyncSessionChannelLifecycle(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, db.Release(dataDir))
		db.ResetPool()
	})

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	sessions := session.NewService(db.New(conn), conn)
	c := &coordinator{sessions: sessions}

	created, err := sessions.Create(t.Context(), "channel session")
	require.NoError(t, err)

	// A channel-originated turn binds the session to its channel.
	c.syncSessionChannel(t.Context(), created.ID, "signal")
	got, err := sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "signal", got.Channel)

	// A push from another channel rebinds: the newest push wins.
	c.syncSessionChannel(t.Context(), created.ID, "switchboard")
	got, err = sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "switchboard", got.Channel)

	// A local turn (no originating channel) clears the binding.
	c.syncSessionChannel(t.Context(), created.ID, "")
	got, err = sessions.Get(t.Context(), created.ID)
	require.NoError(t, err)
	require.Empty(t, got.Channel)

	// A missing session is a no-op, not an error.
	c.syncSessionChannel(t.Context(), "does-not-exist", "signal")
}
