package home

// Home dò máy chủ định kỳ khi danh sách workspace đang hiện, và đọc sổ đăng ký
// có giới hạn thời gian (T14).

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/ui/styles"
)

// stepModel dựng Home (có dò định kỳ) và đi từng bước thay vì dùng drive, vì
// chuỗi dò định kỳ không bao giờ hết.
func stepModel(src Source) *Model {
	sty := styles.CharmtonePantera()
	m := New(src, &sty, "/home/u/here")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// answer chạy cmd (một lá) và trả message của nó.
func answer(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	require.NotNil(t, cmd)
	return cmd()
}

// Dò xong thì cờ "đang dò" tắt: lần nạp sổ đăng ký sau đó (ghim, thêm, xoá) dò
// lại, thay vì giữ nguyên số liệu cũ mãi mãi.
func TestRegistryReloadProbesAgain(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:2], live: map[string]Live{"/w/a": {State: ServerLive, Running: 1}}}
	m := stepModel(src)
	m.probeEvery = 0 // Chỉ xem cờ, không có hẹn giờ.

	_, probe := m.Update(workspacesMsg{list: src.workspaces})
	_, _ = m.Update(answer(t, probe))
	require.Equal(t, 1, src.probes)
	require.False(t, m.probing, "the answer ends the probe")

	_, probe = m.Update(workspacesMsg{list: src.workspaces}) // Một lần nạp lại sổ đăng ký.
	require.NotNil(t, probe, "a reload after the first probe probes again")
	answer(t, probe)
	require.Equal(t, 2, src.probes)
}

// Dò xong thì hẹn lần dò kế; hết hẹn thì dò lại khi danh sách workspace đang hiện.
func TestProbeRepeatsWhileListIsShown(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:2], live: map[string]Live{}}
	m := stepModel(src)
	m.probeEvery = time.Millisecond

	_, probe := m.Update(workspacesMsg{list: src.workspaces})
	_, tick := m.Update(answer(t, probe))
	require.True(t, m.tickArmed, "the finished probe arms the next one")
	require.NotNil(t, tick)
	require.IsType(t, probeTickMsg{}, answer(t, tick))
	require.Equal(t, 1, src.probes)

	_, probe = m.Update(probeTickMsg{})
	require.True(t, m.probing)
	require.False(t, m.tickArmed)
	_, tick = m.Update(answer(t, probe))
	require.Equal(t, 2, src.probes)
	require.NotNil(t, tick, "and so on, for as long as the list is shown")
	require.True(t, m.tickArmed)
}

// Một lần dò đang chạy thì hết hẹn không dò chồng; một hẹn giờ không hẹn đôi.
func TestProbeTickNeverOverlaps(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:2], live: map[string]Live{}}
	m := stepModel(src)
	m.probeEvery = time.Millisecond

	_, probe := m.Update(workspacesMsg{list: src.workspaces})
	require.True(t, m.probing)
	_, cmd := m.Update(probeTickMsg{})
	require.Nil(t, cmd, "a probe is already running")
	require.Equal(t, 0, src.probes)

	_, tick := m.Update(answer(t, probe))
	require.NotNil(t, tick)
	require.Nil(t, m.armProbeTick(), "one timer at a time")
}

// Ngoài danh sách workspace (đang xem session, nhập đường dẫn) thì không dò,
// nhưng chuỗi hẹn giờ vẫn sống để dò lại khi quay về danh sách.
func TestProbeTickSkipsOtherScreens(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:1], live: map[string]Live{}, sessions: map[string][]Session{"/w/a": {{ID: "s1", Title: "one"}}}}
	m := newTestModel(src) // drive; không hẹn giờ.
	selectRow(t, m, isPath("/w/a"))
	press(m, keyEnter)
	require.Equal(t, screenSessions, m.screen)
	probes := src.probes

	m.probeEvery = time.Millisecond
	_, cmd := m.Update(probeTickMsg{})
	require.NotNil(t, cmd, "the timer chain keeps going")
	require.IsType(t, probeTickMsg{}, answer(t, cmd))
	require.Equal(t, probes, src.probes, "no probe while the session list is shown")

	press(m, keyEsc)
	require.Equal(t, screenWorkspaces, m.screen)
	_, cmd = m.Update(probeTickMsg{})
	require.True(t, m.probing)
	answer(t, cmd)
	require.Equal(t, probes+1, src.probes)
}

// probeEvery bằng 0 tắt việc dò định kỳ (các test dùng drive cần vậy).
func TestProbeIntervalZeroDisablesTicks(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:1], live: map[string]Live{}}
	m := stepModel(src)
	m.probeEvery = 0
	_, probe := m.Update(workspacesMsg{list: src.workspaces})
	_, tick := m.Update(answer(t, probe))
	require.Nil(t, tick)
	require.False(t, m.tickArmed)
}

// ctxSource ghi lại ctx mà Workspaces nhận và có thể kẹt không nhìn ctx.
type ctxSource struct {
	fakeSource

	mu       sync.Mutex
	deadline time.Time
	hasDL    bool
	stuck    chan struct{} // Nil: trả lời ngay.
}

func (s *ctxSource) Workspaces(ctx context.Context) ([]Workspace, error) {
	s.mu.Lock()
	s.deadline, s.hasDL = ctx.Deadline()
	s.mu.Unlock()
	if s.stuck != nil {
		<-s.stuck // Như os.Stat trên điểm gắn kết chết: không nhìn ctx.
	}
	return s.fakeSource.Workspaces(ctx)
}

// Nạp sổ đăng ký chạy với hạn chót, và một Source kẹt không giữ Home mãi.
func TestLoadWorkspacesIsBounded(t *testing.T) {
	t.Parallel()

	src := &ctxSource{fakeSource: fakeSource{workspaces: tenWorkspaces()[:2]}}
	m := stepModel(src)
	start := time.Now()
	msg := m.loadWorkspaces()
	require.Len(t, msg.(workspacesMsg).list, 2)
	src.mu.Lock()
	require.True(t, src.hasDL, "the registry is read under a deadline")
	require.WithinDuration(t, start.Add(probeBudget), src.deadline, time.Second)
	src.mu.Unlock()

	stuck := &ctxSource{fakeSource: fakeSource{workspaces: tenWorkspaces()[:2]}, stuck: make(chan struct{})}
	m = stepModel(stuck)
	m.budget = 30 * time.Millisecond
	start = time.Now()
	got := m.loadWorkspaces().(workspacesMsg)
	require.Less(t, time.Since(start), 5*time.Second, "a stuck source must not hold Home")
	require.Error(t, got.err)
	require.Empty(t, got.list)
	close(stuck.stuck) // Để goroutine bị bỏ lại kết thúc.
}

// Đọc lại sổ đăng ký hỏng (quá hạn) thì giữ danh sách đang hiện và báo lỗi; chỉ
// lần đầu hỏng mới ra trạng thái rỗng.
func TestFailedReloadKeepsTheList(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:3]}
	m := newTestModel(src)
	require.Len(t, m.workspaces, 3)

	m.Update(workspacesMsg{err: errors.New("registry timed out")})
	require.Len(t, m.workspaces, 3, "the list stays")
	require.Contains(t, m.note, "registry timed out")

	first := newTestModel(&fakeSource{})
	first.loaded, first.workspaces = false, nil
	first.Update(workspacesMsg{err: errors.New("boom")})
	require.True(t, first.loaded, "the first load that failed still ends the loading state")
	require.Empty(t, first.workspaces)
	require.Contains(t, first.note, "boom")
}
