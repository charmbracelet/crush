package dialog

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// openedOutput mở khung xem output của job đang chọn (003, đang chạy) và đưa
// kết quả nạp đầu vào hộp thoại.
func openedOutput(t *testing.T, ws *jobsWorkspace) *Jobs {
	t.Helper()
	d := loadedJobs(t, ws)
	results := msgsOf[jobsOutputMsg](feed(d, keyEnter))
	require.Len(t, results, 1)
	d.HandleMsg(results[0])
	return d
}

func outputWorkspace(output string) *jobsWorkspace {
	return &jobsWorkspace{jobs: jobsFixture(), outputs: map[string]string{"003": output, "002": "PASS\nok  \tpkg\t0.3s"}}
}

// settle đưa các message về hộp thoại và chạy tiếp mọi Cmd mà nó sinh ra, như
// vòng lặp thật, cho tới khi hết việc. Message nhịp không được đưa vào (nhịp
// luôn sinh nhịp mới); trả nhịp cuối thấy được.
func settle(d *Jobs, msgs []tea.Msg) (next jobsTickMsg) {
	for queue := msgs; len(queue) > 0; queue = queue[1:] {
		if tick, ok := queue[0].(jobsTickMsg); ok {
			next = tick
			continue
		}
		queue = append(queue, feed(d, queue[0])...)
	}
	return next
}

// tickAll gửi một nhịp và đưa mọi kết quả nạp về lại hộp thoại.
func tickAll(d *Jobs) jobsTickMsg {
	return settle(d, feed(d, jobsTickMsg{jobsMsg{d.instance}}))
}

// --- Mở, đóng, nội dung ---

func TestJobsEnterOpensOutputViewerAndEscReturns(t *testing.T) {
	t.Parallel()

	ws := outputWorkspace("listening on :3000\nready")
	d := loadedJobs(t, ws)

	act := d.HandleMsg(keyEnter)
	_, outputs, _ := ws.counts()
	require.Zero(t, outputs, "Enter itself does no IO: the output loads in a command")
	require.IsType(t, ActionCmd{}, act)
	require.Contains(t, drawJobs(d, 100, 24), "Loading output")

	results := msgsOf[jobsOutputMsg](runCmd(act.(ActionCmd).Cmd))
	require.Len(t, results, 1)
	_, outputs, _ = ws.counts()
	require.Equal(t, 1, outputs)
	d.HandleMsg(results[0])

	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "Job 003")
	require.Contains(t, text, "$ npm run dev", "the command, without the cd prefix")
	require.Contains(t, text, "Dev server · Fix login bug · bg")
	require.Contains(t, text, "running 1m35s")
	require.Contains(t, text, "listening on :3000")
	require.Contains(t, text, "ready")
	require.Contains(t, text, "esc back")

	// Esc về danh sách, không đóng hộp thoại; Esc lần nữa mới đóng.
	require.Nil(t, d.HandleMsg(keyEsc))
	text = drawJobs(d, 100, 24)
	require.Contains(t, text, "1 running · 3 finished")
	require.Equal(t, "003", d.selectedID)
	require.NotContains(t, text, "listening on :3000")
	require.IsType(t, ActionClose{}, d.HandleMsg(keyEsc))
}

// Dòng "[... N bytes of output dropped ...]" của server hiện nguyên văn.
func TestJobsOutputShowsDroppedLineAsIs(t *testing.T) {
	t.Parallel()

	const dropped = "[... 1048576 bytes of output dropped ...]"
	d := openedOutput(t, outputWorkspace("head line\n"+dropped+"\ntail line"))
	text := drawJobs(d, 100, 24)
	require.Contains(t, text, dropped)
	require.Less(t, lineIndex(text, "head line"), lineIndex(text, dropped))
	require.Less(t, lineIndex(text, dropped), lineIndex(text, "tail line"))
}

func TestJobsOutputPlaceholders(t *testing.T) {
	t.Parallel()

	// Job đang chạy chưa in gì.
	d := openedOutput(t, outputWorkspace(""))
	require.Contains(t, drawJobs(d, 100, 24), "(no output yet)")

	// Job đã xong không in gì.
	ws := outputWorkspace("")
	ws.outputs["002"] = ""
	d = loadedJobs(t, ws)
	d.HandleMsg(keyDown)
	for _, msg := range msgsOf[jobsOutputMsg](feed(d, keyEnter)) {
		d.HandleMsg(msg)
	}
	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "(no output)")
	require.NotContains(t, text, "no output yet")
	require.Contains(t, text, "succeeded 1m02s", "a finished job shows its result and its final time")
}

// Mã ANSI của chương trình con không làm hỏng khung hình: con trỏ bị bỏ, màu
// được giữ, thanh tiến trình viết đè (\r) chỉ còn lần cuối.
func TestPrepareJobOutput(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	plain := func(raw string) []string {
		var out []string
		for _, l := range prepareJobOutput(raw, s.ANSI) {
			out = append(out, ansi.Strip(l))
		}
		return out
	}

	require.Equal(t, []string{"done 100%", "next"}, plain("done 10%\rdone 50%\rdone 100%\nnext\n"))
	require.Equal(t, []string{"gone", "kept"}, plain("\x1b[2K\x1b[1Ggone\n\x1b[?25lkept\x1b[?25h"))
	require.Equal(t, []string{"a    b", "x"}, plain("a\tb\r\nx"), "tabs become spaces and CRLF is one line break")
	require.Equal(t, []string{"red", "plain"}, plain("\x1b[31mred\x1b[0m\nplain"))
	require.Nil(t, prepareJobOutput("", s.ANSI))
	require.Nil(t, prepareJobOutput("\n\n", s.ANSI))

	lines := prepareJobOutput("\x1b[31mred\x1b[0m", s.ANSI)
	require.Contains(t, lines[0], "\x1b[", "colours are kept (remapped to the theme)")

	const dropped = "[... 4096 bytes of output dropped ...]"
	require.Equal(t, []string{"a", dropped, "b"}, plain("a\n"+dropped+"\nb"))
}

// --- Làm tươi khi job còn chạy ---

func TestJobsOutputRefreshesWhileRunning(t *testing.T) {
	t.Parallel()

	ws := outputWorkspace("one")
	d := openedOutput(t, ws)
	require.Contains(t, drawJobs(d, 100, 24), "one")

	ws.outputs["003"] = "one\ntwo"
	_, before, _ := ws.counts()

	// Nhịp: nạp danh sách và nạp lại output của job đang chạy.
	msgs := feed(d, jobsTickMsg{jobsMsg{d.instance}})
	_, after, _ := ws.counts()
	require.Equal(t, before+1, after)
	outs := msgsOf[jobsOutputMsg](msgs)
	require.Len(t, outs, 1)

	// Kết quả chưa về: nhịp tiếp theo không chồng yêu cầu output.
	feed(d, jobsTickMsg{jobsMsg{d.instance}})
	feed(d, jobsTickMsg{jobsMsg{d.instance}})
	_, still, _ := ws.counts()
	require.Equal(t, after, still, "no overlapping output fetch")

	d.HandleMsg(outs[0])
	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "one")
	require.Contains(t, text, "two")

	// Output không đổi: nạp lại nhưng khỏi xử lý lại.
	next := msgsOf[jobsOutputMsg](feed(d, jobsTickMsg{jobsMsg{d.instance}}))
	require.Len(t, next, 1)
	require.True(t, next[0].unchanged)
	require.Nil(t, next[0].lines)
	d.HandleMsg(next[0])
	require.Contains(t, drawJobs(d, 100, 24), "two")
}

// Job vừa xong thì lấy nốt phần output cuối đúng một lần rồi thôi gọi.
func TestJobsOutputStopsRefreshingWhenTheJobFinishes(t *testing.T) {
	t.Parallel()

	ws := outputWorkspace("a")
	d := openedOutput(t, ws)

	ws.jobs[0].Status = shell.JobSucceeded
	ws.jobs[0].FinishedAt = jobsNow
	ws.outputs["003"] = "a\nfinal words"

	for range 6 {
		tickAll(d)
	}
	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "final words", "the last output was fetched after the job finished")
	require.Contains(t, text, "succeeded")

	_, settled, _ := ws.counts()
	for range 4 {
		tickAll(d)
	}
	_, after, _ := ws.counts()
	require.Equal(t, settled, after, "a finished job's output is not fetched again")
}

// Job đã xong từ trước chỉ nạp một lần khi mở, các nhịp sau không gọi nữa.
func TestJobsOutputOfAFinishedJobIsFetchedOnce(t *testing.T) {
	t.Parallel()

	ws := outputWorkspace("")
	d := loadedJobs(t, ws)
	d.HandleMsg(keyDown) // 002, đã xong.
	for _, msg := range msgsOf[jobsOutputMsg](feed(d, keyEnter)) {
		d.HandleMsg(msg)
	}
	require.Contains(t, drawJobs(d, 100, 24), "PASS")

	for range 5 {
		tickAll(d)
	}
	_, outputs, _ := ws.counts()
	require.Equal(t, 1, outputs)
}

// --- Lỗi ---

func TestJobsOutputErrorIsShownAndReportedOnce(t *testing.T) {
	t.Parallel()

	ws := outputWorkspace("")
	ws.outputErr = errors.New("job not found: 003")
	d := loadedJobs(t, ws)

	msgs := feed(d, keyEnter)
	results := msgsOf[jobsOutputMsg](msgs)
	require.Len(t, results, 1)
	reports := msgsOf[util.InfoMsg](feed(d, results[0]))
	require.Len(t, reports, 1)
	require.Equal(t, util.InfoTypeWarn, reports[0].Type)
	require.Contains(t, reports[0].Msg, "job not found: 003")
	require.Contains(t, drawJobs(d, 100, 24), "Could not read the output: job not found: 003")

	// Job còn chạy thì nhịp sau thử lại; lỗi cũ không báo lại.
	again := msgsOf[jobsOutputMsg](feed(d, jobsTickMsg{jobsMsg{d.instance}}))
	require.Len(t, again, 1)
	require.Empty(t, msgsOf[util.InfoMsg](feed(d, again[0])), "the same ongoing failure is not reported again")

	// Hết lỗi: nội dung hiện, lỗi biến mất.
	ws.outputErr = nil
	ws.outputs["003"] = "back again"
	for _, msg := range msgsOf[jobsOutputMsg](feed(d, jobsTickMsg{jobsMsg{d.instance}})) {
		d.HandleMsg(msg)
	}
	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "back again")
	require.NotContains(t, text, "Could not read")

	// Job đã xong mà không đọc được output: một lần, không gọi dồn mỗi nhịp.
	ws = outputWorkspace("")
	ws.outputErr = errors.New("job not found: 002")
	d = loadedJobs(t, ws)
	d.HandleMsg(keyDown)
	for _, msg := range msgsOf[jobsOutputMsg](feed(d, keyEnter)) {
		d.HandleMsg(msg)
	}
	for range 3 {
		tickAll(d)
	}
	_, outputs, _ := ws.counts()
	require.Equal(t, 1, outputs)
}

// Kết quả của lần mở trước về muộn không đè lên khung xem job khác.
func TestJobsOutputStaleResultIsIgnored(t *testing.T) {
	t.Parallel()

	ws := outputWorkspace("from the dev server")
	d := loadedJobs(t, ws)

	first := msgsOf[jobsOutputMsg](feed(d, keyEnter)) // Job 003, chưa đưa kết quả về.
	require.Len(t, first, 1)
	d.HandleMsg(keyEsc)
	d.HandleMsg(keyDown)
	second := msgsOf[jobsOutputMsg](feed(d, keyEnter)) // Job 002.
	require.Len(t, second, 1)

	d.HandleMsg(first[0])
	text := drawJobs(d, 100, 24)
	require.NotContains(t, text, "from the dev server")
	require.Contains(t, text, "Loading output")

	d.HandleMsg(second[0])
	text = drawJobs(d, 100, 24)
	require.Contains(t, text, "PASS")
	require.NotContains(t, text, "from the dev server")

	// Đóng khung xem rồi kết quả mới về: bỏ.
	d.HandleMsg(keyEsc)
	d.HandleMsg(second[0])
	require.Empty(t, d.view.lines)
}

// --- Cuộn ---

func numberedLines(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("L%03d", i)
	}
	return strings.Join(lines, "\n")
}

func TestJobsOutputFollowsTheTailUntilScrolledUp(t *testing.T) {
	t.Parallel()

	ws := outputWorkspace(numberedLines(100))
	d := openedOutput(t, ws)
	text := drawJobs(d, 100, 24)
	rows := d.view.rows
	require.Greater(t, rows, 5)
	require.Less(t, rows, 100)
	require.Contains(t, text, "L099", "a running job's log follows the tail")
	require.NotContains(t, text, "L000")
	require.Contains(t, text, "following")
	require.True(t, d.view.follow)
	require.Contains(t, text, "┃", "a scrollbar when lines are hidden")

	// Cuộn lên một dòng: dừng bám, dòng cuối rời khỏi cửa sổ.
	d.HandleMsg(keyUp)
	text = drawJobs(d, 100, 24)
	require.False(t, d.view.follow)
	require.NotContains(t, text, "L099")
	require.Contains(t, text, "L098")
	require.Contains(t, text, "paused")

	// Output mới về trong lúc đang xem chỗ khác: cửa sổ đứng yên.
	ws.outputs["003"] = numberedLines(110)
	for _, msg := range msgsOf[jobsOutputMsg](feed(d, jobsTickMsg{jobsMsg{d.instance}})) {
		d.HandleMsg(msg)
	}
	text = drawJobs(d, 100, 24)
	require.Contains(t, text, "L098")
	require.NotContains(t, text, "L109")
	require.False(t, d.view.follow)

	// End bám đuôi lại; Home về đầu; trang xuống đi đúng một trang trừ một
	// dòng.
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnd})
	require.Contains(t, drawJobs(d, 100, 24), "L109")
	require.True(t, d.view.follow)

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyHome})
	text = drawJobs(d, 100, 24)
	require.Contains(t, text, "L000")
	require.NotContains(t, text, "L109")
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyPgDown})
	text = drawJobs(d, 100, 24)
	require.NotContains(t, text, "L000")
	require.Contains(t, text, fmt.Sprintf("L%03d", rows-1), "a page down keeps one line of context")

	// Phím cách cũng là trang xuống (terminal gửi nó là "space", không phải " ").
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyHome})
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	text = drawJobs(d, 100, 24)
	require.NotContains(t, text, "L000")
	require.Contains(t, text, fmt.Sprintf("L%03d", rows-1))

	// Cuộn xuống tới đáy thì bám đuôi lại.
	for range 120 {
		d.HandleMsg(keyDown)
	}
	require.True(t, d.view.follow)
	require.Contains(t, drawJobs(d, 100, 24), "L109")

	// Bánh xe chuột cuộn khung output chứ không đổi lựa chọn ở danh sách.
	d.HandleMsg(common.CoalescedWheelMsg{DeltaY: -3})
	require.False(t, d.view.follow)
	require.Equal(t, "003", d.selectedID)
}

// Log ngắn vừa cửa sổ thì không có thanh cuộn và không cuộn được.
func TestJobsOutputThatFitsDoesNotScroll(t *testing.T) {
	t.Parallel()

	d := openedOutput(t, outputWorkspace(numberedLines(3)))
	text := drawJobs(d, 100, 24)
	require.NotContains(t, text, "┃")
	d.HandleMsg(keyUp)
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyPgDown})
	after := drawJobs(d, 100, 24)
	for _, want := range []string{"L000", "L001", "L002"} {
		require.Contains(t, after, want)
	}
}

func TestJobsOutputPansLongLines(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("0123456789", 30) + "END"
	d := openedOutput(t, outputWorkspace("short\n"+long))
	text := drawJobs(d, 80, 24)
	require.NotContains(t, text, "END", "a long line is clipped, not wrapped")
	require.Contains(t, text, "0123456789")

	for range 40 {
		d.HandleMsg(keyRight)
	}
	require.Contains(t, drawJobs(d, 80, 24), "END")
	require.NotContains(t, drawJobs(d, 80, 24), "short", "short lines scroll out of the window")

	for range 100 {
		d.HandleMsg(keyLeft)
	}
	require.Zero(t, d.view.left, "panning stops at the first column")
	require.Contains(t, drawJobs(d, 80, 24), "short")

	// Không cuộn quá cuối dòng dài nhất.
	for range 500 {
		d.HandleMsg(keyMsg('l'))
	}
	require.Less(t, d.view.left, len(long))
}

// Phím trong khung xem không lọt ra danh sách bên dưới.
func TestJobsOutputKeysDoNotMoveTheListSelection(t *testing.T) {
	t.Parallel()

	d := openedOutput(t, outputWorkspace(numberedLines(100)))
	drawJobs(d, 100, 24)
	for _, k := range []tea.KeyPressMsg{keyDown, keyUp, keyMsg('j'), keyMsg('k'), {Code: tea.KeyPgDown}, {Code: tea.KeyEnd}} {
		d.HandleMsg(k)
		require.Equal(t, "003", d.selectedID, k.String())
	}
}

// --- Log dài ---

// Output tới vài MB (tối đa mà server giữ lại) không làm chậm vòng vẽ: việc
// nặng xong trong Cmd, mỗi khung hình chỉ cắt các dòng đang hiện.
func TestJobsLongLogDoesNotHangTheUI(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	// ~1,2MB toàn dòng ngắn: nhiều dòng nhất mà đuôi 1MB của server có thể
	// chứa.
	manyLines := strings.Repeat("y\n", 600_000)
	start := time.Now()
	lines := prepareJobOutput(manyLines, s.ANSI)
	require.Len(t, lines, 600_000)
	t.Logf("prepared %d lines in %s", len(lines), time.Since(start))

	// Một dòng khổng lồ bị cắt, không kéo cả khung hình.
	huge := prepareJobOutput(strings.Repeat("a", 1<<20), s.ANSI)
	require.Len(t, huge, 1)
	require.LessOrEqual(t, ansi.StringWidth(huge[0]), jobsOutputMaxLineCells)
	require.Less(t, len(huge[0]), 1<<14)

	ws := outputWorkspace("")
	d := openedOutput(t, ws)
	// Đưa thẳng kết quả đã xử lý vào hộp thoại: phần của Update chỉ gán
	// slice.
	start = time.Now()
	d.HandleMsg(jobsOutputMsg{
		jobsMsg: jobsMsg{d.instance}, view: d.view.seq, jobID: "003",
		raw: manyLines, lines: lines, longest: longestLine(lines),
	})
	handle := time.Since(start)
	t.Logf("handling the result took %s", handle)

	start = time.Now()
	for range 30 {
		d.HandleMsg(keyUp)
		text := drawJobs(d, 120, 40)
		require.Contains(t, text, "y")
	}
	t.Logf("30 scroll+draw cycles took %s", time.Since(start))

	// Mốc rộng để bắt hồi quy O(n) mỗi khung hình (vd tính chiều rộng mọi
	// dòng hay gọi SetContent của viewport), không phải để đo hiệu năng.
	require.Less(t, handle, 500*time.Millisecond)
	require.Less(t, time.Since(start), 5*time.Second)
}

// --- Khung xem dùng được ở mọi cỡ ---

func TestJobsOutputFitsSmallTerminals(t *testing.T) {
	t.Parallel()

	d := openedOutput(t, outputWorkspace(numberedLines(200)))
	for _, size := range [][2]int{{30, 10}, {50, 12}, {80, 24}, {120, 40}, {200, 80}} {
		text := drawJobs(d, size[0], size[1])
		lines := strings.Split(text, "\n")
		require.LessOrEqual(t, len(lines), size[1])
		for _, line := range lines {
			require.LessOrEqual(t, ansi.StringWidth(line), size[0], "%v: %q", size, line)
		}
	}
}

// Workspace thôi báo job đang xem (server khởi động lại, lịch sử bị cắt):
// khung xem không còn hiện "running" giả và không gọi dồn output nữa; job báo
// lại thì theo dõi tiếp (roadmap P1.4: mất runtime thì đánh dấu, không hiện
// running giả).
func TestJobsOutputOfAJobThatDisappearsIsNotShownAsRunning(t *testing.T) {
	t.Parallel()

	ws := outputWorkspace("served 3 requests")
	d := openedOutput(t, ws)
	require.Contains(t, drawJobs(d, 100, 24), "running")

	// Danh sách mới không còn job 003. Lấy nốt output một lần rồi thôi.
	settle(d, feed(d, jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: jobsFixture()[1:]}))
	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "no longer listed")
	require.NotContains(t, text, "● running")
	require.NotContains(t, text, "following", "a vanished job has no live tail")
	require.Contains(t, text, "served 3 requests", "what was read stays on screen")

	require.NotContains(t, text, "ctrl+k stop")
	infos := msgsOf[util.InfoMsg](press(d, keyStop))
	require.Len(t, infos, 1)
	require.Contains(t, infos[0].Msg, "is not running")
	require.Nil(t, d.confirm)

	_, settled, _ := ws.counts()
	ws.jobs = jobsFixture()[1:]
	for range 4 {
		tickAll(d)
	}
	_, after, _ := ws.counts()
	require.Equal(t, settled, after, "no more output polling for a job the workspace no longer lists")

	// Job báo lại (kết nối lại): theo dõi tiếp.
	ws.jobs = jobsFixture()
	ws.outputs["003"] = "served 4 requests"
	for range 2 {
		tickAll(d)
	}
	text = drawJobs(d, 100, 24)
	require.Contains(t, text, "running")
	require.Contains(t, text, "served 4 requests")
	require.NotContains(t, text, "no longer listed")
}
