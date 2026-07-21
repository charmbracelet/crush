package mcp

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// livePromptSession spins up an in-memory MCP server that exposes a single
// prompt (which templates its "name" argument) and returns a connected
// *ClientSession, mirroring liveSession in lifecycle_test.go.
func livePromptSession(t *testing.T) *ClientSession {
	t.Helper()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "srv"}, nil)
	server.AddPrompt(
		&mcp.Prompt{
			Name:        "greet",
			Description: "Greet someone by name",
			Arguments: []*mcp.PromptArgument{
				{Name: "name", Description: "Who to greet", Required: true},
			},
		},
		func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			who := req.Params.Arguments["name"]
			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: "Hello, " + who + "!"}},
				},
			}, nil
		},
	)
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	client := mcp.NewClient(&mcp.Implementation{Name: "crush-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)

	return &ClientSession{ClientSession: clientSession, cancel: cancel}
}

// TestGetPrompts_ReturnsServerPrompts exercises the happy path for the
// internal getPrompts helper (the slice of the prompts pipeline that the
// robustness fix changes): a server that advertises prompts capability and
// answers prompts/list returns its prompts.
func TestGetPrompts_ReturnsServerPrompts(t *testing.T) {
	const name = "test-list-prompts"
	t.Cleanup(func() {
		sessions.Del(name)
		allPrompts.Del(name)
		states.Del(name)
	})

	sess := livePromptSession(t)
	t.Cleanup(func() { _ = sess.Close() })

	prompts, err := getPrompts(context.Background(), sess)
	require.NoError(t, err)
	require.Len(t, prompts, 1)
	require.Equal(t, "greet", prompts[0].Name)
	require.Equal(t, "Greet someone by name", prompts[0].Description)
	require.Len(t, prompts[0].Arguments, 1)
	require.Equal(t, "name", prompts[0].Arguments[0].Name)
	require.True(t, prompts[0].Arguments[0].Required)
}

func TestGetPromptMessages_RendersArguments(t *testing.T) {
	const name = "test-get-prompt"
	t.Cleanup(func() {
		sessions.Del(name)
		allPrompts.Del(name)
		states.Del(name)
	})

	sess := livePromptSession(t)
	t.Cleanup(func() { _ = sess.Close() })
	sessions.Set(name, sess)

	cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{name: {Type: config.MCPStdio}}})

	messages, err := GetPromptMessages(context.Background(), cfg, name, "greet", map[string]string{"name": "Ada"})
	require.NoError(t, err)
	require.Equal(t, []string{"Hello, Ada!"}, messages)
}

// TestGetPrompts_MethodNotFoundReturnsEmpty pins the robustness fix: some
// MCP servers advertise the prompts capability in their initialize result
// but reject prompts/list with JSON-RPC "Method not found" at call time
// (partial implementations, proxies, or older SDK versions). The resources
// path already degrades this to "no resources" (see getResources); prompts
// must do the same rather than surfacing a hard error that blocks the agent
// tool. We add a prompt so the SDK advertises the capability (otherwise
// the Capabilities.Prompts == nil guard short-circuits before the call),
// then install receiving middleware that returns MethodNotFound for
// prompts/list — simulating a server that claims the capability but won't
// serve the list.
func TestGetPrompts_MethodNotFoundReturnsEmpty(t *testing.T) {
	const name = "test-prompts-method-not-found"
	t.Cleanup(func() {
		sessions.Del(name)
		allPrompts.Del(name)
		states.Del(name)
	})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "srv"}, nil)
	// Add a prompt so the server advertises prompts capability in its
	// initialize result. Without this, Capabilities.Prompts is nil and
	// getPrompts returns early without ever calling ListPrompts.
	server.AddPrompt(
		&mcp.Prompt{Name: "stub"},
		func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{}, nil
		},
	)
	// Intercept prompts/list and return MethodNotFound, the exact shape a
	// broken/partial server produces. Other methods pass through.
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "prompts/list" {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "prompts not supported"}
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

	// Sanity: capability is advertised, so the fix's code path runs.
	require.NotNil(t, sess.InitializeResult().Capabilities.Prompts,
		"test setup must advertise prompts capability or the fix path is never reached")

	// Direct call to getPrompts — must NOT bubble the RPC error.
	prompts, err := getPrompts(context.Background(), sess)
	require.NoError(t, err, "Method-not-found from prompts/list must degrade to empty, not a hard error")
	require.Empty(t, prompts, "Method-not-found must yield no prompts")
}

// TestGetPromptMessages_IncludesAllTextContent pins the data-loss fix:
// GetPromptMessages used to skip any message whose role was not "user",
// silently dropping assistant-role text that a multi-turn prompt template
// deliberately injects (the MCP spec allows prompts to seed both user and
// assistant turns). The fix extracts text from TextContent regardless of
// role so the agent sees the full template.
func TestGetPromptMessages_IncludesAllTextContent(t *testing.T) {
	const name = "test-prompt-multi-role"
	t.Cleanup(func() {
		sessions.Del(name)
		allPrompts.Del(name)
		states.Del(name)
	})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "srv"}, nil)
	server.AddPrompt(
		&mcp.Prompt{Name: "conversation"},
		func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{
					{Role: "user", Content: &mcp.TextContent{Text: "Here is the style guide."}},
					{Role: "assistant", Content: &mcp.TextContent{Text: "Understood, I will follow it."}},
					{Role: "user", Content: &mcp.TextContent{Text: "Great."}},
				},
			}, nil
		},
	)
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	client := mcp.NewClient(&mcp.Implementation{Name: "crush-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	sess := &ClientSession{ClientSession: clientSession, cancel: cancel}
	t.Cleanup(func() { _ = sess.Close() })
	sessions.Set(name, sess)

	cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{name: {Type: config.MCPStdio}}})

	got, err := GetPromptMessages(context.Background(), cfg, name, "conversation", nil)
	require.NoError(t, err)
	// All three text messages must be present (previously only the two
	// user-role messages survived, and the assistant turn was silently
	// dropped — losing prompt-authored context).
	require.Equal(t, []string{
		"Here is the style guide.",
		"Understood, I will follow it.",
		"Great.",
	}, got)
}

// TestGetPromptMessages_SkipsNonTextContent pins that non-text content
// (image, audio, embedded resources) does not panic and is skipped without
// silently swallowing the whole message: any TextContent in the same
// message still survives, matching how read_mcp_resource handles mixed
// content. A prompt that returns ONLY non-text content yields no messages
// rather than a garbled or empty-text entry.
func TestGetPromptMessages_SkipsNonTextContent(t *testing.T) {
	const name = "test-prompt-non-text"
	t.Cleanup(func() {
		sessions.Del(name)
		allPrompts.Del(name)
		states.Del(name)
	})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "srv"}, nil)
	server.AddPrompt(
		&mcp.Prompt{Name: "mixed"},
		func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Messages: []*mcp.PromptMessage{
					// Text content survives.
					{Role: "user", Content: &mcp.TextContent{Text: "look at this"}},
					// Image-only message is skipped (no text to extract).
					{Role: "user", Content: &mcp.ImageContent{Data: []byte("fakepng"), MIMEType: "image/png"}},
				},
			}, nil
		},
	)
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	client := mcp.NewClient(&mcp.Implementation{Name: "crush-test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	sess := &ClientSession{ClientSession: clientSession, cancel: cancel}
	t.Cleanup(func() { _ = sess.Close() })
	sessions.Set(name, sess)

	cfg := config.NewTestStore(&config.Config{MCP: config.MCPs{name: {Type: config.MCPStdio}}})

	got, err := GetPromptMessages(context.Background(), cfg, name, "mixed", nil)
	require.NoError(t, err)
	// Only the text message survives; the image message is skipped cleanly.
	require.Equal(t, []string{"look at this"}, got)
}
