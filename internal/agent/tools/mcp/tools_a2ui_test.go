package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const testA2UIToolPayload = `{"version":"v0.9","updateComponents":{"surfaceId":"card","components":[` +
	`{"component":"Text","id":"t","text":"Recipe"}]}}`

// liveA2UIToolSession spins up an in-memory MCP server whose single tool
// returns the given content items, and returns a connected client session.
func liveA2UIToolSession(t *testing.T, toolName string, content []mcp.Content) *ClientSession {
	t.Helper()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "srv"}, nil)
	mcp.AddTool(
		server,
		&mcp.Tool{Name: toolName, Description: "test tool"},
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: content}, nil, nil
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
	return sess
}

// runToolWithSession calls the tool through the same extraction path RunTool
// uses, without needing a registered config client.
func runToolWithSession(t *testing.T, sess *ClientSession, toolName string) ToolResult {
	t.Helper()
	result, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      toolName,
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	return extractToolResult(result)
}

func TestRunTool_A2UISurfaceExtraction(t *testing.T) {
	t.Parallel()

	t.Run("embedded A2UI resource becomes a surface, not text", func(t *testing.T) {
		t.Parallel()

		sess := liveA2UIToolSession(t, "get_card", []mcp.Content{
			&mcp.TextContent{Text: "Your recipe card"},
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "a2ui://recipe-card",
					MIMEType: A2UIJSONMIMEType,
					Text:     testA2UIToolPayload,
				},
			},
		})
		res := runToolWithSession(t, sess, "get_card")

		require.Equal(t, "Your recipe card", res.Content)
		require.Len(t, res.Surfaces, 1)
		require.Equal(t, testA2UIToolPayload, res.Surfaces[0].Payload)
		require.Equal(t, "a2ui://recipe-card", res.Surfaces[0].URI)
		require.True(t, res.Surfaces[0].AssistantVisible, "no audience annotation means visible to both")
		require.NotContains(t, res.Content, "updateComponents")
	})

	t.Run("legacy MIME spelling is recognized", func(t *testing.T) {
		t.Parallel()

		sess := liveA2UIToolSession(t, "get_card", []mcp.Content{
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "a2ui://form",
					MIMEType: A2UILegacyMIMEType,
					Text:     testA2UIToolPayload,
				},
			},
		})
		res := runToolWithSession(t, sess, "get_card")
		require.Len(t, res.Surfaces, 1)
	})

	t.Run("user-only audience hides the payload from the model", func(t *testing.T) {
		t.Parallel()

		sess := liveA2UIToolSession(t, "get_card", []mcp.Content{
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "a2ui://card",
					MIMEType: A2UIJSONMIMEType,
					Text:     testA2UIToolPayload,
				},
				Annotations: &mcp.Annotations{Audience: []mcp.Role{"user"}},
			},
		})
		res := runToolWithSession(t, sess, "get_card")
		require.Len(t, res.Surfaces, 1)
		require.False(t, res.Surfaces[0].AssistantVisible)
		require.True(t, res.Surfaces[0].RenderForUser, "user audience still renders")
	})

	t.Run("assistant-only audience is visible to the model but not rendered", func(t *testing.T) {
		sess := liveA2UIToolSession(t, "get_card", []mcp.Content{
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "a2ui://card",
					MIMEType: A2UIJSONMIMEType,
					Text:     testA2UIToolPayload,
				},
				Annotations: &mcp.Annotations{Audience: []mcp.Role{"assistant"}},
			},
		})
		res := runToolWithSession(t, sess, "get_card")
		require.Len(t, res.Surfaces, 1)
		require.True(t, res.Surfaces[0].AssistantVisible)
		require.False(t, res.Surfaces[0].RenderForUser, "assistant-only audience must not render")
	})

	t.Run("blob-delivered surface is normalized to text", func(t *testing.T) {
		t.Parallel()

		sess := liveA2UIToolSession(t, "get_card", []mcp.Content{
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "a2ui://card",
					MIMEType: A2UIJSONMIMEType,
					Blob:     []byte(testA2UIToolPayload),
				},
			},
		})
		res := runToolWithSession(t, sess, "get_card")
		require.Len(t, res.Surfaces, 1)
		require.Equal(t, testA2UIToolPayload, res.Surfaces[0].Payload)
	})

	t.Run("non-A2UI embedded resource stays in the text", func(t *testing.T) {
		t.Parallel()

		sess := liveA2UIToolSession(t, "get_doc", []mcp.Content{
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "file:///doc.md",
					MIMEType: "text/markdown",
					Text:     "# hello",
				},
			},
		})
		res := runToolWithSession(t, sess, "get_doc")
		require.Empty(t, res.Surfaces)
		require.NotEmpty(t, res.Content)
	})
}

func TestIsA2UIMIMEType(t *testing.T) {
	t.Parallel()
	require.True(t, IsA2UIMIMEType("application/a2ui+json"))
	require.True(t, IsA2UIMIMEType("application/json+a2ui"))
	require.False(t, IsA2UIMIMEType("application/json"))
	require.False(t, IsA2UIMIMEType("text/plain"))
}
