package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func widestLine(s string) int {
	w := 0
	for _, l := range strings.Split(ansi.Strip(s), "\n") {
		w = max(w, ansi.StringWidth(l))
	}
	return w
}

// Trên màn hình rộng, output tool (ở đây là bash) dùng hết khung chat thay vì
// bị bẻ ở 120 cột và để trống nửa bên phải.
func TestToolOutputUsesFullChatWidth(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "b", Name: "bash", Input: `{"command":"cat wide.txt"}`, Finished: true}
	result := &message.ToolResult{ToolCallID: "b", Content: strings.Repeat("x", 180)}

	item := NewBashToolMessageItem(&sty, tc, result, false, "")
	require.Greater(t, widestLine(item.RawRender(220)), maxTextWidth, "tool output must not be capped at the prose width")
}

// Đoạn văn của model vẫn giữ trần đọc được, kể cả khi khung chat rất rộng.
func TestAssistantProseStaysCapped(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	msg := assistantWith(message.TextContent{Text: strings.Repeat("word ", 80)}, message.Finish{Reason: message.FinishReasonEndTurn, Time: 1})

	item, ok := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	require.True(t, ok)
	require.LessOrEqual(t, widestLine(item.RawRender(220)), maxTextWidth+MessageLeftPaddingTotal)
}
