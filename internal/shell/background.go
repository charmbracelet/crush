package shell

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
)

const (
	// MaxBackgroundJobs is the maximum number of concurrent background jobs allowed
	MaxBackgroundJobs = 50
	// CompletedJobRetentionMinutes is how long to keep completed jobs before auto-cleanup (8 hours)
	CompletedJobRetentionMinutes = 8 * 60
	// MaxFinishedJobs giới hạn số job đã xong còn nằm trong bảng job (agent
	// đọc được qua job_output); cũ nhất bị bỏ trước.
	MaxFinishedJobs = 250
)

// Trần job đang chạy và job đã xong còn giữ; biến để test rút ngắn.
var (
	maxRunningJobs  = MaxBackgroundJobs
	maxFinishedJobs = MaxFinishedJobs
)

// Trần bộ nhớ cho output của một job: giữ outputHeadCap byte đầu và
// outputTailCap byte cuối của mỗi luồng, bỏ phần giữa. Biến để test rút ngắn.
var (
	outputHeadCap = 256 << 10
	outputTailCap = 1 << 20
)

// cappedBuffer giữ đầu và đuôi của output, bỏ phần giữa khi quá dài, để một
// lệnh in log vô tận (dev server, tail -f, docker logs) không làm phình bộ nhớ
// của server. An toàn khi ghi và đọc đồng thời.
type cappedBuffer struct {
	mu     sync.RWMutex
	head   []byte
	tail   []byte // Giữ tối đa 2*tailCap rồi dồn về tailCap byte cuối: chi phí khấu hao O(1) mỗi byte.
	totalN int64
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	b.totalN += int64(n)
	if room := outputHeadCap - len(b.head); room > 0 {
		k := min(room, len(p))
		b.head = append(b.head, p[:k]...)
		p = p[k:]
	}
	if len(p) == 0 {
		return n, nil
	}
	if len(p) >= outputTailCap {
		// Một lần ghi lớn: chỉ giữ đuôi của nó trong mảng mới, không nối cả
		// khối (mảng cũ có thể đã to).
		b.tail = append(make([]byte, 0, 2*outputTailCap), p[len(p)-outputTailCap:]...)
		return n, nil
	}
	b.tail = append(b.tail, p...)
	if len(b.tail) > 2*outputTailCap {
		kept := b.tail[len(b.tail)-outputTailCap:]
		if cap(b.tail) > 4*outputTailCap {
			b.tail = append(make([]byte, 0, 2*outputTailCap), kept...)
		} else {
			b.tail = append(b.tail[:0], kept...)
		}
	}
	return n, nil
}

func (b *cappedBuffer) WriteString(s string) (int, error) {
	return b.Write([]byte(s))
}

// keptTail trả phần đuôi đang giữ (tối đa outputTailCap byte). Gọi khi đã giữ khoá.
func (b *cappedBuffer) keptTail() []byte {
	if len(b.tail) > outputTailCap {
		return b.tail[len(b.tail)-outputTailCap:]
	}
	return b.tail
}

func (b *cappedBuffer) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	tail := b.keptTail()
	dropped := b.totalN - int64(len(b.head)) - int64(len(tail))
	if dropped <= 0 {
		return string(b.head) + string(tail)
	}
	return string(b.head) + fmt.Sprintf("\n[... %d bytes of output dropped ...]\n", dropped) + string(tail)
}

// total trả tổng số byte đã ghi.
func (b *cappedBuffer) total() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.totalN
}

// dropped trả số byte ở giữa đã bị bỏ.
func (b *cappedBuffer) dropped() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return max(0, b.totalN-int64(len(b.head))-int64(len(b.keptTail())))
}

// BackgroundShell represents a shell running in the background.
type BackgroundShell struct {
	ID          string
	Command     string
	Description string
	Shell       *Shell
	WorkingDir  string
	// Owner là session và tool call đã chạy job; StartedAt là lúc bắt đầu.
	Owner       JobOwner
	StartedAt   time.Time
	root        string // Owner.Root đã giải symlink
	ctx         context.Context
	cancel      context.CancelFunc
	stdout      *cappedBuffer
	stderr      *cappedBuffer
	done        chan struct{}
	exitErr     error
	completedAt atomic.Int64 // UnixNano when job completed (0 if still running)
	background  atomic.Bool  // Chạy tiếp sau khi tool bash đã trả về.
	killed      atomic.Bool  // Bị dừng bằng Kill/KillAll.
}

// BackgroundShellManager manages background shell instances.
type BackgroundShellManager struct {
	shells *csync.Map[string, *BackgroundShell]
	// history nhớ thông tin các job đã xong gần đây; recent giữ output của
	// vài job cuối (job chạy-rồi-chờ bị gỡ khỏi shells ngay khi tool trả về).
	history  jobHistory
	recentMu sync.Mutex
	recent   []*BackgroundShell
	// startMu giữ đếm-rồi-thêm của StartFor nguyên tử, để trần job đúng.
	startMu sync.Mutex
}

var (
	backgroundManager     *BackgroundShellManager
	backgroundManagerOnce sync.Once
	idCounter             atomic.Uint64
)

// newBackgroundShellManager creates a new BackgroundShellManager instance.
func newBackgroundShellManager() *BackgroundShellManager {
	return &BackgroundShellManager{
		shells: csync.NewMap[string, *BackgroundShell](),
	}
}

// GetBackgroundShellManager returns the singleton background shell manager.
func GetBackgroundShellManager() *BackgroundShellManager {
	backgroundManagerOnce.Do(func() {
		backgroundManager = newBackgroundShellManager()
	})
	return backgroundManager
}

// Start creates and starts a new background shell with the given command.
func (m *BackgroundShellManager) Start(ctx context.Context, workingDir string, blockFuncs []BlockFunc, command string, description string) (*BackgroundShell, error) {
	return m.StartFor(ctx, JobOwner{}, workingDir, blockFuncs, command, description)
}

// StartFor như Start, gắn chủ của job (session, tool call) để bảng Activity
// và `agent-cli jobs` biết job thuộc việc nào.
func (m *BackgroundShellManager) StartFor(ctx context.Context, owner JobOwner, workingDir string, blockFuncs []BlockFunc, command string, description string) (*BackgroundShell, error) {
	// Chỉ job đang chạy chiếm chỗ (port upstream 0ac04e4f): job nền đã xong
	// nằm trong bảng tới 8 giờ, trước đây đủ 50 cái là mọi lệnh bash hỏng.
	m.startMu.Lock()
	defer m.startMu.Unlock()
	if running := m.prune(); running >= maxRunningJobs {
		return nil, fmt.Errorf("maximum number of running background jobs (%d) reached. Please terminate or wait for some jobs to complete", maxRunningJobs)
	}

	id := fmt.Sprintf("%03X", idCounter.Add(1))

	shell := NewShell(&Options{
		WorkingDir: workingDir,
		BlockFuncs: blockFuncs,
	})

	shellCtx, cancel := context.WithCancel(ctx)

	bgShell := &BackgroundShell{
		ID:          id,
		Command:     command,
		Description: description,
		WorkingDir:  workingDir,
		Shell:       shell,
		Owner:       owner,
		StartedAt:   time.Now(),
		root:        resolveDir(owner.Root),
		ctx:         shellCtx,
		cancel:      cancel,
		stdout:      &cappedBuffer{},
		stderr:      &cappedBuffer{},
		done:        make(chan struct{}),
	}

	m.shells.Set(id, bgShell)

	go func() {
		err := shell.ExecStream(shellCtx, command, bgShell.stdout, bgShell.stderr)

		bgShell.exitErr = err
		bgShell.completedAt.Store(time.Now().UnixNano())
		// Ghi lịch sử TRƯỚC close(done): ai thấy job xong (vd tool bash chờ done
		// rồi Remove) cũng thấy nó trong lịch sử, không có lúc job biến mất.
		m.history.add(bgShell.finishedInfo())
		m.keepRecent(bgShell)
		close(bgShell.done)
	}()

	return bgShell, nil
}

// prune bỏ khỏi bảng các job đã xong quá hạn giữ hoặc vượt maxFinishedJobs
// (cũ nhất trước) và trả số job đang chạy. Gọi khi giữ startMu.
func (m *BackgroundShellManager) prune() (running int) {
	cutoff := time.Now().Add(-CompletedJobRetentionMinutes * time.Minute).UnixNano()
	var finished []*BackgroundShell
	for bg := range m.shells.Seq() { // Seq duyệt bản sao: xoá trong vòng lặp an toàn.
		switch at := bg.completedAt.Load(); {
		case at == 0:
			running++
		case at < cutoff:
			m.shells.Del(bg.ID)
		default:
			finished = append(finished, bg)
		}
	}
	if over := len(finished) - maxFinishedJobs; over > 0 {
		slices.SortFunc(finished, func(a, b *BackgroundShell) int {
			return cmp.Compare(a.completedAt.Load(), b.completedAt.Load())
		})
		for _, bg := range finished[:over] {
			m.shells.Del(bg.ID)
		}
	}
	return running
}

// Get retrieves a background shell by ID.
func (m *BackgroundShellManager) Get(id string) (*BackgroundShell, bool) {
	return m.shells.Get(id)
}

// Remove removes a background shell from the manager without terminating it.
// This is useful when a shell has already completed and you just want to clean up tracking.
func (m *BackgroundShellManager) Remove(id string) error {
	_, ok := m.shells.Take(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}
	return nil
}

// Kill terminates a background shell by ID.
func (m *BackgroundShellManager) Kill(id string) error {
	// Lấy, dừng, chờ thoát rồi mới gỡ khỏi bảng: job còn hiện (running) tới
	// khi tiến trình thật sự thoát.
	shell, ok := m.shells.Get(id)
	if !ok {
		return fmt.Errorf("background shell not found: %s", id)
	}

	if !shell.IsDone() {
		shell.killed.Store(true)
	}
	shell.cancel()
	<-shell.done
	m.shells.Del(id)
	return nil
}

// Stop dừng job id theo yêu cầu của người dùng nhưng vẫn để nó trong bảng job
// (khác Kill): agent hỏi job_output sau đó vẫn thấy output và trạng thái đã
// dừng. Job đã xong thì không làm gì; ctx giới hạn thời gian chờ job dừng.
func (m *BackgroundShellManager) Stop(ctx context.Context, id string) error {
	bg, ok := m.shells.Get(id)
	if !ok {
		if _, finished := m.finishedShell(id); finished || m.history.has(id) {
			return nil
		}
		return fmt.Errorf("%w: %s", ErrJobNotFound, id)
	}
	if bg.IsDone() {
		return nil
	}
	bg.killed.Store(true)
	bg.cancel()
	select {
	case <-bg.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("job %s did not stop: %w", id, ctx.Err())
	}
}

// BackgroundShellInfo contains information about a background shell.
type BackgroundShellInfo struct {
	ID          string
	Command     string
	Description string
}

// List returns all background shell IDs.
func (m *BackgroundShellManager) List() []string {
	ids := make([]string, 0, m.shells.Len())
	for id := range m.shells.Seq2() {
		ids = append(ids, id)
	}
	return ids
}

// Cleanup removes completed jobs that have been finished for more than the retention period
func (m *BackgroundShellManager) Cleanup() int {
	now := time.Now().UnixNano()
	retention := int64(CompletedJobRetentionMinutes * time.Minute)

	var toRemove []string
	for shell := range m.shells.Seq() {
		completedAt := shell.completedAt.Load()
		if completedAt > 0 && now-completedAt > retention {
			toRemove = append(toRemove, shell.ID)
		}
	}

	for _, id := range toRemove {
		m.Remove(id)
	}

	return len(toRemove)
}

// KillAll terminates all background shells. The provided context bounds how
// long the function waits for each shell to exit.
func (m *BackgroundShellManager) KillAll(ctx context.Context) {
	shells := slices.Collect(m.shells.Seq())
	m.shells.Reset(map[string]*BackgroundShell{})

	var wg sync.WaitGroup
	for _, shell := range shells {
		wg.Go(func() {
			if !shell.IsDone() {
				shell.killed.Store(true)
			}
			shell.cancel()
			select {
			case <-shell.done:
			case <-ctx.Done():
			}
		})
	}
	wg.Wait()
}

// GetOutput returns the current output of a background shell.
func (bs *BackgroundShell) GetOutput() (stdout string, stderr string, done bool, err error) {
	select {
	case <-bs.done:
		return bs.stdout.String(), bs.stderr.String(), true, bs.exitErr
	default:
		return bs.stdout.String(), bs.stderr.String(), false, nil
	}
}

// IsDone checks if the background shell has finished execution.
func (bs *BackgroundShell) IsDone() bool {
	select {
	case <-bs.done:
		return true
	default:
		return false
	}
}

// Wait blocks until the background shell completes.
func (bs *BackgroundShell) Wait() {
	<-bs.done
}

func (bs *BackgroundShell) WaitContext(ctx context.Context) bool {
	select {
	case <-bs.done:
		return true
	case <-ctx.Done():
		return false
	}
}
