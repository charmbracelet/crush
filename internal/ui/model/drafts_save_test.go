package model

// Lưu bản nháp có trễ (T13): ghi xuống đĩa sau khi người dùng ngừng đổi bản
// nháp một lúc, không chờ tới lúc thoát sạch.

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// newSaveUI là UI trên s1 có tệp bản nháp, với khoảng yên lặng ngắn.
func newSaveUI(t *testing.T) (*UI, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "drafts.json")
	m, _ := newDraftUI(t)
	m.draftsPath = path
	m.draftSave.delay = 20 * time.Millisecond
	return m, path
}

var keyX = tea.KeyPressMsg{Code: 'x', Text: "x"}

// flushDrafts giả lập hết giờ sau một khoảng yên lặng dài: chạy lần ghi nền tới
// cùng và đưa kết quả về Update như vòng lặp Bubble Tea.
func flushDrafts(t *testing.T, m *UI) draftsSavedMsg {
	t.Helper()
	m.draftSave.armed = false
	m.draftSave.changedAt = time.Now().Add(-time.Hour)
	write := m.handleDraftSaveTick()
	require.NotNil(t, write, "a write was due")
	saved, ok := write().(draftsSavedMsg)
	require.True(t, ok)
	require.NoError(t, saved.err)
	m.Update(saved)
	return saved
}

// Một chuỗi phím chỉ hẹn một giờ, không phải một giờ cho mỗi phím.
func TestDraftSaveArmsOneTimerPerBurst(t *testing.T) {
	t.Parallel()

	m, path := newSaveUI(t)
	m.textarea.SetValue("a")
	require.NotNil(t, m.trackDraftChange(keyX), "the first change arms the timer")
	for _, v := range []string{"ab", "abc", "abcd"} {
		m.textarea.SetValue(v)
		require.Nil(t, m.trackDraftChange(keyX), "a timer is already pending")
	}
	require.True(t, m.draftSave.dirty)
	require.NoFileExists(t, path, "nothing is written while the timer runs")
}

// Không đổi gì thì không hẹn thêm giờ; message không thể đổi bản nháp thì bị bỏ
// qua dù chữ trong ô nhập đã đổi (đỡ so sánh chữ cho mỗi tick hoạt ảnh).
func TestDraftSaveIgnoresUnchangedAndIrrelevant(t *testing.T) {
	t.Parallel()

	m, _ := newSaveUI(t)
	m.textarea.SetValue("changed behind its back")
	require.Nil(t, m.trackDraftChange(plainMsg{}), "plainMsg stands for ticks and events that cannot edit")
	require.False(t, m.draftSave.armed)

	require.NotNil(t, m.trackDraftChange(keyX))
	m.draftSave.armed = false
	require.Nil(t, m.trackDraftChange(keyX), "a key that left the draft as it was (an arrow, say) is no change")
	require.False(t, m.draftSave.armed)

	// Không có tệp bản nháp (UI dựng cho test hoặc dữ liệu chưa cấu hình): không lưu.
	m, _ = newSaveUI(t)
	m.draftsPath = ""
	m.textarea.SetValue("x")
	require.Nil(t, m.trackDraftChange(keyX))
}

// Hết giờ mà người dùng vừa gõ tiếp (chưa yên lặng đủ lâu): hẹn lại phần còn
// thiếu, chưa ghi.
func TestDraftSaveTickDuringTypingRearms(t *testing.T) {
	t.Parallel()

	m, path := newSaveUI(t)
	m.draftSave.delay = time.Hour
	m.textarea.SetValue("abc")
	m.trackDraftChange(keyX)

	cmd := m.handleDraftSaveTick()
	require.NotNil(t, cmd, "the missing quiet time is waited out")
	require.True(t, m.draftSave.armed)
	require.True(t, m.draftSave.dirty)
	require.False(t, m.draftSave.inFlight)
	require.NoFileExists(t, path)
}

// Yên lặng đủ lâu thì ghi, bằng một Cmd chạy nền dùng lại logic gộp/khoá của
// SaveDrafts; xong thì hết dirty và không hẹn thêm.
func TestDraftSaveWritesAfterQuietPeriod(t *testing.T) {
	t.Parallel()

	m, path := newSaveUI(t)
	typeText(m, "abc")
	require.True(t, m.draftSave.armed)
	m.draftSave.changedAt = time.Now().Add(-time.Hour) // Đã yên lặng từ lâu.

	_, cmd := m.Update(draftSaveTickMsg{})
	require.NotNil(t, cmd)
	require.True(t, m.draftSave.inFlight)
	require.False(t, m.draftSave.dirty)
	require.False(t, m.draftSave.armed)

	saved, ok := cmd().(draftsSavedMsg)
	require.True(t, ok, "the write runs in the command and reports back")
	require.NoError(t, saved.err)
	require.JSONEq(t, `{"s1":"abc"}`, readDrafts(t, path))

	_, next := m.Update(saved)
	require.Nil(t, next, "nothing changed while writing: no further timer")
	require.False(t, m.draftSave.inFlight)

	// Hết giờ lần nữa mà không có gì mới: không ghi lại.
	require.Nil(t, m.handleDraftSaveTick())
}

// Đổi tiếp trong lúc đang ghi thì hẹn lại sau khi ghi xong, kể cả khi giờ nổ giữa
// lúc đang ghi.
func TestDraftSaveChangeDuringWriteIsSavedNext(t *testing.T) {
	t.Parallel()

	m, path := newSaveUI(t)
	typeText(m, "one")
	m.draftSave.changedAt = time.Now().Add(-time.Hour)
	write := m.handleDraftSaveTick()
	require.True(t, m.draftSave.inFlight)

	typeText(m, "two") // Trong lúc ghi lần một.
	require.True(t, m.draftSave.dirty)
	m.draftSave.changedAt = time.Now().Add(-time.Hour)
	require.Nil(t, m.handleDraftSaveTick(), "a write is running: the timer must not start a second one")
	require.False(t, m.draftSave.armed)

	saved := write().(draftsSavedMsg)
	require.NoError(t, saved.err)
	require.JSONEq(t, `{"s1":"one"}`, readDrafts(t, path), "the first snapshot was taken before the second edit")
	again := m.handleDraftsSaved(saved)
	require.NotNil(t, again, "the later edit gets its own save")

	m.draftSave.armed = false
	second := m.handleDraftSaveTick()
	require.NotNil(t, second)
	require.NoError(t, second().(draftsSavedMsg).err)
	require.JSONEq(t, `{"s1":"onetwo"}`, readDrafts(t, path))
}

// Ghi hỏng thì chỉ ghi nhật ký, không thử lại liên tục; lần đổi sau mới thử lại.
func TestDraftSaveFailureWaitsForNextChange(t *testing.T) {
	t.Parallel()

	m, _ := newSaveUI(t)
	m.draftsPath = filepath.Join(t.TempDir(), "missing-dir", "drafts.json")
	typeText(m, "abc")
	m.draftSave.changedAt = time.Now().Add(-time.Hour)
	write := m.handleDraftSaveTick()

	saved := write().(draftsSavedMsg)
	require.Error(t, saved.err)
	require.Nil(t, m.handleDraftsSaved(saved), "no retry loop on a persistent error")
	require.False(t, m.draftSave.inFlight)
	require.False(t, m.draftSave.dirty)

	typeText(m, "d")
	require.True(t, m.draftSave.dirty, "the next change tries again")
	require.True(t, m.draftSave.armed)
}

// Chuyển session cũng là đổi bản nháp (cất bản nháp s1, mở ô nhập s2): được lưu.
func TestDraftSaveSeesSessionSwitch(t *testing.T) {
	t.Parallel()

	m, path := newSaveUI(t)
	typeText(m, "left behind")
	flushDrafts(t, m)
	require.JSONEq(t, `{"s1":"left behind"}`, readDrafts(t, path))
	require.False(t, m.draftSave.dirty)

	switchTo(m, "s2")
	require.True(t, m.draftSave.dirty, "the editor shows s2 now: what is stored changed")
	typeText(m, "second")
	flushDrafts(t, m)
	require.JSONEq(t, `{"s1":"left behind","s2":"second"}`, readDrafts(t, path))
}

// Gửi đi (ô nhập về rỗng) cũng là một thay đổi: bản nháp đã gửi bị xoá khỏi đĩa.
func TestDraftSaveRemovesSentDraft(t *testing.T) {
	t.Parallel()

	m, path := newSaveUI(t)
	typeText(m, "send me")
	flushDrafts(t, m)
	require.JSONEq(t, `{"s1":"send me"}`, readDrafts(t, path))

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	runCmds(m, cmd)
	require.Equal(t, "", m.textarea.Value())
	require.True(t, m.draftSave.dirty)
	flushDrafts(t, m)
	require.JSONEq(t, `{}`, readDrafts(t, path))
}

// Lưu có trễ rồi thoát sạch: SaveDrafts không ghi trùng, và vẫn ghi phần đổi sau
// lần lưu có trễ cuối.
func TestSaveDraftsAfterDebouncedSave(t *testing.T) {
	t.Parallel()

	m, path := newSaveUI(t)
	typeText(m, "first")
	flushDrafts(t, m)
	before, err := filepath.Glob(path + "*")
	require.NoError(t, err)

	require.NoError(t, m.SaveDrafts(), "nothing new to write")
	typeText(m, " and more")
	require.NoError(t, m.SaveDrafts())
	require.JSONEq(t, `{"s1":"first and more"}`, readDrafts(t, path))
	after, err := filepath.Glob(path + "*")
	require.NoError(t, err)
	require.Equal(t, before, after, "no stray temp or corrupt files")
}

// Ảnh chụp cũ đến muộn (Cmd ghi nền chạy chậm hơn lần thoát sạch hoặc một lần lưu
// khác) không được đè bản mới hơn đã nằm trên đĩa.
func TestStaleDebouncedWriteDoesNotOverwriteNewerSave(t *testing.T) {
	t.Parallel()

	m, path := newSaveUI(t)
	typeText(m, "old")
	m.draftSave.changedAt = time.Now().Add(-time.Hour)
	stale := m.handleDraftSaveTick() // Ảnh chụp {"s1":"old"} đã lấy, chưa ghi.
	require.NotNil(t, stale)

	typeText(m, " and newer")
	switchTo(m, "s2")
	typeText(m, "other")
	require.NoError(t, m.SaveDrafts()) // Bản mới hơn tới đĩa trước.
	const want = `{"s1":"old and newer","s2":"other"}`
	require.JSONEq(t, want, readDrafts(t, path))

	saved := stale().(draftsSavedMsg) // Cmd chậm cuối cùng cũng chạy.
	require.NoError(t, saved.err)
	require.JSONEq(t, want, readDrafts(t, path), "an older snapshot must not undo a newer save")
}

// Cmd ghi nền chạy ngoài luồng Update: nó chỉ giữ bản chụp chữ và kho ghi có
// mutex, không đụng trạng thái UI. Chạy với -race để thấy. Thứ tự hai lần ghi
// tuỳ lịch chạy của goroutine, nhưng bản mới hơn luôn thắng.
func TestDraftSaveCommandDoesNotShareUIState(t *testing.T) {
	m, path := newSaveUI(t)
	typeText(m, "snapshot")
	m.draftSave.changedAt = time.Now().Add(-time.Hour)
	write := m.handleDraftSaveTick()

	var (
		wg    sync.WaitGroup
		saved draftsSavedMsg
	)
	wg.Go(func() { saved = write().(draftsSavedMsg) })
	// Trong lúc Cmd chạy, Update tiếp tục đổi mọi thứ nó muốn.
	typeText(m, " plus typing during the write")
	switchTo(m, "s2")
	typeText(m, "other")
	m.SaveDrafts() //nolint:errcheck // Cũng tranh kho ghi với Cmd; kết quả xem bên dưới.
	wg.Wait()

	require.NoError(t, saved.err)
	require.JSONEq(t, `{"s1":"snapshot plus typing during the write","s2":"other"}`, readDrafts(t, path))
}

// landingWS là workspace của một dự án đã khởi tạo, nên New() vào thẳng màn landing.
type landingWS struct{ *countingWorkspace }

func (landingWS) ProjectNeedsInitialization() (bool, error) { return false, nil }

// Dựng UI bằng New() (không phải struct dựng tay): thư mục dữ liệu của cấu hình
// thành đường dẫn drafts.json, và một chuỗi phím gõ qua Update hẹn giờ lưu rồi
// ghi bản nháp của màn landing xuống đó.
func TestNewWiresDebouncedDraftSave(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("p", config.ProviderConfig{ID: "p", APIKey: "k", Models: []catwalk.Model{{ID: "m"}}})
	ws := landingWS{&countingWorkspace{
		ready: true,
		cfg: &config.Config{
			Providers: providers,
			Options:   &config.Options{DataDirectory: dir, TUI: &config.TUIOptions{}},
			Models:    map[config.SelectedModelType]config.SelectedModel{},
		},
	}}
	m := New(common.DefaultCommon(ws), "", false)
	require.Equal(t, uiLanding, m.state)
	require.Equal(t, filepath.Join(dir, "drafts.json"), m.draftsPath)

	typeText(m, "typed at landing")
	require.True(t, m.draftSave.armed, "typing through Update arms the timer")
	flushDrafts(t, m)
	require.JSONEq(t, `{"":"typed at landing"}`, readDrafts(t, m.draftsPath))

	// Mở lại: bản nháp landing về lại ô nhập.
	m2 := New(common.DefaultCommon(ws), "", false)
	require.Equal(t, "typed at landing", m2.textarea.Value())
}
