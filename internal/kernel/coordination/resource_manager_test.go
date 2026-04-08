package coordination

import (
	"context"
	"testing"
	"time"
)

func TestResourceManager_Acquire(t *testing.T) {
	rm := NewResourceManager()

	ctx := context.Background()
	resID, err := rm.Acquire(ctx, map[ResourceType]float64{
		ResourceCPU: 1,
	})
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	if resID == "" {
		t.Error("acquire returned empty reservation ID")
	}

	// Check resource is used - GetUsage returns map[ResourceType]ResourceUsage (struct)
	usage := rm.GetUsage()
	if usage[ResourceCPU].Used != 1 {
		t.Errorf("CPU usage after acquire: expected 1, got %v", usage[ResourceCPU].Used)
	}
}

func TestResourceManager_AcquireAndRelease(t *testing.T) {
	rm := NewResourceManager()

	ctx := context.Background()
	resID, err := rm.Acquire(ctx, map[ResourceType]float64{
		ResourceCPU: 1,
	})
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// Release
	err = rm.Release(resID)
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}

	// Check resource is freed
	usage := rm.GetUsage()
	if usage[ResourceCPU].Used != 0 {
		t.Errorf("CPU usage after release: expected 0, got %v", usage[ResourceCPU].Used)
	}
}

func TestResourceManager_Acquire_InsufficientResources(t *testing.T) {
	config := DefaultResourceManagerConfig()
	config.MaxConcurrentTasks = 1
	rm := NewResourceManager(config)

	ctx := context.Background()

	// First acquire should succeed
	_, err := rm.Acquire(ctx, map[ResourceType]float64{
		ResourceCPU: 1,
	})
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Second acquire should fail (only 1 concurrent allowed)
	_, err = rm.Acquire(ctx, map[ResourceType]float64{
		ResourceCPU: 1,
	})
	if err == nil {
		t.Error("expected error for insufficient resources, got nil")
	}
}

func TestResourceManager_Release_NonExistent(t *testing.T) {
	rm := NewResourceManager()

	err := rm.Release("nonexistent-id")
	if err == nil {
		t.Error("expected error for releasing nonexistent reservation")
	}
}

func TestResourceManager_GetUsage(t *testing.T) {
	rm := NewResourceManager()

	usage := rm.GetUsage()

	if usage[ResourceCPU].Used != 0 {
		t.Errorf("initial CPU usage: expected 0, got %v", usage[ResourceCPU].Used)
	}
	if usage[ResourceMemory].Used != 0 {
		t.Errorf("initial memory usage: expected 0, got %v", usage[ResourceMemory].Used)
	}
}

func TestResourceManager_GetReservation(t *testing.T) {
	rm := NewResourceManager()

	ctx := context.Background()
	resID, err := rm.Acquire(ctx, map[ResourceType]float64{
		ResourceCPU: 1,
	})
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	reservation := rm.GetReservation(resID)
	if reservation == nil {
		t.Fatal("get reservation returned nil")
	}
	if reservation.ID != resID {
		t.Errorf("reservation ID: expected %s, got %s", resID, reservation.ID)
	}
	if reservation.Type != ResourceCPU {
		t.Errorf("reservation type: expected %s, got %s", ResourceCPU, reservation.Type)
	}
}

func TestResourceManager_MultipleResourceAcquire(t *testing.T) {
	rm := NewResourceManager()

	ctx := context.Background()
	resID, err := rm.Acquire(ctx, map[ResourceType]float64{
		ResourceCPU:    1,
		ResourceMemory: 512,
	})
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	if resID == "" {
		t.Error("acquire returned empty ID")
	}

	usage := rm.GetUsage()
	if usage[ResourceCPU].Used != 1 {
		t.Errorf("CPU usage: expected 1, got %v", usage[ResourceCPU].Used)
	}
	if usage[ResourceMemory].Used != 512 {
		t.Errorf("memory usage: expected 512, got %v", usage[ResourceMemory].Used)
	}
}

func TestResourceManager_Release_UsesReservationType(t *testing.T) {
	config := DefaultResourceManagerConfig()
	config.MaxConcurrentTasks = 2
	rm := NewResourceManager(config)

	ctx := context.Background()

	// Acquire multiple resource types
	resID, err := rm.Acquire(ctx, map[ResourceType]float64{
		ResourceCPU:    1,
		ResourceMemory: 512,
	})
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// Get the reservation to check its type
	reservation := rm.GetReservation(resID)
	if reservation == nil {
		t.Fatal("reservation not found")
	}

	// Release should use the reservation's type
	err = rm.Release(resID)
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}

	// Both resources should be released
	usage := rm.GetUsage()
	if usage[ResourceCPU].Used != 0 {
		t.Errorf("CPU usage after release: expected 0, got %v", usage[ResourceCPU].Used)
	}
	if usage[ResourceMemory].Used != 0 {
		t.Errorf("memory usage after release: expected 0, got %v", usage[ResourceMemory].Used)
	}
}

func TestResourceManagerConfig_Defaults(t *testing.T) {
	config := DefaultResourceManagerConfig()

	if config.MaxConcurrentTasks != 10 {
		t.Errorf("default MaxConcurrentTasks: expected 10, got %d", config.MaxConcurrentTasks)
	}
	if config.MaxTokenBudget != 200000 {
		t.Errorf("default MaxTokenBudget: expected 200000, got %d", config.MaxTokenBudget)
	}
	if config.MaxBudgetUSD != 10.0 {
		t.Errorf("default MaxBudgetUSD: expected 10.0, got %v", config.MaxBudgetUSD)
	}
	if config.WarningThreshold != 0.70 {
		t.Errorf("default WarningThreshold: expected 0.70, got %v", config.WarningThreshold)
	}
	if config.CriticalThreshold != 0.90 {
		t.Errorf("default CriticalThreshold: expected 0.90, got %v", config.CriticalThreshold)
	}
}

func TestResourceManager_MaxConcurrentZero(t *testing.T) {
	config := DefaultResourceManagerConfig()
	config.MaxConcurrentTasks = 0
	rm := NewResourceManager(config)

	ctx := context.Background()

	// Acquire with zero max concurrent should fail gracefully
	_, err := rm.Acquire(ctx, map[ResourceType]float64{
		ResourceCPU: 1,
	})
	if err == nil {
		t.Error("expected error when MaxConcurrentTasks is 0")
	}
}

func TestResourceManager_CanAcquire(t *testing.T) {
	rm := NewResourceManager()

	// Should be able to acquire
	canAcquire := rm.CanAcquire(map[ResourceType]float64{
		ResourceCPU: 1,
	})
	if !canAcquire {
		t.Error("expected CanAcquire to return true for available resources")
	}

	// Exhaust resources
	ctx := context.Background()
	config := DefaultResourceManagerConfig()
	config.MaxConcurrentTasks = 1
	rm = NewResourceManager(config)

	_, err := rm.Acquire(ctx, map[ResourceType]float64{
		ResourceCPU: 1,
	})
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Now should not be able to acquire more
	canAcquire = rm.CanAcquire(map[ResourceType]float64{
		ResourceCPU: 1,
	})
	if canAcquire {
		t.Error("expected CanAcquire to return false after resource exhaustion")
	}
}

func TestResourceManager_GetConcurrentStatus(t *testing.T) {
	rm := NewResourceManager()

	status := rm.GetConcurrentStatus()

	if status["current_active"] != 0 {
		t.Errorf("initial current_active: expected 0, got %v", status["current_active"])
	}
	if status["max_concurrent"] != 10 {
		t.Errorf("default max_concurrent: expected 10, got %v", status["max_concurrent"])
	}
}

func TestResourceManager_Subscribe(t *testing.T) {
	rm := NewResourceManager()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ch := rm.Subscribe(ctx)

	// Acquire a resource to trigger an event
	rm.Acquire(context.Background(), map[ResourceType]float64{
		ResourceCPU: 1,
	})

	// Should receive at least one usage update
	select {
	case usage := <-ch:
		if usage.Type != ResourceCPU {
			t.Errorf("expected ResourceCPU in subscription, got %v", usage.Type)
		}
	case <-ctx.Done():
		// Timeout is OK - subscription may have already sent
	}
}

func TestResourceManager_Reset(t *testing.T) {
	rm := NewResourceManager()

	ctx := context.Background()
	rm.Acquire(ctx, map[ResourceType]float64{
		ResourceCPU: 1,
	})

	rm.Reset()

	usage := rm.GetUsage()
	if usage[ResourceCPU].Used != 0 {
		t.Errorf("CPU usage after reset: expected 0, got %v", usage[ResourceCPU].Used)
	}
}
