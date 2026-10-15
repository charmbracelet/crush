package model

// Hộp thoại Jobs nối vào UI: phím alt+j, mục "Jobs" trong bảng lệnh, tiêu đề
// session, vòng làm tươi khi có hộp thoại khác mở đè. Phần hành vi của chính
// hộp thoại (thứ tự, xác nhận dừng, khung xem output) test ở
// dialog/jobs_test.go.

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

var altJ = tea.KeyPressMsg{Code: 'j', Mod: tea.ModAlt}

// jobsWS là workspace giả có ba hàm job, đếm số lần gọi. Kế thừa
// countingWorkspace cho phần còn lại mà UI cần.
type jobsWS struct {
	*countingWorkspace

	mu           sync.Mutex
	jobs         []shell.JobInfo
	outputs      map[string]string
	listCount    int
	outputCount  int
	killedJobIDs []string
}

func (w *jobsWS) ListJobs(context.Context) ([]shell.JobInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.listCount++
	return slices.Clone(w.jobs), nil
}

func (w *jobsWS) JobOutput(_ context.Context, id string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.outputCount++
	return w.outputs[id], nil
}

func (w *jobsWS) KillJob(_ context.Context, id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.killedJobIDs = append(w.killedJobIDs, id)
	return nil
}

func (w *jobsWS) calls() (list, output, kill int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.listCount, w.outputCount, len(w.killedJobIDs)
}

// newJobsUI là UI đang xem s1 trong 3 session, nối với workspace giả.
func newJobsUI(jobs ...shell.JobInfo) (*UI, *jobsWS) {
	cw := &countingWorkspace{
		ready:    true,
		sessions: makeSessions(3),
		cfg:      &config.Config{Options: &config.Options{TUI: &config.TUIOptions{}}},
	}
	ws := &jobsWS{countingWorkspace: cw, jobs: jobs}
	m := newPanelUI(cw)
	m.com.Workspace = ws
	m.sessionList = cw.sessions
	return m, ws
}

func runningJob(id, sessionID, command string) shell.JobInfo {
	return shell.JobInfo{
		ID: id, SessionID: sessionID, Command: command,
		StartedAt: time.Now().Add(-time.Minute), Status: shell.JobRunning,
	}
}

// openCmdParts tách lệnh mở hộp thoại thành lệnh nạp danh sách và nhịp làm
// tươi (tea.Batch giữ thứ tự: nạp trước, nhịp sau). Chỉ chạy phần nạp; chạy
// nhịp thì phải chờ một giây thật.
func openCmdParts(t *testing.T, cmd tea.Cmd) (load, tick tea.Cmd) {
	t.Helper()
	require.NotNil(t, cmd)
	batch, ok := cmd().(tea.BatchMsg)
	require.True(t, ok, "opening the dialog returns a load and a heartbeat")
	require.Len(t, batch, 2)
	return batch[0], batch[1]
}

// quickMsgs chạy cây Cmd như Bubble Tea (mỗi lá một goroutine) và trả message
// của các lá xong trong wait. Lá còn chờ, như nhịp 1 giây, bị bỏ lại.
func quickMsgs(cmd tea.Cmd, wait time.Duration) []tea.Msg {
	if cmd == nil {
		return nil
	}
	results := make(chan tea.Msg, 16)
	var run func(c tea.Cmd)
	run = func(c tea.Cmd) {
		go func() {
			msg := c()
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, sub := range batch {
					run(sub)
				}
				return
			}
			results <- msg
		}()
	}
	run(cmd)

	var msgs []tea.Msg
	timeout := time.After(wait)
	for {
		select {
		case msg := <-results:
			msgs = append(msgs, msg)
		case <-timeout:
			return msgs
		}
	}
}

// targetedOnly giữ các message gửi đích danh cho một hộp thoại. Update còn
// trả cả lệnh làm tươi bộ nhớ đệm của chính UI, không liên quan tới Jobs.
func targetedOnly(msgs []tea.Msg) []tea.Msg {
	return slices.DeleteFunc(slices.Clone(msgs), func(msg tea.Msg) bool {
		_, ok := msg.(dialog.TargetedMsg)
		return !ok
	})
}

// jobsScreen vẽ các hộp thoại đang mở và trả chữ trên màn hình.
func jobsScreen(m *UI, w, h int) string {
	scr := uv.NewScreenBuffer(w, h)
	m.dialog.Draw(scr, scr.Bounds())
	rows := make([]string, h)
	for y := range h {
		var b strings.Builder
		for x := range w {
			if cell := scr.CellAt(x, y); cell != nil {
				b.WriteString(cell.Content)
			}
		}
		rows[y] = b.String()
	}
	return strings.Join(rows, "\n")
}

// --- Mở hộp thoại ---

// alt+j mở hộp thoại ở mọi tiêu điểm. Bấm phím không đụng workspace: danh
// sách nạp bằng Cmd (một vòng HTTP ở chế độ client/server).
func TestAltJOpensTheJobsDialog(t *testing.T) {
	t.Parallel()

	for _, focus := range []uiFocusState{uiFocusEditor, uiFocusMain} {
		m, ws := newJobsUI(runningJob("001", "s2", "npm run dev"))
		m.focus = focus

		cmd := m.handleKeyPressMsg(altJ)
		require.True(t, m.dialog.ContainsDialog(dialog.JobsID), "focus %d", focus)
		list, output, kill := ws.calls()
		require.Zero(t, list+output+kill, "focus %d: opening the dialog does no IO in Update", focus)

		load, _ := openCmdParts(t, cmd)
		msg := load()
		list, _, _ = ws.calls()
		require.Equal(t, 1, list, "focus %d: the command lists the jobs", focus)

		_, next := m.Update(msg)
		require.Nil(t, next, "focus %d", focus)
		text := jobsScreen(m, 120, 30)
		require.Contains(t, text, "Jobs")
		require.Contains(t, text, "npm run dev", "focus %d", focus)
	}
}

// Mở khi đã mở thì chỉ đưa lên trước, không nạp thêm.
func TestOpeningJobsTwiceDoesNotLoadTwice(t *testing.T) {
	t.Parallel()

	m, ws := newJobsUI()
	require.NotNil(t, m.openJobsDialog())
	require.Nil(t, m.openJobsDialog())
	require.Equal(t, 1, strings.Count(jobsScreen(m, 100, 24), "Jobs"), "one dialog")
	list, _, _ := ws.calls()
	require.Zero(t, list)
}

// Khi hộp thoại Jobs mở, Esc chỉ đóng hộp thoại (không huỷ run) và Enter
// không gửi bản nháp trong ô nhập.
func TestEscAndEnterInTheJobsDialogLeaveTheRunAndDraftAlone(t *testing.T) {
	t.Parallel()

	m, ws := newJobsUI(runningJob("001", "s1", "sleep 100"))
	ws.agentBusy = true
	m.textarea.SetValue("draft")

	m.handleKeyPressMsg(altJ)
	m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Equal(t, "draft", m.textarea.Value(), "enter must not send the draft")
	require.True(t, m.dialog.ContainsDialog(dialog.JobsID))

	m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, m.dialog.HasDialogs())
	require.False(t, m.isCanceling, "esc closed the dialog only")
	require.Zero(t, ws.cancelCalls)
	require.Equal(t, "draft", m.textarea.Value())
}

// Mục "Jobs" trong bảng lệnh (ctrl+p) mở cùng hộp thoại; gõ "jobs" lọc ra nó,
// và tên khác ("activity") cũng tìm được.
func TestJobsPaletteEntryOpensTheDialog(t *testing.T) {
	t.Parallel()

	for _, query := range []string{"jobs", "activity"} {
		m, _ := newJobsUI(runningJob("001", "s1", "sleep 100"))

		m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
		require.True(t, m.dialog.ContainsDialog(dialog.CommandsID), query)
		for _, r := range query {
			m.handleKeyPressMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
		cmd := m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

		require.True(t, m.dialog.ContainsDialog(dialog.JobsID), query)
		require.False(t, m.dialog.ContainsDialog(dialog.CommandsID), "the palette closes: %s", query)
		require.NotNil(t, cmd, "the dialog's load and heartbeat are returned to the runtime: %s", query)
	}
}

// --- Tiêu đề session ---

// Tiêu đề lấy từ danh sách session UI đã nạp; session lạ hiện vài ký tự đầu
// của ID; job của sub-agent đang chạy hiện tiêu đề session cha; danh sách
// session nạp lại (đổi tên) thì hộp thoại đang mở cập nhật theo.
func TestJobsDialogUsesTheLoadedSessionTitles(t *testing.T) {
	t.Parallel()

	m, _ := newJobsUI(
		runningJob("001", "s2", "go test ./..."),
		runningJob("002", "9f8e7d6c-aaaa-bbbb-cccc-000000000000", "make build"),
		runningJob("003", "msg1$$call1", "grep -r TODO ."),
	)
	m.childRuns = map[string]*childRun{"msg1$$call1": {id: "msg1$$call1", parentID: "s3"}}

	load, _ := openCmdParts(t, m.openJobsDialog())
	m.Update(load())
	text := jobsScreen(m, 140, 30)
	row := func(snippet string) string {
		for line := range strings.SplitSeq(text, "\n") {
			if strings.Contains(line, snippet) {
				return line
			}
		}
		t.Fatalf("no row with %q in:\n%s", snippet, text)
		return ""
	}
	require.Contains(t, row("go test"), "title 2", "a listed session shows its title")
	require.Contains(t, row("make build"), "9f8e7d6c", "an unknown session shows its short ID")
	require.NotContains(t, row("make build"), "9f8e7d6c-aaaa")
	require.Contains(t, row("grep -r"), "title 3", "a running sub-agent's job shows its parent's title")

	// Danh sách session nạp lại với tiêu đề mới.
	renamed := slices.Clone(m.sessionList)
	renamed[1] = session.Session{ID: "s2", Title: "Renamed session", CreatedAt: renamed[1].CreatedAt}
	m.Update(sessionListMsg{gen: m.activity.version, sessions: renamed})
	text = jobsScreen(m, 140, 30)
	require.Contains(t, row("go test"), "Renamed session")
	require.NotContains(t, text, "title 2")
}

// --- Vòng làm tươi trong UI ---

// Nhịp làm tươi tới được hộp thoại Jobs kể cả khi một hộp thoại khác (câu hỏi
// xin quyền) đang mở đè, và vòng lặp dừng khi hộp thoại Jobs đóng.
func TestJobsRefreshLoopRunsUnderAPermissionPromptAndStopsOnClose(t *testing.T) {
	t.Parallel()

	m, ws := newJobsUI(runningJob("001", "s1", "sleep 100"))
	load, tick := openCmdParts(t, m.openJobsDialog())
	m.Update(load())
	tickMsg := tick() // Chờ đúng một nhịp thật (1 giây).
	targeted, ok := tickMsg.(dialog.TargetedMsg)
	require.True(t, ok, "the heartbeat is addressed to the Jobs dialog")
	require.Equal(t, dialog.JobsID, targeted.TargetDialogID())

	// Một câu hỏi xin quyền bật lên đè hộp thoại Jobs.
	m.dialog.OpenDialogWithGrace(dialog.NewPermissions(m.com, permission.PermissionRequest{ID: "p", ToolCallID: "tc", ToolName: "bash"}))
	require.Equal(t, dialog.PermissionsID, m.dialog.DialogLast().ID())

	listBefore, _, _ := ws.calls()
	_, next := m.Update(tickMsg)
	require.NotNil(t, next, "the tick reached the Jobs dialog under the prompt: it re-arms the heartbeat")
	require.Equal(t, dialog.PermissionsID, m.dialog.DialogLast().ID(), "the prompt is still in front")
	msgs := targetedOnly(quickMsgs(next, 300*time.Millisecond))
	listAfter, _, _ := ws.calls()
	require.Equal(t, listBefore+1, listAfter, "the Jobs dialog reloaded its list while the prompt was in front")
	require.Len(t, msgs, 1, "the reload's result; the heartbeat is still waiting")

	// Đóng hết: nhịp tới sau đó không còn nơi nhận, vòng lặp dừng.
	m.dialog.CloseDialog(dialog.PermissionsID)
	m.handleKeyPressMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, m.dialog.HasDialogs())
	listBefore, _, _ = ws.calls()
	_, next = m.Update(tickMsg)
	require.Empty(t, targetedOnly(quickMsgs(next, 100*time.Millisecond)), "no dialog, no loop: nothing re-arms the heartbeat")
	listAfter, _, _ = ws.calls()
	require.Equal(t, listBefore, listAfter, "and no reload")
}

// --- Phím trong bản đồ phím ---

func TestJobsKeyBinding(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	reg := keyRegistry(&km)
	require.Same(t, &km.Jobs, reg["jobs"], "the action is named after the KeyMap field")
	require.Equal(t, []string{"alt+j"}, km.Jobs.Keys())
	require.Equal(t, "alt+j", km.Jobs.Help().Key)
	require.Equal(t, "jobs", km.Jobs.Help().Desc)
	require.True(t, km.Jobs.Enabled())

	var found bool
	for _, e := range KeyEntries(&km) {
		if e.Action == "jobs" {
			found = true
			require.Equal(t, []string{"alt+j"}, e.Keys)
			require.True(t, e.Enabled)
			require.False(t, e.DisplayOnly)
		}
	}
	require.True(t, found, "`agent-cli keys` lists it")

	// Không phím mặc định nào khác dùng alt+j, ở bất kỳ nhóm nào.
	for name, b := range reg {
		if name == "jobs" || !b.Enabled() {
			continue
		}
		require.NotContains(t, b.Keys(), "alt+j", "%s also binds alt+j", name)
	}

	// Đổi phím qua keybindings.json được, và đổi sang phím đang dùng thì cảnh
	// báo.
	warns := applyKeyOverrides(&km, map[string][]string{"jobs": {"alt+x"}})
	require.Empty(t, warns)
	require.Equal(t, []string{"alt+x"}, km.Jobs.Keys())
	km = DefaultKeyMap()
	warns = applyKeyOverrides(&km, map[string][]string{"jobs": {"alt+s"}})
	require.Len(t, warns, 1)
	require.Contains(t, warns[0], "sessions")
}

// Phím bị đổi trong keybindings.json thì phím mới mở được hộp thoại, phím mặc
// định thì không (alt+j rơi vào ô nhập như mọi phím khác).
func TestRebindingJobsKeyMovesTheShortcut(t *testing.T) {
	t.Parallel()

	m, _ := newJobsUI()
	m.keyMap.Jobs.SetKeys("alt+x")
	m.handleKeyPressMsg(altJ)
	require.False(t, m.dialog.ContainsDialog(dialog.JobsID))
	m.handleKeyPressMsg(tea.KeyPressMsg{Code: 'x', Mod: tea.ModAlt})
	require.True(t, m.dialog.ContainsDialog(dialog.JobsID))

	m, _ = newJobsUI()
	m.keyMap.Jobs.SetEnabled(false)
	m.handleKeyPressMsg(altJ)
	require.False(t, m.dialog.ContainsDialog(dialog.JobsID), "an empty key list disables the action")
}

// "ctrl+g more" liệt kê alt+j cùng alt+p, alt+e; thanh chân trang ngắn vẫn là
// bốn phím cốt lõi, không thêm alt+j.
func TestJobsKeyInHelp(t *testing.T) {
	t.Parallel()

	m, _ := newJobsUI()
	var full []string
	for _, row := range m.FullHelp() {
		for _, b := range row {
			full = append(full, b.Help().Key+" "+b.Help().Desc)
		}
	}
	require.Contains(t, full, "alt+j jobs")
	require.Contains(t, full, "alt+p provider")
	require.Contains(t, full, "alt+e effort")

	for _, b := range m.ShortHelp() {
		require.NotEqual(t, "alt+j", b.Help().Key, "the footer keeps the core keys")
	}
}
