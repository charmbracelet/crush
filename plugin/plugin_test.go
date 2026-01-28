package plugin

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestRegisterTool(t *testing.T) {
	t.Cleanup(ResetTools)

	// Register a tool.
	factory := func(ctx context.Context, app *App) (Tool, error) {
		return fantasy.NewAgentTool(
			"test_tool",
			"A test tool",
			func(ctx context.Context, params struct{}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
				return fantasy.NewTextResponse("test"), nil
			},
		), nil
	}

	RegisterTool("test_tool", factory)

	// Verify registration.
	tools := RegisteredTools()
	require.Equal(t, []string{"test_tool"}, tools)

	// Get factory and create tool.
	f, ok := GetToolFactory("test_tool")
	require.True(t, ok)

	app := NewApp(WithWorkingDir("/tmp"))
	tool, err := f(context.Background(), app)
	require.NoError(t, err)
	require.Equal(t, "test_tool", tool.Info().Name)
}

func TestRegisterToolPanicOnDuplicate(t *testing.T) {
	t.Cleanup(ResetTools)

	factory := func(ctx context.Context, app *App) (Tool, error) {
		return nil, nil
	}

	RegisterTool("dup_tool", factory)

	require.Panics(t, func() {
		RegisterTool("dup_tool", factory)
	})
}

func TestRegistrationOrder(t *testing.T) {
	t.Cleanup(ResetTools)

	factory := func(ctx context.Context, app *App) (Tool, error) {
		return nil, nil
	}

	RegisterTool("tool_a", factory)
	RegisterTool("tool_b", factory)
	RegisterTool("tool_c", factory)

	tools := RegisteredTools()
	require.Equal(t, []string{"tool_a", "tool_b", "tool_c"}, tools)
}
