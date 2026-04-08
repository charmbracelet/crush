// Copyright 2026 CrushCL. All rights reserved.
//
// Budget Persistence - 預算追蹤持久化
// 跨會話預算管理和追蹤

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BudgetConfig 預算配置
type BudgetConfig struct {
	MaxBudgetUSD      float64            `json:"max_budget_usd"`
	WarningThreshold  float64            `json:"warning_threshold"`
	DailyLimitUSD     float64            `json:"daily_limit_usd"`
	MonthlyLimitUSD   float64            `json:"monthly_limit_usd"`
	PerTaskLimitUSD   float64            `json:"per_task_limit_usd"`
	ModelCostOverride map[string]float64 `json:"model_cost_override,omitempty"`
}

// BudgetRecord 預算記錄
type BudgetRecord struct {
	Date        time.Time `json:"date"`
	TaskType   string    `json:"task_type"`
	Executor   string    `json:"executor"`
	CostUSD    float64   `json:"cost_usd"`
	Tokens     int       `json:"tokens"`
	SessionID  string    `json:"session_id"`
}

// BudgetManager 預算管理器
type BudgetManager struct {
	mu       sync.RWMutex
	config   *BudgetConfig
	usedUSD  float64
	records  []BudgetRecord

	// 每日/每月追蹤
	dailySpent   map[string]float64 // 日期 -> 金額
	monthlySpent map[string]float64 // 月份 -> 金額

	// 持久化
	persistPath string
}

// NewBudgetManager 創建新的預算管理器
func NewBudgetManager(config *BudgetConfig, persistPath string) *BudgetManager {
	bm := &BudgetManager{
		config:   config,
		usedUSD:  0,
		records:  make([]BudgetRecord, 0),
		dailySpent: make(map[string]float64),
		monthlySpent: make(map[string]float64),
		persistPath: persistPath,
	}

	// 嘗試加載持久化數據
	bm.Load()

	return bm
}

// CanSpend 檢查是否可以花費
func (bm *BudgetManager) CanSpend(amount float64) bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	// 總預算檢查
	if bm.usedUSD+amount > bm.config.MaxBudgetUSD {
		return false
	}

	// 每日限制檢查
	today := time.Now().Format("2006-01-02")
	if daily, ok := bm.dailySpent[today]; ok {
		if daily+amount > bm.config.DailyLimitUSD {
			return false
		}
	}

	// 每月限制檢查
	month := time.Now().Format("2006-01")
	if monthly, ok := bm.monthlySpent[month]; ok {
		if monthly+amount > bm.config.MonthlyLimitUSD {
			return false
		}
	}

	// 單任務限制檢查
	if amount > bm.config.PerTaskLimitUSD {
		return false
	}

	return true
}

// Spend 記錄支出
func (bm *BudgetManager) Spend(record BudgetRecord) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	amount := record.CostUSD

	// 再次檢查
	if bm.usedUSD+amount > bm.config.MaxBudgetUSD {
		return fmt.Errorf("budget exceeded: $%.4f + $%.4f > $%.4f",
			bm.usedUSD, amount, bm.config.MaxBudgetUSD)
	}

	// 更新總支出
	bm.usedUSD += amount

	// 添加記錄
	bm.records = append(bm.records, record)

	// 更新每日/每月支出
	today := record.Date.Format("2006-01-02")
	month := record.Date.Format("2006-01")
	bm.dailySpent[today] += amount
	bm.monthlySpent[month] += amount

	// 持久化
	bm.persist()

	return nil
}

// GetRemaining 返回剩餘預算
func (bm *BudgetManager) GetRemaining() float64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.config.MaxBudgetUSD - bm.usedUSD
}

// GetDailyRemaining 返回今日剩餘
func (bm *BudgetManager) GetDailyRemaining() float64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	today := time.Now().Format("2006-01-02")
	dailySpent := bm.dailySpent[today]
	return bm.config.DailyLimitUSD - dailySpent
}

// GetMonthlyRemaining 返回本月剩餘
func (bm *BudgetManager) GetMonthlyRemaining() float64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	month := time.Now().Format("2006-01")
	monthlySpent := bm.monthlySpent[month]
	return bm.config.MonthlyLimitUSD - monthlySpent
}

// IsWarning 返回是否達到警告閾值
func (bm *BudgetManager) IsWarning() bool {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.usedUSD/bm.config.MaxBudgetUSD > bm.config.WarningThreshold
}

// GetUsagePercent 返回使用百分比
func (bm *BudgetManager) GetUsagePercent() float64 {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	if bm.config.MaxBudgetUSD == 0 {
		return 0
	}
	return bm.usedUSD / bm.config.MaxBudgetUSD * 100
}

// GetStats 返回預算統計
func (bm *BudgetManager) GetStats() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return map[string]interface{}{
		"max_budget_usd":       bm.config.MaxBudgetUSD,
		"used_budget_usd":      bm.usedUSD,
		"remaining_budget_usd": bm.config.MaxBudgetUSD - bm.usedUSD,
		"daily_limit_usd":      bm.config.DailyLimitUSD,
		"daily_spent_usd":      bm.dailySpent[time.Now().Format("2006-01-02")],
		"daily_remaining_usd":   bm.config.DailyLimitUSD - bm.dailySpent[time.Now().Format("2006-01-02")],
		"monthly_limit_usd":    bm.config.MonthlyLimitUSD,
		"monthly_spent_usd":    bm.monthlySpent[time.Now().Format("2006-01")],
		"monthly_remaining_usd": bm.config.MonthlyLimitUSD - bm.monthlySpent[time.Now().Format("2006-01")],
		"per_task_limit_usd":   bm.config.PerTaskLimitUSD,
		"usage_percent":        bm.GetUsagePercent(),
		"warning":              bm.IsWarning(),
		"total_records":        len(bm.records),
	}
}

// GetRecords 返回支出記錄
func (bm *BudgetManager) GetRecords(limit int) []BudgetRecord {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	records := bm.records
	if limit > 0 && len(records) > limit {
		records = records[len(records)-limit:]
	}

	result := make([]BudgetRecord, len(records))
	copy(result, records)
	return result
}

// Reset 重置預算（月初/新年）
func (bm *BudgetManager) Reset() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.usedUSD = 0
	bm.records = make([]BudgetRecord, 0)
	bm.dailySpent = make(map[string]float64)
	bm.monthlySpent = make(map[string]float64)

	bm.persist()
}

// persist 保存到磁盤
func (bm *BudgetManager) persist() {
	if bm.persistPath == "" {
		return
	}

	data := struct {
		UsedUSD       float64            `json:"used_usd"`
		Records       []BudgetRecord     `json:"records"`
		DailySpent    map[string]float64 `json:"daily_spent"`
		MonthlySpent  map[string]float64 `json:"monthly_spent"`
	}{
		UsedUSD:      bm.usedUSD,
		Records:      bm.records,
		DailySpent:   bm.dailySpent,
		MonthlySpent: bm.monthlySpent,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal budget data: %v\n", err)
		return
	}

	// 確保目錄存在
	dir := filepath.Dir(bm.persistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create budget directory: %v\n", err)
		return
	}

	if err := os.WriteFile(bm.persistPath, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write budget file: %v\n", err)
	}
}

// Load 從磁盤加載
func (bm *BudgetManager) Load() error {
	if bm.persistPath == "" {
		return nil
	}

	data, err := os.ReadFile(bm.persistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read budget file: %w", err)
	}

	var budgetData struct {
		UsedUSD       float64            `json:"used_usd"`
		Records       []BudgetRecord     `json:"records"`
		DailySpent    map[string]float64 `json:"daily_spent"`
		MonthlySpent  map[string]float64 `json:"monthly_spent"`
	}

	if err := json.Unmarshal(data, &budgetData); err != nil {
		return fmt.Errorf("failed to unmarshal budget data: %w", err)
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.usedUSD = budgetData.UsedUSD
	bm.records = budgetData.Records
	bm.dailySpent = budgetData.DailySpent
	bm.monthlySpent = budgetData.MonthlySpent

	return nil
}

// SetMaxBudget 設置最大預算
func (bm *BudgetManager) SetMaxBudget(amount float64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.config.MaxBudgetUSD = amount
}

// SetDailyLimit 設置每日限制
func (bm *BudgetManager) SetDailyLimit(amount float64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.config.DailyLimitUSD = amount
}

// SetMonthlyLimit 設置每月限制
func (bm *BudgetManager) SetMonthlyLimit(amount float64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.config.MonthlyLimitUSD = amount
}
