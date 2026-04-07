package chat

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestRequestUserInputToolMessageItemRendersStructuredSummary(t *testing.T) {
	t.Parallel()

	params, err := json.Marshal(tools.RequestUserInputParams{
		Questions: []tools.RequestUserInputQuestion{
			{
				Header:   "实现方案",
				ID:       "approach",
				Question: "采用哪个方案？",
				Options: []tools.RequestUserInputOption{
					{Label: "方案A", Description: "desc"},
					{Label: "方案B", Description: "desc"},
				},
			},
		},
	})
	require.NoError(t, err)

	resultPayload, err := json.Marshal(tools.RequestUserInputResult{
		Status: "submitted",
		Answers: []tools.RequestUserInputAnswer{
			{
				QuestionID:  "approach",
				CustomInput: "用你推荐的方式修改",
			},
		},
	})
	require.NoError(t, err)

	theme := styles.DefaultStyles()
	item := NewRequestUserInputToolMessageItem(&theme, message.ToolCall{
		ID:       "tool-req",
		Name:     tools.RequestUserInputToolName,
		Input:    string(params),
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "tool-req",
		Content:    string(resultPayload),
	}, false)

	rendered := ansi.Strip(item.Render(100))
	require.Contains(t, rendered, "Request User Input")
	require.Contains(t, rendered, "Status: submitted")
	require.Contains(t, rendered, "Q: 采用哪个方案？")
	require.Contains(t, rendered, "A: Custom: 用你推荐的方式修改")
}
