package chat

import (
	"encoding/json"
	"testing"

	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestTodosToolMessageItemRendersStructuredTaskUpdate(t *testing.T) {
	t.Parallel()

	metaJSON, err := json.Marshal(agenttools.TodosResponseMetadata{
		Action: "update",
		Todos: []session.Todo{{
			ID:         "task-1",
			Content:    "Implement typed memory",
			Status:     session.TodoStatusInProgress,
			ActiveForm: "Implementing typed memory",
			Progress:   55,
		}},
		Current: &session.Todo{
			ID:         "task-1",
			Content:    "Implement typed memory",
			Status:     session.TodoStatusInProgress,
			ActiveForm: "Implementing typed memory",
			Progress:   55,
		},
		AffectedID:  "task-1",
		Completed:   0,
		Total:       1,
		JustStarted: "Implementing typed memory",
	})
	require.NoError(t, err)

	paramsJSON, err := json.Marshal(agenttools.TodosParams{Action: "update", ID: "task-1", Todos: []agenttools.TodoItem{{Progress: 55}}})
	require.NoError(t, err)

	sty := styles.DefaultStyles()
	item := NewTodosToolMessageItem(&sty, message.ToolCall{
		ID:       "todo-tool",
		Name:     agenttools.TodosToolName,
		Input:    string(paramsJSON),
		Finished: true,
	}, &message.ToolResult{ToolCallID: "todo-tool", Content: "Updated tracked task.", Metadata: string(metaJSON)}, false)

	rendered := ansi.Strip(item.Render(120))
	require.Contains(t, rendered, "0/1 · 55%")
	require.Contains(t, rendered, "Implementing typed memory")
	require.Contains(t, rendered, "task-1")
}
