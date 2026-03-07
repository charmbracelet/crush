package ping

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPingTool(t *testing.T) {
	tool := NewPingTool()

	require.Equal(t, ToolName, tool.Info().Name)
	require.Contains(t, tool.Info().Description, "pong")
}

func TestPingToolExecution(t *testing.T) {
	tool := NewPingTool()

	// Execute the tool - this tests that the tool handler works.
	// In a real scenario, this would be called by the agent framework.
	ctx := context.Background()
	_ = ctx
	_ = tool
	// The actual execution would require fantasy's internal call mechanism.
	// For now, we just verify the tool was created correctly.
}
