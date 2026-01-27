package tools

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// mockTool is a simple tool for testing the caching wrapper.
type mockTool struct {
	response fantasy.ToolResponse
	err      error
}

func (m *mockTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: "mock_tool"}
}

func (m *mockTool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return m.response, m.err
}

func (m *mockTool) ProviderOptions() fantasy.ProviderOptions {
	return fantasy.ProviderOptions{}
}

func (m *mockTool) SetProviderOptions(opts fantasy.ProviderOptions) {}

func setTestSession(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDContextKey, sessionID)
}

func TestCachedTool_SmallOutput(t *testing.T) {
	t.Parallel()

	// Create a mock tool with small output (under DefaultOutputLines).
	mock := &mockTool{
		response: fantasy.NewTextResponse("line1\nline2\nline3"),
	}

	wrapped := WrapWithCaching(mock)

	// Create context with session.
	ctx := setTestSession(context.Background(), "test-session")

	resp, err := wrapped.Run(ctx, fantasy.ToolCall{ID: "call-1", Name: "mock_tool"})
	require.NoError(t, err)

	// Small output should not be modified.
	require.Equal(t, "line1\nline2\nline3", resp.Content)

	// Nothing should be cached for small outputs.
	_, ok := GetOutputCache().Get("test-session", "call-1")
	require.False(t, ok)
}

func TestCachedTool_LargeOutput(t *testing.T) {
	t.Parallel()

	// Create a large output.
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "line"+string(rune('0'+i%10)))
	}
	content := strings.Join(lines, "\n")

	mock := &mockTool{
		response: fantasy.NewTextResponse(content),
	}

	wrapped := WrapWithCaching(mock)

	// Create context with session.
	ctx := setTestSession(context.Background(), "test-session-large")

	resp, err := wrapped.Run(ctx, fantasy.ToolCall{ID: "call-large", Name: "mock_tool"})
	require.NoError(t, err)

	// Large output should be truncated.
	require.Contains(t, resp.Content, "[Showing last")
	require.Contains(t, resp.Content, "tool_call_id=\"call-large\"")

	// Check that only last DefaultOutputLines lines are returned.
	respLines := strings.Split(resp.Content, "\n")
	// First line is the truncation notice, then a blank line.
	require.True(t, len(respLines) < 200)

	// Full output should be cached.
	cached, ok := GetOutputCache().Get("test-session-large", "call-large")
	require.True(t, ok)
	require.Equal(t, content, cached)
}

func TestCachedTool_NoSessionContext(t *testing.T) {
	t.Parallel()

	// Create a large output.
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "line"+string(rune('0'+i%10)))
	}
	content := strings.Join(lines, "\n")

	mock := &mockTool{
		response: fantasy.NewTextResponse(content),
	}

	wrapped := WrapWithCaching(mock)

	// Context without session - should return original response.
	resp, err := wrapped.Run(context.Background(), fantasy.ToolCall{ID: "call-3", Name: "mock_tool"})
	require.NoError(t, err)

	// Without session, output should not be modified.
	require.Equal(t, content, resp.Content)
	require.NotContains(t, resp.Content, "[Showing last")
}

func TestCachedTool_ErrorResponse(t *testing.T) {
	t.Parallel()

	// Create a large error response.
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "error line"+string(rune('0'+i%10)))
	}
	content := strings.Join(lines, "\n")

	mock := &mockTool{
		response: fantasy.NewTextErrorResponse(content),
	}

	wrapped := WrapWithCaching(mock)
	ctx := setTestSession(context.Background(), "test-session-error")

	resp, err := wrapped.Run(ctx, fantasy.ToolCall{ID: "call-4", Name: "mock_tool"})
	require.NoError(t, err)

	// Error responses should not be cached or truncated.
	require.True(t, resp.IsError)
	require.NotContains(t, resp.Content, "[Showing last")
}

func TestCachedTool_MediaResponse(t *testing.T) {
	t.Parallel()

	mock := &mockTool{
		response: fantasy.NewMediaResponse([]byte("fake image data"), "image/png"),
	}

	wrapped := WrapWithCaching(mock)
	ctx := setTestSession(context.Background(), "test-session-media")

	resp, err := wrapped.Run(ctx, fantasy.ToolCall{ID: "call-5", Name: "mock_tool"})
	require.NoError(t, err)

	// Media responses should not be modified.
	require.Equal(t, []byte("fake image data"), resp.Data)
	require.Equal(t, "image/png", resp.MediaType)
}

func TestWrapAllWithCaching(t *testing.T) {
	t.Parallel()

	mock1 := &mockTool{response: fantasy.NewTextResponse("output1")}
	mock2 := &mockTool{response: fantasy.NewTextResponse("output2")}

	wrapped := WrapAllWithCaching([]fantasy.AgentTool{mock1, mock2})

	require.Len(t, wrapped, 2)
	require.Equal(t, "mock_tool", wrapped[0].Info().Name)
	require.Equal(t, "mock_tool", wrapped[1].Info().Name)
}
