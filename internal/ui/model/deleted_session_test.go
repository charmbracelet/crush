package model

// Dọn dẹp khi session biến mất: xoá session (prompt đang giữ, hoạt động), bản
// nháp của session không còn tồn tại, tệp tạm khi ghi hỏng và drafts.json chứa
// "null".

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

func deletedEvent(id string) pubsub.Event[session.Session] {
	return pubsub.Event[session.Session]{Type: pubsub.DeletedEvent, Payload: session.Session{ID: id}}
}

// Xoá session thì prompt đang giữ cho nó (không ai trả lời được nữa) và trạng
// thái hoạt động của nó cũng phải biến mất, kẻo bảng còn ▲ ma và Alt+S mở lại
// một câu hỏi vô chủ.
func TestDeletedSessionDropsHeldPromptsAndActivity(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.Update(permEvent("s2", "p2"))
	m.Update(questionEvent("s3", "q3"))
	require.Contains(t, m.heldPerms, "s2")
	require.Contains(t, m.heldQuestions, "s3")
	require.Equal(t, activityWaiting, m.activity.Get("s2"))
	require.Equal(t, activityWaiting, m.activity.Get("s3"))
	m.drafts = map[string]draft{"s2": {text: "unsent"}}

	m.Update(deletedEvent("s2"))
	require.NotContains(t, m.heldPerms, "s2")
	require.Equal(t, activityIdle, m.activity.Get("s2"))
	require.NotContains(t, m.drafts, "s2")
	require.Contains(t, m.heldQuestions, "s3", "other sessions keep their prompts")
	require.Equal(t, activityWaiting, m.activity.Get("s3"))

	m.Update(deletedEvent("s3"))
	require.NotContains(t, m.heldQuestions, "s3")
	require.Equal(t, activityIdle, m.activity.Get("s3"))
}

// Xoá chính session đang xem (rồi UI sang màn mới) cũng không để lại prompt bị
// "đỗ" của nó.
func TestDeletingTheShownSessionDropsItsParkedPrompt(t *testing.T) {
	t.Parallel()

	m, _ := newPromptUI(t)
	m.Update(permEvent("s1", "p1"))
	require.NotEmpty(t, openPermToolCall(m))

	m.Update(deletedEvent("s1"))
	require.Nil(t, m.session)
	require.NotContains(t, m.heldPerms, "s1")
	require.Equal(t, activityIdle, m.activity.Get("s1"))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// Bản nháp trên đĩa của session không còn tồn tại (xoá từ TUI khác hoặc CLI lúc
// TUI này không chạy) bị bỏ khi biết danh sách session, và lần ghi kế tiếp xoá
// luôn khỏi tệp. Bản nháp màn landing ("") thì giữ.
func TestLoadDraftsPrunesDeletedSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.json")
	writeFile(t, path, `{"":"landing","s1":"live","gone":"deleted elsewhere"}`)

	m, _ := newDraftUI(t)
	m.session = nil
	m.draftsPath = path
	m.sessionList = makeSessions(2) // Danh sách đã biết: s1, s2.
	m.loadDrafts()
	require.Contains(t, m.drafts, "s1")
	require.Contains(t, m.drafts, "")
	require.NotContains(t, m.drafts, "gone", "its session no longer exists")

	m.textarea.SetValue(m.drafts[""].text) // Như New() làm với bản nháp landing.
	require.NoError(t, m.SaveDrafts())
	require.JSONEq(t, `{"":"landing","s1":"live"}`, readDrafts(t, path))
}

// Lúc dựng UI chưa biết danh sách session: bản nháp mồ côi bị bỏ ở lần danh sách
// đầu tiên về, và đúng một lần (sau đó session mới tạo chưa kịp vào danh sách cũ
// không bị mất bản nháp).
func TestFirstSessionListPrunesStaleDrafts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.json")
	writeFile(t, path, `{"s2":"keep","gone":"drop"}`)

	m, ws := newDraftUI(t)
	ws.sessions = makeSessions(3)
	m.session = nil
	m.draftsPath = path
	m.loadDrafts()
	require.Contains(t, m.drafts, "gone", "the list is not known yet, so nothing can be called stale")

	refreshPanel(m)
	require.Contains(t, m.drafts, "s2")
	require.NotContains(t, m.drafts, "gone")

	m.drafts["fresh"] = draft{text: "typed in a session created after the first list"}
	refreshPanel(m)
	require.Contains(t, m.drafts, "fresh", "pruning happens once, not on every list")
}

// drafts.json chứa đúng chữ "null" giải mã thành map nil: lưu không được panic.
func TestSaveDraftsSurvivesNullFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.json")
	writeFile(t, path, "null")

	m, _ := newDraftUI(t)
	m.draftsPath = path
	m.loadDrafts()
	typeText(m, "after null")
	require.NotPanics(t, func() { require.NoError(t, m.SaveDrafts()) })
	require.JSONEq(t, `{"s1":"after null"}`, readDrafts(t, path))
}

// Ghi hỏng (hết chỗ, lỗi đĩa) không được để tệp tạm nằm lại. Thư mục đặt sẵn đúng
// tên tệp tạm làm WriteFile hỏng, thay cho một lần ghi chết giữa chừng.
func TestSaveDraftsRemovesTempFileWhenWriteFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.json")
	tmp := path + ".tmp." + strconv.Itoa(os.Getpid())
	require.NoError(t, os.Mkdir(tmp, 0o700))

	m, _ := newDraftUI(t)
	m.draftsPath = path
	typeText(m, "will not land")
	require.Error(t, m.SaveDrafts())
	require.NoDirExists(t, tmp)
	require.NoFileExists(t, tmp)
	require.NoFileExists(t, path, "a failed save leaves no half-written drafts file")
}
