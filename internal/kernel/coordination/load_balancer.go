package coordination

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// ExecutorType 執行者類型
type ExecutorType string

const (
	ExecutorCL         ExecutorType = "cl"
	ExecutorClaudeCode ExecutorType = "cc"
	ExecutorHybrid     ExecutorType = "hybrid"
)

// ExecutorStats 執行者統計
type ExecutorStats struct {
	Type           ExecutorType
	ActiveTasks    int
	CompletedTasks int
	FailedTasks    int
	TotalTokens    int
	TotalCostUSD   float64
	AvgLatency     time.Duration
	TotalLatency   time.Duration
	LastUsed       time.Time
	HealthStatus   string // "healthy", "degraded", "unhealthy"
}

// LoadBalancer 負載均衡器
type LoadBalancer struct {
	mu sync.RWMutex

	// 執行者池
	executors map[ExecutorType]*ExecutorStats

	// 配置
	config LoadBalancerConfig

	// 選擇策略
	strategy LoadBalanceStrategy

	// round-robin 追蹤
	roundRobinIndex int
}

// LoadBalanceStrategy 負載均衡策略
type LoadBalanceStrategy string

const (
	StrategyRoundRobin    LoadBalanceStrategy = "round_robin"
	StrategyLeastLoad     LoadBalanceStrategy = "least_load"
	StrategyWeighted      LoadBalanceStrategy = "weighted"
	StrategyCostOptimized LoadBalanceStrategy = "cost_optimized"
	StrategyAdaptive      LoadBalanceStrategy = "adaptive"
)

// LoadBalancerConfig 負載均衡器配置
type LoadBalancerConfig struct {
	Strategies          map[LoadBalanceStrategy]bool
	HealthCheckInterval time.Duration
	DefaultStrategy     LoadBalanceStrategy
	ExecutorWeights     map[ExecutorType]float64       // 權重配置
	CostThresholds      map[ExecutorType]float64       // 成本閾值
	LatencyThresholds   map[ExecutorType]time.Duration // 延遲閾值
	EnableFallback      bool                           // 是否啟用降級
}

// DefaultLoadBalancerConfig 返回預設配置
func DefaultLoadBalancerConfig() LoadBalancerConfig {
	return LoadBalancerConfig{
		Strategies: map[LoadBalanceStrategy]bool{
			StrategyRoundRobin:    true,
			StrategyLeastLoad:     true,
			StrategyWeighted:      true,
			StrategyCostOptimized: true,
			StrategyAdaptive:      true,
		},
		HealthCheckInterval: 30 * time.Second,
		DefaultStrategy:     StrategyAdaptive,
		ExecutorWeights: map[ExecutorType]float64{
			ExecutorCL:         1.0, // CL 成本低，權重高
			ExecutorClaudeCode: 0.5, // CC 成本高，權重低
			ExecutorHybrid:     0.7,
		},
		CostThresholds: map[ExecutorType]float64{
			ExecutorCL:         0.01, // CL 閾值
			ExecutorClaudeCode: 0.05, // CC 閾值
			ExecutorHybrid:     0.03,
		},
		LatencyThresholds: map[ExecutorType]time.Duration{
			ExecutorCL:         100 * time.Millisecond,
			ExecutorClaudeCode: 5 * time.Second,
			ExecutorHybrid:     1 * time.Second,
		},
		EnableFallback: true,
	}
}

// NewLoadBalancer 創建新的負載均衡器
func NewLoadBalancer(config ...LoadBalancerConfig) *LoadBalancer {
	cfg := DefaultLoadBalancerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	lb := &LoadBalancer{
		executors: make(map[ExecutorType]*ExecutorStats),
		config:    cfg,
		strategy:  cfg.DefaultStrategy,
	}

	// 初始化執行者統計
	for _, et := range []ExecutorType{ExecutorCL, ExecutorClaudeCode, ExecutorHybrid} {
		lb.executors[et] = &ExecutorStats{
			Type:         et,
			HealthStatus: "healthy",
			LastUsed:     time.Now(),
		}
	}

	return lb
}

// SelectExecutor 選擇執行者
func (lb *LoadBalancer) SelectExecutor(ctx context.Context, options SelectOptions) ExecutorType {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	switch lb.strategy {
	case StrategyRoundRobin:
		return lb.selectRoundRobin()
	case StrategyLeastLoad:
		return lb.selectLeastLoad()
	case StrategyWeighted:
		return lb.selectWeighted()
	case StrategyCostOptimized:
		return lb.selectCostOptimized(options)
	case StrategyAdaptive:
		return lb.selectAdaptive(options)
	default:
		return lb.selectAdaptive(options)
	}
}

// SelectOptions 選擇選項
type SelectOptions struct {
	TaskComplexity    float64 // 0.0-1.0
	MaxCost           float64
	MaxLatency        time.Duration
	PreferredExecutor ExecutorType
}

// selectRoundRobin 輪詢選擇
func (lb *LoadBalancer) selectRoundRobin() ExecutorType {
	executors := []ExecutorType{ExecutorCL, ExecutorClaudeCode, ExecutorHybrid}

	for i := 0; i < len(executors); i++ {
		idx := (lb.roundRobinIndex + i) % len(executors)
		et := executors[idx]
		if lb.isHealthy(et) {
			lb.roundRobinIndex = (idx + 1) % len(executors)
			return et
		}
	}

	return ExecutorCL
}

// selectLeastLoad 最小負載選擇
func (lb *LoadBalancer) selectLeastLoad() ExecutorType {
	var selected ExecutorType
	minLoad := 1.0

	for _, et := range []ExecutorType{ExecutorCL, ExecutorClaudeCode, ExecutorHybrid} {
		stats := lb.executors[et]
		if !lb.isHealthy(et) {
			continue
		}

		load := float64(stats.ActiveTasks)
		if lb.config.ExecutorWeights[et] > 0 {
			load = load / lb.config.ExecutorWeights[et]
		}

		if load < minLoad {
			minLoad = load
			selected = et
		}
	}

	if selected == "" {
		selected = ExecutorCL
	}
	return selected
}

// selectWeighted 權重選擇
func (lb *LoadBalancer) selectWeighted() ExecutorType {
	// 根據權重選擇，CL 權重最高
	totalWeight := lb.config.ExecutorWeights[ExecutorCL] +
		lb.config.ExecutorWeights[ExecutorClaudeCode] +
		lb.config.ExecutorWeights[ExecutorHybrid]

	if totalWeight <= 0 {
		return ExecutorCL
	}

	r := rand.Float64() * totalWeight

	cumulative := 0.0
	for _, et := range []ExecutorType{ExecutorCL, ExecutorClaudeCode, ExecutorHybrid} {
		if !lb.isHealthy(et) {
			continue
		}
		cumulative += lb.config.ExecutorWeights[et]
		if r <= cumulative {
			return et
		}
	}

	return ExecutorCL
}

// selectCostOptimized 成本優化選擇
func (lb *LoadBalancer) selectCostOptimized(options SelectOptions) ExecutorType {
	// 根據任務複雜度選擇
	if options.TaskComplexity < 0.3 {
		// 簡單任務：CL
		if lb.isHealthy(ExecutorCL) {
			return ExecutorCL
		}
	}

	if options.TaskComplexity < 0.7 {
		// 中等任務：Hybrid
		if lb.isHealthy(ExecutorHybrid) {
			return ExecutorHybrid
		}
	}

	// 複雜任務：CC
	if lb.isHealthy(ExecutorClaudeCode) {
		return ExecutorClaudeCode
	}

	// 降級策略
	if lb.config.EnableFallback {
		if lb.isHealthy(ExecutorHybrid) {
			return ExecutorHybrid
		}
		if lb.isHealthy(ExecutorCL) {
			return ExecutorCL
		}
	}

	return ExecutorCL
}

// selectAdaptive 自適應選擇
func (lb *LoadBalancer) selectAdaptive(options SelectOptions) ExecutorType {
	// 綜合考慮健康狀態、負載、成本、延遲

	// 檢查是否有偏好執行者
	if options.PreferredExecutor != "" && lb.isHealthy(options.PreferredExecutor) {
		stats := lb.executors[options.PreferredExecutor]
		costOK := options.MaxCost <= 0 || stats.TotalCostUSD <= options.MaxCost
		latencyOK := options.MaxLatency <= 0 || stats.AvgLatency <= options.MaxLatency
		if costOK && latencyOK {
			return options.PreferredExecutor
		}
	}

	// 檢查各執行者的健康狀態
	healthyExecutors := lb.getHealthyExecutors()
	if len(healthyExecutors) == 0 {
		return ExecutorCL // 最後手段：即使不健康也要選擇
	}

	// 根據任務複雜度選擇
	switch {
	case options.TaskComplexity < 0.2:
		// 極簡單任務：CL
		if lb.isHealthy(ExecutorCL) {
			return ExecutorCL
		}
	case options.TaskComplexity < 0.5:
		// 簡單任務：CL 或 Hybrid
		if lb.isHealthy(ExecutorCL) && lb.getActiveTasks(ExecutorCL) < 5 {
			return ExecutorCL
		}
		if lb.isHealthy(ExecutorHybrid) {
			return ExecutorHybrid
		}
	case options.TaskComplexity < 0.8:
		// 中等任務：Hybrid 或 CC
		if lb.isHealthy(ExecutorHybrid) && lb.getActiveTasks(ExecutorHybrid) < 3 {
			return ExecutorHybrid
		}
		if lb.isHealthy(ExecutorClaudeCode) {
			return ExecutorClaudeCode
		}
	default:
		// 複雜任務：CC
		if lb.isHealthy(ExecutorClaudeCode) && lb.getActiveTasks(ExecutorClaudeCode) < 2 {
			return ExecutorClaudeCode
		}
	}

	// 最後手段：選擇最空閒的執行者
	return lb.selectLeastLoad()
}

// RecordTaskStart 記錄任務開始
func (lb *LoadBalancer) RecordTaskStart(executor ExecutorType) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	stats := lb.executors[executor]
	if stats != nil {
		stats.ActiveTasks++
		stats.LastUsed = time.Now()
	}
}

// RecordTaskComplete 記錄任務完成
func (lb *LoadBalancer) RecordTaskComplete(executor ExecutorType, tokens int, costUSD float64, latency time.Duration) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	stats := lb.executors[executor]
	if stats != nil {
		stats.ActiveTasks--
		stats.CompletedTasks++
		stats.TotalTokens += tokens
		stats.TotalCostUSD += costUSD
		stats.TotalLatency += latency
		if stats.CompletedTasks > 0 && stats.TotalLatency > 0 {
			stats.AvgLatency = stats.TotalLatency / time.Duration(stats.CompletedTasks)
		}
	}
}

// RecordTaskFail 記錄任務失敗
func (lb *LoadBalancer) RecordTaskFail(executor ExecutorType) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	stats := lb.executors[executor]
	if stats != nil {
		stats.ActiveTasks--
		stats.FailedTasks++

		// 連續失敗超過閾值，標記為不健康
		totalTasks := stats.CompletedTasks + stats.FailedTasks
		if totalTasks > 0 {
			failRate := float64(stats.FailedTasks) / float64(totalTasks)
			if failRate > 0.5 {
				stats.HealthStatus = "degraded"
			}
			if failRate > 0.8 {
				stats.HealthStatus = "unhealthy"
			}
		}
	}
}

// UpdateHealthStatus 更新健康狀態
func (lb *LoadBalancer) UpdateHealthStatus(executor ExecutorType, status string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	stats := lb.executors[executor]
	if stats != nil {
		stats.HealthStatus = status
	}
}

// GetStats 獲取所有執行者統計
func (lb *LoadBalancer) GetStats() map[ExecutorType]ExecutorStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make(map[ExecutorType]ExecutorStats)
	for et, stats := range lb.executors {
		result[et] = *stats
	}
	return result
}

// GetExecutorStats 獲取單個執行者統計
func (lb *LoadBalancer) GetExecutorStats(executor ExecutorType) *ExecutorStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	stats := lb.executors[executor]
	if stats != nil {
		return stats
	}
	return nil
}

// isHealthy 檢查執行者是否健康
func (lb *LoadBalancer) isHealthy(executor ExecutorType) bool {
	stats := lb.executors[executor]
	if stats == nil {
		return false
	}
	return stats.HealthStatus == "healthy"
}

// getHealthyExecutors 獲取所有健康執行者
func (lb *LoadBalancer) getHealthyExecutors() []ExecutorType {
	var healthy []ExecutorType
	for _, et := range []ExecutorType{ExecutorCL, ExecutorClaudeCode, ExecutorHybrid} {
		if lb.isHealthy(et) {
			healthy = append(healthy, et)
		}
	}
	return healthy
}

// getActiveTasks 獲取執行者的活躍任務數
func (lb *LoadBalancer) getActiveTasks(executor ExecutorType) int {
	stats := lb.executors[executor]
	if stats == nil {
		return 0
	}
	return stats.ActiveTasks
}

// SetStrategy 設置選擇策略
func (lb *LoadBalancer) SetStrategy(strategy LoadBalanceStrategy) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.strategy = strategy
}

// GetStrategy 獲取當前策略
func (lb *LoadBalancer) GetStrategy() LoadBalanceStrategy {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return lb.strategy
}

// GetTotalStats 獲取總體統計
func (lb *LoadBalancer) GetTotalStats() map[string]interface{} {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	totalTasks := 0
	totalTokens := 0
	totalCost := 0.0
	totalLatency := time.Duration(0)
	healthyCount := 0

	for _, stats := range lb.executors {
		totalTasks += stats.CompletedTasks
		totalTokens += stats.TotalTokens
		totalCost += stats.TotalCostUSD
		totalLatency += stats.TotalLatency
		if stats.HealthStatus == "healthy" {
			healthyCount++
		}
	}

	avgLatency := time.Duration(0)
	if totalTasks > 0 {
		avgLatency = totalLatency / time.Duration(totalTasks)
	}

	return map[string]interface{}{
		"total_completed_tasks": totalTasks,
		"total_tokens":          totalTokens,
		"total_cost_usd":        totalCost,
		"avg_latency":           avgLatency,
		"healthy_executors":     healthyCount,
		"current_strategy":      lb.strategy,
	}
}
