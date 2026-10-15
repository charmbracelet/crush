package model

import (
	"context"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// modelRestore là một lần khôi phục model của session vừa mở.
type modelRestore struct {
	sel       config.SelectedModel
	needSmall bool
}

// modelRestoredMsg báo một lần khôi phục đã ghi xong cấu hình (err rỗng)
// hoặc lỗi.
type modelRestoredMsg struct {
	req modelRestore
	err error
}

// modelRestorer cho các lần khôi phục model chạy lần lượt, lần sau thắng.
// Việc ghi cấu hình (qua HTTP ở chế độ client/server, không có hạn giờ) nằm
// trong tea.Cmd chứ không trong Update, nên server chậm không làm TUI đứng.
// Cmd chạy song song, nên hai lần đổi session nhanh mà cho chạy cùng lúc thì
// lần ghi cũ có thể tới sau và đè model của session mới mở.
type modelRestorer struct {
	running *modelRestore // Lần đang ghi.
	queued  *modelRestore // Lần mới nhất chờ tới lượt.
}

// target trả model mới nhất đã yêu cầu mà chưa ghi xong, hoặc nil.
func (r *modelRestorer) target() *config.SelectedModel {
	switch {
	case r.queued != nil:
		return &r.queued.sel
	case r.running != nil:
		return &r.running.sel
	}
	return nil
}

// requestModelRestore xếp một lần khôi phục: chạy ngay nếu rảnh, không thì
// thay lần đang chờ (chỉ lần mới nhất có nghĩa).
func (m *UI) requestModelRestore(req modelRestore) tea.Cmd {
	if m.restorer.running != nil {
		m.restorer.queued = &req
		return nil
	}
	m.restorer.running = &req
	ws := m.com.Workspace
	return func() tea.Msg {
		err := ws.UpdatePreferredModel(config.ScopeWorkspace, config.SelectedModelTypeLarge, req.sel)
		if err == nil && req.needSmall {
			small := ws.GetDefaultSmallModel(req.sel.Provider)
			if serr := ws.UpdatePreferredModel(config.ScopeWorkspace, config.SelectedModelTypeSmall, small); serr != nil {
				slog.Error("Failed to set small model during session restore", "error", serr)
			}
		}
		return modelRestoredMsg{req: req, err: err}
	}
}

// handleModelRestored chạy trên luồng Update khi một lần ghi xong: có lần mới
// hơn đang chờ thì chạy nó (bỏ phần áp dụng của lần cũ), không thì đổi theme
// và dựng lại model của agent.
func (m *UI) handleModelRestored(msg modelRestoredMsg) tea.Cmd {
	m.restorer.running = nil
	if next := m.restorer.queued; next != nil {
		m.restorer.queued = nil
		return m.requestModelRestore(*next)
	}
	if msg.err != nil {
		slog.Error("Failed to restore model from session", "error", msg.err)
		return util.ReportWarn("Could not switch to this session's model: " + msg.err.Error())
	}
	m.applyThemeForProvider(msg.req.sel.Provider)
	ws := m.com.Workspace
	sel := msg.req.sel
	return m.updateAgentModelCmd(func() tea.Msg {
		if err := ws.UpdateAgentModel(context.TODO()); err != nil {
			return util.NewErrorMsg(err)
		}
		slog.Info("Restored model from session", "provider", sel.Provider, "model", sel.Model)
		return nil
	})
}

// appliesNextNote là phần nói thêm khi đổi model/effort lúc có run: lượt đang
// chạy giữ thiết lập cũ, thiết lập mới áp cho tin nhắn gửi sau.
func appliesNextNote(busy bool) string {
	if !busy {
		return ""
	}
	return " (applies to your next message; running tasks keep their model)"
}
