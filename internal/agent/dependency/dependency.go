package dependency

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// 錯誤定義
var (
	ErrTaskNotFound       = errors.New("task not found in dependency graph")
	ErrCycleDetected      = errors.New("dependency cycle detected")
	ErrInvalidDependency  = errors.New("invalid dependency relationship")
	ErrMaxDepthExceeded   = errors.New("maximum dependency depth exceeded")
	ErrCircularDependency = errors.New("circular dependency detected")
)

// 節點狀態
type NodeState int

const (
	NodeStatePending NodeState = iota
	NodeStateReady
	NodeStateRunning
	NodeStateCompleted
	NodeStateFailed
	NodeStateBlocked
)

func (s NodeState) String() string {
	switch s {
	case NodeStatePending:
		return "pending"
	case NodeStateReady:
		return "ready"
	case NodeStateRunning:
		return "running"
	case NodeStateCompleted:
		return "completed"
	case NodeStateFailed:
		return "failed"
	case NodeStateBlocked:
		return "blocked"
	default:
		return "unknown"
	}
}

// 任務節點
type TaskNode struct {
	TaskID     string
	DependsOn  []string
	DependedBy []string
	State      NodeState
	Priority   int
	Metadata   map[string]interface{}
}

// 依賴圖
type DependencyGraph struct {
	nodes map[string]*TaskNode
	edges map[string][]string // adjacency list: from -> []to
	mu    sync.RWMutex
}

// 新建依賴圖
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*TaskNode),
		edges: make(map[string][]string),
	}
}

// 添加邊
func (dg *DependencyGraph) AddEdge(from, to string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	// 確保節點存在
	if _, ok := dg.nodes[from]; !ok {
		dg.nodes[from] = &TaskNode{TaskID: from, Metadata: make(map[string]interface{})}
	}
	if _, ok := dg.nodes[to]; !ok {
		dg.nodes[to] = &TaskNode{TaskID: to, Metadata: make(map[string]interface{})}
	}

	// 添加邊
	dg.edges[from] = append(dg.edges[from], to)

	// 更新節點的依賴關係
	dg.nodes[from].DependedBy = append(dg.nodes[from].DependedBy, to)
	dg.nodes[to].DependsOn = append(dg.nodes[to].DependsOn, from)
}

// 移除邊
func (dg *DependencyGraph) RemoveEdge(from, to string) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	// 從 edges[from] 移除 to
	edges := dg.edges[from]
	for i, v := range edges {
		if v == to {
			dg.edges[from] = append(edges[:i], edges[i+1:]...)
			break
		}
	}

	// 從 nodes[from].DependedBy 移除 to
	node := dg.nodes[from]
	for i, v := range node.DependedBy {
		if v == to {
			node.DependedBy = append(node.DependedBy[:i], node.DependedBy[i+1:]...)
			break
		}
	}

	// 從 nodes[to].DependsOn 移除 from
	node = dg.nodes[to]
	for i, v := range node.DependsOn {
		if v == from {
			node.DependsOn = append(node.DependsOn[:i], node.DependsOn[i+1:]...)
			break
		}
	}
}

// 獲取節點
func (dg *DependencyGraph) GetNode(taskID string) (*TaskNode, bool) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	node, ok := dg.nodes[taskID]
	return node, ok
}

// 獲取所有節點
func (dg *DependencyGraph) GetAllNodes() map[string]*TaskNode {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	result := make(map[string]*TaskNode)
	for k, v := range dg.nodes {
		result[k] = v
	}
	return result
}

// 依賴配置
type DependencyConfig struct {
	MaxDepth           int           // 最大依賴深度
	Timeout            time.Duration // 依賴等待超時
	RetryEnabled       bool          // 是否啟用重試
	MaxRetries         int           // 最大重試次數
	ParallelEnabled    bool          // 是否啟用並行執行
	MaxParallelTasks   int           // 最大並行任務數
	CycleCheckEnabled  bool          // 是否啟用循環檢測
	AutoResolveEnabled bool          // 是否自動解決依賴
}

// 默認配置
func DefaultDependencyConfig() DependencyConfig {
	return DependencyConfig{
		MaxDepth:           10,
		Timeout:            5 * time.Minute,
		RetryEnabled:       true,
		MaxRetries:         3,
		ParallelEnabled:    true,
		MaxParallelTasks:   5,
		CycleCheckEnabled:  true,
		AutoResolveEnabled: true,
	}
}

// 依賴事件
type DependencyEvent int

const (
	EventDependencyAdded DependencyEvent = iota
	EventDependencyRemoved
	EventTaskReady
	EventTaskCompleted
	EventTaskFailed
	EventTaskBlocked
	EventCycleDetected
)

func (e DependencyEvent) String() string {
	switch e {
	case EventDependencyAdded:
		return "dependency_added"
	case EventDependencyRemoved:
		return "dependency_removed"
	case EventTaskReady:
		return "task_ready"
	case EventTaskCompleted:
		return "task_completed"
	case EventTaskFailed:
		return "task_failed"
	case EventTaskBlocked:
		return "task_blocked"
	case EventCycleDetected:
		return "cycle_detected"
	default:
		return "unknown"
	}
}

// 依賴監聽器
type DependencyListener interface {
	OnDependencyEvent(event DependencyEvent, taskID string, details ...interface{})
}

// 依賴管理器
type DependencyManager struct {
	mu        sync.RWMutex
	graph     *DependencyGraph
	config    DependencyConfig
	listeners []DependencyListener
}

// 新建依賴管理器
func NewDependencyManager(config DependencyConfig) *DependencyManager {
	return &DependencyManager{
		graph:  NewDependencyGraph(),
		config: config,
	}
}

// 添加監聽器
func (dm *DependencyManager) AddListener(listener DependencyListener) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.listeners = append(dm.listeners, listener)
}

func (dm *DependencyManager) notifyListeners(event DependencyEvent, taskID string, details ...interface{}) {
	for _, listener := range dm.listeners {
		listener.OnDependencyEvent(event, taskID, details...)
	}
}

// 添加依賴關係
func (dm *DependencyManager) AddDependency(taskID, dependsOn string) error {
	dm.mu.Lock()

	// 檢查是否是自己依賴自己
	if taskID == dependsOn {
		dm.mu.Unlock()
		return ErrInvalidDependency
	}

	// 檢查是否會造成循環
	if dm.config.CycleCheckEnabled {
		if dm.wouldCreateCycle(dependsOn, taskID) {
			dm.mu.Unlock()
			dm.notifyListeners(EventCycleDetected, taskID, dependsOn)
			return ErrCycleDetected
		}
	}

	// 確保節點存在
	dm.getOrCreateNode(taskID)
	dm.getOrCreateNode(dependsOn)

	// 添加邊
	dm.graph.AddEdge(dependsOn, taskID)

	// 檢查最大深度 (在添加邊之後檢查)
	// MaxDepth=3 允許 depth 0,1,2,3,4，超过 MaxDepth+1 才失敗
	if dm.config.MaxDepth > 0 {
		depth := dm.getDependencyDepth(taskID)
		if depth > dm.config.MaxDepth+1 {
			// 回滾：移除剛添加的邊
			dm.graph.RemoveEdge(dependsOn, taskID)
			dm.mu.Unlock()
			return ErrMaxDepthExceeded
		}
	}

	dm.mu.Unlock()
	dm.notifyListeners(EventDependencyAdded, taskID, dependsOn)
	return nil
}

// 檢查是否會造成循環
func (dm *DependencyManager) wouldCreateCycle(from, to string) bool {
	visited := make(map[string]bool)
	return dm.canReach(to, from, visited)
}

// 檢查 from 是否能到達 to
func (dm *DependencyManager) canReach(from, to string, visited map[string]bool) bool {
	if from == to {
		return true
	}
	if visited[from] {
		return false
	}
	visited[from] = true

	dm.graph.mu.RLock()
	edges := dm.graph.edges[from]
	dm.graph.mu.RUnlock()

	for _, neighbor := range edges {
		if dm.canReach(neighbor, to, visited) {
			return true
		}
	}
	return false
}

// 獲取依賴深度
func (dm *DependencyManager) getDependencyDepth(taskID string) int {
	visited := make(map[string]bool)
	return dm.getDependencyDepthInternal(taskID, visited)
}

func (dm *DependencyManager) getDependencyDepthInternal(taskID string, visited map[string]bool) int {
	dm.graph.mu.RLock()
	node, ok := dm.graph.nodes[taskID]
	dm.graph.mu.RUnlock()

	if !ok || len(node.DependsOn) == 0 {
		return 0
	}

	// 檢測循環依賴
	if visited[taskID] {
		return 0 // 循環依賴，返回 0 避免無限遞歸
	}
	visited[taskID] = true

	maxDepth := 0
	for _, depID := range node.DependsOn {
		depth := dm.getDependencyDepthInternal(depID, visited)
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth + 1
}

// 創建或獲取節點
func (dm *DependencyManager) getOrCreateNode(taskID string) *TaskNode {
	dm.graph.mu.Lock()
	defer dm.graph.mu.Unlock()

	if node, ok := dm.graph.nodes[taskID]; ok {
		return node
	}

	node := &TaskNode{
		TaskID:   taskID,
		Metadata: make(map[string]interface{}),
	}
	dm.graph.nodes[taskID] = node
	return node
}

// 移除依賴關係
func (dm *DependencyManager) RemoveDependency(taskID, dependsOn string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	node, ok := dm.graph.nodes[taskID]
	if !ok {
		return ErrTaskNotFound
	}

	// 從 DependsOn 列表移除
	found := false
	for i, dep := range node.DependsOn {
		if dep == dependsOn {
			node.DependsOn = append(node.DependsOn[:i], node.DependsOn[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return ErrInvalidDependency
	}

	// 從依賴者的 DependedBy 列表移除
	if depNode, ok := dm.graph.nodes[dependsOn]; ok {
		for i, dep := range depNode.DependedBy {
			if dep == taskID {
				depNode.DependedBy = append(depNode.DependedBy[:i], depNode.DependedBy[i+1:]...)
				break
			}
		}
	}

	// 從邊列表移除
	dm.graph.mu.Lock()
	if edges, ok := dm.graph.edges[dependsOn]; ok {
		var newEdges []string
		for _, edge := range edges {
			if edge != taskID {
				newEdges = append(newEdges, edge)
			}
		}
		dm.graph.edges[dependsOn] = newEdges
	}
	dm.graph.mu.Unlock()

	dm.notifyListeners(EventDependencyRemoved, taskID, dependsOn)
	return nil
}

// 獲取任務的依賴項
func (dm *DependencyManager) GetDependencies(taskID string) ([]string, error) {
	dm.graph.mu.RLock()
	defer dm.graph.mu.RUnlock()

	node, ok := dm.graph.nodes[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return node.DependsOn, nil
}

// 獲取依賴該任務的任務
func (dm *DependencyManager) GetDependents(taskID string) ([]string, error) {
	dm.graph.mu.RLock()
	defer dm.graph.mu.RUnlock()

	node, ok := dm.graph.nodes[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return node.DependedBy, nil
}

// 檢查任務是否可以執行
func (dm *DependencyManager) CanExecute(taskID string) (bool, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	node, ok := dm.graph.nodes[taskID]
	if !ok {
		return false, ErrTaskNotFound
	}

	// 檢查狀態
	if node.State != NodeStatePending && node.State != NodeStateReady {
		return false, nil
	}

	// 檢查依賴是否都已完成
	for _, depID := range node.DependsOn {
		depNode, ok := dm.graph.nodes[depID]
		if !ok {
			continue // 依賴的任務不存在，視為已完成
		}

		if depNode.State != NodeStateCompleted {
			return false, nil
		}
	}

	return true, nil
}

// 獲取可執行的任務列表
func (dm *DependencyManager) GetReadyTasks() ([]string, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var ready []string

	for nodeID, node := range dm.graph.nodes {
		if node.State != NodeStatePending && node.State != NodeStateReady {
			continue
		}

		// 檢查所有依賴是否都已完成
		allDepsCompleted := true
		for _, depID := range node.DependsOn {
			if depNode, ok := dm.graph.nodes[depID]; ok {
				if depNode.State != NodeStateCompleted {
					allDepsCompleted = false
					break
				}
			}
		}

		if allDepsCompleted {
			ready = append(ready, nodeID)
		}
	}

	// 按優先級排序
	sort.Slice(ready, func(i, j int) bool {
		return dm.graph.nodes[ready[i]].Priority > dm.graph.nodes[ready[j]].Priority
	})

	return ready, nil
}

// 標記任務完成
func (dm *DependencyManager) MarkCompleted(taskID string) error {
	dm.mu.Lock()

	node, ok := dm.graph.nodes[taskID]
	if !ok {
		dm.mu.Unlock()
		return ErrTaskNotFound
	}

	node.State = NodeStateCompleted

	// 檢查是否有依賴該任務的任務變為就緒
	for _, dependentID := range node.DependedBy {
		dependent, ok := dm.graph.nodes[dependentID]
		if !ok {
			continue
		}

		if dependent.State == NodeStatePending {
			allDone := true
			for _, depID := range dependent.DependsOn {
				if dm.graph.nodes[depID].State != NodeStateCompleted {
					allDone = false
					break
				}
			}

			if allDone {
				dependent.State = NodeStateReady
			}
		}
	}

	dm.mu.Unlock()

	for _, dependentID := range node.DependedBy {
		if dependent, ok := dm.graph.nodes[dependentID]; ok && dependent.State == NodeStateReady {
			dm.notifyListeners(EventTaskReady, dependentID)
		}
	}
	dm.notifyListeners(EventTaskCompleted, taskID)
	return nil
}

// 標記任務失敗
func (dm *DependencyManager) MarkFailed(taskID string, err error) error {
	dm.mu.Lock()

	node, ok := dm.graph.nodes[taskID]
	if !ok {
		dm.mu.Unlock()
		return ErrTaskNotFound
	}

	node.State = NodeStateFailed
	node.Metadata["error"] = err.Error()

	var blockedDependents []string
	dm.blockDependentsRecursive(taskID, err, &blockedDependents)

	dm.mu.Unlock()

	for _, dependentID := range blockedDependents {
		dm.notifyListeners(EventTaskBlocked, dependentID, taskID, err)
	}
	dm.notifyListeners(EventTaskFailed, taskID, err)
	return nil
}

func (dm *DependencyManager) blockDependentsRecursive(taskID string, err error, blocked *[]string) {
	for _, dependentID := range dm.graph.nodes[taskID].DependedBy {
		dependent, ok := dm.graph.nodes[dependentID]
		if !ok {
			continue
		}

		if dependent.State == NodeStatePending || dependent.State == NodeStateReady {
			dependent.State = NodeStateBlocked
			dependent.Metadata["blockReason"] = fmt.Sprintf("dependency %s failed: %v", taskID, err)
			*blocked = append(*blocked, dependentID)
			dm.blockDependentsRecursive(dependentID, err, blocked)
		}
	}
}

// 檢測循環依賴
func (dm *DependencyManager) DetectCycles() ([]string, error) {
	dm.graph.mu.Lock()
	defer dm.graph.mu.Unlock()

	var cycles []string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for nodeID := range dm.graph.nodes {
		if !visited[nodeID] {
			path := make([]string, 0)
			if dm.detectCycleDFS(nodeID, visited, recStack, &path) {
				cycles = append(cycles, strings.Join(path, " -> "))
			}
		}
	}

	if len(cycles) > 0 {
		dm.notifyListeners(EventCycleDetected, "", cycles)
	}

	return cycles, nil
}

func (dm *DependencyManager) detectCycleDFS(nodeID string, visited, recStack map[string]bool, path *[]string) bool {
	visited[nodeID] = true
	recStack[nodeID] = true
	*path = append(*path, nodeID)

	edges := dm.graph.edges[nodeID]

	for _, neighbor := range edges {
		if !visited[neighbor] {
			if dm.detectCycleDFS(neighbor, visited, recStack, path) {
				return true
			}
		} else if recStack[neighbor] {
			*path = append(*path, neighbor)
			return true
		}
	}

	*path = (*path)[:len(*path)-1]
	recStack[nodeID] = false
	return false
}

// 獲取執行順序（拓撲排序）
func (dm *DependencyManager) GetExecutionOrder() ([]string, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	// 複製圖以避免鎖競爭
	inDegree := make(map[string]int)
	for nodeID := range dm.graph.nodes {
		inDegree[nodeID] = 0
	}

	// 計算入度
	for _, edges := range dm.graph.edges {
		for _, to := range edges {
			inDegree[to]++
		}
	}

	// 找到所有入度為 0 的節點
	var queue []string
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}

	// 按優先級排序初始節點
	sort.Slice(queue, func(i, j int) bool {
		return dm.graph.nodes[queue[i]].Priority > dm.graph.nodes[queue[j]].Priority
	})

	var result []string

	for len(queue) > 0 {
		// 取出隊首
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// 處理所有依賴該節點的節點
		dependentIDs := dm.graph.nodes[current].DependedBy
		sort.Slice(dependentIDs, func(i, j int) bool {
			return dm.graph.nodes[dependentIDs[i]].Priority > dm.graph.nodes[dependentIDs[j]].Priority
		})

		for _, depID := range dependentIDs {
			inDegree[depID]--
			if inDegree[depID] == 0 {
				queue = append(queue, depID)
			}
		}
	}

	// 檢查是否有循環
	if len(result) != len(dm.graph.nodes) {
		return nil, ErrCycleDetected
	}

	return result, nil
}

// 設置任務優先級
func (dm *DependencyManager) SetPriority(taskID string, priority int) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	node := dm.getOrCreateNode(taskID)
	node.Priority = priority
	return nil
}

// 獲取任務狀態
func (dm *DependencyManager) GetState(taskID string) (NodeState, error) {
	dm.graph.mu.RLock()
	defer dm.graph.mu.RUnlock()

	node, ok := dm.graph.nodes[taskID]
	if !ok {
		return 0, ErrTaskNotFound
	}
	return node.State, nil
}

// 獲取任務元數據
func (dm *DependencyManager) GetMetadata(taskID string) (map[string]interface{}, error) {
	dm.graph.mu.RLock()
	defer dm.graph.mu.RUnlock()

	node, ok := dm.graph.nodes[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return node.Metadata, nil
}

// 獲取圖的 DOT 表示（用於調試/視覺化）
func (dm *DependencyManager) ToDOT() string {
	dm.graph.mu.RLock()
	defer dm.graph.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("digraph Dependencies {\n")

	// 節點
	for nodeID, node := range dm.graph.nodes {
		label := fmt.Sprintf("%s\\n(%s)", nodeID, node.State.String())
		sb.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\"];\n", nodeID, label))
	}

	// 邊
	for from, edges := range dm.graph.edges {
		for _, to := range edges {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", from, to))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}
