package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// liveSession spins up a real in-memory MCP server exposing a single tool and
// returns a connected client session wrapped as a *ClientSession, mirroring
// what createSession produces in production. The returned context is the one
// bound to the session's cancel func, so a test can assert the session was
// actually closed (ctx cancelled) rather than merely dropped. Both sides are
// torn down via t.Cleanup.
func liveSession(t *testing.T, toolName string) (*ClientSession, context.Context) {
	t.Helper()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "srv"}, nil)
	mcp.AddTool(
		server,
		&mcp.Tool{Name: toolName, Description: "test tool"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
		},
	)
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	client := mcp.NewClient(&mcp.Implementation{Name: "crush-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)

	return &ClientSession{ClientSession: clientSession, cancel: cancel}, ctx
}

// TestUpdateState_ErrorClosesSessionAndClearsTools pins the primary fix: a
// StateError transition must (1) remove the session from the map, (2) actually
// close it so its child process/pipes are released, and (3) clear its tools
// from the registry. Before the fix updateState only did a bare
// sessions.Del(name): the session was leaked and its tools lingered, so
// crush_info kept reading "connected, N tools" while the LLM's tool list and
// the live session had diverged.
func TestUpdateState_ErrorClosesSessionAndClearsTools(t *testing.T) {
	const name = "test-error-cleanup"
	t.Cleanup(func() {
		sessions.Del(name)
		allTools.Del(name)
		states.Del(name)
	})

	sess, sessCtx := liveSession(t, "do_thing")
	sessions.Set(name, sess)
	allTools.Set(name, []*Tool{{Name: "do_thing"}})

	// Preconditions: tool registered and session live.
	_, ok := allTools.Get(name)
	require.True(t, ok)
	require.NoError(t, sessCtx.Err(), "session context must be live before the error")

	updateState(name, StateError, errors.New("stdio pipe broke"), nil, Counts{Tools: 1})

	// The dead session is removed from the map...
	_, ok = sessions.Get(name)
	require.False(t, ok, "errored session must be removed from the sessions map")

	// ...actually closed (its context is cancelled, not merely dropped)...
	require.ErrorIs(t, sessCtx.Err(), context.Canceled, "errored session must be closed, not just dropped from the map")

	// ...and its tools cleared from the registry the agent sends to the LLM.
	_, ok = allTools.Get(name)
	require.False(t, ok, "errored session's tools must be cleared from the registry")

	info, ok := GetState(name)
	require.True(t, ok)
	require.Equal(t, StateError, info.State)
}

// TestRegisterSessionTools_PopulatesRegistry pins that registerSessionTools —
// the single seam through which a (re)connected session's tools enter the
// registry — lists a live session's tools and writes them to allTools.
func TestRegisterSessionTools_PopulatesRegistry(t *testing.T) {
	const name = "test-register-tools"
	t.Cleanup(func() { allTools.Del(name) })

	sess, _ := liveSession(t, "send_message")
	t.Cleanup(func() { _ = sess.Close() })

	cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{name: {Type: config.MCPStdio}}})

	count, err := registerSessionTools(context.Background(), cfg, name, sess)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	got, ok := allTools.Get(name)
	require.True(t, ok, "a live session's tools must be registered")
	require.Len(t, got, 1)
	require.Equal(t, "send_message", got[0].Name)
}

// TestSessionErrorThenRenew_RestoresTools is the end-to-end regression for the
// reported bug: an MCP tool works, the stdio session drops mid-conversation,
// and afterwards every call returned "tool not found" forever. It walks the
// exact registry transitions the production code performs — initial connect
// registers tools, a StateError clears them (and closes the session), and the
// lazy renew re-registers them — so a regression in any leg (tools left stale
// on error, or tools never restored on renew) fails here.
func TestSessionErrorThenRenew_RestoresTools(t *testing.T) {
	const name = "test-error-then-renew"
	t.Cleanup(func() {
		if s, ok := sessions.Take(name); ok {
			_ = s.Close()
		}
		allTools.Del(name)
		states.Del(name)
	})

	cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{name: {Type: config.MCPStdio}}})

	// 1. Initial connect registers the tool (mirrors initClient).
	sess1, _ := liveSession(t, "send_message")
	sessions.Set(name, sess1)
	_, err := registerSessionTools(context.Background(), cfg, name, sess1)
	require.NoError(t, err)
	_, ok := allTools.Get(name)
	require.True(t, ok, "tool should be registered after the initial connect")

	// 2. The session drops mid-conversation -> StateError. Post-fix this clears
	//    the tools and closes the dead session.
	updateState(name, StateError, errors.New("pipe broke"), nil, Counts{Tools: 1})
	_, ok = allTools.Get(name)
	require.False(t, ok, "tools must be cleared when the session errors")
	_, ok = sessions.Get(name)
	require.False(t, ok, "errored session must be removed from the map")

	// 3. The lazy renew path creates a fresh session and MUST re-register the
	//    tools. The bug was that it never did: the LLM's tool list stayed empty
	//    and every subsequent call returned "tool not found".
	sess2, _ := liveSession(t, "send_message")
	count, err := registerSessionTools(context.Background(), cfg, name, sess2)
	require.NoError(t, err)
	sessions.Set(name, sess2)
	require.Equal(t, 1, count)

	got, ok := allTools.Get(name)
	require.True(t, ok, "tools must be restored after the session is renewed")
	require.Len(t, got, 1)
	require.Equal(t, "send_message", got[0].Name)
}
