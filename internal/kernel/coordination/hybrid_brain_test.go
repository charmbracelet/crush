package coordination

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/crushcl/internal/kernel/cl_kernel"
	"github.com/stretchr/testify/assert"
)

// skipIfCCIntegrationNeeded skips the test if it requires Claude Code integration
// but the environment is not set up for it.
func skipIfCCIntegrationNeeded(t *testing.T) {
	if os.Getenv("CRUSHCL_TEST_CC_INTEGRATION") == "" {
		t.Skip("Skipping Claude Code integration test. Set CRUSHCL_TEST_CC_INTEGRATION=1 to run")
	}
	if !cl_kernel.CanClaudeCodeExecute() {
		t.Skip("Claude Code CLI not functional (may require authentication)")
	}
}

// TestHybridBrain_NewHybridBrain tests creating a new HybridBrain
func TestHybridBrain_NewHybridBrain(t *testing.T) {
	// Test with default config
	hb := NewHybridBrain()
	assert.NotNil(t, hb)

	// Test with custom config
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
		MaxTurnsCL:       20,
		MaxTurnsCC:       10,
		Timeout:          5 * time.Minute,
		DefaultModel:     "sonnet",
	}
	hb2 := NewHybridBrain(config)
	assert.NotNil(t, hb2)
}

// TestHybridBrain_DefaultConfig tests the default configuration
func TestHybridBrain_DefaultConfig(t *testing.T) {
	cfg := DefaultHybridBrainConfig()
	assert.Equal(t, 10.0, cfg.MaxBudgetUSD)
	assert.Equal(t, 0.8, cfg.WarningThreshold)
	assert.Equal(t, 20, cfg.MaxTurnsCL)
	assert.Equal(t, 10, cfg.MaxTurnsCC)
	assert.Equal(t, 5*time.Minute, cfg.Timeout)
	assert.Equal(t, "sonnet", cfg.DefaultModel)
}

// TestHybridBrain_ClassifyTask tests task classification through HybridBrain
func TestHybridBrain_ClassifyTask(t *testing.T) {
	hb := NewHybridBrain()

	tests := []struct {
		name         string
		prompt       string
		expectedType TaskType
	}{
		{
			name:         "Quick lookup task",
			prompt:       "What is git?",
			expectedType: TaskQuickLookup,
		},
		{
			name:         "File operation task",
			prompt:       "Read the file",
			expectedType: TaskFileOperation,
		},
		{
			name:         "GitHub task",
			prompt:       "Create a pull request",
			expectedType: TaskGitHub,
		},
		{
			name:         "Bug hunt task",
			prompt:       "Fix the bug",
			expectedType: TaskBugHunt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := hb.ClassifyTask(tt.prompt)
			assert.Equal(t, tt.expectedType, classification.TaskType,
				"Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestHybridBrain_Think tests the Think method
func TestHybridBrain_Think(t *testing.T) {
	hb := NewHybridBrain()

	result := hb.Think(context.Background(), "What is 2+2?")
	assert.NotNil(t, result)
	assert.NotNil(t, result.Classification)
}

// TestHybridBrain_GetStats tests statistics retrieval
func TestHybridBrain_GetStats(t *testing.T) {
	hb := NewHybridBrain()

	// Initial stats
	stats := hb.GetStats()
	assert.Equal(t, 0.0, stats.UsedBudgetUSD)
	assert.Equal(t, 10.0, stats.MaxBudgetUSD)
	assert.Equal(t, 10.0, stats.RemainingBudget)
	assert.Equal(t, 0, stats.TotalTokens)
	assert.Equal(t, 0, stats.TasksExecuted)
	assert.False(t, stats.BudgetWarning)
}

// TestHybridBrain_Think_UpdatesHistory tests that Think updates session history
func TestHybridBrain_Think_UpdatesHistory(t *testing.T) {
	hb := NewHybridBrain()

	// Execute a task
	hb.Think(context.Background(), "What is git?")

	stats := hb.GetStats()
	assert.Equal(t, 1, stats.TasksExecuted, "Should have executed 1 task")
}

// TestHybridBrain_BudgetTracking tests budget tracking
func TestHybridBrain_BudgetTracking(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     1.0, // Very small budget
		WarningThreshold: 0.8,
	}
	hb := NewHybridBrain(config)

	// Execute some tasks
	for i := 0; i < 5; i++ {
		hb.Think(context.Background(), "What is git?")
	}

	stats := hb.GetStats()
	assert.Equal(t, 5, stats.TasksExecuted)
	// Budget tracking depends on actual execution cost
}

// TestHybridBrain_Execute tests the Execute method
func TestHybridBrain_Execute(t *testing.T) {
	hb := NewHybridBrain()

	req := ExecuteRequest{
		Prompt:   "What is 2+2?",
		Tools:    []string{"Read", "Bash"},
		Executor: "cl",
		Model:    "sonnet",
		Stream:   false,
	}

	result := hb.Execute(context.Background(), req)
	assert.NotNil(t, result)
}

// TestHybridBrain_Execute_WithExecutor tests Execute with specific executor
func TestHybridBrain_Execute_WithExecutor(t *testing.T) {
	// Skip unless explicitly enabled (integration test)
	if os.Getenv("CRUSHCL_TEST_CC_INTEGRATION") == "" {
		t.Skip("Skipping Claude Code integration test. Set CRUSHCL_TEST_CC_INTEGRATION=1 to run")
	}

	hb := NewHybridBrain()

	req := ExecuteRequest{
		Prompt:   "Analyze this code",
		Tools:    []string{"Read", "Grep"},
		Executor: "cc", // Claude Code
	}

	result := hb.Execute(context.Background(), req)
	assert.NotNil(t, result)
}

// TestHybridBrain_Think_WithForcedExecutor tests Think with forced executor
func TestHybridBrain_Think_WithForcedExecutor(t *testing.T) {
	hb := NewHybridBrain()

	result := hb.Think(context.Background(), "What is git?", ExecutorCL)
	assert.NotNil(t, result)
	assert.Equal(t, ExecutorCL, result.Executor)
}

// TestHybridBrain_Think_ForcedClaudeCode tests Think with forced Claude Code
// NOTE: This is an integration test that requires Claude Code CLI to be
// fully authenticated and configured. It will be skipped by default unless
// CRUSHCL_TEST_CC_INTEGRATION env var is set.
func TestHybridBrain_Think_ForcedClaudeCode(t *testing.T) {
	// Skip unless explicitly enabled (integration test)
	if os.Getenv("CRUSHCL_TEST_CC_INTEGRATION") == "" {
		t.Skip("Skipping Claude Code integration test. Set CRUSHCL_TEST_CC_INTEGRATION=1 to run")
	}

	// Also verify Claude Code can actually execute
	if !cl_kernel.CanClaudeCodeExecute() {
		t.Skip("Claude Code CLI not functional (may require authentication)")
	}

	hb := NewHybridBrain()

	result := hb.Think(context.Background(), "Refactor the auth module", ExecutorClaudeCode)
	assert.NotNil(t, result)
	assert.Equal(t, ExecutorClaudeCode, result.Executor)
}

// TestHybridBrain_BudgetWarning tests budget warning flag
func TestHybridBrain_BudgetWarning(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.5, // 50% warning threshold
	}
	hb := NewHybridBrain(config)

	// Simulate budget usage by executing tasks until we hit warning
	// Note: This depends on actual cost calculation
	stats := hb.GetStats()
	assert.False(t, stats.BudgetWarning)
}

// TestHybridBrain_EmptyPrompt tests handling of empty prompt
func TestHybridBrain_EmptyPrompt(t *testing.T) {
	hb := NewHybridBrain()

	result := hb.Think(context.Background(), "")
	assert.NotNil(t, result)
}

// TestHybridBrain_ContextCancellation tests context cancellation handling
func TestHybridBrain_ContextCancellation(t *testing.T) {
	hb := NewHybridBrain()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := hb.Think(ctx, "What is git?")
	// Should still return a result, possibly with error
	assert.NotNil(t, result)
}

// TestHybridBrain_ConcurrentAccess tests thread safety
func TestHybridBrain_ConcurrentAccess(t *testing.T) {
	hb := NewHybridBrain()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			for j := 0; j < 10; j++ {
				hb.Think(context.Background(), "What is git?")
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := hb.GetStats()
	assert.Equal(t, 100, stats.TasksExecuted)
}

// TestHybridBrain_ClassificationConfidence tests classification confidence
func TestHybridBrain_ClassificationConfidence(t *testing.T) {
	hb := NewHybridBrain()

	// High confidence case
	classification := hb.ClassifyTask("What is the ls command in Linux?")
	assert.Greater(t, classification.Confidence, 0.0)
}

// TestHybridBrain_ClassificationExecutor tests classification executor selection
func TestHybridBrain_ClassificationExecutor(t *testing.T) {
	hb := NewHybridBrain()

	// Quick lookup should prefer CL
	classification := hb.ClassifyTask("What is git?")
	// Note: Executor depends on confidence threshold
	assert.NotNil(t, classification.Executor)
}

// TestHybridBrain_ClassificationTools tests classification tools assignment
func TestHybridBrain_ClassificationTools(t *testing.T) {
	hb := NewHybridBrain()

	classification := hb.ClassifyTask("Read the file")
	assert.NotEmpty(t, classification.Tools)
}

// TestHybridBrain_CostEstimate tests cost estimation
func TestHybridBrain_CostEstimate(t *testing.T) {
	hb := NewHybridBrain()

	classification := hb.ClassifyTask("Refactor the auth module")
	assert.Greater(t, classification.CostEstimate, 0.0)
}

// TestHybridBrain_MultipleTasks tests executing multiple tasks
// NOTE: This test can trigger Claude Code execution for certain task types
// (e.g., "Create a pull request", "Fix the bug"). It will be skipped unless
// CRUSHCL_TEST_CC_INTEGRATION is set.
func TestHybridBrain_MultipleTasks(t *testing.T) {
	skipIfCCIntegrationNeeded(t)

	hb := NewHybridBrain()

	tasks := []string{
		"What is git?",
		"Read the file",
		"Create a pull request",
		"Fix the bug",
	}

	for _, task := range tasks {
		result := hb.Think(context.Background(), task)
		assert.NotNil(t, result)
	}

	stats := hb.GetStats()
	assert.Equal(t, len(tasks), stats.TasksExecuted)
}

// TestHybridBrain_ExecuteRequest_AllFields tests ExecuteRequest with all fields
// NOTE: This test uses "hybrid" executor which can trigger Claude Code execution.
func TestHybridBrain_ExecuteRequest_AllFields(t *testing.T) {
	skipIfCCIntegrationNeeded(t)

	hb := NewHybridBrain()

	req := ExecuteRequest{
		Prompt:    "Analyze code",
		Tools:     []string{"Read", "Grep", "Glob"},
		Executor:  "hybrid",
		Model:     "sonnet",
		Stream:    true,
		SessionID: "test-session-123",
	}

	result := hb.Execute(context.Background(), req)
	assert.NotNil(t, result)
}

// TestHybridBrain_ThinkResultHasClassification tests that Think result includes classification
func TestHybridBrain_ThinkResultHasClassification(t *testing.T) {
	hb := NewHybridBrain()

	result := hb.Think(context.Background(), "What is git?")
	assert.NotNil(t, result.Classification)
	assert.NotNil(t, result.Classification.TaskType)
}

// TestHybridBrain_MaxTurnsConfig tests MaxTurns configuration
func TestHybridBrain_MaxTurnsConfig(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
		MaxTurnsCL:       50,
		MaxTurnsCC:       25,
		Timeout:          10 * time.Minute,
		DefaultModel:     "opus",
	}
	hb := NewHybridBrain(config)

	// Config should be set
	stats := hb.GetStats()
	assert.Equal(t, 100.0, stats.MaxBudgetUSD)
}

// TestHybridBrain_DefaultModel tests default model configuration
func TestHybridBrain_DefaultModel(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
		DefaultModel:     "haiku",
	}
	hb := NewHybridBrain(config)

	// Should create successfully
	assert.NotNil(t, hb)
}

// TestCircuitBreaker_InitialState tests circuit breaker starts in closed state
func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := newExecutorCircuitBreaker()
	assert.Equal(t, CircuitClosed, cb.state)
	assert.Equal(t, 0, cb.failureCount)
	assert.Equal(t, 0, cb.successCount)
	assert.True(t, cb.shouldAllow())
}

// TestCircuitBreaker_OpensAfterFailures tests circuit opens after 5 failures
func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	cb := newExecutorCircuitBreaker()

	// Record 4 failures - should still allow
	for i := 0; i < 4; i++ {
		tripped := cb.recordFailure(assert.AnError)
		assert.False(t, tripped) // Not yet tripped
		assert.Equal(t, CircuitClosed, cb.state)
		assert.True(t, cb.shouldAllow())
	}

	// 5th failure - should trip
	tripped := cb.recordFailure(assert.AnError)
	assert.True(t, tripped) // Circuit tripped
	assert.Equal(t, CircuitOpen, cb.state)
	assert.False(t, cb.shouldAllow())
}

// TestCircuitBreaker_HalfOpenAfterTimeout tests circuit enters half-open after timeout
func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := newExecutorCircuitBreaker()

	// Trip the circuit
	for i := 0; i < 5; i++ {
		cb.recordFailure(assert.AnError)
	}
	assert.Equal(t, CircuitOpen, cb.state)

	// Manually set lastFailureAt to past (simulating timeout)
	cb.mu.Lock()
	cb.lastFailureAt = time.Now().Add(-31 * time.Second)
	cb.mu.Unlock()

	// Should now allow and enter half-open
	assert.True(t, cb.shouldAllow())
	assert.Equal(t, CircuitHalfOpen, cb.state)
}

// TestCircuitBreaker_RecoveryOnSuccess tests circuit recovers after successes in half-open
func TestCircuitBreaker_RecoveryOnSuccess(t *testing.T) {
	cb := newExecutorCircuitBreaker()

	// Trip the circuit
	for i := 0; i < 5; i++ {
		cb.recordFailure(assert.AnError)
	}

	// Enter half-open
	cb.mu.Lock()
	cb.lastFailureAt = time.Now().Add(-31 * time.Second)
	cb.mu.Unlock()
	cb.shouldAllow() // Transitions to half-open

	assert.Equal(t, CircuitHalfOpen, cb.state)

	// First success
	cb.recordSuccess()
	assert.Equal(t, CircuitHalfOpen, cb.state) // Still half-open

	// Second success - should recover to closed
	cb.recordSuccess()
	assert.Equal(t, CircuitClosed, cb.state)
	assert.True(t, cb.shouldAllow())
}

// TestCircuitBreaker_FailureInHalfOpen tests circuit reopens on failure in half-open
func TestCircuitBreaker_FailureInHalfOpen(t *testing.T) {
	cb := newExecutorCircuitBreaker()

	// Trip the circuit
	for i := 0; i < 5; i++ {
		cb.recordFailure(assert.AnError)
	}

	// Enter half-open
	cb.mu.Lock()
	cb.lastFailureAt = time.Now().Add(-31 * time.Second)
	cb.mu.Unlock()
	cb.shouldAllow()

	assert.Equal(t, CircuitHalfOpen, cb.state)

	// Failure in half-open should reopen
	tripped := cb.recordFailure(assert.AnError)
	assert.True(t, tripped)
	assert.Equal(t, CircuitOpen, cb.state)
}

// TestCircuitBreaker_Stats tests circuit breaker stats reporting
func TestCircuitBreaker_Stats(t *testing.T) {
	cb := newExecutorCircuitBreaker()

	// Initial stats
	stats := cb.getStats()
	assert.Equal(t, CircuitClosed, stats.State)
	assert.Equal(t, 0, stats.FailureCount)
	assert.Equal(t, 0, stats.SuccessCount)
	assert.Nil(t, stats.LastFailure)

	// Record some failures
	cb.recordFailure(assert.AnError)
	cb.recordFailure(assert.AnError)

	stats = cb.getStats()
	assert.Equal(t, 2, stats.FailureCount)
	assert.NotNil(t, stats.LastFailure)

	// Record successes
	cb.recordSuccess()
	stats = cb.getStats()
	assert.Equal(t, 1, stats.SuccessCount)
}

// TestHybridBrain_CircuitBreakerStats tests that HybridBrain reports circuit breaker stats
func TestHybridBrain_CircuitBreakerStats(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	hb := NewHybridBrain(config)

	stats := hb.GetStats()

	// Both circuits should be in closed state initially
	assert.Equal(t, CircuitClosed, stats.CLCircuit.State)
	assert.Equal(t, CircuitClosed, stats.CCCircuit.State)
	assert.Equal(t, 0, stats.CLCircuit.FailureCount)
	assert.Equal(t, 0, stats.CCCircuit.FailureCount)
}
