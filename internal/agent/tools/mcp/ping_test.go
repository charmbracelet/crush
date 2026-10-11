package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// noPingSession connects to an in-memory server that answers ping with
// MethodNotFound, like a server that never implemented the method.
func noPingSession(t *testing.T) *ClientSession {
	t.Helper()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "no-ping"}, nil)
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "ping" {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not found"}
			}
			return next(ctx, method, req)
		}
	})
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	client := mcp.NewClient(&mcp.Implementation{Name: "crush-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)

	return &ClientSession{ClientSession: clientSession, cancel: cancel}
}

func TestGetOrRenewClient_KeepsSessionWhenPingUnsupported(t *testing.T) {
	const name = "test-no-ping"

	sess := noPingSession(t)
	sessions.Set(name, sess)
	t.Cleanup(func() {
		if s, ok := sessions.Take(name); ok {
			_ = s.Close()
		}
		states.Del(name)
	})

	origNewSession := newSession
	newSession = func(context.Context, *config.ConfigStore, string, config.MCPConfig, config.VariableResolver, bool) (*ClientSession, error) {
		t.Fatal("a server that answers ping with MethodNotFound must not be rebuilt")
		return nil, nil
	}
	t.Cleanup(func() { newSession = origNewSession })

	cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{name: {Type: config.MCPStdio}}})
	for range 3 {
		got, err := getOrRenewClient(context.Background(), cfg, name)
		require.NoError(t, err)
		require.Same(t, sess, got)
	}
}

func TestPingSession_ClosedSessionStillFails(t *testing.T) {
	sess := noPingSession(t)
	require.NoError(t, sess.Close())

	require.Error(t, pingSession(context.Background(), sess, time.Second))
}
