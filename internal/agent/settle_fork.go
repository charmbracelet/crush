package agent

import (
	"context"

	"github.com/charmbracelet/crush/internal/session"
)

// maxSessionDepth chặn vòng lặp khi dò session cha (sub-agent lồng nhau
// chỉ vài tầng; dữ liệu hỏng có vòng thì dừng ở đây).
const maxSessionDepth = 8

// BusyWithAncestors trả SessionBusyFunc coi một session là bận khi chính nó
// hoặc một session tổ tiên đang có run.
//
// Session của sub-agent (tool agent, agentic_fetch) chạy trên một agent
// riêng, nên IsSessionBusy của coordinator không thấy nó. Không có bước này,
// mở một session đang chạy sub-agent (TUI đọc cả session con để vẽ tool lồng
// nhau) sẽ ghi "Interrupted" lên tool call còn sống của sub-agent.
func BusyWithAncestors(ctx context.Context, sessions session.Service, busy SessionBusyFunc) SessionBusyFunc {
	if busy == nil || sessions == nil {
		return busy
	}
	return func(sessionID string) bool {
		id := sessionID
		for range maxSessionDepth {
			if busy(id) {
				return true
			}
			s, err := sessions.Get(ctx, id)
			if err != nil {
				// Không đọc được thì coi là bận: bỏ lượt settle an toàn hơn
				// settle nhầm.
				return true
			}
			if s.ParentSessionID == "" {
				return false
			}
			id = s.ParentSessionID
		}
		return true
	}
}
