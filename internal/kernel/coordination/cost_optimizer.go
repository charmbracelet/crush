// Copyright 2026 CrushCL. All rights reserved.
//
// CostOptimizer - 成本優化器
// 負責估算任務成本並決策執行者

package coordination

// CostOptimizerImpl 成本優化器實現
type CostOptimizerImpl struct {
	config *HybridBrainConfig
}

// NewCostOptimizerImpl 創建新的成本優化器
func NewCostOptimizerImpl(config HybridBrainConfig) *CostOptimizerImpl {
	return &CostOptimizerImpl{config: &config}
}

// Estimate 估算任務成本
func (c *CostOptimizerImpl) Estimate(classification TaskClassification) float64 {
	return classification.CostEstimate
}

// ShouldUseClaudeCode 判斷是否使用 Claude Code
func (c *CostOptimizerImpl) ShouldUseClaudeCode(classification TaskClassification) bool {
	// 預算不足時，降級到 CL
	if c.config.MaxBudgetUSD <= 0 {
		return false
	}

	// 簡單任務不應該使用 Claude Code（快速本地操作）
	if classification.TaskType == TaskQuickLookup {
		return false
	}

	// 預算警告時，只執行高置信度任務
	budgetThreshold := c.config.MaxBudgetUSD * c.config.WarningThreshold
	if budgetThreshold > 0 && classification.Confidence < 0.7 {
		return false
	}

	return true
}

// ShouldDowngrade 判斷是否應該降級
func (c *CostOptimizerImpl) ShouldDowngrade(classification TaskClassification, remainingBudget float64) bool {
	// 預算不足
	if remainingBudget < classification.CostEstimate {
		return true
	}

	// 任務太貴
	if classification.CostEstimate > remainingBudget*0.5 {
		return true
	}

	return false
}

// GetCostBreakdown 返回成本細分
func (c *CostOptimizerImpl) GetCostBreakdown(classification TaskClassification) map[string]float64 {
	return map[string]float64{
		"base_cost":           classification.CostEstimate,
		"remaining_budget":    c.config.MaxBudgetUSD,
		"estimated_remaining": c.config.MaxBudgetUSD - classification.CostEstimate,
	}
}
