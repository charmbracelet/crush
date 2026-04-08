package coordination

import (
	"context"
	"sync"
	"time"

	"charm.land/fantasy"
)

// TaskState 任務狀態
type TaskState int

const (
	TaskStatePending TaskState = iota
	TaskStateRunning
	TaskStateCompleted
	TaskStateFailed
	TaskStateCancelled
)

// TaskPriority 任務優先級
type TaskPriority int

const (
	PriorityLow TaskPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// Task 任務結構
type Task struct {
	ID          string
	Prompt      string
	Messages    []fantasy.Message
	Tools       []string
	Priority    TaskPriority
	State       TaskState
	Executor    string // "cl", "cc", "hybrid"
	CreatedAt   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	Result      *TaskResult
	Error       error
	Metadata    map[string]interface{}
}

// TaskResult 任務結果
type TaskResult struct {
	Output       string
	Tokens       int
	CostUSD      float64
	Duration     time.Duration
	CacheHit     bool
	ExecutorUsed string
}

// TaskScheduler 任務調度器
type TaskScheduler struct {
	mu sync.RWMutex

	// 任務隊列（按優先級分組）
	queues map[TaskPriority]*PriorityQueue

	// 運行中的任務
	running map[string]*Task

	// 歷史記錄
	history []*Task

	// 配置
	config SchedulerConfig

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc
}

// SchedulerConfig 調度器配置
type SchedulerConfig struct {
	MaxConcurrentTasks int           // 最大並發任務數
	TaskTimeout        time.Duration // 任務超時
	QueueSize          int           // 隊列大小
	CleanupInterval    time.Duration // 清理間隔
}

// DefaultSchedulerConfig 返回預設配置
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		MaxConcurrentTasks: 10,
		TaskTimeout:        5 * time.Minute,
		QueueSize:          100,
		CleanupInterval:    1 * time.Hour,
	}
}

// PriorityQueue 優先級隊列
type PriorityQueue struct {
	items []*Task
	mu    sync.Mutex
}

// NewPriorityQueue 創建優先級隊列
func NewPriorityQueue() *PriorityQueue {
	return &PriorityQueue{
		items: make([]*Task, 0),
	}
}

// Push 添加任務到隊列
func (pq *PriorityQueue) Push(task *Task) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.items = append(pq.items, task)
	pq.bubbleUp(len(pq.items) - 1)
}

// Pop 彈出最高優先級任務
func (pq *PriorityQueue) Pop() *Task {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.items) == 0 {
		return nil
	}

	task := pq.items[0]
	last := len(pq.items) - 1
	pq.items[0] = pq.items[last]
	pq.items = pq.items[:last]
	pq.bubbleDown(0)

	return task
}

// Size 返回隊列大小
func (pq *PriorityQueue) Size() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return len(pq.items)
}

// bubbleUp 向上調整堆
func (pq *PriorityQueue) bubbleUp(idx int) {
	for idx > 0 {
		parent := (idx - 1) / 2
		if pq.items[parent].Priority >= pq.items[idx].Priority {
			break
		}
		pq.items[parent], pq.items[idx] = pq.items[idx], pq.items[parent]
		idx = parent
	}
}

// bubbleDown 向下調整堆
func (pq *PriorityQueue) bubbleDown(idx int) {
	length := len(pq.items)
	for {
		left := 2*idx + 1
		right := 2*idx + 2
		largest := idx

		if left < length && pq.items[left].Priority > pq.items[largest].Priority {
			largest = left
		}
		if right < length && pq.items[right].Priority > pq.items[largest].Priority {
			largest = right
		}
		if largest == idx {
			break
		}
		pq.items[idx], pq.items[largest] = pq.items[largest], pq.items[idx]
		idx = largest
	}
}

// NewTaskScheduler 創建新的任務調度器
func NewTaskScheduler(config ...SchedulerConfig) *TaskScheduler {
	cfg := DefaultSchedulerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	ctx, cancel := context.WithCancel(context.Background())

	scheduler := &TaskScheduler{
		queues:  make(map[TaskPriority]*PriorityQueue),
		running: make(map[string]*Task),
		history: make([]*Task, 0),
		config:  cfg,
		ctx:     ctx,
		cancel:  cancel,
	}

	// 初始化優先級隊列
	for i := PriorityLow; i <= PriorityCritical; i++ {
		scheduler.queues[i] = NewPriorityQueue()
	}

	return scheduler
}

// Submit 提交新任務
func (ts *TaskScheduler) Submit(task *Task) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// 如果沒有運行中的任務，先檢查佇列
	if len(ts.running) < ts.config.MaxConcurrentTasks {
		// 嘗試從佇列調度
		if ts.scheduleNextFromQueueLocked() {
			// 佇列還有空位，運行新任務
			task.State = TaskStateRunning
			task.StartedAt = time.Now()
			ts.running[task.ID] = task
			return nil
		}
	}

	// 佇列已滿或無法調度，加入佇列
	ts.queues[task.Priority].Push(task)
	return nil
}

// scheduleNextFromQueueLocked 從佇列調度下一個任務
// 返回是否成功調度
func (ts *TaskScheduler) scheduleNextFromQueueLocked() bool {
	if len(ts.running) >= ts.config.MaxConcurrentTasks {
		return false
	}

	// 從高優先級到低優先級查找
	for priority := PriorityCritical; priority >= PriorityLow; priority-- {
		pq := ts.queues[priority]
		if pq.Size() > 0 {
			queuedTask := pq.Pop()
			if queuedTask != nil {
				queuedTask.State = TaskStateRunning
				queuedTask.StartedAt = time.Now()
				ts.running[queuedTask.ID] = queuedTask
				return true
			}
		}
	}
	return false
}

// SubmitWithPriority 提交帶優先級的任務
func (ts *TaskScheduler) SubmitWithPriority(prompt string, priority TaskPriority, tools []string) *Task {
	task := &Task{
		ID:        generateTaskID(),
		Prompt:    prompt,
		Tools:     tools,
		Priority:  priority,
		State:     TaskStatePending,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ts.Submit(task)
	return task
}

// GetTask 獲取任務
func (ts *TaskScheduler) GetTask(id string) *Task {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if task, ok := ts.running[id]; ok {
		return task
	}

	// 在歷史中查找
	for _, task := range ts.history {
		if task.ID == id {
			return task
		}
	}

	return nil
}

// CompleteTask 標記任務完成
func (ts *TaskScheduler) CompleteTask(id string, result *TaskResult) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	task, ok := ts.running[id]
	if !ok {
		return
	}

	task.State = TaskStateCompleted
	task.CompletedAt = time.Now()
	task.Result = result

	// 移出運行列表，加入歷史
	delete(ts.running, id)
	ts.history = append(ts.history, task)

	// 嘗試調度下一個任務
	ts.scheduleNextLocked()
}

// FailTask 標記任務失敗
func (ts *TaskScheduler) FailTask(id string, err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	task, ok := ts.running[id]
	if !ok {
		return
	}

	task.State = TaskStateFailed
	task.CompletedAt = time.Now()
	task.Error = err

	delete(ts.running, id)
	ts.history = append(ts.history, task)

	ts.scheduleNextLocked()
}

// CancelTask 取消任務
func (ts *TaskScheduler) CancelTask(id string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// 檢查運行中的任務
	if _, ok := ts.running[id]; ok {
		delete(ts.running, id)
		return true
	}

	// 在佇列中查找並移除
	for priority, pq := range ts.queues {
		found, idx := pq.findByID(id)
		if found && idx >= 0 {
			pq.removeAt(idx)
			return true
		}
		_ = priority // 標記已使用
	}

	return false
}

// findByID 在佇列中查找任務索引
func (pq *PriorityQueue) findByID(id string) (bool, int) {
	for i, task := range pq.items {
		if task.ID == id {
			return true, i
		}
	}
	return false, -1
}

// removeAt 移除指定索引的任務
func (pq *PriorityQueue) removeAt(idx int) {
	last := len(pq.items) - 1
	if idx < last {
		pq.items[idx] = pq.items[last]
		pq.bubbleUp(idx)
		pq.bubbleDown(idx)
	}
	pq.items = pq.items[:last]
}

// scheduleNext 調度下一個任務
func (ts *TaskScheduler) scheduleNext() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.scheduleNextLocked()
}

func (ts *TaskScheduler) scheduleNextLocked() {
	// 如果已達並發上限，不再調度
	if len(ts.running) >= ts.config.MaxConcurrentTasks {
		return
	}

	// 從高優先級到低優先級查找
	for priority := PriorityCritical; priority >= PriorityLow; priority-- {
		pq := ts.queues[priority]
		if pq.Size() > 0 {
			task := pq.Pop()
			if task != nil {
				task.State = TaskStateRunning
				task.StartedAt = time.Now()
				ts.running[task.ID] = task
				return
			}
		}
	}
}

// GetStats 返回調度器統計
func (ts *TaskScheduler) GetStats() map[string]interface{} {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	queueSizes := make(map[string]int)
	for priority, pq := range ts.queues {
		queueSizes[priority.String()] = pq.Size()
	}

	return map[string]interface{}{
		"running_count":   len(ts.running),
		"history_count":   len(ts.history),
		"queue_sizes":     queueSizes,
		"max_concurrent":  ts.config.MaxConcurrentTasks,
		"total_submitted": len(ts.history) + len(ts.running),
	}
}

// Shutdown 關閉調度器
func (ts *TaskScheduler) Shutdown() {
	ts.cancel()
}

// GetRunningTasks 返回運行中的任務
func (ts *TaskScheduler) GetRunningTasks() []*Task {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	tasks := make([]*Task, 0, len(ts.running))
	for _, task := range ts.running {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetHistory 返回任務歷史
func (ts *TaskScheduler) GetHistory(limit int) []*Task {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if limit <= 0 || limit > len(ts.history) {
		limit = len(ts.history)
	}

	history := make([]*Task, limit)
	copy(history, ts.history[len(ts.history)-limit:])
	return history
}

// String 返回優先級字串
func (p TaskPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}
