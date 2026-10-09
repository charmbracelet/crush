package mcp

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

type echoIn struct {
	Text string `json:"text"`
}

// callToolSession connects an in-memory MCP server offering echo, which
// returns its text, and fail, which reports a tool error.
func callToolSession(t *testing.T) (*ClientSession, []*Tool) {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "calltool-server"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo"}, func(_ context.Context, _ *mcp.CallToolRequest, in echoIn) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: in.Text}}}, nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "fail"}, func(context.Context, *mcp.CallToolRequest, echoIn) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "it failed"}}}, nil, nil
	})
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	client := mcp.NewClient(&mcp.Implementation{Name: "crush-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	sess := &ClientSession{ClientSession: clientSession, cancel: cancel}
	tools, err := getTools(context.Background(), sess)
	require.NoError(t, err)
	return sess, tools
}

func TestCallTool(t *testing.T) {
	const name = "test-calltool"
	sess, tools := callToolSession(t)
	sessions.Set(name, sess)
	allTools.Set(name, tools)
	t.Cleanup(func() {
		if s, ok := sessions.Take(name); ok {
			_ = s.Close()
		}
		allTools.Del(name)
		states.Del(name)
	})
	cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{name: {Type: config.MCPStdio}}})

	t.Run("returns the server's result", func(t *testing.T) {
		res, err := CallTool(context.Background(), cfg, name, "echo", map[string]any{"text": "hello"})
		require.NoError(t, err)
		require.False(t, res.IsError)
		require.Len(t, res.Content, 1)
		require.Equal(t, "hello", res.Content[0].(*mcp.TextContent).Text)
	})

	t.Run("keeps the server's error flag", func(t *testing.T) {
		res, err := CallTool(context.Background(), cfg, name, "fail", map[string]any{})
		require.NoError(t, err)
		require.True(t, res.IsError)
	})

	t.Run("refuses a tool the server does not offer", func(t *testing.T) {
		_, err := CallTool(context.Background(), cfg, name, "missing", nil)
		require.ErrorIs(t, err, ErrToolNotFound)
	})

	t.Run("refuses a server that is not connected", func(t *testing.T) {
		_, err := CallTool(context.Background(), cfg, "absent", "echo", nil)
		require.ErrorIs(t, err, ErrToolNotFound)
	})
}
