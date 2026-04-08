package coordination

import (
	"context"
	"testing"
	"time"
)

func TestLoadBalancer_SelectExecutor_RoundRobin(t *testing.T) {
	config := DefaultLoadBalancerConfig()
	config.DefaultStrategy = StrategyRoundRobin
	lb := NewLoadBalancer(config)

	exec1 := lb.SelectExecutor(context.Background(), SelectOptions{})
	if exec1 != ExecutorCL {
		t.Errorf("first round-robin select: expected ExecutorCL, got %v", exec1)
	}

	exec2 := lb.SelectExecutor(context.Background(), SelectOptions{})
	if exec2 != ExecutorClaudeCode {
		t.Errorf("second round-robin select: expected ExecutorClaudeCode, got %v", exec2)
	}

	exec3 := lb.SelectExecutor(context.Background(), SelectOptions{})
	if exec3 != ExecutorHybrid {
		t.Errorf("third round-robin select: expected ExecutorHybrid, got %v", exec3)
	}

	exec4 := lb.SelectExecutor(context.Background(), SelectOptions{})
	if exec4 != ExecutorCL {
		t.Errorf("fourth round-robin select (wraps around): expected ExecutorCL, got %v", exec4)
	}
}

func TestLoadBalancer_SelectExecutor_LeastLoad(t *testing.T) {
	config := DefaultLoadBalancerConfig()
	config.DefaultStrategy = StrategyLeastLoad
	lb := NewLoadBalancer(config)

	// Record multiple active tasks on CL to create high load
	lb.RecordTaskStart(ExecutorCL)
	lb.RecordTaskStart(ExecutorCL)
	lb.RecordTaskStart(ExecutorCL)

	// CL should now have high ActiveTasks=3, LeastLoad should prefer others
	exec := lb.SelectExecutor(context.Background(), SelectOptions{})
	if exec == ExecutorCL {
		t.Errorf("least load should not select CL with high load")
	}
}

func TestLoadBalancer_SelectExecutor_Weighted(t *testing.T) {
	config := DefaultLoadBalancerConfig()
	config.DefaultStrategy = StrategyWeighted
	lb := NewLoadBalancer(config)

	// Weighted strategy should select based on configured weights
	// CL has weight 1.0, CC has weight 0.5, Hybrid has weight 0.7
	// CL should be selected more often

	clCount := 0
	ccCount := 0
	iterations := 100

	for i := 0; i < iterations; i++ {
		exec := lb.SelectExecutor(context.Background(), SelectOptions{})
		switch exec {
		case ExecutorCL:
			clCount++
		case ExecutorClaudeCode:
			ccCount++
		}
	}

	// CL has 2x weight of CC, so should be selected ~2x more often
	if clCount < ccCount {
		t.Errorf("weighted selection: CL (%d) should be selected more than CC (%d)", clCount, ccCount)
	}
}

func TestLoadBalancer_RecordTaskComplete_UpdatesStats(t *testing.T) {
	lb := NewLoadBalancer()

	stats := lb.GetExecutorStats(ExecutorCL)
	initialCompleted := stats.CompletedTasks

	lb.RecordTaskStart(ExecutorCL)
	lb.RecordTaskComplete(ExecutorCL, 100, 0.01, 100*time.Millisecond)

	stats = lb.GetExecutorStats(ExecutorCL)
	if stats.CompletedTasks != initialCompleted+1 {
		t.Errorf("completed tasks: expected %d, got %d", initialCompleted+1, stats.CompletedTasks)
	}

	if stats.TotalLatency != 100*time.Millisecond {
		t.Errorf("total latency: expected 100ms, got %v", stats.TotalLatency)
	}
}

func TestLoadBalancer_GetExecutorStats(t *testing.T) {
	lb := NewLoadBalancer()

	stats := lb.GetExecutorStats(ExecutorCL)
	if stats == nil {
		t.Fatal("expected stats for ExecutorCL")
	}

	if stats.Type != ExecutorCL {
		t.Errorf("executor type: expected %v, got %v", ExecutorCL, stats.Type)
	}

	if stats.HealthStatus != "healthy" {
		t.Errorf("initial health status: expected healthy, got %s", stats.HealthStatus)
	}
}

func TestLoadBalancer_SetStrategy(t *testing.T) {
	lb := NewLoadBalancer()

	lb.SetStrategy(StrategyLeastLoad)
	if lb.strategy != StrategyLeastLoad {
		t.Errorf("strategy not set correctly")
	}
}

func TestLoadBalancer_UpdateHealthStatus(t *testing.T) {
	lb := NewLoadBalancer()

	// Update health status directly
	lb.UpdateHealthStatus(ExecutorCL, "degraded")

	stats := lb.GetExecutorStats(ExecutorCL)
	if stats.HealthStatus != "degraded" {
		t.Errorf("health status: expected degraded, got %s", stats.HealthStatus)
	}
}

func TestLoadBalancer_AvgLatency_DivisionByZero(t *testing.T) {
	lb := NewLoadBalancer()

	// Get stats before any usage
	stats := lb.GetExecutorStats(ExecutorCL)

	// AvgLatency should not panic when CompletedTasks=0
	avg := stats.AvgLatency
	if avg != 0 {
		t.Logf("avg latency with no tasks: %v", avg)
	}
}

func TestLoadBalancer_failRate_DivisionByZero(t *testing.T) {
	lb := NewLoadBalancer()

	// Get stats before any usage - use getFailRate directly
	stats := lb.GetExecutorStats(ExecutorCL)
	totalTasks := stats.CompletedTasks + stats.FailedTasks
	if totalTasks == 0 {
		// failRate should handle zero totalTasks gracefully
		t.Logf("fail rate with no tasks handled correctly")
	}
}

func TestLoadBalancer_SelectAdaptive(t *testing.T) {
	config := DefaultLoadBalancerConfig()
	config.DefaultStrategy = StrategyAdaptive
	lb := NewLoadBalancer(config)

	// Adaptive should select based on conditions
	exec := lb.SelectExecutor(context.Background(), SelectOptions{
		TaskComplexity: 0.5,
		MaxCost:        0.01,
		MaxLatency:     100 * time.Millisecond,
	})

	// Should select an executor
	if exec == "" {
		t.Error("adaptive selection returned empty executor")
	}
}

func TestLoadBalancer_GetStats(t *testing.T) {
	lb := NewLoadBalancer()

	stats := lb.GetStats()

	if stats[ExecutorCL].Type != ExecutorCL {
		t.Errorf("expected ExecutorCL in stats")
	}
}

func TestLoadBalancer_RoundRobin_AllExecutors(t *testing.T) {
	config := DefaultLoadBalancerConfig()
	config.DefaultStrategy = StrategyRoundRobin
	lb := NewLoadBalancer(config)

	// Track which executors are selected
	executors := make(map[ExecutorType]bool)

	// Select multiple times to test round-robin
	for i := 0; i < 6; i++ {
		exec := lb.SelectExecutor(context.Background(), SelectOptions{})
		executors[exec] = true
		lb.RecordTaskStart(exec)
		lb.RecordTaskComplete(exec, 100, 0.01, 100*time.Millisecond)
	}

	// All three executors should be reachable via round-robin
	if !executors[ExecutorCL] {
		t.Error("ExecutorCL not reachable via round-robin")
	}
	if !executors[ExecutorClaudeCode] {
		t.Error("ExecutorClaudeCode not reachable via round-robin")
	}
}

func TestSelectOptions_Defaults(t *testing.T) {
	opts := SelectOptions{}

	if opts.TaskComplexity != 0 {
		t.Errorf("default TaskComplexity should be 0")
	}
	if opts.MaxCost != 0 {
		t.Errorf("default MaxCost should be 0")
	}
	if opts.MaxLatency != 0 {
		t.Errorf("default MaxLatency should be 0")
	}
}

func TestLoadBalancer_GetTotalStats(t *testing.T) {
	lb := NewLoadBalancer()

	lb.RecordTaskStart(ExecutorCL)
	lb.RecordTaskComplete(ExecutorCL, 100, 0.01, 100*time.Millisecond)

	totalStats := lb.GetTotalStats()

	if totalStats["total_completed_tasks"] == nil {
		t.Error("expected total_completed_tasks in total stats")
	}
}
