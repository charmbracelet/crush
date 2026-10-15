package shell

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// JobStatus là trạng thái của một job (lệnh bash do agent chạy).
type JobStatus string

const (
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobKilled    JobStatus = "killed"
)

// JobOwner là chủ của một job: session và tool call đã chạy nó. Rỗng khi
// người gọi không gắn (vd test, lệnh nội bộ).
type JobOwner struct {
	SessionID  string
	ToolCallID string
	// Root là thư mục gốc của workspace đã chạy job (thư mục làm việc của
	// tool), dùng để lọc job theo workspace. Rỗng thì lọc theo thư mục chạy.
	Root string
}

// JobInfo là ảnh chụp một job cho bảng Activity, `agent-cli jobs` và API.
type JobInfo struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id,omitempty"`
	ToolCallID  string    `json:"tool_call_id,omitempty"`
	Command     string    `json:"command"`
	Description string    `json:"description,omitempty"`
	WorkingDir  string    `json:"working_dir"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at,omitzero"`
	Status      JobStatus `json:"status"`
	ExitCode    int       `json:"exit_code"`
	// Background: job chạy tiếp sau khi tool bash đã trả về (chạy nền theo yêu
	// cầu hoặc tự chuyển nền khi quá lâu). False là lệnh chạy-rồi-chờ.
	Background bool `json:"background"`
	// OutputBytes là tổng số byte stdout+stderr job đã in; DroppedBytes là
	// phần giữa bị bỏ để giữ bộ nhớ có trần (xem cappedBuffer).
	OutputBytes  int64 `json:"output_bytes"`
	DroppedBytes int64 `json:"dropped_bytes,omitempty"`

	// root là gốc workspace đã giải symlink (lọc phía server, không gửi đi).
	root string
}

// Elapsed trả thời gian chạy tính tới lúc xong, hoặc tới now nếu còn chạy.
func (j JobInfo) Elapsed(now time.Time) time.Duration {
	end := j.FinishedAt
	if end.IsZero() {
		end = now
	}
	if d := end.Sub(j.StartedAt); d > 0 {
		return d
	}
	return 0
}

// jobHistorySize giới hạn số job đã xong được nhớ để hiển thị.
const jobHistorySize = 100

// jobHistory nhớ các job đã xong gần đây (mới nhất cuối), có trần.
type jobHistory struct {
	mu   sync.Mutex
	jobs []JobInfo
}

func (h *jobHistory) add(j JobInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.jobs = append(h.jobs, j)
	if len(h.jobs) > jobHistorySize {
		h.jobs = slices.Delete(h.jobs, 0, len(h.jobs)-jobHistorySize)
	}
}

func (h *jobHistory) has(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, j := range h.jobs {
		if j.ID == id {
			return true
		}
	}
	return false
}

func (h *jobHistory) snapshot() []JobInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.jobs)
}

// resolveDir trả đường dẫn sạch đã giải symlink (lỗi thì bản sạch).
func resolveDir(dir string) string {
	if dir == "" {
		return ""
	}
	clean := filepath.Clean(dir)
	if r, err := filepath.EvalSymlinks(clean); err == nil {
		return r
	}
	return clean
}

// belongsTo cho biết job thuộc workspace có gốc root (đã giải symlink).
// Job có gốc workspace thì so đúng gốc: workspace lồng nhau không thấy job
// của nhau, job chạy ở thư mục tương đối hay ngoài gốc vẫn thuộc workspace
// của nó. Job không có gốc (gọi Start không qua tool) thì so thư mục chạy.
func belongsTo(root string, j JobInfo) bool {
	if root == "" {
		return true
	}
	if j.root != "" {
		return j.root == root
	}
	return within(root, j.WorkingDir)
}

// ErrJobNotFound: không có job id (hoặc output của nó không còn được giữ).
var ErrJobNotFound = errors.New("job not found")

// within cho biết dir nằm trong root (cùng thư mục hoặc thư mục con).
func within(root, dir string) bool {
	if root == "" {
		return true
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(dir))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Jobs trả các job chạy trong root (rỗng = mọi job): đang chạy trước, rồi các
// job đã xong gần đây; mỗi nhóm mới nhất trước. Server có thể giữ nhiều
// workspace, nên lọc theo thư mục làm việc.
func (m *BackgroundShellManager) Jobs(root string) []JobInfo {
	root = resolveDir(root)
	var running []JobInfo
	for bg := range m.shells.Seq() {
		if bg.IsDone() {
			continue
		}
		if j := bg.Info(); belongsTo(root, j) {
			running = append(running, j)
		}
	}
	// Chụp lịch sử SAU bảng job: job xong giữa hai lần chụp đã nằm trong lịch
	// sử (ghi trước khi done đóng), nên bỏ bản "running" cũ của nó.
	history := m.history.snapshot()
	finishedIDs := make(map[string]struct{}, len(history))
	var finished []JobInfo
	for _, j := range history {
		finishedIDs[j.ID] = struct{}{}
		if belongsTo(root, j) {
			finished = append(finished, j)
		}
	}
	running = slices.DeleteFunc(running, func(j JobInfo) bool {
		_, ok := finishedIDs[j.ID]
		return ok
	})
	slices.SortFunc(running, func(a, b JobInfo) int { return b.StartedAt.Compare(a.StartedAt) })
	slices.Reverse(finished)
	return append(running, finished...)
}

// JobOutput trả output của job id (đang chạy hoặc đã xong và còn được giữ):
// stdout, rồi stderr nếu có.
func (m *BackgroundShellManager) JobOutput(id string) (string, error) {
	bg, ok := m.shells.Get(id)
	if !ok {
		bg, ok = m.finishedShell(id)
	}
	if !ok {
		if m.history.has(id) {
			return "", fmt.Errorf("%w: output of finished job %s is no longer kept (only the last %d finished jobs keep output)", ErrJobNotFound, id, recentOutputs)
		}
		return "", fmt.Errorf("%w: %s", ErrJobNotFound, id)
	}
	stdout, stderr, _, _ := bg.GetOutput()
	if stderr == "" {
		return stdout, nil
	}
	if stdout == "" {
		return stderr, nil
	}
	return stdout + "\n" + stderr, nil
}

// finishedShell tìm job đã xong mà output còn được giữ (lệnh chạy-rồi-chờ bị
// gỡ khỏi bảng job ngay khi tool trả về, nên chỉ giữ output của vài job cuối).
func (m *BackgroundShellManager) finishedShell(id string) (*BackgroundShell, bool) {
	m.recentMu.Lock()
	defer m.recentMu.Unlock()
	for _, bg := range m.recent {
		if bg.ID == id {
			return bg, true
		}
	}
	return nil, false
}

// recentOutputs là số job đã xong còn giữ output trong bộ nhớ.
const recentOutputs = 20

func (m *BackgroundShellManager) keepRecent(bg *BackgroundShell) {
	m.recentMu.Lock()
	defer m.recentMu.Unlock()
	m.recent = append(m.recent, bg)
	if len(m.recent) > recentOutputs {
		m.recent = slices.Delete(m.recent, 0, len(m.recent)-recentOutputs)
	}
}

// Info chụp trạng thái hiện tại của job.
func (bs *BackgroundShell) Info() JobInfo {
	if !bs.IsDone() {
		return bs.baseInfo()
	}
	return bs.finishedInfo()
}

// baseInfo là ảnh chụp với trạng thái running.
func (bs *BackgroundShell) baseInfo() JobInfo {
	return JobInfo{
		ID:           bs.ID,
		SessionID:    bs.Owner.SessionID,
		ToolCallID:   bs.Owner.ToolCallID,
		Command:      bs.Command,
		Description:  bs.Description,
		WorkingDir:   bs.WorkingDir,
		StartedAt:    bs.StartedAt,
		root:         bs.root,
		Status:       JobRunning,
		Background:   bs.background.Load(),
		OutputBytes:  bs.stdout.total() + bs.stderr.total(),
		DroppedBytes: bs.stdout.dropped() + bs.stderr.dropped(),
	}
}

// finishedInfo là ảnh chụp của job đã xong. Chỉ gọi khi exitErr đã được ghi:
// từ goroutine của job, hoặc sau khi done đã đóng.
func (bs *BackgroundShell) finishedInfo() JobInfo {
	j := bs.baseInfo()
	if at := bs.completedAt.Load(); at > 0 {
		j.FinishedAt = time.Unix(0, at)
	}
	switch {
	case bs.killed.Load():
		j.Status = JobKilled
		j.ExitCode = ExitCode(bs.exitErr)
	case bs.exitErr != nil:
		j.Status = JobFailed
		j.ExitCode = ExitCode(bs.exitErr)
	default:
		j.Status = JobSucceeded
	}
	return j
}

// MarkBackground ghi nhận job id chạy tiếp sau khi tool bash đã trả về.
func (m *BackgroundShellManager) MarkBackground(id string) {
	if bg, ok := m.shells.Get(id); ok {
		bg.background.Store(true)
	}
}
