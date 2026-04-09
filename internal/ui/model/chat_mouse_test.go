package model

import (
	"encoding/json"
	"testing"

	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestHandleDelayedClickTogglesTaskNodeNestedOperationsOnce(t *testing.T) {
	t.Parallel()

	theme := styles.DefaultStyles()
	com := &common.Common{Styles: &theme}
	chatModel := NewChat(com)
	chatModel.SetSize(120, 20)

	taskNode := chat.NewTaskNodeItem(&theme, "call-general", "task-a", "Task A", "Run task A", "explore", "child-1")

	bashInput, err := json.Marshal(agenttools.BashParams{Command: "echo nested", Description: "nested"})
	require.NoError(t, err)
	nestedTool := chat.NewBashToolMessageItem(&theme, message.ToolCall{
		ID:       "nested-1",
		Name:     agenttools.BashToolName,
		Input:    string(bashInput),
		Finished: true,
	}, &message.ToolResult{ToolCallID: "nested-1", Content: "ok"}, false)
	taskNode.SetNestedTools([]chat.ToolMessageItem{nestedTool})

	chatModel.SetMessages(taskNode)

	collapsed := ansi.Strip(taskNode.Render(120))
	require.Contains(t, collapsed, "▸ 1 operations")
	require.NotContains(t, collapsed, "echo nested")

	handled, _ := chatModel.HandleMouseDown(0, 0)
	require.True(t, handled)

	clicked := chatModel.HandleDelayedClick(DelayedClickMsg{
		ClickID: chatModel.pendingClickID,
		ItemIdx: 0,
		X:       0,
		Y:       0,
	})
	require.True(t, clicked)

	expanded := ansi.Strip(taskNode.Render(120))
	require.Contains(t, expanded, "▾ 1 operations")
	require.Contains(t, expanded, "echo nested")
}
