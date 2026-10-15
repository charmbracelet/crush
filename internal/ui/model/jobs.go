package model

// Hộp thoại Jobs (alt+j hoặc lệnh "Jobs" trong bảng lệnh): các lệnh bash mà
// agent đã chạy ở workspace này. Hộp thoại tự nạp dữ liệu bằng tea.Cmd và tự
// giữ vòng làm tươi (xem dialog/jobs.go); phần ở đây chỉ mở nó và đưa vào
// bản chụp tiêu đề session từ danh sách UI đã nạp sẵn, không làm IO.

import (
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/ui/dialog"
)

// openJobsDialog mở hộp thoại Jobs, hoặc đưa nó lên trước nếu đã mở. Lệnh trả
// về nạp danh sách job lần đầu ngoài luồng Update.
func (m *UI) openJobsDialog() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.JobsID) {
		m.dialog.BringToFront(dialog.JobsID)
		return nil
	}
	jobs, cmd := dialog.NewJobs(m.com, m.sessionTitles())
	m.dialog.OpenDialog(jobs)
	return cmd
}

// sessionTitles ánh xạ session ID sang tiêu đề từ những gì UI đã nạp: danh
// sách session, session đang xem, và các sub-agent đang chạy (job của
// sub-agent thuộc session cha của nó). Session nằm ngoài đó hiện bằng vài ký
// tự đầu của ID.
func (m *UI) sessionTitles() map[string]string {
	titles := make(map[string]string, len(m.sessionList)+len(m.childRuns)+1)
	if m.session != nil {
		titles[m.session.ID] = m.session.Title
	}
	for _, s := range m.sessionList {
		titles[s.ID] = s.Title
	}
	for id, c := range m.childRuns {
		if title, ok := titles[c.parentID]; ok && c.parentID != "" {
			titles[id] = title
		}
	}
	return titles
}

// refreshJobsTitles đưa tiêu đề mới nhất cho hộp thoại Jobs đang mở, khi
// danh sách session vừa được nạp lại (session mới, đổi tên).
func (m *UI) refreshJobsTitles() {
	if d, ok := m.dialog.Dialog(dialog.JobsID).(*dialog.Jobs); ok {
		d.SetSessionTitles(m.sessionTitles())
	}
}
