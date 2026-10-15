package agent

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// Tool call có arguments JSON hỏng (bị cắt) được thay "{}" trước khi gửi
// provider, kể cả khi messages dựng từ step result trong bộ nhớ (không qua
// DB). Đây là chỗ gây HTTP 400 ở sự cố 15/10 ~14:38.
func TestSanitizeOutgoingToolCalls(t *testing.T) {
	t.Parallel()
	bad := `{"file_path":"a.txt","content":"xxx` // thiếu dấu đóng
	msgs := []fantasy.Message{
		fantasy.NewUserMessage("do it"),
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{
			fantasy.TextPart{Text: "writing"},
			fantasy.ToolCallPart{ToolCallID: "c1", ToolName: "write", Input: bad},
		}},
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{
			fantasy.ToolCallPart{ToolCallID: "c2", ToolName: "bash", Input: `{"command":"ls"}`},
		}},
	}
	out := sanitizeOutgoingToolCalls(msgs)

	c1, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](out[1].Content[1])
	require.True(t, ok)
	require.Equal(t, "{}", c1.Input, "malformed arguments are replaced")
	require.Equal(t, "c1", c1.ToolCallID, "id kept")
	require.Equal(t, "write", c1.ToolName)

	text, ok := fantasy.AsMessagePart[fantasy.TextPart](out[1].Content[0])
	require.True(t, ok)
	require.Equal(t, "writing", text.Text, "other parts untouched")

	c2, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](out[2].Content[0])
	require.True(t, ok)
	require.Equal(t, `{"command":"ls"}`, c2.Input, "valid arguments are left as-is")
}
