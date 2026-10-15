package agent

import (
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

// Tool ghi/sửa tệp hay chạy lệnh: arguments JSON hỏng KHÔNG được tự vá rồi
// chạy (sẽ ghi nội dung cụt mà không báo). repairToolCall trả nil → fantasy
// đánh Invalid → model nhận lỗi và gọi lại cho đúng.
func TestRepairToolCall_NeverGuessesMutatingToolArgs(t *testing.T) {
	t.Parallel()
	cut := `{"file_path":"a.txt","content":"nua chung` // chuỗi chưa đóng, repair được thành hợp lệ nhưng cụt
	for _, name := range []string{tools.WriteToolName, tools.EditToolName, tools.MultiEditToolName, tools.BashToolName, tools.DownloadToolName, tools.JobKillToolName} {
		got, err := repairToolCall(t.Context(), fantasy.ToolCallRepairOptions{
			OriginalToolCall: fantasy.ToolCallContent{ToolName: name, Input: cut},
		})
		require.NoError(t, err)
		require.Nilf(t, got, "%s: malformed args must not be auto-repaired into an execution", name)
	}
}

// Tool chỉ-đọc vẫn được vá JSON (tiện khi model thừa dấu phẩy); nội dung
// không bị đoán thêm.
func TestRepairToolCall_RepairsReadOnlyTools(t *testing.T) {
	t.Parallel()
	got, err := repairToolCall(t.Context(), fantasy.ToolCallRepairOptions{
		OriginalToolCall: fantasy.ToolCallContent{ToolName: tools.GrepToolName, Input: `{"pattern":"x",}`},
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.JSONEq(t, `{"pattern":"x"}`, got.Input)

	// JSON đã hợp lệ thì không đổi gì (trả nil để fantasy giữ nguyên).
	got, err = repairToolCall(t.Context(), fantasy.ToolCallRepairOptions{
		OriginalToolCall: fantasy.ToolCallContent{ToolName: tools.GrepToolName, Input: `{"pattern":"x"}`},
	})
	require.NoError(t, err)
	require.Nil(t, got)
}
