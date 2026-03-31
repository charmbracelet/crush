package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func runFetchTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params FetchParams) (fantasy.ToolResponse, error) {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	return tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  FetchToolName,
		Input: string(input),
	})
}

func TestFetchTool_ReturnsToolErrorForAutoModePolicyBlock(t *testing.T) {
	t.Parallel()

	permissions := &mockToolPermissionService{
		Broker: pubsub.NewBroker[permission.PermissionRequest](),
		err: permission.NewPermissionBlockedError(
			"This action was blocked by the Auto Mode safety policy.",
			"Reason: Fetch URL denied by policy.",
		),
	}
	tool := NewFetchTool(permissions, t.TempDir(), nil)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	resp, err := runFetchTool(t, tool, ctx, FetchParams{
		URL:    "https://example.com",
		Format: "text",
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "This action was blocked by the Auto Mode safety policy.")
	require.Contains(t, resp.Content, "Reason: Fetch URL denied by policy.")
}

func TestFetchTool_ReturnsFatalErrorForUserDeniedPermission(t *testing.T) {
	t.Parallel()

	permissions := &mockToolPermissionService{
		Broker:  pubsub.NewBroker[permission.PermissionRequest](),
		granted: false,
	}
	tool := NewFetchTool(permissions, t.TempDir(), nil)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	_, err := runFetchTool(t, tool, ctx, FetchParams{
		URL:    "https://example.com",
		Format: "text",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, permission.ErrorPermissionDenied)
}
