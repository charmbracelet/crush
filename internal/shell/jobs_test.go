package shell

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func waitJob(t *testing.T, bg *BackgroundShell) {
	t.Helper()
	select {
	case <-bg.done:
	case <-time.After(10 * time.Second):
		t.Fatal("job did not finish")
	}
}

// findJob tìm job id trong danh sách Jobs(root).
func findJob(t *testing.T, m *BackgroundShellManager, root, id string) JobInfo {
	t.Helper()
	var got JobInfo
	require.Eventually(t, func() bool {
		for _, j := range m.Jobs(root) {
			if j.ID == id {
				got = j
				return true
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond, "job %s not listed", id)
	return got
}

// Job ghi chủ (session, tool call), giờ bắt đầu; xong thì trạng thái, mã
// thoát, giờ kết thúc đúng và còn trong lịch sử dù đã gỡ khỏi bảng job (như
// lệnh chạy-rồi-chờ của tool bash).
func TestJobsRecordOwnerStatusAndHistory(t *testing.T) {
	t.Parallel()
	m := newBackgroundShellManager()
	dir := t.TempDir()
	owner := JobOwner{SessionID: "s1", ToolCallID: "call-1"}

	ok, err := m.StartFor(t.Context(), owner, dir, nil, "echo hi", "say hi")
	require.NoError(t, err)
	bad, err := m.StartFor(t.Context(), owner, dir, nil, "exit 3", "fail")
	require.NoError(t, err)
	waitJob(t, ok)
	waitJob(t, bad)
	require.NoError(t, m.Remove(ok.ID)) // Như tool bash sau lệnh chạy-rồi-chờ.

	j := findJob(t, m, dir, ok.ID)
	require.Equal(t, JobSucceeded, j.Status)
	require.Equal(t, "s1", j.SessionID)
	require.Equal(t, "call-1", j.ToolCallID)
	require.Equal(t, "say hi", j.Description)
	require.False(t, j.StartedAt.IsZero())
	require.False(t, j.FinishedAt.Before(j.StartedAt))
	require.False(t, j.Background)

	f := findJob(t, m, dir, bad.ID)
	require.Equal(t, JobFailed, f.Status)
	require.Equal(t, 3, f.ExitCode)

	out, err := m.JobOutput(ok.ID)
	require.NoError(t, err, "output of a finished foreground job stays readable")
	require.Equal(t, "hi", strings.TrimSpace(out))
}

// Job chạy nền: liệt kê là running, có cờ bg; bị dừng thì trạng thái killed.
func TestJobsBackgroundAndKilled(t *testing.T) {
	t.Parallel()
	m := newBackgroundShellManager()
	dir := t.TempDir()
	bg, err := m.StartFor(t.Context(), JobOwner{SessionID: "s2"}, dir, nil, "sleep 30", "")
	require.NoError(t, err)
	m.MarkBackground(bg.ID)

	j := findJob(t, m, dir, bg.ID)
	require.Equal(t, JobRunning, j.Status)
	require.True(t, j.Background)
	require.Positive(t, j.Elapsed(time.Now().Add(time.Second)))

	require.NoError(t, m.Kill(bg.ID))
	k := findJob(t, m, dir, bg.ID)
	require.Equal(t, JobKilled, k.Status)
}

// Jobs chỉ trả job trong thư mục của workspace (server có thể giữ nhiều
// workspace, trình quản lý job là chung), đang chạy trước.
func TestJobsFilteredByWorkspaceRoot(t *testing.T) {
	t.Parallel()
	m := newBackgroundShellManager()
	mine, other := t.TempDir(), t.TempDir()
	done, err := m.StartFor(context.Background(), JobOwner{}, mine, nil, "true", "")
	require.NoError(t, err)
	waitJob(t, done)
	running, err := m.StartFor(context.Background(), JobOwner{}, mine, nil, "sleep 30", "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Kill(running.ID) })
	foreign, err := m.StartFor(context.Background(), JobOwner{}, other, nil, "true", "")
	require.NoError(t, err)
	waitJob(t, foreign)

	require.Eventually(t, func() bool { return len(m.Jobs(mine)) == 2 }, 5*time.Second, 10*time.Millisecond)
	jobs := m.Jobs(mine)
	require.Equal(t, running.ID, jobs[0].ID, "running first")
	require.Equal(t, done.ID, jobs[1].ID)
	for _, j := range jobs {
		require.NotEqual(t, foreign.ID, j.ID)
	}
}

// Output quá dài: giữ đầu và đuôi, ghi số byte bỏ ở giữa; bộ nhớ có trần.
func TestCappedBufferKeepsHeadAndTail(t *testing.T) {
	// Không parallel: đổi trần cấp gói.
	prevHead, prevTail := outputHeadCap, outputTailCap
	outputHeadCap, outputTailCap = 10, 20
	t.Cleanup(func() { outputHeadCap, outputTailCap = prevHead, prevTail })

	var b cappedBuffer
	for i := range 100 {
		_, _ = fmt.Fprintf(&b, "%03d|", i) // 4 byte mỗi lần, 400 byte tổng.
	}
	require.EqualValues(t, 400, b.total())
	require.EqualValues(t, 400-10-20, b.dropped())
	s := b.String()
	require.True(t, strings.HasPrefix(s, "000|001|00"), s)
	require.True(t, strings.HasSuffix(s, "095|096|097|098|099|"), s)
	require.Contains(t, s, "[... 370 bytes of output dropped ...]")
	require.LessOrEqual(t, cap(b.tail), 4*outputTailCap+64, "tail memory stays bounded")
}

// Stop (người dùng dừng job) để job lại trong bảng cho job_output của agent;
// job đã xong thì không làm gì; id lạ thì báo lỗi.
func TestStopKeepsJobForAgent(t *testing.T) {
	t.Parallel()
	m := newBackgroundShellManager()
	dir := t.TempDir()
	bg, err := m.StartFor(t.Context(), JobOwner{}, dir, nil, "sleep 30", "")
	require.NoError(t, err)

	require.NoError(t, m.Stop(t.Context(), bg.ID))
	require.True(t, bg.IsDone())
	got, ok := m.Get(bg.ID)
	require.True(t, ok)
	_, _, done, _ := got.GetOutput()
	require.True(t, done)
	require.Equal(t, JobKilled, findJob(t, m, dir, bg.ID).Status)

	require.NoError(t, m.Stop(t.Context(), bg.ID), "already finished")
	require.Error(t, m.Stop(t.Context(), "nope"))
}

// Job xong tự nhiên rồi mới bị Kill vẫn giữ trạng thái thật trong lịch sử.
func TestKillAfterNaturalExitKeepsStatus(t *testing.T) {
	t.Parallel()
	m := newBackgroundShellManager()
	dir := t.TempDir()
	bg, err := m.StartFor(t.Context(), JobOwner{}, dir, nil, "true", "")
	require.NoError(t, err)
	waitJob(t, bg)
	require.NoError(t, m.Kill(bg.ID))
	require.Equal(t, JobSucceeded, findJob(t, m, dir, bg.ID).Status)
}

// Port upstream 0ac04e4f: chỉ job đang chạy chiếm chỗ trong trần; job đã xong
// không chặn lệnh mới, và bảng chỉ giữ maxFinishedJobs job đã xong mới nhất.
func TestJobBudgetCountsOnlyRunningJobs(t *testing.T) {
	// Không parallel: đổi trần cấp gói.
	prevRun, prevFin := maxRunningJobs, maxFinishedJobs
	maxRunningJobs, maxFinishedJobs = 2, 3
	t.Cleanup(func() { maxRunningJobs, maxFinishedJobs = prevRun, prevFin })

	m := newBackgroundShellManager()
	dir := t.TempDir()
	var finished []*BackgroundShell
	for range 5 {
		bg, err := m.StartFor(t.Context(), JobOwner{}, dir, nil, "true", "")
		require.NoError(t, err, "finished jobs must not use up the budget")
		waitJob(t, bg)
		finished = append(finished, bg)
	}

	for range 2 {
		bg, err := m.StartFor(t.Context(), JobOwner{}, dir, nil, "sleep 30", "")
		require.NoError(t, err)
		t.Cleanup(func() { _ = m.Kill(bg.ID) })
	}
	_, err := m.StartFor(t.Context(), JobOwner{}, dir, nil, "true", "")
	require.ErrorContains(t, err, "maximum number of running background jobs (2)")

	// Lần Start trước đã dọn: còn 3 job xong mới nhất, 2 cũ nhất bị bỏ.
	for i, bg := range finished {
		_, ok := m.Get(bg.ID)
		require.Equal(t, i >= 2, ok, "job %d", i)
	}
}
