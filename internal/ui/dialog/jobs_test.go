package dialog

import (
	"context"
	"errors"
	"fmt"
	"image"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

// jobsWorkspace là workspace giả chỉ cài ba hàm job. Interface nhúng để nil:
// hộp thoại mà gọi hàm nào khác là panic, nên test cũng chứng minh nó chỉ
// dùng ba hàm này. Bộ đếm cho biết lời gọi nào đã thật sự chạy: hộp thoại chỉ
// được gọi workspace từ trong tea.Cmd, không từ HandleMsg hay Draw.
type jobsWorkspace struct {
	workspace.Workspace

	mu        sync.Mutex
	jobs      []shell.JobInfo
	listErr   error
	outputs   map[string]string
	outputErr error
	killErr   error

	listCalls   int
	outputCalls int
	killed      []string
	noDeadline  int // Số lời gọi không kèm hạn chót.
}

func (w *jobsWorkspace) ListJobs(ctx context.Context) ([]shell.JobInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.listCalls++
	w.noteDeadline(ctx)
	if w.listErr != nil {
		return nil, w.listErr
	}
	return slices.Clone(w.jobs), nil
}

func (w *jobsWorkspace) JobOutput(ctx context.Context, id string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.outputCalls++
	w.noteDeadline(ctx)
	if w.outputErr != nil {
		return "", w.outputErr
	}
	return w.outputs[id], nil
}

func (w *jobsWorkspace) KillJob(ctx context.Context, id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.noteDeadline(ctx)
	w.killed = append(w.killed, id)
	return w.killErr
}

func (w *jobsWorkspace) noteDeadline(ctx context.Context) {
	if _, ok := ctx.Deadline(); !ok {
		w.noDeadline++
	}
}

func (w *jobsWorkspace) counts() (list, output, kill int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.listCalls, w.outputCalls, len(w.killed)
}

var jobsNow = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) time.Time { return jobsNow.Add(-d) }

var jobsTitles = map[string]string{"s1": "Fix login bug", "s2": "Add pagination"}

// jobsFixture: một job chạy nền, một xong, một lỗi, một bị dừng, theo thứ tự
// workspace trả (đang chạy trước, mới nhất trước).
func jobsFixture() []shell.JobInfo {
	return []shell.JobInfo{
		{
			ID: "003", SessionID: "s1", Command: "cd /w && npm run dev", WorkingDir: "/w",
			Description: "Dev server", StartedAt: ago(95 * time.Second),
			Status: shell.JobRunning, Background: true,
		},
		{
			ID: "002", SessionID: "s2", Command: "go test ./...",
			StartedAt: ago(time.Hour), FinishedAt: ago(time.Hour - 62*time.Second),
			Status: shell.JobSucceeded,
		},
		{
			ID: "001", SessionID: "deadbeef-0000-4000-8000-000000000000", Command: "make build",
			StartedAt: ago(2 * time.Hour), FinishedAt: ago(2*time.Hour - 3*time.Second),
			Status: shell.JobFailed, ExitCode: 2,
		},
		{
			ID: "000", SessionID: "s1", Command: "sleep 600",
			StartedAt: ago(3 * time.Hour), FinishedAt: ago(3*time.Hour - 45*time.Second),
			Status: shell.JobKilled, Background: true,
		},
	}
}

// newTestJobs dựng hộp thoại với nhịp 1µs (tea.Tick chạy ngay trong test) và
// đồng hồ cố định.
func newTestJobs(ws *jobsWorkspace) (*Jobs, tea.Cmd) {
	s := styles.CharmtonePantera()
	com := &common.Common{Styles: &s, Workspace: ws}
	return newJobs(com, jobsTitles, time.Microsecond, func() time.Time { return jobsNow })
}

// runCmd chạy cây Cmd như vòng lặp Bubble Tea và trả message của từng lá.
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, runCmd(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// feed đưa một message cho hộp thoại và chạy Cmd mà nó trả về.
func feed(d *Jobs, msg tea.Msg) []tea.Msg {
	if ac, ok := d.HandleMsg(msg).(ActionCmd); ok {
		return runCmd(ac.Cmd)
	}
	return nil
}

func msgsOf[T any](msgs []tea.Msg) []T {
	var out []T
	for _, m := range msgs {
		if v, ok := m.(T); ok {
			out = append(out, v)
		}
	}
	return out
}

// loadedJobs mở hộp thoại và đưa kết quả nạp đầu tiên vào.
func loadedJobs(t *testing.T, ws *jobsWorkspace) *Jobs {
	t.Helper()
	d, cmd := newTestJobs(ws)
	loaded := msgsOf[jobsLoadedMsg](runCmd(cmd))
	require.Len(t, loaded, 1)
	d.HandleMsg(loaded[0])
	return d
}

func drawJobs(d *Jobs, w, h int) string {
	scr := uv.NewScreenBuffer(w, h)
	d.Draw(scr, image.Rect(0, 0, w, h))
	return screenText(scr)
}

func lineWith(t *testing.T, text, snippet string) string {
	t.Helper()
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, snippet) {
			return line
		}
	}
	t.Fatalf("no line contains %q in:\n%s", snippet, text)
	return ""
}

func lineIndex(text, snippet string) int {
	for i, line := range strings.Split(text, "\n") {
		if strings.Contains(line, snippet) {
			return i
		}
	}
	return -1
}

// jobsCtrl là phím Ctrl+<r> như terminal gửi lên.
func jobsCtrl(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

var (
	keyUp    = tea.KeyPressMsg{Code: tea.KeyUp}
	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	keyLeft  = tea.KeyPressMsg{Code: tea.KeyLeft}
	keyRight = tea.KeyPressMsg{Code: tea.KeyRight}
	keyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	keyStop  = jobsCtrl('k')
)

// --- Danh sách: thứ tự và cách vẽ ---

func TestJobsListRendersEachJob(t *testing.T) {
	t.Parallel()

	d := loadedJobs(t, &jobsWorkspace{jobs: jobsFixture()})
	text := drawJobs(d, 100, 24)

	require.Contains(t, text, "Jobs")
	require.Contains(t, text, "1 running · 3 finished")

	running := lineWith(t, text, "Dev server") // Description beats the command.
	require.Contains(t, running, "●")
	require.Contains(t, running, "003", "the job ID, as in `agent-cli jobs` and the chat's Job header")
	require.Contains(t, running, "1m35s", "elapsed = now - StartedAt")
	require.Contains(t, running, "bg", "a background job is marked")
	require.Contains(t, running, "Fix login bug", "the owning session's title")
	require.NotContains(t, text, "npm run dev", "the description is shown, not the command")

	done := lineWith(t, text, "go test ./...") // No description: the command.
	require.Contains(t, done, "✓")
	require.Contains(t, done, "1m02s", "elapsed = FinishedAt - StartedAt")
	require.Contains(t, done, "Add pagination")
	require.NotContains(t, done, "bg")

	failed := lineWith(t, text, "make build")
	require.Contains(t, failed, "×")
	require.Contains(t, failed, "exit 2", "a failed job shows its exit code")
	require.Contains(t, failed, "deadbeef", "an unknown session shows its short ID")
	require.NotContains(t, failed, "deadbeef-0000", "only a short ID")

	killed := lineWith(t, text, "sleep 600")
	require.Contains(t, killed, "■")
	require.Contains(t, killed, "killed")
	require.Contains(t, killed, "bg")
	require.Contains(t, killed, "45s")
}

// Đang chạy luôn đứng trước, kể cả khi server trả thứ tự khác; trong mỗi nhóm
// giữ thứ tự server (mới nhất trước).
func TestJobsRunningFirstThenNewestFirst(t *testing.T) {
	t.Parallel()

	jobs := jobsFixture()
	jobs[1].ID, jobs[1].Command = "run2", "second runner"
	jobs[1].Status, jobs[1].FinishedAt, jobs[1].StartedAt = shell.JobRunning, time.Time{}, ago(10*time.Second)
	// Server cũ trả các job đã xong trước.
	reordered := []shell.JobInfo{jobs[2], jobs[3], jobs[1], jobs[0]}
	d := loadedJobs(t, &jobsWorkspace{jobs: reordered})
	text := drawJobs(d, 100, 24)

	order := []string{"second runner", "Dev server", "make build", "sleep 600"}
	last := -1
	for _, want := range order {
		i := lineIndex(text, want)
		require.Greater(t, i, last, "%q must come after the previous row:\n%s", want, text)
		last = i
	}
}

// Thời gian chạy tính lúc vẽ: cùng một dữ liệu, đồng hồ nhích thì chữ nhích;
// job đã xong thì đứng yên.
func TestJobsElapsedTicksAtRender(t *testing.T) {
	t.Parallel()

	d := loadedJobs(t, &jobsWorkspace{jobs: jobsFixture()})
	before := drawJobs(d, 100, 24)
	require.Contains(t, lineWith(t, before, "Dev server"), "1m35s")

	d.now = func() time.Time { return jobsNow.Add(30 * time.Second) }
	after := drawJobs(d, 100, 24)
	require.Contains(t, lineWith(t, after, "Dev server"), "2m05s")
	require.Contains(t, lineWith(t, after, "go test ./..."), "1m02s", "a finished job's time stops at FinishedAt")
}

func TestFormatJobElapsed(t *testing.T) {
	t.Parallel()

	for in, want := range map[time.Duration]string{
		-5 * time.Second:                "0s",
		0:                               "0s",
		999 * time.Millisecond:          "0s",
		12 * time.Second:                "12s",
		59 * time.Second:                "59s",
		60 * time.Second:                "1m00s",
		3*time.Minute + 5*time.Second:   "3m05s",
		59*time.Minute + 59*time.Second: "59m59s",
		time.Hour:                       "1h00m",
		2*time.Hour + 3*time.Minute:     "2h03m",
		23*time.Hour + 59*time.Minute:   "23h59m",
		24 * time.Hour:                  "1d00h",
		26*time.Hour + 30*time.Minute:   "1d02h",
		1000 * 24 * time.Hour:           "99d00h",
		48*time.Hour + 59*time.Minute + time.Second: "2d00h",
	} {
		require.Equal(t, want, formatJobElapsed(in), in.String())
		require.LessOrEqual(t, len(formatJobElapsed(in)), jobsElapsedWidth, in.String())
	}
}

func TestJobsSessionLabels(t *testing.T) {
	t.Parallel()

	d := loadedJobs(t, &jobsWorkspace{})
	require.Equal(t, "Fix login bug", d.sessionLabel("s1"))
	require.Equal(t, "deadbeef", d.sessionLabel("deadbeef-0000-4000"))
	require.Equal(t, "abc", d.sessionLabel("abc"), "a short ID stays whole")
	require.Equal(t, "-", d.sessionLabel(""), "a job without an owner")

	d.SetSessionTitles(map[string]string{"deadbeef-0000-4000": "  Renamed\n session  "})
	require.Equal(t, "Renamed session", d.sessionLabel("deadbeef-0000-4000"), "the UI refreshes titles while the dialog is open")
	require.Equal(t, "s1", d.sessionLabel("s1"), "a session missing from the new snapshot falls back to its ID")
}

// Mô tả trống thì dùng lệnh, bỏ tiền tố "cd <thư mục> && " và gộp xuống dòng.
func TestJobLabel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "make all", jobLabel(shell.JobInfo{Command: "cd /w && make all", WorkingDir: "/w"}))
	require.Equal(t, "cd /other && make all", jobLabel(shell.JobInfo{Command: "cd /other && make all", WorkingDir: "/w"}))
	require.Equal(t, "echo a b", jobLabel(shell.JobInfo{Command: "echo\ta\n  b\n"}))
	require.Equal(t, "Build the thing", jobLabel(shell.JobInfo{Command: "make", Description: "  Build\nthe thing "}))
}

// Lệnh, mô tả và tiêu đề là chữ do model hay người viết: mã ANSI và ký tự điều
// khiển trong đó không đi vào khung hình.
func TestJobsLabelsAreSanitized(t *testing.T) {
	t.Parallel()

	require.Equal(t, "red text", joinFields("\x1b[31mred\x1b[0m\ttext"))
	require.Equal(t, "ab", joinFields("a\x00b\x07"))
	require.Equal(t, "title", joinFields("\x1b]0;evil window title\x07title"))
	require.Equal(t, "xin chào, thế giới", joinFields("xin  chào,\nthế\tgiới"), "non-ASCII text is untouched")

	job := shell.JobInfo{
		ID: "007", SessionID: "s1", Status: shell.JobRunning, StartedAt: ago(time.Second),
		Description: "\x1b[2J\x1b[31mscary\x1b[0m description",
		Command:     "echo \x1b[1;1H hi",
	}
	d := loadedJobs(t, &jobsWorkspace{jobs: []shell.JobInfo{job}})
	d.SetSessionTitles(map[string]string{"s1": "\x1b[41mtitle\x1b[0m"})
	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "scary description")
	require.Contains(t, text, "title")
	require.NotContains(t, text, "\x1b")
	require.NotContains(t, text, "[31m")
}

func TestJobsPlaceholders(t *testing.T) {
	t.Parallel()

	// Chưa có kết quả nạp nào: đang nạp, chưa phải "trống".
	ws := &jobsWorkspace{}
	d, _ := newTestJobs(ws)
	text := drawJobs(d, 80, 20)
	require.Contains(t, text, "Loading jobs")
	require.NotContains(t, text, "No jobs yet")

	// Nạp xong mà không có job.
	d = loadedJobs(t, ws)
	text = drawJobs(d, 80, 20)
	require.Contains(t, text, "No jobs yet.")
	require.NotContains(t, text, "Loading jobs")

	// Nạp lỗi ngay lần đầu: hiện lỗi, không giả vờ là trống.
	ws = &jobsWorkspace{listErr: errors.New("server unreachable")}
	d = loadedJobs(t, ws)
	text = drawJobs(d, 80, 20)
	require.Contains(t, text, "Could not list jobs: server unreachable")
	require.NotContains(t, text, "No jobs yet")
}

// Màn hẹp: hộp thoại không tràn vùng vẽ, mô tả vẫn đọc được, cột session
// nhường chỗ.
func TestJobsListFitsNarrowTerminals(t *testing.T) {
	t.Parallel()

	d := loadedJobs(t, &jobsWorkspace{jobs: jobsFixture()})
	for _, width := range []int{30, 40, 50, 80, 200} {
		text := drawJobs(d, width, 14)
		for line := range strings.SplitSeq(text, "\n") {
			require.LessOrEqual(t, ansi.StringWidth(line), width, "width %d: %q", width, line)
		}
		require.Contains(t, text, "Dev ", "width %d", width)
		if width >= 40 {
			require.Contains(t, text, "Dev server", "width %d", width)
		}
	}
	require.NotContains(t, drawJobs(d, 36, 14), "Fix login bug", "no room for the session column")
	require.Contains(t, drawJobs(d, 100, 14), "Fix login bug")
}

// Tiêu đề session ngắn ("OK") vẫn hiện: cột session chỉ bị bỏ khi bị ép hẹp,
// không phải vì nội dung vốn ngắn. Bắt được ở smoke test với TUI thật.
func TestJobsShortSessionTitleStillShows(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()[:1]}
	d := loadedJobs(t, ws)
	d.SetSessionTitles(map[string]string{"s1": "OK"})
	require.Contains(t, lineWith(t, drawJobs(d, 130, 24), "Dev server"), "OK")

	// Job không có chủ (SessionID rỗng) hiện "-".
	ws.jobs[0].SessionID = ""
	d.HandleMsg(jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: ws.jobs})
	row := lineWith(t, drawJobs(d, 130, 24), "Dev server")
	require.Regexp(t, `Dev server\s+-\s*│`, row)
}

// Màn hẹp bỏ cột phụ theo thứ tự: session trước, rồi ID, rồi nhãn; mô tả luôn
// đọc được.
func TestJobsNarrowColumnsDropInOrder(t *testing.T) {
	t.Parallel()

	d := loadedJobs(t, &jobsWorkspace{jobs: jobsFixture()})
	row := func(width int) string {
		return lineWith(t, drawJobs(d, width, 14), "Dev ")
	}
	wide := row(100)
	require.Contains(t, wide, "003")
	require.Contains(t, wide, "bg")
	require.Contains(t, wide, "Fix login bug")

	noSession := row(48)
	require.Contains(t, noSession, "003")
	require.Contains(t, noSession, "bg")
	require.NotContains(t, noSession, "Fix login")

	noID := row(40)
	require.NotContains(t, noID, "003")
	require.Contains(t, noID, "bg")
	require.Contains(t, noID, "Dev server")

	noTags := row(34)
	require.NotContains(t, noTags, "bg")
	require.NotContains(t, noTags, "003")
	require.Contains(t, noTags, "Dev server")
}

// Quá nhiều job: cuộn theo lựa chọn, có thanh cuộn, và vẫn vừa chiều cao.
func TestJobsListScrollsWithSelection(t *testing.T) {
	t.Parallel()

	var jobs []shell.JobInfo
	for i := range 40 {
		jobs = append(jobs, shell.JobInfo{
			ID: fmt.Sprintf("%03d", i), SessionID: "s1", Command: fmt.Sprintf("job-number-%02d", i),
			StartedAt: ago(time.Hour), FinishedAt: ago(time.Hour - time.Second), Status: shell.JobSucceeded,
		})
	}
	d := loadedJobs(t, &jobsWorkspace{jobs: jobs})
	text := drawJobs(d, 100, 20)
	require.Contains(t, text, "job-number-00")
	require.NotContains(t, text, "job-number-39")
	require.LessOrEqual(t, len(strings.Split(text, "\n")), 20)
	require.Contains(t, text, "┃", "a scrollbar when rows are hidden")

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnd})
	require.Equal(t, "039", d.selectedID)
	text = drawJobs(d, 100, 20)
	require.Contains(t, text, "job-number-39")
	require.NotContains(t, text, "job-number-00")

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyHome})
	require.Equal(t, "000", d.selectedID)
	require.Contains(t, drawJobs(d, 100, 20), "job-number-00")

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyPgDown})
	require.Greater(t, d.selectedIndex(), 5, "a page moves several rows")
}

// --- normalizeJobs: bỏ trùng, đang chạy trước ---

func TestNormalizeJobs(t *testing.T) {
	t.Parallel()

	run := func(id string) shell.JobInfo { return shell.JobInfo{ID: id, Status: shell.JobRunning} }
	done := func(id string) shell.JobInfo { return shell.JobInfo{ID: id, Status: shell.JobSucceeded} }
	ids := func(js []shell.JobInfo) []string {
		out := make([]string, len(js))
		for i, j := range js {
			out[i] = j.ID
		}
		return out
	}

	// Khoảnh khắc job vừa xong, server liệt kê nó hai lần (đang chạy và lịch
	// sử): giữ bản đã xong, không nhân đôi dòng.
	got := normalizeJobs([]shell.JobInfo{run("a"), run("b"), done("b"), done("c")})
	require.Equal(t, []string{"a", "b", "c"}, ids(got))
	require.Equal(t, shell.JobRunning, got[0].Status)
	require.Equal(t, shell.JobSucceeded, got[1].Status, "the finished copy wins")

	// Nạp lại (kết nối lại) cùng danh sách hai lần liền không nhân đôi.
	again := normalizeJobs(slices.Concat(got, got))
	require.Equal(t, []string{"a", "b", "c"}, ids(again))

	// Đang chạy trước, thứ tự trong nhóm giữ nguyên.
	got = normalizeJobs([]shell.JobInfo{done("x"), done("y"), run("p"), done("z"), run("q")})
	require.Equal(t, []string{"p", "q", "x", "y", "z"}, ids(got))

	require.Empty(t, normalizeJobs(nil))
}

// Danh sách nạp lại nhiều lần (nhịp, kết nối lại) không bao giờ nhân đôi
// dòng.
func TestJobsReloadDoesNotDuplicateRows(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	for range 3 {
		d.HandleMsg(jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: slices.Concat(ws.jobs, ws.jobs)})
	}
	require.Len(t, d.jobs, 4)
	require.Equal(t, 1, strings.Count(drawJobs(d, 100, 24), "Dev server"))
}

// --- Lựa chọn ổn định ---

func TestJobsSelectionFollowsTheJobNotTheRow(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	require.Equal(t, "003", d.selectedID, "the first row is selected at first")

	d.HandleMsg(keyDown)
	d.HandleMsg(keyDown)
	require.Equal(t, "001", d.selectedID)

	// Một job mới chạy chen lên đầu: lựa chọn vẫn ở job cũ.
	fresh := shell.JobInfo{ID: "004", Command: "new", StartedAt: ago(time.Second), Status: shell.JobRunning}
	d.HandleMsg(jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: append([]shell.JobInfo{fresh}, jobsFixture()...)})
	require.Equal(t, "001", d.selectedID)
	require.Equal(t, 3, d.selectedIndex())

	// Job đang chọn biến mất: lựa chọn ở lại gần vị trí cũ, không nhảy lên
	// đầu.
	d.HandleMsg(jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: []shell.JobInfo{fresh, jobsFixture()[0], jobsFixture()[1], jobsFixture()[3]}})
	require.Equal(t, "000", d.selectedID, "the row that took its place")

	// Hết job.
	d.HandleMsg(jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}})
	require.Empty(t, d.selectedID)
	require.Nil(t, d.HandleMsg(keyDown))
	require.Nil(t, d.HandleMsg(keyEnter), "enter on an empty list does nothing")
}

func TestJobsKeysMoveSelection(t *testing.T) {
	t.Parallel()

	d := loadedJobs(t, &jobsWorkspace{jobs: jobsFixture()})
	for _, tc := range []struct {
		key  tea.KeyPressMsg
		want string
	}{
		{keyDown, "002"}, {keyDown, "001"}, {keyUp, "002"},
		{keyMsg('j'), "001"}, {keyMsg('k'), "002"},
		{jobsCtrl('n'), "001"}, {jobsCtrl('p'), "002"},
		{tea.KeyPressMsg{Code: tea.KeyEnd}, "000"}, {keyDown, "000"},
		{tea.KeyPressMsg{Code: tea.KeyHome}, "003"}, {keyUp, "003"},
	} {
		d.HandleMsg(tc.key)
		require.Equal(t, tc.want, d.selectedID, tc.key.String())
	}

	d.HandleMsg(common.CoalescedWheelMsg{DeltaY: 2})
	require.Equal(t, "001", d.selectedID, "the wheel scrolls the list")
	d.HandleMsg(common.CoalescedWheelMsg{DeltaY: -1})
	require.Equal(t, "002", d.selectedID)
}

func TestJobsEscClosesAndOnlyEsc(t *testing.T) {
	t.Parallel()

	d := loadedJobs(t, &jobsWorkspace{jobs: jobsFixture()})
	require.IsType(t, ActionClose{}, d.HandleMsg(keyEsc))
	require.IsType(t, ActionClose{}, d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape, Mod: tea.ModAlt}))
	require.Nil(t, d.HandleMsg(keyMsg('q')), "no stray key closes the dialog")
	require.Equal(t, JobsID, d.ID())
}

// --- Vòng làm tươi ---

func TestJobsRefreshIsOneSecond(t *testing.T) {
	t.Parallel()

	require.Equal(t, time.Second, jobsRefreshInterval)
	s := styles.CharmtonePantera()
	d, _ := NewJobs(&common.Common{Styles: &s, Workspace: &jobsWorkspace{}}, nil)
	require.Equal(t, time.Second, d.interval)

	// Nhịp thật chờ ~1 giây (không bấm đồng hồ giả).
	start := time.Now()
	msg := d.tickCmd()()
	require.IsType(t, jobsTickMsg{}, msg)
	require.GreaterOrEqual(t, time.Since(start), 900*time.Millisecond)
	require.Less(t, time.Since(start), 5*time.Second)
}

// Dựng hộp thoại, HandleMsg và Draw không gọi workspace: chỉ Cmd mới gọi.
func TestJobsNoIOOutsideCommands(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture(), outputs: map[string]string{"003": "hello"}}
	d, cmd := newTestJobs(ws)
	require.NotNil(t, cmd)
	list, out, kill := ws.counts()
	require.Zero(t, list+out+kill, "constructing the dialog touches no workspace")

	// Một chuỗi thao tác: vẽ, bấm phím, mở output, hỏi và xác nhận dừng,
	// cuộn.
	var pending []tea.Cmd
	step := func(msg tea.Msg) {
		if ac, ok := d.HandleMsg(msg).(ActionCmd); ok && ac.Cmd != nil {
			pending = append(pending, ac.Cmd)
		}
		drawJobs(d, 100, 24)
	}
	drawJobs(d, 100, 24)
	step(jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: jobsFixture()})
	step(jobsTickMsg{jobsMsg{d.instance}})
	step(keyDown)
	step(keyUp)
	step(keyEnter) // Mở output.
	step(jobsOutputMsg{jobsMsg: jobsMsg{d.instance}, view: d.view.seq, jobID: "003", raw: "hello", lines: []string{"hello"}})
	step(jobsTickMsg{jobsMsg{d.instance}})
	step(tea.KeyPressMsg{Code: tea.KeyPgDown})
	step(keyStop)
	step(keyRight)
	step(keyEnter) // Xác nhận dừng.
	step(jobsStopMsg{jobsMsg: jobsMsg{d.instance}, jobID: "003"})
	step(common.CoalescedWheelMsg{DeltaY: 3})
	step(keyEsc)

	list, out, kill = ws.counts()
	require.Zero(t, list+out+kill, "HandleMsg and Draw never call the workspace; got list=%d output=%d kill=%d", list, out, kill)
	require.NotEmpty(t, pending, "the work was handed out as commands instead")

	for _, c := range pending {
		runCmd(c)
	}
	list, out, kill = ws.counts()
	require.Positive(t, list)
	require.Positive(t, out)
	require.Equal(t, 1, kill)
	require.Zero(t, ws.noDeadline, "every workspace call is bounded by a timeout")
}

// Nhịp tới khi lần nạp trước chưa xong thì chỉ hẹn nhịp sau, không chồng yêu
// cầu.
func TestJobsRefreshDoesNotOverlap(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d, cmd := newTestJobs(ws)

	first := runCmd(cmd) // Lần nạp đầu và nhịp đầu.
	list, _, _ := ws.counts()
	require.Equal(t, 1, list)
	loaded, ticks := msgsOf[jobsLoadedMsg](first), msgsOf[jobsTickMsg](first)
	require.Len(t, loaded, 1)
	require.Len(t, ticks, 1)

	// Nhịp tới trong lúc lần nạp đầu chưa về: chỉ giữ nhịp.
	next := feed(d, ticks[0])
	list, _, _ = ws.counts()
	require.Equal(t, 1, list, "a tick while a load is in flight must not start another")
	require.Empty(t, msgsOf[jobsLoadedMsg](next))
	require.Len(t, msgsOf[jobsTickMsg](next), 1, "the heartbeat continues")
	ticks = msgsOf[jobsTickMsg](next)

	// Thêm nhiều nhịp nữa vẫn không chồng.
	for range 5 {
		next = feed(d, ticks[0])
		ticks = msgsOf[jobsTickMsg](next)
		require.Len(t, ticks, 1)
	}
	list, _, _ = ws.counts()
	require.Equal(t, 1, list)

	// Lần nạp về thì nhịp kế mới nạp lại.
	feed(d, loaded[0])
	next = feed(d, ticks[0])
	list, _, _ = ws.counts()
	require.Equal(t, 2, list)
	require.Len(t, msgsOf[jobsLoadedMsg](next), 1)
	require.Len(t, msgsOf[jobsTickMsg](next), 1)
}

// Hộp thoại đã đóng không còn nhận nhịp; hộp thoại mở lại không nhận nhịp của
// hộp thoại cũ, nên không bao giờ có hai vòng chạy song song.
func TestJobsRefreshStopsWhenClosed(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d1, cmd := newTestJobs(ws)
	oldTick := msgsOf[jobsTickMsg](runCmd(cmd))[0]

	o := NewOverlay(d1)
	require.NotNil(t, o.Update(oldTick), "an open dialog continues the loop")
	o.CloseDialog(JobsID)
	listBefore, _, _ := ws.counts()
	require.Nil(t, o.Update(oldTick), "a closed dialog gets nothing: the loop ends")
	listAfter, _, _ := ws.counts()
	require.Equal(t, listBefore, listAfter)

	// Mở lại ngay: nhịp của hộp thoại cũ rơi vào hộp thoại mới và bị bỏ.
	d2, _ := newTestJobs(ws)
	require.NotEqual(t, d1.instance, d2.instance)
	o.OpenDialog(d2)
	require.Nil(t, o.Update(oldTick), "a stale tick must not start a second loop")
	for _, stale := range []tea.Msg{
		jobsLoadedMsg{jobsMsg: jobsMsg{d1.instance}, jobs: jobsFixture()},
		jobsOutputMsg{jobsMsg: jobsMsg{d1.instance}, raw: "x"},
		jobsStopMsg{jobsMsg: jobsMsg{d1.instance}, jobID: "003"},
	} {
		require.Nil(t, o.Update(stale))
	}
	require.Empty(t, d2.jobs, "the old dialog's results do not leak into the new one")
}

// Có hộp thoại khác mở đè (câu hỏi xin quyền) thì vòng làm tươi vẫn chạy.
func TestJobsRefreshSurvivesADialogOnTop(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d, cmd := newTestJobs(ws)
	tick := msgsOf[jobsTickMsg](runCmd(cmd))[0]
	top := &stubDialog{id: "permissions"}
	o := NewOverlay(d, top)

	act := o.Update(tick)
	require.IsType(t, ActionCmd{}, act)
	require.Empty(t, top.received, "the dialog on top must not see the tick")
	require.NotEmpty(t, msgsOf[jobsTickMsg](runCmd(act.(ActionCmd).Cmd)))
}

// Trả lời message gửi đích danh chỉ bằng ActionCmd hoặc nil: ActionClose sẽ
// đóng hộp thoại ở trên cùng, có thể là hộp thoại khác.
func TestJobsTargetedMessagesNeverReturnOtherActions(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	d.HandleMsg(keyEnter)
	for _, msg := range []tea.Msg{
		jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: jobsFixture()},
		jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, err: errors.New("x")},
		jobsTickMsg{jobsMsg{d.instance}},
		jobsOutputMsg{jobsMsg: jobsMsg{d.instance}, view: d.view.seq, jobID: "003", err: errors.New("x")},
		jobsOutputMsg{jobsMsg: jobsMsg{d.instance}, view: d.view.seq, jobID: "003", raw: "a", lines: []string{"a"}},
		jobsStopMsg{jobsMsg: jobsMsg{d.instance}, jobID: "003"},
		jobsStopMsg{jobsMsg: jobsMsg{d.instance}, jobID: "003", err: errors.New("x")},
	} {
		if _, ok := msg.(TargetedMsg); !ok {
			t.Fatalf("%T is not a TargetedMsg", msg)
		}
		switch act := d.HandleMsg(msg).(type) {
		case nil, ActionCmd:
		default:
			t.Fatalf("%T answered with %T", msg, act)
		}
	}
}

// --- Lỗi khi nạp ---

// Lỗi liên tiếp chỉ báo một lần (nhịp 1 giây mà báo hoài thì thanh trạng thái
// nháy liên tục), hết lỗi rồi lỗi lại thì báo lại; danh sách cũ vẫn hiện.
func TestJobsLoadFailureReportsOnce(t *testing.T) {
	t.Parallel()

	ws := &jobsWorkspace{jobs: jobsFixture()}
	d := loadedJobs(t, ws)
	fail := func() []tea.Msg {
		return feed(d, jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, err: errors.New("connection refused")})
	}

	reports := msgsOf[util.InfoMsg](fail())
	require.Len(t, reports, 1)
	require.Equal(t, util.InfoTypeWarn, reports[0].Type)
	require.Contains(t, reports[0].Msg, "connection refused")

	require.Empty(t, msgsOf[util.InfoMsg](fail()), "the same ongoing failure is not reported again")
	require.Empty(t, msgsOf[util.InfoMsg](fail()))

	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "Dev server", "the last good list stays on screen")
	require.Contains(t, text, "Refresh failed: connection refused")

	// Hết lỗi rồi lỗi lại.
	require.Empty(t, feed(d, jobsLoadedMsg{jobsMsg: jobsMsg{d.instance}, jobs: jobsFixture()}))
	require.NotContains(t, drawJobs(d, 100, 24), "Refresh failed")
	require.Len(t, msgsOf[util.InfoMsg](fail()), 1)
}

// --- Gợi ý phím ---

func TestJobsHelpFollowsTheSelection(t *testing.T) {
	t.Parallel()

	d := loadedJobs(t, &jobsWorkspace{jobs: jobsFixture()})
	help := func() string {
		var parts []string
		for _, b := range d.ShortHelp() {
			if b.Enabled() {
				parts = append(parts, b.Help().Key+" "+b.Help().Desc)
			}
		}
		return strings.Join(parts, "|")
	}

	require.Contains(t, help(), "ctrl+k stop", "a running job can be stopped")
	require.Contains(t, help(), "enter output")
	d.HandleMsg(keyDown) // Job đã xong.
	require.NotContains(t, help(), "ctrl+k", "the stop hint is hidden for a finished job")

	text := drawJobs(d, 100, 24)
	require.Contains(t, text, "enter output")
	require.NotContains(t, text, "ctrl+k stop")
	d.HandleMsg(keyUp)
	require.Contains(t, drawJobs(d, 100, 24), "ctrl+k stop")
}

// Vùng vẽ bé xíu (cửa sổ terminal bị kéo hẹp) không được làm hộp thoại panic,
// ở cả ba màn: danh sách, khung xem output và câu hỏi dừng job.
func TestJobsDrawSurvivesTinyAreas(t *testing.T) {
	t.Parallel()

	ws := outputWorkspace(numberedLines(50))
	for _, size := range [][2]int{{0, 0}, {1, 1}, {2, 2}, {5, 3}, {8, 4}, {12, 5}, {20, 6}, {30, 8}} {
		w, h := size[0], size[1]
		list := loadedJobs(t, ws)
		drawJobs(list, w, h)

		list.HandleMsg(keyStop)
		drawJobs(list, w, h) // Câu hỏi dừng job trên danh sách.

		out := openedOutput(t, ws)
		drawJobs(out, w, h)
		out.HandleMsg(keyStop)
		drawJobs(out, w, h) // Câu hỏi dừng job trên khung xem.

		failed := loadedJobs(t, &jobsWorkspace{listErr: errors.New("a rather long failure message")})
		drawJobs(failed, w, h)
	}
}
