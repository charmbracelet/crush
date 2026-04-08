package dependency

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// 測試輔助函數：創建帶監聽器的測試管理器
type testListener struct {
	events []string
	mu     sync.Mutex
}

func (l *testListener) OnDependencyEvent(event DependencyEvent, taskID string, details ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event.String()+"::"+taskID)
}

func (l *testListener) getEvents() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	events := make([]string, len(l.events))
	copy(events, l.events)
	return events
}

func (l *testListener) clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = nil
}

// 基礎功能測試

func TestNewDependencyManager(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())
	if dm == nil {
		t.Fatal("NewDependencyManager returned nil")
	}
	if dm.graph == nil {
		t.Error("graph is nil")
	}
}

func TestAddDependency(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	err := dm.AddDependency("task-B", "task-A")
	if err != nil {
		t.Errorf("AddDependency failed: %v", err)
	}

	// 驗證依賴關係
	deps, err := dm.GetDependencies("task-B")
	if err != nil {
		t.Errorf("GetDependencies failed: %v", err)
	}
	if len(deps) != 1 || deps[0] != "task-A" {
		t.Errorf("expected [task-A], got %v", deps)
	}

	// 驗證反向關係
	dependents, err := dm.GetDependents("task-A")
	if err != nil {
		t.Errorf("GetDependents failed: %v", err)
	}
	if len(dependents) != 1 || dependents[0] != "task-B" {
		t.Errorf("expected [task-B], got %v", dependents)
	}
}

func TestAddDependencySelfReference(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	err := dm.AddDependency("task-A", "task-A")
	if err != ErrInvalidDependency {
		t.Errorf("expected ErrInvalidDependency, got %v", err)
	}
}

func TestAddDependencyCycleDetection(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	// A -> B -> C
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-B")

	// 嘗試添加 C -> A（會造成循環）
	err := dm.AddDependency("task-A", "task-C")
	if err != ErrCycleDetected {
		t.Errorf("expected ErrCycleDetected, got %v", err)
	}
}

func TestAddDependencyCycleDetectionDisabled(t *testing.T) {
	config := DefaultDependencyConfig()
	config.CycleCheckEnabled = false
	dm := NewDependencyManager(config)

	// A -> B -> C
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-B")

	// 禁用循環檢測後，不應該返回錯誤（但圖中有循環）
	err := dm.AddDependency("task-A", "task-C")
	if err != nil {
		t.Errorf("with CycleCheckEnabled=false, AddDependency should not fail, got: %v", err)
	}
}

func TestRemoveDependency(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	dm.AddDependency("task-B", "task-A")

	// 移除依賴
	err := dm.RemoveDependency("task-B", "task-A")
	if err != nil {
		t.Errorf("RemoveDependency failed: %v", err)
	}

	// 驗證依賴已移除
	deps, _ := dm.GetDependencies("task-B")
	if len(deps) != 0 {
		t.Errorf("expected empty deps, got %v", deps)
	}
}

func TestRemoveDependencyNotFound(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	err := dm.RemoveDependency("task-B", "task-A")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

// 執行判斷測試

func TestCanExecute(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	// 添加依賴：A -> B
	dm.AddDependency("task-B", "task-A")

	// B 依賴 A，所以不能執行
	canExec, _ := dm.CanExecute("task-B")
	if canExec {
		t.Error("task-B should not be executable (depends on incomplete task-A)")
	}

	// 標記 A 完成
	dm.MarkCompleted("task-A")

	// 現在 B 可以執行
	canExec, _ = dm.CanExecute("task-B")
	if !canExec {
		t.Error("task-B should be executable (dependency completed)")
	}
}

func TestCanExecuteTaskNotFound(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	_, err := dm.CanExecute("non-existent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestGetReadyTasks(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	// 添加依賴：A -> B, A -> C, B -> D, C -> D
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-A")
	dm.AddDependency("task-D", "task-B")
	dm.AddDependency("task-D", "task-C")

	// 初始只有 A 就緒
	ready, _ := dm.GetReadyTasks()
	if len(ready) != 1 || ready[0] != "task-A" {
		t.Errorf("expected [task-A], got %v", ready)
	}

	// 完成 A
	dm.MarkCompleted("task-A")

	// 現在 B 和 C 都就緒
	ready, _ = dm.GetReadyTasks()
	if len(ready) != 2 {
		t.Errorf("expected 2 ready tasks, got %d", len(ready))
	}

	// 完成 B
	dm.MarkCompleted("task-B")

	// C 仍在等待，但 D 仍不能執行
	ready, _ = dm.GetReadyTasks()
	if len(ready) != 1 || ready[0] != "task-C" {
		t.Errorf("expected [task-C], got %v", ready)
	}
}

func TestGetReadyTasksWithPriority(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	// 添加多個獨立任務
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-A")
	dm.SetPriority("task-B", 10)
	dm.SetPriority("task-C", 20)

	dm.MarkCompleted("task-A")

	// 更高優先級的任務應該排在前面
	ready, _ := dm.GetReadyTasks()
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready tasks, got %d", len(ready))
	}
	if ready[0] != "task-C" || ready[1] != "task-B" {
		t.Errorf("expected [task-C, task-B] by priority, got %v", ready)
	}
}

// 任務狀態測試

func TestMarkCompleted(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	dm.AddDependency("task-B", "task-A")

	err := dm.MarkCompleted("task-A")
	if err != nil {
		t.Errorf("MarkCompleted failed: %v", err)
	}

	state, _ := dm.GetState("task-A")
	if state != NodeStateCompleted {
		t.Errorf("expected NodeStateCompleted, got %v", state)
	}
}

func TestMarkCompletedNotFound(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	err := dm.MarkCompleted("non-existent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestMarkFailed(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	// A -> B -> C
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-B")

	// 標記 A 失敗
	err := dm.MarkFailed("task-A", errors.New("test error"))
	if err != nil {
		t.Errorf("MarkFailed failed: %v", err)
	}

	// 檢查 A 的狀態
	state, _ := dm.GetState("task-A")
	if state != NodeStateFailed {
		t.Errorf("expected NodeStateFailed, got %v", state)
	}

	// 檢查 B 和 C 是否被 blocking
	bState, _ := dm.GetState("task-B")
	if bState != NodeStateBlocked {
		t.Errorf("expected task-B to be blocked, got %v", bState)
	}

	cState, _ := dm.GetState("task-C")
	if cState != NodeStateBlocked {
		t.Errorf("expected task-C to be blocked, got %v", cState)
	}
}

func TestMarkFailedNotFound(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	err := dm.MarkFailed("non-existent", errors.New("test error"))
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

// 循環檢測測試

func TestDetectCycles(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())
	config := DefaultDependencyConfig()
	config.CycleCheckEnabled = false // 禁用自動檢測，這樣我們可以手動添加循環
	dm = NewDependencyManager(config)

	// 手動創建循環：A -> B -> C -> A
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-B")
	dm.AddDependency("task-A", "task-C")

	// 檢測循環
	cycles, err := dm.DetectCycles()
	if err != nil {
		t.Errorf("DetectCycles failed: %v", err)
	}
	if len(cycles) == 0 {
		t.Error("expected to detect cycle")
	}
}

func TestDetectCyclesNoCycle(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	// A -> B -> C（沒有循環）
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-B")

	cycles, err := dm.DetectCycles()
	if err != nil {
		t.Errorf("DetectCycles failed: %v", err)
	}
	if len(cycles) != 0 {
		t.Errorf("expected no cycles, got %v", cycles)
	}
}

// 執行順序測試

func TestGetExecutionOrder(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	// 添加複雜依賴：A -> B, A -> C, B -> D, C -> D
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-A")
	dm.AddDependency("task-D", "task-B")
	dm.AddDependency("task-D", "task-C")

	order, err := dm.GetExecutionOrder()
	if err != nil {
		t.Errorf("GetExecutionOrder failed: %v", err)
	}

	// 驗證順序：A 必须在 B 和 C 前面，D 在 B 和 C 後面
	pos := make(map[string]int)
	for i, id := range order {
		pos[id] = i
	}

	if pos["task-A"] >= pos["task-B"] || pos["task-A"] >= pos["task-C"] {
		t.Error("task-A should come before task-B and task-C")
	}
	if pos["task-D"] <= pos["task-B"] || pos["task-D"] <= pos["task-C"] {
		t.Error("task-D should come after task-B and task-C")
	}
}

func TestGetExecutionOrderWithPriority(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	// A -> B, A -> C
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-A")
	dm.SetPriority("task-B", 10)
	dm.SetPriority("task-C", 20)

	order, err := dm.GetExecutionOrder()
	if err != nil {
		t.Errorf("GetExecutionOrder failed: %v", err)
	}

	// C 優先級更高，應該在 B 前面
	pos := make(map[string]int)
	for i, id := range order {
		pos[id] = i
	}
	if pos["task-C"] > pos["task-B"] {
		t.Error("higher priority task-C should come before task-B")
	}
}

func TestGetExecutionOrderWithCycle(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())
	config := DefaultDependencyConfig()
	config.CycleCheckEnabled = false
	dm = NewDependencyManager(config)

	// 手動創建循環
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-B")
	dm.AddDependency("task-A", "task-C")

	_, err := dm.GetExecutionOrder()
	if err != ErrCycleDetected {
		t.Errorf("expected ErrCycleDetected, got %v", err)
	}
}

// 監聽器測試

func TestListener(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())
	listener := &testListener{}
	dm.AddListener(listener)

	// 添加依賴
	dm.AddDependency("task-B", "task-A")

	events := listener.getEvents()
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}

	listener.clear()

	// 完成任務
	dm.MarkCompleted("task-A")

	events = listener.getEvents()
	found := false
	for _, e := range events {
		if e == "task_completed::task-A" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected task_completed event")
	}
}

func TestListenerCycleDetection(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())
	listener := &testListener{}
	dm.AddListener(listener)

	// 添加會造成循環的依賴
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-A", "task-B")

	events := listener.getEvents()
	found := false
	for _, e := range events {
		if len(e) >= 14 && e[:14] == "cycle_detected" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cycle_detected event")
	}
}

// 狀態獲取測試

func TestGetState(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	dm.AddDependency("task-B", "task-A")

	// 初始狀態
	state, _ := dm.GetState("task-B")
	if state != NodeStatePending {
		t.Errorf("expected NodeStatePending, got %v", state)
	}

	// 完成依賴
	dm.MarkCompleted("task-A")

	state, _ = dm.GetState("task-B")
	if state != NodeStateReady {
		t.Errorf("expected NodeStateReady, got %v", state)
	}
}

func TestGetStateNotFound(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	_, err := dm.GetState("non-existent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

// 元數據測試

func TestGetMetadata(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	dm.AddDependency("task-B", "task-A")
	dm.MarkFailed("task-A", errors.New("test error"))

	metadata, err := dm.GetMetadata("task-A")
	if err != nil {
		t.Errorf("GetMetadata failed: %v", err)
	}

	if metadata["error"] != "test error" {
		t.Errorf("expected error in metadata, got %v", metadata["error"])
	}
}

// DOT 導出測試

func TestToDOT(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	dm.AddDependency("task-B", "task-A")

	dot := dm.ToDOT()
	if dot == "" {
		t.Error("ToDOT returned empty string")
	}
	if len(dot) < 50 {
		t.Error("DOT output seems too short")
	}
}

// 並發安全測試

func TestConcurrentAccess(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	var wg sync.WaitGroup
	wg.Add(10)

	// 並發添加依賴
	for i := 0; i < 10; i++ {
		go func(idx int) {
			defer wg.Done()
			taskID := "task-" + string(rune('A'+idx))
			for j := 0; j < 10; j++ {
				depID := "dep-" + string(rune('0'+j))
				dm.AddDependency(taskID, depID)
			}
		}(i)
	}

	wg.Wait()

	// 驗證圖仍然一致
	order, err := dm.GetExecutionOrder()
	if err != nil {
		t.Errorf("GetExecutionOrder failed after concurrent access: %v", err)
	}
	if len(order) == 0 {
		t.Error("expected some tasks in execution order")
	}
}

func TestConcurrentMarkCompleted(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	// 創建依賴鏈 task-1 -> task-2 -> ... -> task-10
	for i := 2; i <= 10; i++ {
		dm.AddDependency(fmt.Sprintf("task-%d", i), fmt.Sprintf("task-%d", i-1))
	}

	var wg sync.WaitGroup
	wg.Add(10)

	// 並發完成任務
	for i := 1; i <= 10; i++ {
		go func(idx int) {
			defer wg.Done()
			taskID := fmt.Sprintf("task-%d", idx)
			time.Sleep(time.Millisecond * 10) // 確保按順序完成
			dm.MarkCompleted(taskID)
		}(i)
	}

	wg.Wait()

	// 驗證所有任務完成
	state, _ := dm.GetState("task-10")
	if state != NodeStateCompleted {
		t.Errorf("expected task-10 to be completed, got %v", state)
	}
}

// 深度限制測試

func TestMaxDepthExceeded(t *testing.T) {
	config := DefaultDependencyConfig()
	config.MaxDepth = 3
	dm := NewDependencyManager(config)

	// A -> B -> C -> D -> E
	dm.AddDependency("task-B", "task-A")
	dm.AddDependency("task-C", "task-B")
	dm.AddDependency("task-D", "task-C")
	err := dm.AddDependency("task-E", "task-D")
	if err != nil {
		t.Errorf("should not fail at depth 4, got: %v", err)
	}

	// 嘗試添加深度為 5 的依賴
	err = dm.AddDependency("task-F", "task-E")
	if err != ErrMaxDepthExceeded {
		t.Errorf("expected ErrMaxDepthExceeded, got %v", err)
	}
}

// 優先級測試

func TestSetPriority(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	err := dm.SetPriority("task-A", 100)
	if err != nil {
		t.Errorf("SetPriority failed: %v", err)
	}

	dm.AddDependency("task-B", "task-A")
	dm.MarkCompleted("task-A")

	ready, _ := dm.GetReadyTasks()
	if len(ready) != 1 || ready[0] != "task-B" {
		t.Errorf("expected [task-B], got %v", ready)
	}
}

func TestSetPriorityNotFound(t *testing.T) {
	dm := NewDependencyManager(DefaultDependencyConfig())

	// SetPriority should auto-create the task if it doesn't exist
	err := dm.SetPriority("non-existent", 100)
	if err != nil {
		t.Errorf("SetPriority should auto-create task, got error: %v", err)
	}

	// Verify the task was created with the correct priority
	dm.mu.Lock()
	node, ok := dm.graph.nodes["non-existent"]
	dm.mu.Unlock()
	if !ok {
		t.Error("expected task to be created")
	}
	if node.Priority != 100 {
		t.Errorf("expected priority 100, got %d", node.Priority)
	}
}
