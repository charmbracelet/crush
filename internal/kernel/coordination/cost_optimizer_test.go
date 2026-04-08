package coordination

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCostOptimizerImpl_NewCostOptimizerImpl tests creating a new CostOptimizerImpl
func TestCostOptimizerImpl_NewCostOptimizerImpl(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}

	co := NewCostOptimizerImpl(config)
	assert.NotNil(t, co)
}

// TestCostOptimizerImpl_Estimate tests cost estimation
func TestCostOptimizerImpl_Estimate(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskQuickLookup,
		Executor:     ExecutorCL,
		Confidence:   0.9,
		CostEstimate: 0.001,
		Reason:       "Quick lookup task",
		Tools:        []string{"Read", "Bash"},
	}

	cost := co.Estimate(classification)
	assert.Equal(t, 0.001, cost)
}

// TestCostOptimizerImpl_ShouldUseClaudeCode_BudgetExceeded tests when budget is exceeded
func TestCostOptimizerImpl_ShouldUseClaudeCode_BudgetExceeded(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     0.0, // No budget
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskBugHunt,
		Executor:     ExecutorClaudeCode,
		Confidence:   0.9,
		CostEstimate: 0.05,
	}

	shouldUse := co.ShouldUseClaudeCode(classification)
	assert.False(t, shouldUse, "Should not use Claude Code when budget is 0")
}

// TestCostOptimizerImpl_ShouldUseClaudeCode_SimpleTaskHighCost tests simple task with high cost
func TestCostOptimizerImpl_ShouldUseClaudeCode_SimpleTaskHighCost(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskQuickLookup,
		Executor:     ExecutorCL,
		Confidence:   0.5,
		CostEstimate: 0.01, // High cost for quick lookup
	}

	shouldUse := co.ShouldUseClaudeCode(classification)
	assert.False(t, shouldUse, "Should not use Claude Code for simple task with high cost")
}

// TestCostOptimizerImpl_ShouldUseClaudeCode_HighConfidenceLowBudget tests high confidence with low budget
func TestCostOptimizerImpl_ShouldUseClaudeCode_HighConfidenceLowBudget(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskBugHunt,
		Executor:     ExecutorClaudeCode,
		Confidence:   0.9, // High confidence
		CostEstimate: 0.05,
	}

	shouldUse := co.ShouldUseClaudeCode(classification)
	assert.True(t, shouldUse, "Should use Claude Code for high confidence task")
}

// TestCostOptimizerImpl_ShouldDowngrade_BudgetInsufficient tests when budget is insufficient
func TestCostOptimizerImpl_ShouldDowngrade_BudgetInsufficient(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskComplexRefactor,
		Executor:     ExecutorClaudeCode,
		Confidence:   0.9,
		CostEstimate: 50.0,
	}

	remainingBudget := 10.0
	shouldDowngrade := co.ShouldDowngrade(classification, remainingBudget)
	assert.True(t, shouldDowngrade, "Should downgrade when budget is insufficient")
}

// TestCostOptimizerImpl_ShouldDowngrade_TaskTooExpensive tests when task is too expensive relative to budget
func TestCostOptimizerImpl_ShouldDowngrade_TaskTooExpensive(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskComplexRefactor,
		Executor:     ExecutorClaudeCode,
		Confidence:   0.9,
		CostEstimate: 60.0, // More than 50% of remaining budget
	}

	remainingBudget := 100.0
	shouldDowngrade := co.ShouldDowngrade(classification, remainingBudget)
	assert.True(t, shouldDowngrade, "Should downgrade when task costs more than 50% of remaining budget")
}

// TestCostOptimizerImpl_ShouldDowngrade_AffordableTask tests affordable task
func TestCostOptimizerImpl_ShouldDowngrade_AffordableTask(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskBugHunt,
		Executor:     ExecutorClaudeCode,
		Confidence:   0.9,
		CostEstimate: 10.0,
	}

	remainingBudget := 100.0
	shouldDowngrade := co.ShouldDowngrade(classification, remainingBudget)
	assert.False(t, shouldDowngrade, "Should not downgrade for affordable task")
}

// TestCostOptimizerImpl_GetCostBreakdown tests cost breakdown
func TestCostOptimizerImpl_GetCostBreakdown(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskBugHunt,
		Executor:     ExecutorClaudeCode,
		Confidence:   0.9,
		CostEstimate: 0.08,
		Reason:       "Bug hunt task",
	}

	breakdown := co.GetCostBreakdown(classification)
	assert.NotNil(t, breakdown)
	assert.Equal(t, 0.08, breakdown["base_cost"])
	assert.Equal(t, 100.0, breakdown["remaining_budget"])
	assert.Equal(t, 99.92, breakdown["estimated_remaining"])
}

// TestCostOptimizerImpl_Config tests configuration
func TestCostOptimizerImpl_Config(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     50.0,
		WarningThreshold: 0.7,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskCodeReview,
		Executor:     ExecutorClaudeCode,
		Confidence:   0.9,
		CostEstimate: 0.05,
	}

	// Budget threshold is 50.0 * 0.7 = 35.0
	// With confidence 0.9 > 0.7, should use Claude Code
	shouldUse := co.ShouldUseClaudeCode(classification)
	assert.True(t, shouldUse)
}

// TestCostOptimizerImpl_WarningThreshold tests warning threshold behavior
func TestCostOptimizerImpl_WarningThreshold(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.5, // 50% warning
	}
	co := NewCostOptimizerImpl(config)

	// Low confidence (0.5) should not use Claude Code when budget is tight
	classification := TaskClassification{
		TaskType:     TaskBugHunt,
		Executor:     ExecutorClaudeCode,
		Confidence:   0.5, // Below 0.7 threshold
		CostEstimate: 0.05,
	}

	shouldUse := co.ShouldUseClaudeCode(classification)
	assert.False(t, shouldUse, "Should not use Claude Code for low confidence task when budget is tight")
}

// TestCostOptimizerImpl_QuickLookupCost tests quick lookup cost behavior
func TestCostOptimizerImpl_QuickLookupCost(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	// Quick lookup with low cost should not use Claude Code
	classification := TaskClassification{
		TaskType:     TaskQuickLookup,
		Executor:     ExecutorCL,
		Confidence:   0.9,
		CostEstimate: 0.001,
	}

	shouldUse := co.ShouldUseClaudeCode(classification)
	assert.False(t, shouldUse)
}

// TestCostOptimizerImpl_ComplexRefactorCost tests complex refactor cost behavior
func TestCostOptimizerImpl_ComplexRefactorCost(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	// Complex refactor should use Claude Code
	classification := TaskClassification{
		TaskType:     TaskComplexRefactor,
		Executor:     ExecutorClaudeCode,
		Confidence:   0.9,
		CostEstimate: 0.10,
	}

	shouldUse := co.ShouldUseClaudeCode(classification)
	assert.True(t, shouldUse)
}

// TestCostOptimizerImpl_EmptyClassification tests with empty classification
func TestCostOptimizerImpl_EmptyClassification(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskUnknown,
		Executor:     ExecutorCL,
		Confidence:   0.0,
		CostEstimate: 0.0,
	}

	cost := co.Estimate(classification)
	assert.Equal(t, 0.0, cost)
}

// TestCostOptimizerImpl_ZeroBudget tests zero budget behavior
func TestCostOptimizerImpl_ZeroBudget(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     0.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	classification := TaskClassification{
		TaskType:     TaskBugHunt,
		Executor:     ExecutorClaudeCode,
		Confidence:   0.9,
		CostEstimate: 0.05,
	}

	shouldUse := co.ShouldUseClaudeCode(classification)
	assert.False(t, shouldUse, "Should not use Claude Code when budget is zero")

	shouldDowngrade := co.ShouldDowngrade(classification, 0.0)
	assert.True(t, shouldDowngrade, "Should downgrade when budget is zero")
}

// TestCostOptimizerImpl_DifferentTaskTypes tests different task types
func TestCostOptimizerImpl_DifferentTaskTypes(t *testing.T) {
	config := HybridBrainConfig{
		MaxBudgetUSD:     100.0,
		WarningThreshold: 0.8,
	}
	co := NewCostOptimizerImpl(config)

	taskTypes := []TaskType{
		TaskUnknown,
		TaskQuickLookup,
		TaskFileOperation,
		TaskDataProcessing,
		TaskGitHub,
		TaskComplexRefactor,
		TaskCodeReview,
		TaskBugHunt,
		TaskCreative,
		TaskMCPTask,
	}

	for _, taskType := range taskTypes {
		classification := TaskClassification{
			TaskType:     taskType,
			Executor:     ExecutorClaudeCode,
			Confidence:   0.9,
			CostEstimate: 0.05,
		}

		cost := co.Estimate(classification)
		assert.GreaterOrEqual(t, cost, 0.0, "Cost should be non-negative for %v", taskType)
	}
}
