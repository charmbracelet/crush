package coordination

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ResourceType 資源類型
type ResourceType string

const (
	ResourceCPU    ResourceType = "cpu"
	ResourceMemory ResourceType = "memory"
	ResourceToken  ResourceType = "token"
	ResourceBudget ResourceType = "budget"
)

// ResourceUsage 資源使用情況
type ResourceUsage struct {
	Type        ResourceType
	Used        float64
	Limit       float64
	Utilization float64 // 使用率 0.0-1.0
}

// ResourceReservation 資源預留
type ResourceReservation struct {
	ID                string
	Type              ResourceType             // 主要資源類型
	Amount            float64                  // 主要資源數量
	AcquiredResources map[ResourceType]float64 // 所有獲取的資源及其數量
	CreatedAt         time.Time
	ExpiresAt         time.Time
	Released          bool
}

// ResourceManager 資源管理器
type ResourceManager struct {
	mu sync.RWMutex

	// 資源限制
	limits map[ResourceType]float64

	// 已使用資源
	used map[ResourceType]float64

	// 資源預留
	reservations map[string]*ResourceReservation

	// 並發控制
	maxConcurrent int
	currentActive int

	// 配置
	config ResourceManagerConfig

	// 監控通道
	monitorCh chan ResourceUsage
}

// ResourceManagerConfig 資源管理器配置
type ResourceManagerConfig struct {
	MaxConcurrentTasks int
	MaxTokenBudget     int
	MaxMemoryMB        int
	MaxBudgetUSD       float64
	WarningThreshold   float64 // 警告閾值
	CriticalThreshold  float64 // 緊急閾值
}

// DefaultResourceManagerConfig 返回預設配置
func DefaultResourceManagerConfig() ResourceManagerConfig {
	return ResourceManagerConfig{
		MaxConcurrentTasks: 10,
		MaxTokenBudget:     200000,
		MaxMemoryMB:        1024,
		MaxBudgetUSD:       10.0,
		WarningThreshold:   0.70,
		CriticalThreshold:  0.90,
	}
}

// NewResourceManager 創建新的資源管理器
func NewResourceManager(config ...ResourceManagerConfig) *ResourceManager {
	cfg := DefaultResourceManagerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	rm := &ResourceManager{
		limits: map[ResourceType]float64{
			ResourceCPU:    float64(cfg.MaxConcurrentTasks),
			ResourceMemory: float64(cfg.MaxMemoryMB),
			ResourceToken:  float64(cfg.MaxTokenBudget),
			ResourceBudget: cfg.MaxBudgetUSD,
		},
		used: map[ResourceType]float64{
			ResourceCPU:    0,
			ResourceMemory: 0,
			ResourceToken:  0,
			ResourceBudget: 0,
		},
		reservations:  make(map[string]*ResourceReservation),
		maxConcurrent: cfg.MaxConcurrentTasks,
		config:        cfg,
		monitorCh:     make(chan ResourceUsage, 100),
	}

	return rm
}

// Acquire 並發許可證
func (rm *ResourceManager) Acquire(ctx context.Context, resources map[ResourceType]float64) (string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 生成預留 ID
	reservationID := fmt.Sprintf("res-%d", time.Now().UnixNano())

	// 檢查資源是否足夠
	for resType, amount := range resources {
		available := rm.limits[resType] - rm.used[resType]
		if amount > available {
			return "", fmt.Errorf("insufficient %s: requested %.2f, available %.2f", resType, amount, available)
		}
	}

	// 創建預留 - 使用第一個資源類型作為主要類型
	var mainType ResourceType
	var mainAmount float64
	for resType, amount := range resources {
		if mainType == "" {
			mainType = resType
			mainAmount = amount
		}
	}
	reservation := &ResourceReservation{
		ID:                reservationID,
		Type:              mainType,
		Amount:            mainAmount,
		AcquiredResources: resources,
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(5 * time.Minute),
		Released:          false,
	}

	// 佔用資源
	for resType, amount := range resources {
		rm.used[resType] += amount
	}

	rm.reservations[reservationID] = reservation
	rm.currentActive++

	usage := make(map[ResourceType]ResourceUsage)
	for resType := range resources {
		limit := rm.limits[resType]
		used := rm.used[resType]
		utilization := 0.0
		if limit > 0 {
			utilization = used / limit
		}
		usage[resType] = ResourceUsage{
			Type:        resType,
			Used:        used,
			Limit:       limit,
			Utilization: utilization,
		}
	}
	rm.emitUsage(usage)

	return reservationID, nil
}

// Release 釋放資源
func (rm *ResourceManager) Release(reservationID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	reservation, ok := rm.reservations[reservationID]
	if !ok {
		return fmt.Errorf("reservation not found: %s", reservationID)
	}

	if reservation.Released {
		return fmt.Errorf("reservation already released: %s", reservationID)
	}

	// 釋放資源 - 釋放所有獲取的資源
	for resType, amount := range reservation.AcquiredResources {
		rm.used[resType] -= amount
	}
	if reservation.Type != ResourceCPU {
		rm.currentActive--
	}

	reservation.Released = true
	delete(rm.reservations, reservationID)

	usage := make(map[ResourceType]ResourceUsage)
	for resType := range reservation.AcquiredResources {
		limit := rm.limits[resType]
		used := rm.used[resType]
		utilization := 0.0
		if limit > 0 {
			utilization = used / limit
		}
		usage[resType] = ResourceUsage{
			Type:        resType,
			Used:        used,
			Limit:       limit,
			Utilization: utilization,
		}
	}
	rm.emitUsage(usage)

	return nil
}

// CanAcquire 檢查是否可以獲取資源
func (rm *ResourceManager) CanAcquire(resources map[ResourceType]float64) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	for resType, amount := range resources {
		available := rm.limits[resType] - rm.used[resType]
		if amount > available {
			return false
		}
	}
	return true
}

// GetUsage 獲取資源使用情況
func (rm *ResourceManager) GetUsage() map[ResourceType]ResourceUsage {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	usage := make(map[ResourceType]ResourceUsage)
	for resType, limit := range rm.limits {
		used := rm.used[resType]
		utilization := 0.0
		if limit > 0 {
			utilization = used / limit
		}
		usage[resType] = ResourceUsage{
			Type:        resType,
			Used:        used,
			Limit:       limit,
			Utilization: utilization,
		}
	}
	return usage
}

// GetWarningLevel 獲取警告級別
func (rm *ResourceManager) GetWarningLevel() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	maxUtil := 0.0
	for _, u := range rm.GetUsage() {
		if u.Utilization > maxUtil {
			maxUtil = u.Utilization
		}
	}

	if maxUtil >= rm.config.CriticalThreshold {
		return "critical"
	}
	if maxUtil >= rm.config.WarningThreshold {
		return "warning"
	}
	return "normal"
}

// SetLimit 設置資源限制
func (rm *ResourceManager) SetLimit(resourceType ResourceType, limit float64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.limits[resourceType] = limit
}

// RecordUsage 記錄資源使用
func (rm *ResourceManager) RecordUsage(resourceType ResourceType, amount float64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.used[resourceType] += amount
	usage := make(map[ResourceType]ResourceUsage)
	for resType, limit := range rm.limits {
		used := rm.used[resType]
		utilization := 0.0
		if limit > 0 {
			utilization = used / limit
		}
		usage[resType] = ResourceUsage{
			Type:        resType,
			Used:        used,
			Limit:       limit,
			Utilization: utilization,
		}
	}
	rm.emitUsage(usage)
}

// GetConcurrentStatus 返回並發狀態
func (rm *ResourceManager) GetConcurrentStatus() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	utilization := 0.0
	if rm.maxConcurrent > 0 {
		utilization = float64(rm.currentActive) / float64(rm.maxConcurrent)
	}
	return map[string]interface{}{
		"current_active": rm.currentActive,
		"max_concurrent": rm.maxConcurrent,
		"available":      rm.maxConcurrent - rm.currentActive,
		"utilization":    utilization,
	}
}

// Subscribe 訂閱資源監控
func (rm *ResourceManager) Subscribe(ctx context.Context) <-chan ResourceUsage {
	ch := make(chan ResourceUsage, 10)
	go func() {
		for {
			select {
			case usage := <-rm.monitorCh:
				select {
				case ch <- usage:
				case <-ctx.Done():
					close(ch)
					return
				}
			case <-ctx.Done():
				close(ch)
				return
			}
		}
	}()
	return ch
}

func (rm *ResourceManager) emitUsage(usage map[ResourceType]ResourceUsage) {
	types := make([]ResourceType, 0, len(usage))
	for t := range usage {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	for _, t := range types {
		select {
		case rm.monitorCh <- usage[t]:
		default:
		}
	}
}

// Reset 重置資源狀態
func (rm *ResourceManager) Reset() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for resType := range rm.used {
		rm.used[resType] = 0
	}
	rm.reservations = make(map[string]*ResourceReservation)
	rm.currentActive = 0
}

// GetReservation 獲取預留
func (rm *ResourceManager) GetReservation(id string) *ResourceReservation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.reservations[id]
}
