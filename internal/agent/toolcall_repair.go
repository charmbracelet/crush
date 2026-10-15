package agent

import (
	"context"
	"encoding/json"

	"charm.land/fantasy"
	"charm.land/fantasy/jsonrepair"

	"github.com/charmbracelet/crush/internal/agent/tools"
)

// Khi không cấu hình RepairToolCall, fantasy tự `jsonrepair.RepairJSON` cho
// tool call có JSON hỏng. Với arguments bị CẮT (vd `{"file_path":"a",
// "content":"nửa chừng`), repair có thể vá thành JSON hợp lệ nhưng NỘI DUNG
// ĐÃ CỤT, rồi tool chạy luôn — ghi/sửa tệp với nội dung thiếu, không báo gì.
//
// mutatingTools là các tool gây tác động (ghi/sửa tệp, chạy lệnh, tải, dừng
// job): với chúng KHÔNG bao giờ chạy tham số do máy đoán. Tool call JSON hỏng
// của chúng để fantasy đánh Invalid (model nhận lỗi và tự gọi lại cho đúng).
var mutatingTools = map[string]bool{
	tools.BashToolName:      true,
	tools.WriteToolName:     true,
	tools.EditToolName:      true,
	tools.MultiEditToolName: true,
	tools.DownloadToolName:  true,
	tools.JobKillToolName:   true,
}

// repairToolCall là RepairToolCallFunction cho agent chính. Tool tác động thì
// không vá (trả nil → fantasy để Invalid). Tool chỉ-đọc thì vá JSON như mặc
// định của fantasy (tiện khi model lỡ thừa dấu phẩy), nhưng vẫn phải khớp
// schema mới được chạy (fantasy validate lại sau repair).
func repairToolCall(_ context.Context, opts fantasy.ToolCallRepairOptions) (*fantasy.ToolCallContent, error) {
	tc := opts.OriginalToolCall
	if mutatingTools[tc.ToolName] {
		return nil, nil
	}
	if json.Valid([]byte(tc.Input)) {
		return nil, nil // đã hợp lệ: để fantasy validate schema, không đụng
	}
	repaired, err := jsonrepair.RepairJSON(tc.Input)
	if err != nil || repaired == tc.Input {
		return nil, nil
	}
	tc.Input = repaired
	return &tc, nil
}
