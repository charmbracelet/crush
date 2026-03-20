package mcp

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMCPSession_CancelOnClose(t *testing.T) {
	defer goleak.VerifyNone(t)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()

	ctx, cancel := context.WithCancel(context.Background())

	client := mcp.NewClient(&mcp.Implementation{Name: "crush-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)

	sess := &ClientSession{clientSession, cancel}

	// Verify the context is not cancelled before close.
	require.NoError(t, ctx.Err())

	err = sess.Close()
	require.NoError(t, err)

	// After Close, the context must be cancelled.
	require.ErrorIs(t, ctx.Err(), context.Canceled)
}

func TestInitClient_PopulatesResources(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreAnyFunction("net/http.(*http2Transport).newClientConn"),
		goleak.IgnoreAnyFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreAnyFunction("gopkg.in/natefinch/lumberjack%2ev2.(*Logger).millRun"),
	)

	const name = "test-resources"

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	server.AddResource(
		&mcp.Resource{URI: "file:///readme.md", Name: "readme"},
		func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: "file:///readme.md"}},
			}, nil
		},
	)
	server.AddResource(
		&mcp.Resource{URI: "file:///license", Name: "license"},
		func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{URI: "file:///license"}},
			}, nil
		},
	)

	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "crush-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	session := &ClientSession{clientSession, cancel}

	cfg, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)

	// Clean up any prior state for this name.
	t.Cleanup(func() {
		allTools.Del(name)
		allPrompts.Del(name)
		allResources.Del(name)
		sessions.Del(name)
		states.Del(name)
	})

	toolCount := updateTools(cfg, name, nil)
	updatePrompts(name, nil)
	resourceCount := updateResources(name, nil)
	require.Equal(t, 0, toolCount)
	require.Equal(t, 0, resourceCount)

	// Simulate what initClient does after creating a session.
	tools, err := getTools(ctx, session)
	require.NoError(t, err)

	prompts, err := getPrompts(ctx, session)
	require.NoError(t, err)

	resources, err := getResources(ctx, session)
	require.NoError(t, err)
	require.Len(t, resources, 2)

	toolCount = updateTools(cfg, name, tools)
	updatePrompts(name, prompts)
	resourceCount = updateResources(name, resources)
	sessions.Set(name, session)

	updateState(name, StateConnected, nil, session, Counts{
		Tools:     toolCount,
		Prompts:   len(prompts),
		Resources: resourceCount,
	})

	// Verify resources are stored and counts are correct.
	storedResources, ok := allResources.Get(name)
	require.True(t, ok)
	require.Len(t, storedResources, 2)

	state, ok := states.Get(name)
	require.True(t, ok)
	require.Equal(t, StateConnected, state.State)
	require.Equal(t, 2, state.Counts.Resources)
}
