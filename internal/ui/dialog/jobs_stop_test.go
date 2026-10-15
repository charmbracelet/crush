package dialog

import (
	"errors"
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// killCount là số lần KillJob đã thật sự chạy, kể cả trong Cmd mà một phím
// trả về: dừng job chỉ xảy ra khi Cmd chạy, nên phải chạy mọi Cmd mới biết.
func killCount(ws *jobsWorkspace) int {
	_, _, kills := ws.counts()
	return kills
}

// press đưa một phím cho hộp thoại và chạy mọi Cmd trả về.
func press(d *Jobs, k tea.KeyPressMsg) []tea.Msg { return feed(d, k) }

// strayKeys là những phím có thể lọt vào hộp thoại do gõ nhầm hay do đang gõ
// dở: mọi ký tự in được, Ctrl+chữ, Alt+chữ, Tab, Backspace...
func strayKeys() []tea.KeyPressMsg {
	var keys []tea.KeyPressMsg
	for _, r := range "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 `~!@#$%^&*()-_=+[]{};:'\",.<>/?\\|" {
		keys = append(keys, keyMsg(r))
	}
	for r := 'a'; r <= 'z'; r++ {
		keys = append(keys,
			jobsCtrl(r),
			tea.KeyPressMsg{Code: r, Mod: tea.ModAlt},
			tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl | tea.ModShift},
		)
	}
	for _, code := range []rune{
		tea.KeyTab, tea.KeyBackspace, tea.KeyDelete, tea.KeyInsert, tea.KeySpace,
		tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd,
	} {
		keys = append(keys, tea.KeyPressMsg{Code: code}, tea.KeyPressMsg{Code: code, Mod: tea.ModShift})
	}
	return keys
}

// Ctrl+K chỉ hỏi: chưa dừng gì, mặc định là No.
func TestJobsStopAsksBeforeStopping(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	require.Equal(t, "003", d.selectedID)

	require.Empty(t, press(d, keyStop), "the key itself returns no command")
	require.NotNil(t, d.confirm)
	require.Equal(t, "003", d.confirm.jobID)
	require.False(t, d.confirm.yes, "the default answer is No")
	require.Zero(t, killCount(ws))

	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "Stop job 003?")
	require.Contains(t, text, "Dev server")
	require.Contains(t, text, "No, keep running")
	require.Contains(t, text, "Yes, stop it")
	require.Contains(t, text, "esc keep running")
	require.NotContains(t, text, "ctrl+k stop", "the hint line shows the answers now")
}

// Enter ngay sau Ctrl+K (mặc định No) không dừng job, và đóng câu hỏi.
func TestJobsStopEnterOnDefaultKeeps(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	press(d, keyStop)
	require.Empty(t, press(d, keyEnter))
	require.Nil(t, d.confirm)
	require.Zero(t, killCount(ws))
	require.Contains(t, drawJobs(d, 100, 24), "1 running", "back on the list")
}

// Không phím tình cờ nào dừng được job, kể cả Enter sau đó: như hộp thoại
// xin quyền, thứ nguy hiểm đòi một thao tác có chủ ý. Bản thân phím dừng bấm
// lần nữa, y/Y, Ctrl+Y, Tab, h/l... đều không phải thao tác đó.
func TestJobsStrayKeysNeverStop(t *testing.T) {
	t.Parallel()

	for _, k := range strayKeys() {
		ws := &jobsWorkspace{jobs: jobsFixture()}
		d := loadedJobs(t, ws)
		press(d, keyStop)
		require.NotNil(t, d.confirm)

		press(d, k)
		if d.confirm != nil {
			require.False(t, d.confirm.yes, "%s moved the highlight to Yes", k.String())
		}
		// Rồi cứ Enter: nếu câu hỏi còn đó thì nó đang ở No.
		press(d, keyEnter)
		require.Zero(t, killCount(ws), "key %q stopped a job", k.String())
		require.Nil(t, d.confirm, "enter on No closes the question (after %q)", k.String())
	}
}

// Cũng thế khi phím lạ tới trước câu hỏi, trên danh sách.
func TestJobsStrayKeysOnTheListNeverStop(t *testing.T) {
	t.Parallel()

	for _, k := range strayKeys() {
		if k.String() == keyStop.String() {
			continue
		}
		ws := &jobsWorkspace{jobs: jobsFixture()}
		d := loadedJobs(t, ws)
		press(d, k)
		press(d, keyEnter)
		press(d, keyEnter)
		require.Zero(t, killCount(ws), "key %q stopped a job", k.String())
	}
}

func TestJobsStopNeedsAnExplicitYes(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	press(d, keyStop)

	require.Empty(t, press(d, keyRight))
	require.True(t, d.confirm.yes, "the right arrow selects Yes")
	require.Zero(t, killCount(ws), "selecting is not confirming")

	// Đổi ý: trái rồi Enter là No.
	press(d, keyLeft)
	require.False(t, d.confirm.yes)
	press(d, keyEnter)
	require.Zero(t, killCount(ws))

	// Mũi tên là idempotent: bấm phải hai lần vẫn là Yes, không lật về No.
	press(d, keyStop)
	press(d, keyRight)
	press(d, keyRight)
	require.True(t, d.confirm.yes)

	msgs := press(d, keyEnter)
	require.Equal(t, 1, killCount(ws))
	require.Equal(t, []string{"003"}, ws.killed)
	require.Nil(t, d.confirm)
	require.Len(t, msgsOf[jobsStopMsg](msgs), 1)
}

func TestJobsStopCancelPaths(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		keys []tea.KeyPressMsg
	}{
		{"esc", []tea.KeyPressMsg{keyEsc}},
		{"alt+esc", []tea.KeyPressMsg{{Code: tea.KeyEscape, Mod: tea.ModAlt}}},
		{"n", []tea.KeyPressMsg{keyMsg('n')}},
		{"N", []tea.KeyPressMsg{keyMsg('N')}},
		{"yes then esc", []tea.KeyPressMsg{keyRight, keyEsc}},
		{"yes then n", []tea.KeyPressMsg{keyRight, keyMsg('n')}},
	} {
		ws := &jobsWorkspace{jobs: jobsFixture()}
		d := loadedJobs(t, ws)
		press(d, keyStop)
		for _, k := range tc.keys {
			act := d.HandleMsg(k)
			require.NotEqual(t, ActionClose{}, act, "%s: cancelling the question must not close the dialog", tc.name)
		}
		require.Nil(t, d.confirm, tc.name)
		require.Zero(t, killCount(ws), tc.name)
		require.Contains(t, drawJobs(d, 100, 24), "Jobs", tc.name)
	}
}

// Sau khi gửi lệnh dừng, hàng hiện "stopping"; kết quả về thì báo trên thanh
// trạng thái và nạp lại danh sách ngay, khỏi đợi nhịp sau.
func TestJobsStopReportsTheResult(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	press(d, keyStop)
	press(d, keyRight)

	act := d.HandleMsg(keyEnter)
	require.IsType(t, ActionCmd{}, act)
	require.Zero(t, killCount(ws), "Enter returns the command, nothing has run yet")
	require.Contains(t, lineWith(t, drawJobs(d, 100, 24), "Dev server"), "stopping")

	// Nhấn lại ctrl+k khi lệnh dừng chưa xong: báo, không hỏi lần nữa.
	repeat := press(d, keyStop)
	require.Nil(t, d.confirm)
	require.Contains(t, msgsOf[util.InfoMsg](repeat)[0].Msg, "already being stopped")

	results := runCmd(act.(ActionCmd).Cmd)
	require.Equal(t, 1, killCount(ws))
	infos := msgsOf[util.InfoMsg](results)
	require.Len(t, infos, 1)
	require.Equal(t, util.InfoTypeInfo, infos[0].Type)
	require.Equal(t, "Stopped job 003", infos[0].Msg)
	stopped := msgsOf[jobsStopMsg](results)
	require.Len(t, stopped, 1)

	ws.jobs[0].Status = shell.JobKilled
	ws.jobs[0].FinishedAt = jobsNow
	listBefore, _, _ := ws.counts()
	msgs := feed(d, stopped[0])
	listAfter, _, _ := ws.counts()
	require.Equal(t, listBefore+1, listAfter, "the list reloads at once")

	for _, m := range msgsOf[jobsLoadedMsg](msgs) {
		d.HandleMsg(m)
	}
	text := drawJobs(d, 100, 24)
	require.NotContains(t, lineWith(t, text, "Dev server"), "stopping")
	require.Contains(t, lineWith(t, text, "Dev server"), "killed")
	require.Contains(t, lineWith(t, text, "Dev server"), "■")
}

func TestJobsStopFailureIsReportedAsAnError(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture(), killErr: errors.New("background shell not found: 003")}
	d := loadedJobs(t, ws)
	press(d, keyStop)
	press(d, keyRight)
	results := press(d, keyEnter)

	infos := msgsOf[util.InfoMsg](results)
	require.Len(t, infos, 1)
	require.Equal(t, util.InfoTypeError, infos[0].Type)
	require.Contains(t, infos[0].Msg, "could not stop job 003")
	require.Contains(t, infos[0].Msg, "background shell not found: 003")

	stopped := msgsOf[jobsStopMsg](results)
	require.Len(t, stopped, 1)
	feed(d, stopped[0])

	// Hết "stopping": thử dừng lại được.
	require.NotContains(t, lineWith(t, drawJobs(d, 100, 24), "Dev server"), "stopping")
	press(d, keyStop)
	require.NotNil(t, d.confirm)
}

// Đóng hộp thoại ngay sau khi xác nhận vẫn thấy kết quả: thông báo nằm trong
// chính lệnh dừng chứ không đi qua hộp thoại đã đóng.
func TestJobsStopResultSurvivesClosingTheDialog(t *testing.T) {
	t.Parallel()

	for name, killErr := range map[string]error{"success": nil, "failure": errors.New("boom")} {
		ws := &jobsWorkspace{jobs: jobsFixture(), killErr: killErr}
		d := loadedJobs(t, ws)
		o := NewOverlay(d)
		press(d, keyStop)
		press(d, keyRight)
		act := d.HandleMsg(keyEnter)
		require.IsType(t, ActionCmd{}, act, name)

		o.CloseDialog(JobsID) // Người dùng đóng hộp thoại trước khi lệnh dừng chạy xong.
		results := runCmd(act.(ActionCmd).Cmd)
		infos := msgsOf[util.InfoMsg](results)
		require.Len(t, infos, 1, name)
		if killErr != nil {
			require.Equal(t, util.InfoTypeError, infos[0].Type, name)
		}
		for _, m := range msgsOf[jobsStopMsg](results) {
			require.Nil(t, o.Update(m), "%s: the dialog is gone, its message is dropped", name)
		}
	}
}

// Chỉ job đang chạy mới dừng được; job đã xong thì báo, không hỏi.
func TestJobsStopOnlyRunningJobs(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	for range 3 {
		d.HandleMsg(keyDown)
		msgs := press(d, keyStop)
		require.Nil(t, d.confirm, d.selectedID)
		infos := msgsOf[util.InfoMsg](msgs)
		require.Len(t, infos, 1)
		require.Contains(t, infos[0].Msg, "is not running")
	}
	require.Zero(t, killCount(ws))

	// Danh sách trống: không làm gì.
	empty := loadedJobs(t, &jobsWorkspace{})
	require.Empty(t, press(empty, keyStop))
	require.Nil(t, empty.confirm)
}

// Job xong trong lúc câu hỏi đang mở thì câu hỏi tự đóng: không còn gì để
// dừng.
func TestJobsStopPromptClosesWhenTheJobEnds(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	press(d, keyStop)
	require.NotNil(t, d.confirm)

	ended := jobsFixture()
	ended[0].Status, ended[0].FinishedAt = shell.JobSucceeded, jobsNow
	d.HandleMsg(jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: ended})
	require.Nil(t, d.confirm)

	// Enter lúc này không dừng gì.
	press(d, keyEnter)
	require.Zero(t, killCount(ws))

	// Job biến khỏi danh sách cũng thế.
	d = loadedJobs(t, ws)
	press(d, keyStop)
	d.HandleMsg(jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: jobsFixture()[1:]})
	require.Nil(t, d.confirm)
}

// Từ khung xem output cũng dừng được, qua cùng câu hỏi; Esc rồi vẫn ở khung
// xem.
func TestJobsStopFromTheOutputViewer(t *testing.T) {
	t.Parallel()

	ws := outputWorkspace("serving")
	d := openedOutput(t, ws)

	press(d, keyStop)
	require.NotNil(t, d.confirm)
	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "Stop job 003?")
	require.Contains(t, text, "serving", "the output stays visible behind the question")
	require.Zero(t, killCount(ws))

	require.Nil(t, d.HandleMsg(keyEsc))
	require.Contains(t, drawJobs(d, 100, 24), "Job 003", "Esc cancels the question only")
	require.Equal(t, jobsModeOutput, d.mode)

	press(d, keyStop)
	press(d, keyRight)
	press(d, keyEnter)
	require.Equal(t, []string{"003"}, ws.killed)

	// Job đã xong: khung xem không đề nghị dừng và phím dừng chỉ báo.
	d = loadedJobs(t, outputWorkspace(""))
	d.HandleMsg(keyDown)
	for _, m := range msgsOf[jobsOutputMsg](feed(d, keyEnter)) {
		d.HandleMsg(m)
	}
	require.NotContains(t, drawJobs(d, 100, 24), "ctrl+k stop")
	infos := msgsOf[util.InfoMsg](press(d, keyStop))
	require.Len(t, infos, 1)
	require.Contains(t, infos[0].Msg, "is not running")
	require.Nil(t, d.confirm)
}

// Câu hỏi không lọt cuộn chuột vào danh sách bên dưới.
func TestJobsStopPromptIgnoresTheWheel(t *testing.T) {
	t.Parallel()

	d := loadedJobs(t, &jobsWorkspace{jobs: jobsFixture()})
	press(d, keyStop)
	d.HandleMsg(common.CoalescedWheelMsg{DeltaY: 2})
	require.Equal(t, "003", d.selectedID)
	require.NotNil(t, d.confirm)
}

// Nút được chọn nhìn thấy được: No sáng mặc định, sang Yes thì Yes đỏ.
func TestJobsStopPromptHighlightsTheDefault(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	d := loadedJobs(t, &jobsWorkspace{jobs: jobsFixture()})
	press(d, keyStop)

	bg := func(snippet string) color.Color {
		scr := uv.NewScreenBuffer(100, 24)
		d.Draw(scr, scr.Bounds())
		cell := findCell(scr, snippet)
		require.NotNil(t, cell, snippet)
		return cell.Style.Bg
	}
	same := func(a, b color.Color) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		ar, ag, ab, aa := a.RGBA()
		br, bgc, bb, ba := b.RGBA()
		return ar == br && ag == bgc && ab == bb && aa == ba
	}

	require.True(t, same(bg("No, keep"), s.Button.Focused.GetBackground()), "No starts selected")
	require.True(t, same(bg("Yes, stop"), s.Button.Blurred.GetBackground()), "Yes starts unselected")

	press(d, keyRight)
	require.True(t, same(bg("Yes, stop"), s.Button.Negative.GetBackground()), "a selected Yes is the destructive colour")
	require.True(t, same(bg("No, keep"), s.Button.Blurred.GetBackground()))
}

// Hàng đang chọn giữ nền chọn tới hết dòng, không mất sau các cột có màu.
func TestJobsSelectedRowKeepsItsBackground(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	d := loadedJobs(t, &jobsWorkspace{jobs: jobsFixture()})
	scr := uv.NewScreenBuffer(100, 24)
	d.Draw(scr, scr.Bounds())

	want := s.Dialog.SelectedItem.GetBackground()
	r1, g1, b1, _ := want.RGBA()
	isSelectedBg := func(c color.Color) bool {
		if c == nil {
			return false
		}
		r, g, b, _ := c.RGBA()
		return r == r1 && g == g1 && b == b1
	}
	for _, snippet := range []string{"●", "1m35s", "bg", "Dev server", "Fix login bug"} {
		cell := findCell(scr, snippet)
		require.NotNil(t, cell, snippet)
		require.True(t, isSelectedBg(cell.Style.Bg), "%q lost the selection background", snippet)
	}
	for _, snippet := range []string{"go test", "Add pagination", "make build"} {
		cell := findCell(scr, snippet)
		require.NotNil(t, cell, snippet)
		require.False(t, isSelectedBg(cell.Style.Bg), "%q is not selected", snippet)
	}
}

// findCell trả ô đầu tiên của đoạn chữ snippet trên màn hình.
func findCell(scr uv.ScreenBuffer, snippet string) *uv.Cell {
	b := scr.Bounds()
	for y := range b.Dy() {
		var row strings.Builder
		var xs []int
		for x := range b.Dx() {
			c := scr.CellAt(x, y)
			if c == nil {
				continue
			}
			for range len(c.Content) {
				xs = append(xs, x)
			}
			row.WriteString(c.Content)
		}
		if i := strings.Index(row.String(), snippet); i >= 0 {
			return scr.CellAt(xs[i], y)
		}
	}
	return nil
}

// Một trường hợp hồi quy: dừng đúng job đã hỏi dù lựa chọn đã đổi trong lúc
// câu hỏi mở (danh sách nạp lại, job mới chen lên).
func TestJobsStopTargetsTheJobThatWasAsked(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	press(d, keyStop)
	press(d, keyRight)

	fresh := shell.JobInfo{ID: "009", Command: "new", StartedAt: ago(time.Second), Status: shell.JobRunning}
	d.HandleMsg(jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: append([]shell.JobInfo{fresh}, jobsFixture()...)})
	require.NotNil(t, d.confirm, "the question survives a reload while its job still runs")
	press(d, keyEnter)
	require.Equal(t, []string{"003"}, ws.killed)
}
