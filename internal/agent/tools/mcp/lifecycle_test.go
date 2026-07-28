package mcp

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
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

// TestUpdateState_ErrorFromStaleSessionPreservesHealthyReplacement pins the
// audit fix: a StateError reported against a session that is NO LONGER the
// registered one (a renewal already replaced it) must not tear down the
// healthy replacement or its registrations. Before the fix updateState closed
// whatever session was in the map, so a stale error transition — e.g. a slow
// ping timing out after another path had already renewed — killed the fresh
// session and wiped its tools.
func TestUpdateState_ErrorFromStaleSessionPreservesHealthyReplacement(t *testing.T) {
	const name = "test-stale-error"
	t.Cleanup(func() {
		sessions.Del(name)
		allTools.Del(name)
		allPrompts.Del(name)
		states.Del(name)
	})

	stale, staleCtx := liveSession(t, "old_tool")
	fresh, freshCtx := liveSession(t, "new_tool")

	// The registry holds the fresh session and its registrations.
	sessions.Set(name, fresh)
	allTools.Set(name, []*Tool{{Name: "new_tool"}})
	allPrompts.Set(name, []*Prompt{{Name: "new_prompt"}})

	// A stale error arrives for the OLD session.
	updateState(name, StateError, errors.New("ping timeout"), stale, Counts{})

	// The fresh session must still be registered and open.
	got, ok := sessions.Get(name)
	require.True(t, ok, "healthy replacement session was removed")
	require.Same(t, fresh, got)
	require.NoError(t, freshCtx.Err(), "healthy replacement session was closed")
	_, ok = allTools.Get(name)
	require.True(t, ok, "healthy replacement's tools were cleared")
	_, ok = allPrompts.Get(name)
	require.True(t, ok, "healthy replacement's prompts were cleared")

	// The stale session must have been closed.
	require.Error(t, staleCtx.Err(), "stale session was not closed")
}

// TestUpdateState_ErrorFromCurrentSessionClearsEverything pins the complement:
// when the erroring session IS the registered one, it is removed, closed, and
// both tools and prompts are cleared (prompts were previously left behind —
// dead servers kept their prompts in the commands menu).
func TestUpdateState_ErrorFromCurrentSessionClearsEverything(t *testing.T) {
	const name = "test-current-error"
	t.Cleanup(func() {
		sessions.Del(name)
		allTools.Del(name)
		allPrompts.Del(name)
		states.Del(name)
	})

	sess, sessCtx := liveSession(t, "a_tool")
	sessions.Set(name, sess)
	allTools.Set(name, []*Tool{{Name: "a_tool"}})
	allPrompts.Set(name, []*Prompt{{Name: "a_prompt"}})

	updateState(name, StateError, errors.New("boom"), sess, Counts{})

	_, ok := sessions.Get(name)
	require.False(t, ok, "erroring session should be removed")
	require.Error(t, sessCtx.Err(), "erroring session should be closed")
	_, ok = allTools.Get(name)
	require.False(t, ok, "tools should be cleared")
	_, ok = allPrompts.Get(name)
	require.False(t, ok, "prompts should be cleared (previously leaked)")

	st, _ := states.Get(name)
	require.Nil(t, st.Client, "error state must not publish a dead session")
}

// TestMaybeStdioErr_UnwrapsChannelTransport pins the diagnostics fix: every
// transport is wrapped in a channelTransport before Connect, so maybeStdioErr
// must unwrap it to find the CommandTransport. Before the fix the type
// assertion always failed and stdio startup errors reported bare EOF with
// none of the child's output attached.
func TestMaybeStdioErr_UnwrapsChannelTransport(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "echo boom-diagnostic >&2; exit 3")
	inner := &mcp.CommandTransport{Command: cmd}
	wrapped := &channelTransport{inner: inner, name: "t", gate: &atomic.Bool{}}

	got := maybeStdioErr(io.EOF, wrapped)
	require.ErrorContains(t, got, "boom-diagnostic",
		"stdio diagnostics should surface the child's output through the channelTransport wrapper")
}

// TestNameLock_ConcurrentFirstUseReturnsOneMutex pins that the per-name
// lifecycle lock is minted atomically: racing first-use callers must all
// receive the same mutex, or the serialization the whole lifecycle relies
// on silently degrades to per-caller locks.
func TestNameLock_ConcurrentFirstUseReturnsOneMutex(t *testing.T) {
	const name = "test-name-lock-race"
	t.Cleanup(func() { nameLocks.Del(name) })

	const n = 32
	locks := make([]*sync.Mutex, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			locks[i] = nameLock(name)
		}()
	}
	close(start)
	wg.Wait()

	for i := 1; i < n; i++ {
		require.Same(t, locks[0], locks[i], "all callers must share one lifecycle mutex")
	}
}

// TestUpdateState_ErrorClearsResources pins that both StateError teardown
// branches clear the resources registry alongside tools and prompts — a
// dead server must not keep advertising resources it can no longer serve.
func TestUpdateState_ErrorClearsResources(t *testing.T) {
	const name = "test-error-clears-resources"
	t.Cleanup(func() {
		sessions.Del(name)
		allTools.Del(name)
		allResources.Del(name)
		states.Del(name)
	})

	// Branch 1: the registered session itself errored (client argument).
	sess, _ := liveSession(t, "res_tool")
	sessions.Set(name, sess)
	allResources.Set(name, []*Resource{{Name: "doc"}})

	updateState(name, StateError, errors.New("listing broke"), sess, Counts{})
	_, ok := allResources.Get(name)
	require.False(t, ok, "current-session error must clear the resources registry")

	// Branch 2: no specific session (connect failed); anything registered
	// under the name is unusable.
	sess2, _ := liveSession(t, "res_tool2")
	sessions.Set(name, sess2)
	allResources.Set(name, []*Resource{{Name: "doc2"}})

	updateState(name, StateError, errors.New("connect broke"), nil, Counts{})
	_, ok = allResources.Get(name)
	require.False(t, ok, "no-session error must clear the resources registry")
}
