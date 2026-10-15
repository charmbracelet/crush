package model

// Đổi session: Alt+. / Alt+, đi vòng theo bảng Sessions (T3), bảng ghim session
// đang bận ngoài 9 dòng đầu (T4) và việc báo session hiện tại cho workspace
// (T6).

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/workspace"
)

var (
	altDot   = tea.KeyPressMsg{Code: '.', Mod: tea.ModAlt}
	altComma = tea.KeyPressMsg{Code: ',', Mod: tea.ModAlt}
)

// reportWS ghi lại mọi lần SetCurrentSession, thứ tự như đã gọi.
type reportWS struct {
	*jumpWorkspace

	mu      sync.Mutex
	reports []string
}

func (w *reportWS) SetCurrentSession(_ context.Context, id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.reports = append(w.reports, id)
	return nil
}

func (w *reportWS) reported() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.reports...)
}

// newReportUI là newJumpUI (12 session, đang xem s1) nhưng ghi lại các lần báo
// session hiện tại.
func newReportUI() (*UI, *reportWS) {
	m, jw := newJumpUI()
	ws := &reportWS{jumpWorkspace: jw}
	m.com.Workspace = ws
	return m, ws
}

// leafMsgs chạy cây Cmd và trả message của từng lá, theo thứ tự.
func leafMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range b {
			out = append(out, leafMsgs(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// loadedMsg tìm loadSessionMsg trong các message lá của cmd.
func loadedMsg(t *testing.T, cmd tea.Cmd) loadSessionMsg {
	t.Helper()
	for _, msg := range leafMsgs(cmd) {
		if lm, ok := msg.(loadSessionMsg); ok {
			return lm
		}
	}
	t.Fatal("the command produced no loadSessionMsg")
	return loadSessionMsg{}
}

func landed(id string) loadSessionMsg {
	return loadSessionMsg{session: &session.Session{ID: id}}
}

// Alt+, từ session mới nhất phải nhảy tới dòng CUỐI của bảng Sessions (dòng 9),
// không phải session cũ nhất của cả lịch sử (12 session ở đây).
func TestCycleSessionStaysWithinPanel(t *testing.T) {
	t.Parallel()

	m, ws := newJumpUI()
	runLeaves(m.handleKeyPressMsg(altComma))
	require.Equal(t, []string{"s9"}, ws.loaded, "the panel shows s1..s9; s10..s12 are not in it")

	// Session ghim ở dòng thứ mười: vòng đi qua nó như một dòng của bảng.
	m, ws = newJumpUI()
	m.session = &session.Session{ID: "s12"}
	require.Len(t, m.panelSessions(), sessionsPanelMax+1)
	runLeaves(m.handleKeyPressMsg(altDot))
	require.Equal(t, []string{"s1"}, ws.loaded, "forward from the pinned row wraps to the first")

	m, ws = newJumpUI()
	m.session = &session.Session{ID: "s12"}
	runLeaves(m.handleKeyPressMsg(altComma))
	require.Equal(t, []string{"s9"}, ws.loaded, "backward from the pinned row is the ninth, not the eleventh")
}

// Bấm hai lần trước khi lần nạp đầu xong phải đi hai bước, không phải hai lần
// vào cùng một session.
func TestCycleSessionTwiceBeforeLoadAdvancesTwice(t *testing.T) {
	t.Parallel()

	m, ws := newJumpUI()
	first := m.handleKeyPressMsg(altDot)
	second := m.handleKeyPressMsg(altDot)
	runLeaves(first)
	runLeaves(second)
	require.Equal(t, []string{"s2", "s3"}, ws.loaded)

	// Khi cả hai đã hiện, đích chờ không còn: lần bấm kế đi từ session đang xem.
	m.Update(landed("s2"))
	m.Update(landed("s3"))
	runLeaves(m.handleKeyPressMsg(altDot))
	require.Equal(t, []string{"s2", "s3", "s4"}, ws.loaded)
}

// Không còn session nào khác thì nói ra, thay vì lặng im.
func TestCycleSessionWithNoOtherSessionSaysSo(t *testing.T) {
	t.Parallel()

	for name, n := range map[string]int{"lone session": 1, "empty list": 0} {
		ws := &jumpWorkspace{countingWorkspace: &countingWorkspace{ready: true, sessions: makeSessions(n)}}
		m := newPanelUI(ws.countingWorkspace)
		m.com.Workspace = ws
		m.sessionList = ws.sessions

		for _, k := range []tea.KeyPressMsg{altDot, altComma} {
			cmd := m.handleKeyPressMsg(k)
			require.NotNil(t, cmd, "%s %s", name, k)
			require.Equal(t, []string{"No other session"}, runPromptCmds(m, cmd), "%s %s", name, k)
		}
		require.Empty(t, ws.loaded, name)
	}
}

// Nạp session chưa xong thì chưa báo gì cho workspace; chuyển xong mới báo
// session mới.
func TestLoadSessionReportsOnlyAfterSwitch(t *testing.T) {
	t.Parallel()

	m, ws := newReportUI()
	cmd := m.loadSession("s2")
	lm := loadedMsg(t, cmd)
	require.Empty(t, ws.reported(), "a load that has not landed must not move the client's presence")

	_, next := m.Update(lm)
	runLeaves(next)
	require.Equal(t, []string{"s2"}, ws.reported())
	require.Equal(t, "s2", m.currentSessionID())
}

// Nạp hỏng: người dùng thấy lỗi, và workspace vẫn biết đúng session đang hiện
// (báo lại session cũ).
func TestFailedLoadShowsErrorAndKeepsReportOnShownSession(t *testing.T) {
	t.Parallel()

	m, ws := newReportUI()
	var infos []string
	for _, msg := range leafMsgs(m.loadSession("missing")) {
		_, next := m.Update(msg)
		infos = append(infos, runPromptCmds(m, next)...)
	}
	require.Contains(t, strings.Join(infos, "|"), "session not found", "the load error must reach the user")
	require.NotContains(t, ws.reported(), "missing", "the failed target is never reported")
	require.Equal(t, []string{"s1"}, ws.reported(), "the session still on screen is reported again")
	require.Equal(t, "s1", m.currentSessionID())
}

// ---- Bảng Sessions ghim session đang bận ngoài 9 dòng đầu ----

func panelRowsN(m *UI, maxItems int) []string {
	var rows []string
	for line := range strings.SplitSeq(ansi.Strip(m.sessionsInfo(60, maxItems, true)), "\n") {
		if line = strings.TrimRight(line, " "); strings.Contains(line, "title ") || strings.Contains(line, "more") {
			rows = append(rows, line)
		}
	}
	return rows
}

// eventRefresh đưa một event vào Update rồi chạy đợt làm tươi danh sách nó gây
// ra (bỏ qua khoảng nghỉ chống dồn), trả các dòng trạng thái đã phát.
func eventRefresh(m *UI, msg tea.Msg) []string {
	m.sessionListFetchedAt = time.Time{}
	_, cmd := m.Update(msg)
	return runPromptCmds(m, cmd)
}

func userMessageFor(id string) pubsub.Event[message.Message] {
	return pubsub.Event[message.Message]{
		Type:    pubsub.CreatedEvent,
		Payload: message.Message{ID: "m-" + id, SessionID: id, Role: message.User},
	}
}

// Session đang chạy nằm ngoài 9 dòng đầu vẫn phải hiện trong bảng và được dò
// trạng thái, giống session đang xem.
func TestSessionsPanelPinsRunningSessionBeyondNine(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{
		ready:         true,
		sessions:      makeSessions(12),
		sessionStatus: map[string]workspace.SessionStatus{"s11": {Busy: true}},
	}
	m := newPanelUI(ws)
	refreshPanel(m)
	require.Len(t, panelRows(m), 9, "nothing is running yet that the panel knows of")

	// Lần chạy bắt đầu bằng tin nhắn của người dùng.
	eventRefresh(m, userMessageFor("s11"))
	ws.statusCalls = 0
	refreshPanel(m)

	rows := panelRowsN(m, 10)
	require.Len(t, rows, 10)
	require.Equal(t, "· ● title 11 running", rows[9])
	require.Equal(t, 10, ws.statusCalls, "the nine shown plus the running one are probed")
	require.Len(t, m.panelSessions(), sessionsPanelMax+1)
	require.Equal(t, makeSessions(12), m.sessionList, "pinning must not overwrite the list")

	// Hết chạy: bỏ ghim.
	ws.sessionStatus = nil
	eventRefresh(m, pubsub.Event[notify.RunComplete]{Type: pubsub.CreatedEvent, Payload: notify.RunComplete{SessionID: "s11"}})
	require.Equal(t, activityDoneUnseen, m.activity.Get("s11"))
	refreshPanel(m)
	require.Contains(t, panelRowsN(m, 10), "· ✓ title 11 done", "an unseen result stays on the panel")
	eventRefresh(m, landed("s11"))
	eventRefresh(m, landed("s1"))
	refreshPanel(m)
	require.Len(t, panelRows(m), 9, "seen and idle again: back to the top nine")
}

// Session ngoài 9 dòng đầu đang chờ cấp quyền phải hiện ▲ và báo "needs your
// input" như mọi session trong bảng.
func TestSessionsPanelWaitingBeyondNineShowsAlert(t *testing.T) {
	t.Parallel()

	ws := &countingWorkspace{
		ready:         true,
		sessions:      makeSessions(12),
		sessionStatus: map[string]workspace.SessionStatus{"s11": {Busy: true, PendingPrompt: true}},
	}
	m := newPanelUI(ws)
	refreshPanel(m)
	infos := eventRefresh(m, userMessageFor("s11"))
	require.Equal(t, activityWaiting, m.activity.Get("s11"))
	require.Contains(t, panelRowsN(m, 10), "· ▲ title 11 waiting")
	require.Contains(t, strings.Join(infos, "|"), `Session "title 11" needs your input`)
}

// Alt+. / Alt+, đi qua cả dòng được ghim.
func TestCycleSessionIncludesPinnedRunningSession(t *testing.T) {
	t.Parallel()

	m, ws := newJumpUI()
	m.activity.Working("s11")
	runLeaves(m.handleKeyPressMsg(altComma))
	require.Equal(t, []string{"s11"}, ws.loaded, "the pinned session is the last row of the panel")
}
