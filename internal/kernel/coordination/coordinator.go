package coordination

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Coordinator 協調器 - 整合所有協調模組
type Coordinator struct {
	mu sync.RWMutex

	scheduler    *TaskScheduler
	resourceMgr  *ResourceManager
	loadBalancer *LoadBalancer

	// 配置
	config CoordinatorConfig

	// 狀態
	state CoordinatorState
}

// CoordinatorConfig 協調器配置
type CoordinatorConfig struct {
	Scheduler       SchedulerConfig
	ResourceManager ResourceManagerConfig
	LoadBalancer    LoadBalancerConfig
}

// CoordinatorState 協調器狀態
type CoordinatorState struct {
	Running        bool
	TasksProcessed int
	TasksFailed    int
	TotalCostUSD   float64
}

// NewCoordinator 創建新的協調器
func NewCoordinator() *Coordinator {
	c := &Coordinator{
		scheduler:    NewTaskScheduler(),
		resourceMgr:  NewResourceManager(),
		loadBalancer: NewLoadBalancer(),
		state: CoordinatorState{
			Running: true,
		},
	}
	return c
}

// NewCoordinatorWithConfig 使用配置創建協調器
func NewCoordinatorWithConfig(config CoordinatorConfig) *Coordinator {
	c := &Coordinator{
		scheduler:    NewTaskScheduler(config.Scheduler),
		resourceMgr:  NewResourceManager(config.ResourceManager),
		loadBalancer: NewLoadBalancer(config.LoadBalancer),
		state: CoordinatorState{
			Running: true,
		},
	}
	return c
}

// SubmitTask 提交任務
func (c *Coordinator) SubmitTask(prompt string, priority TaskPriority, tools []string) (*Task, error) {
	// 選擇執行者
	executor := c.loadBalancer.SelectExecutor(context.Background(), SelectOptions{
		TaskComplexity: estimateComplexity(prompt),
	})

	// 創建任務
	task := &Task{
		ID:        generateTaskID(),
		Prompt:    prompt,
		Tools:     tools,
		Priority:  priority,
		Executor:  string(executor),
		State:     TaskStatePending,
		CreatedAt: now(),
		Metadata:  make(map[string]interface{}),
	}

	// 嘗試獲取資源
	reservationID, err := c.resourceMgr.Acquire(context.Background(), map[ResourceType]float64{
		ResourceCPU: 1,
	})

	if err != nil {
		// 資源不足，加入調度器隊列
		c.scheduler.Submit(task)
		return task, nil
	}

	task.Metadata["reservation_id"] = reservationID
	c.loadBalancer.RecordTaskStart(ExecutorType(task.Executor))

	// 提交到調度器
	c.scheduler.Submit(task)

	return task, nil
}

// CompleteTask 完成任務
func (c *Coordinator) CompleteTask(taskID string, result *TaskResult) {
	task := c.scheduler.GetTask(taskID)
	if task == nil {
		return
	}

	// 釋放資源
	if resID, ok := task.Metadata["reservation_id"].(string); ok {
		c.resourceMgr.Release(resID)
	}

	// 記錄負載均衡
	c.loadBalancer.RecordTaskComplete(
		ExecutorType(task.Executor),
		result.Tokens,
		result.CostUSD,
		result.Duration,
	)

	// 完成任務
	c.scheduler.CompleteTask(taskID, result)

	// 更新狀態
	c.mu.Lock()
	c.state.TasksProcessed++
	c.state.TotalCostUSD += result.CostUSD
	c.mu.Unlock()
}

// FailTask 標記任務失敗
func (c *Coordinator) FailTask(taskID string, err error) {
	task := c.scheduler.GetTask(taskID)
	if task == nil {
		return
	}

	// 釋放資源
	if resID, ok := task.Metadata["reservation_id"].(string); ok {
		c.resourceMgr.Release(resID)
	}

	// 記錄失敗
	c.loadBalancer.RecordTaskFail(ExecutorType(task.Executor))
	c.scheduler.FailTask(taskID, err)

	// 更新狀態
	c.mu.Lock()
	c.state.TasksFailed++
	c.mu.Unlock()
}

// GetScheduler 獲取調度器
func (c *Coordinator) GetScheduler() *TaskScheduler {
	return c.scheduler
}

// GetResourceManager 獲取資源管理器
func (c *Coordinator) GetResourceManager() *ResourceManager {
	return c.resourceMgr
}

// GetLoadBalancer 獲取負載均衡器
func (c *Coordinator) GetLoadBalancer() *LoadBalancer {
	return c.loadBalancer
}

// GetState 獲取協調器狀態
func (c *Coordinator) GetState() CoordinatorState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Shutdown 關閉協調器
func (c *Coordinator) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state.Running = false
	c.scheduler.Shutdown()
}

// GetStats 返回完整統計
func (c *Coordinator) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"coordinator": map[string]interface{}{
			"running":         c.state.Running,
			"tasks_processed": c.state.TasksProcessed,
			"tasks_failed":    c.state.TasksFailed,
			"total_cost_usd":  c.state.TotalCostUSD,
		},
		"scheduler":        c.scheduler.GetStats(),
		"resource_manager": c.resourceMgr.GetUsage(),
		"load_balancer":    c.loadBalancer.GetTotalStats(),
	}
}

// ============================================================================
// 輔助函數
// ============================================================================

func now() time.Time {
	return time.Now()
}

func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

func estimateComplexity(prompt string) float64 {
	// 簡單的複雜度估算
	complexity := 0.5 // 預設中等

	// 關鍵字檢測
	complexKeywords := []string{
		"refactor", "architect", "design", "optimize", "performance",
		"security", "audit", "review", "complex", "difficult",
	}

	simpleKeywords := []string{
		"read", "list", "show", "get", "simple", "quick",
		"lookup", "find", "search", "check",
	}

	promptLower := strings.ToLower(prompt)
	for _, kw := range complexKeywords {
		if contains(promptLower, kw) {
			complexity += 0.1
		}
	}

	for _, kw := range simpleKeywords {
		if contains(promptLower, kw) {
			complexity -= 0.1
		}
	}

	// 長度調整
	if len(prompt) > 500 {
		complexity += 0.1
	}

	// 限制範圍
	if complexity < 0 {
		complexity = 0
	}
	if complexity > 1 {
		complexity = 1
	}

	return complexity
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
