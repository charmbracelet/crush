package mcp

import (
	"context"
	"errors"
	"sync/atomic"
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

// countingPingSession connects to an in-memory server exposing one tool and
// counts the pings it receives.
func countingPingSession(t *testing.T, pings *atomic.Int32) *ClientSession {
	t.Helper()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "counting-ping"}, nil)
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "ping" {
				pings.Add(1)
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

	sess := &ClientSession{ClientSession: clientSession, cancel: cancel}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func TestGetOrRenewClient_SkipsPingForRecentlyUsedRemoteSession(t *testing.T) {
	for _, tc := range []struct {
		typ       config.MCPType
		wantPings int32
	}{
		{config.MCPHttp, 1},
		{config.MCPSSE, 1},
		{config.MCPStdio, 3},
	} {
		t.Run(string(tc.typ), func(t *testing.T) {
			name := "test-recent-ping-" + string(tc.typ)
			var pings atomic.Int32
			sess := countingPingSession(t, &pings)
			sessions.Set(name, sess)
			t.Cleanup(func() {
				sessions.Del(name)
				states.Del(name)
			})

			cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{name: {Type: tc.typ}}})
			for range 3 {
				got, err := getOrRenewClient(context.Background(), cfg, name)
				require.NoError(t, err)
				require.Same(t, sess, got)
			}
			require.Equal(t, tc.wantPings, pings.Load())

			// A failed request forgets the last success, so the next call
			// pings again.
			sess.noteResult(errors.New("boom"))
			_, err := getOrRenewClient(context.Background(), cfg, name)
			require.NoError(t, err)
			require.Equal(t, tc.wantPings+1, pings.Load())
		})
	}
}

func TestRunTool_DeadRecentSessionRenewsOnNextCall(t *testing.T) {
	const name = "test-recent-dead-session"
	t.Cleanup(func() {
		if s, ok := sessions.Take(name); ok {
			_ = s.Close()
		}
		allTools.Del(name)
		states.Del(name)
	})

	cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{name: {Type: config.MCPHttp}}})

	// The session served a request moments ago, then died.
	dead, _ := liveSession(t, "send_message")
	dead.markOK()
	require.NoError(t, dead.Close())
	sessions.Set(name, dead)

	replacement, _ := liveSession(t, "send_message")
	var created atomic.Int32
	origNewSession := newSession
	newSession = func(context.Context, *config.ConfigStore, string, config.MCPConfig, config.VariableResolver, bool) (*ClientSession, error) {
		created.Add(1)
		return replacement, nil
	}
	t.Cleanup(func() { newSession = origNewSession })

	// Inside the window the dead session is trusted, so this call fails.
	_, err := RunTool(context.Background(), cfg, name, "send_message", "{}")
	require.Error(t, err)
	require.Zero(t, created.Load())
	require.False(t, dead.recentlyOK())

	// The failure cleared the mark, so the next call pings and renews.
	got, err := getOrRenewClient(context.Background(), cfg, name)
	require.NoError(t, err)
	require.Same(t, replacement, got)
	require.Equal(t, int32(1), created.Load())
}
