package model

// tea.Cmd chạy ngoài luồng Update, lúc đó m.session có thể đã đổi hoặc là nil:
// mọi closure phải dùng session ID đã chụp lúc tạo, không đọc m.session (T9).

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// captureWS ghi lại session ID mà từng lệnh gọi workspace nhận được.
type captureWS struct {
	*countingWorkspace

	mu       sync.Mutex
	history  []string // ListSessionHistory.
	userMsgs []string // ListUserMessages; "*" là ListAllUserMessages.
	lastRead []string // FileTrackerLastReadTime.
}

func (w *captureWS) note(dst *[]string, id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	*dst = append(*dst, id)
}

func (w *captureWS) ListSessionHistory(_ context.Context, id string) ([]history.File, error) {
	w.note(&w.history, id)
	return nil, nil
}

func (w *captureWS) ListUserMessages(_ context.Context, id string) ([]message.Message, error) {
	w.note(&w.userMsgs, id)
	return nil, nil
}

func (w *captureWS) ListAllUserMessages(context.Context) ([]message.Message, error) {
	w.note(&w.userMsgs, "*")
	return nil, nil
}

func (w *captureWS) FileTrackerLastReadTime(_ context.Context, id, _ string) time.Time {
	w.note(&w.lastRead, id)
	return time.Time{}
}

func newCaptureUI(t *testing.T) (*UI, *captureWS) {
	t.Helper()
	ws := &captureWS{countingWorkspace: &countingWorkspace{ready: true}}
	m := newBusyUI(ws.countingWorkspace)
	m.com = common.DefaultCommon(ws)
	m.status = NewStatus(m.com, nil)
	warmCaches(m, false)
	m.textarea.Focus()
	m.promptHistory.index = -1
	return m, ws
}

// Sự kiện tệp của s1 làm nạp lại tệp của s1, dù đến lúc Cmd chạy UI đã sang s2
// hay đã về màn mới (m.session = nil, trước đây là nil dereference).
func TestFileEventCommandUsesCapturedSession(t *testing.T) {
	t.Parallel()

	for name, after := range map[string]*session.Session{
		"switched to another session": {ID: "s2"},
		"session cleared":             nil,
	} {
		m, ws := newCaptureUI(t)
		cmd := m.handleFileEvent(history.File{SessionID: "s1"})
		require.NotNil(t, cmd)
		m.session = after // Update đã chạy tiếp trước khi Cmd được chạy.

		require.NotPanics(t, func() { cmd() }, name)
		require.Equal(t, []string{"s1"}, ws.history, name)
	}
}

// Lịch sử prompt nạp cho session đã chụp lúc tạo Cmd, không phải session đang
// hiện lúc Cmd chạy (hoặc "tất cả" khi đã về màn mới).
func TestPromptHistoryCommandUsesCapturedSession(t *testing.T) {
	t.Parallel()

	for name, after := range map[string]*session.Session{
		"switched": {ID: "s2"},
		"cleared":  nil,
	} {
		m, ws := newCaptureUI(t)
		cmd := m.loadPromptHistory()
		m.session = after
		cmd()
		require.Equal(t, []string{"s1"}, ws.userMsgs, name)
	}

	// Chụp lúc màn mới (không có session) thì vẫn là "tất cả", dù sau đó đã vào s2.
	m, ws := newCaptureUI(t)
	m.session = nil
	cmd := m.loadPromptHistory()
	m.session = &session.Session{ID: "s2"}
	cmd()
	require.Equal(t, []string{"*"}, ws.userMsgs)
}

// Kiểm tra "đã đọc chưa" của tệp @-mention dùng session lúc chọn tệp.
func TestInsertFileCompletionUsesCapturedSession(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "note.txt")
	require.NoError(t, os.WriteFile(file, []byte("hello"), 0o600))

	m, ws := newCaptureUI(t)
	m.textarea.SetValue("@x")
	m.completionsStartIndex = 0
	cmd := m.insertFileCompletion(file)
	m.session = &session.Session{ID: "s2"} // Đổi session trước khi Cmd chạy.
	leafMsgs(cmd)
	require.Equal(t, []string{"s1"}, ws.lastRead)
}

// Kết quả đọc tệp @-mention quay về luồng Update: ở đó mới ghi nhận "đã đọc" và
// thêm tệp đính kèm, và chỉ khi session vẫn là session đã chọn tệp.
func TestMentionedFileLandsOnlyInItsSession(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "note.txt")
	require.NoError(t, os.WriteFile(file, []byte("hello"), 0o600))
	pick := func(m *UI) mentionedFileMsg {
		m.textarea.SetValue("@x")
		m.completionsStartIndex = 0
		for _, msg := range leafMsgs(m.insertFileCompletion(file)) {
			if mf, ok := msg.(mentionedFileMsg); ok {
				return mf
			}
		}
		t.Fatal("no mentionedFileMsg")
		return mentionedFileMsg{}
	}

	m, _ := newCaptureUI(t)
	mf := pick(m)
	require.Equal(t, "s1", mf.sessionID)
	require.Empty(t, m.sessionFileReads, "the command must not touch UI state")
	require.Empty(t, attNames(m))

	m.Update(mf)
	require.Equal(t, []string{"note.txt"}, attNames(m))
	require.Equal(t, []string{mf.absPath}, m.sessionFileReads)

	// Đổi session trong lúc đọc: kết quả không gắn vào ô nhập của session khác.
	m, _ = newCaptureUI(t)
	mf = pick(m)
	switchTo(m, "s2")
	m.Update(mf)
	require.Empty(t, attNames(m))
	require.Empty(t, m.sessionFileReads)
}
