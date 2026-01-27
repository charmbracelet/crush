package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestModelSemaphore_AcquireUnlimited(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 0, // Unlimited
	}

	// Should acquire immediately without blocking
	release, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release)

	// Release should be a no-op
	release()
}

func TestModelSemaphore_AcquireWithLimit(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 2, // Limit to 2 concurrent
	}

	// Acquire with limit=2
	release1, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release1)

	// Second acquire should also succeed
	release2, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release2)

	// Third acquire should block (we'll test with timeout)
	done := make(chan bool)
	go func() {
		release3, err := ms.Acquire(ctx, modelCfg)
		if err != nil {
			t.Error(err)
			return
		}
		release3()
		done <- true
	}()

	// Should not complete until we release a slot
	select {
	case <-done:
		t.Fatal("Third acquire should have blocked")
	case <-time.After(100 * time.Millisecond):
		// Expected - still waiting
	}

	// Release first slot
	release1()

	// Now third acquire should complete
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Third acquire should have completed after release")
	}

	// Release remaining
	release2()
}

func TestModelSemaphore_ContextCancellation(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 1, // Limit to 1 concurrent
	}

	// Acquire the only slot
	ctx := context.Background()
	release1, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release1)
	defer release1()

	// Create a cancellable context
	cancelCtx, cancel := context.WithCancel(context.Background())

	// Start a goroutine that will be cancelled
	done := make(chan error)
	go func() {
		_, err := ms.Acquire(cancelCtx, modelCfg)
		done <- err
	}()

	// Cancel context before acquisition can complete
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Should get context cancelled error
	select {
	case err := <-done:
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Should have returned context cancelled error")
	}
}

func TestModelSemaphore_ContextDeadline(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 1, // Limit to 1 concurrent
	}

	// Acquire the only slot
	ctx := context.Background()
	release1, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release1)
	defer release1()

	// Create a context with short deadline
	deadlineCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Try to acquire - should time out
	release2, err := ms.Acquire(deadlineCtx, modelCfg)
	require.Error(t, err)
	require.Equal(t, context.DeadlineExceeded, err)
	require.Nil(t, release2) // Should return nil release function on error
}

func TestModelSemaphore_ConcurrentAcquire(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 5, // Limit to 5 concurrent
	}

	// Start 10 goroutines trying to acquire
	var wg sync.WaitGroup
	errors := make(chan error, 10)
	activeCount := 0
	maxActive := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			release, err := ms.Acquire(ctx, modelCfg)
			if err != nil {
				errors <- err
				return
			}

			// Track active count
			mu.Lock()
			activeCount++
			if activeCount > maxActive {
				maxActive = activeCount
			}
			mu.Unlock()

			// Hold for a bit
			time.Sleep(50 * time.Millisecond)

			// Release
			mu.Lock()
			activeCount--
			mu.Unlock()
			release()
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify we never exceeded the limit
	require.LessOrEqual(t, maxActive, 5, "Should not have exceeded concurrency limit")
}

func TestModelSemaphore_Release(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 1,
	}

	// Acquire and release multiple times
	for i := 0; i < 10; i++ {
		release, err := ms.Acquire(ctx, modelCfg)
		require.NoError(t, err)
		require.NotNil(t, release)

		release()

		// Should be able to acquire again immediately after release
		release2, err := ms.Acquire(ctx, modelCfg)
		require.NoError(t, err)
		require.NotNil(t, release2)

		release2()
	}
}

func TestModelSemaphore_MultipleModels(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()

	// Model A with limit 1
	modelA := config.SelectedModel{
		Provider:         "provider-a",
		Model:            "model-a",
		ConcurrencyLevel: 1,
	}

	// Model B with limit 1
	modelB := config.SelectedModel{
		Provider:         "provider-b",
		Model:            "model-b",
		ConcurrencyLevel: 1,
	}

	// Acquire model A
	releaseA, err := ms.Acquire(ctx, modelA)
	require.NoError(t, err)
	require.NotNil(t, releaseA)

	// Should still be able to acquire model B (different model)
	releaseB, err := ms.Acquire(ctx, modelB)
	require.NoError(t, err)
	require.NotNil(t, releaseB)

	// But not another model A
	done := make(chan bool)
	go func() {
		releaseA2, err := ms.Acquire(ctx, modelA)
		if err != nil {
			t.Error(err)
			return
		}
		releaseA2()
		done <- true
	}()

	select {
	case <-done:
		t.Fatal("Second acquire of model A should have blocked")
	case <-time.After(100 * time.Millisecond):
		// Expected - still waiting
	}

	// Release model A
	releaseA()

	// Now second acquire of model A should complete
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Second acquire should have completed")
	}

	// Release model B
	releaseB()
}

func TestDefaultModelSemaphore_Singleton(t *testing.T) {
	t.Parallel()

	// Get default instance twice
	sem1 := DefaultModelSemaphore()
	sem2 := DefaultModelSemaphore()

	// Should be the same instance
	require.Same(t, sem1, sem2, "DefaultModelSemaphore should return singleton instance")

	// Should actually work
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 1,
	}

	release1, err := sem1.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release1)
	defer release1()
}

func TestModelSemaphore_FastPathNoAllocation(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 0, // Unlimited - fast path
	}

	// Acquire many times rapidly - should not block
	for i := 0; i < 100; i++ {
		release, err := ms.Acquire(ctx, modelCfg)
		require.NoError(t, err)
		require.NotNil(t, release)
		release()
	}

	// Test concurrent acquisitions
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				release, err := ms.Acquire(ctx, modelCfg)
				if err != nil {
					t.Error(err)
					return
				}
				release()
			}
		}()
	}

	wg.Wait()
}

func TestModelSemaphore_ModelKey(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore().(*modelSemaphore)

	tests := []struct {
		name     string
		cfg      config.SelectedModel
		expected string
	}{
		{
			name: "Simple model",
			cfg: config.SelectedModel{
				Provider: "openai",
				Model:    "gpt-4",
			},
			expected: "openai:gpt-4",
		},
		{
			name: "Different provider",
			cfg: config.SelectedModel{
				Provider: "anthropic",
				Model:    "claude-3-opus",
			},
			expected: "anthropic:claude-3-opus",
		},
		{
			name: "Complex model name",
			cfg: config.SelectedModel{
				Provider: "azure",
				Model:    "gpt-4-turbo-2024-04-09",
			},
			expected: "azure:gpt-4-turbo-2024-04-09",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ms.modelKey(tt.cfg)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestNewModelSemaphore(t *testing.T) {
	t.Parallel()

	// Create multiple fresh instances
	ms1 := NewModelSemaphore()
	ms2 := NewModelSemaphore()

	// Should be different instances
	require.NotSame(t, ms1, ms2, "NewModelSemaphore should create fresh instances")

	// Both should work independently
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 1,
	}

	// Acquire from ms1
	release1, err := ms1.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release1)

	// Should be able to acquire from ms2 (different instance)
	release2, err := ms2.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release2)

	// Release both
	release1()
	release2()
}

func TestAcquire_NegativeConcurrencyLevel(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: -1, // Negative - should be treated as unlimited
	}

	// Should acquire immediately without blocking
	release, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release)

	// Release should be a no-op
	release()

	// Should be able to acquire many times
	for i := 0; i < 100; i++ {
		release, err := ms.Acquire(ctx, modelCfg)
		require.NoError(t, err)
		require.NotNil(t, release)
		release()
	}
}

func TestAcquire_MaxInt64ConcurrencyLevel(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 9223372036854775807, // Max int64
	}

	// Should acquire successfully
	release1, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release1)
	defer release1()

	// Should be able to acquire another one
	release2, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release2)
	defer release2()
}

func TestGeneration_NewSemaphoreOnFirstAcquire(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore().(*modelSemaphore)
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 2,
	}

	// First acquire should create semaphore
	release, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release)
	defer release()

	// Verify semaphore exists in map
	key := ms.modelKey(modelCfg)
	ms.mu.Lock()
	ref, ok := ms.semaphores[key]
	ms.mu.Unlock()
	require.True(t, ok, "Semaphore should exist in map")
	require.NotNil(t, ref)
	require.Equal(t, int64(2), ref.level)
}

func TestGeneration_ReuseSameGeneration(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore().(*modelSemaphore)
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 2,
	}

	// First acquire
	release1, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release1)

	// Get ref pointer after first acquire
	key := ms.modelKey(modelCfg)
	ms.mu.Lock()
	ref1 := ms.semaphores[key]
	ms.mu.Unlock()

	// Second acquire should reuse same semaphore
	release2, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release2)

	// Verify it's the same ref
	ms.mu.Lock()
	ref2 := ms.semaphores[key]
	ms.mu.Unlock()

	require.Same(t, ref1, ref2, "Should reuse same semaphore ref")

	release1()
	release2()
}

func TestGeneration_NewGenerationOnLevelIncrease(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore().(*modelSemaphore)
	ctx := context.Background()
	modelCfgLow := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 2,
	}

	// Acquire with limit=2
	release1, err := ms.Acquire(ctx, modelCfgLow)
	require.NoError(t, err)
	require.NotNil(t, release1)

	key := ms.modelKey(modelCfgLow)
	ms.mu.Lock()
	refLow := ms.semaphores[key]
	ms.mu.Unlock()
	require.Equal(t, int64(2), refLow.level)

	// Acquire with limit=5 (new generation)
	modelCfgHigh := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 5,
	}
	release2, err := ms.Acquire(ctx, modelCfgHigh)
	require.NoError(t, err)
	require.NotNil(t, release2)

	ms.mu.Lock()
	refHigh := ms.semaphores[key]
	ms.mu.Unlock()
	require.Equal(t, int64(5), refHigh.level)

	// Should be different refs
	require.NotSame(t, refLow, refHigh, "Should create new semaphore ref on level change")

	// Old semaphore should still work for its releases
	release1()
	release2()
}

func TestGeneration_NewGenerationOnLevelDecrease(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore().(*modelSemaphore)
	ctx := context.Background()
	modelCfgHigh := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 5,
	}

	// Acquire with limit=5
	release1, err := ms.Acquire(ctx, modelCfgHigh)
	require.NoError(t, err)
	require.NotNil(t, release1)

	key := ms.modelKey(modelCfgHigh)
	ms.mu.Lock()
	refHigh := ms.semaphores[key]
	ms.mu.Unlock()
	require.Equal(t, int64(5), refHigh.level)

	// Acquire with limit=2 (new generation)
	modelCfgLow := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 2,
	}
	release2, err := ms.Acquire(ctx, modelCfgLow)
	require.NoError(t, err)
	require.NotNil(t, release2)

	ms.mu.Lock()
	refLow := ms.semaphores[key]
	ms.mu.Unlock()
	require.Equal(t, int64(2), refLow.level)

	// Should be different refs
	require.NotSame(t, refHigh, refLow, "Should create new semaphore ref on level change")

	release1()
	release2()
}

func TestGeneration_OldSemaphoreStillWorks(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfgV1 := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 2,
	}

	// Acquire two slots with limit=2
	release1, err := ms.Acquire(ctx, modelCfgV1)
	require.NoError(t, err)
	require.NotNil(t, release1)

	release2, err := ms.Acquire(ctx, modelCfgV1)
	require.NoError(t, err)
	require.NotNil(t, release2)

	// Change to limit=5 (new generation)
	modelCfgV2 := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 5,
	}

	// Acquire with new limit
	release3, err := ms.Acquire(ctx, modelCfgV2)
	require.NoError(t, err)
	require.NotNil(t, release3)

	// Old releases should still work
	release1()
	release2()
	release3()
}

func TestGeneration_ConfigChangeImmediateEffect(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfgV1 := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 1,
	}

	// Acquire the only slot with limit=1
	release1, err := ms.Acquire(ctx, modelCfgV1)
	require.NoError(t, err)
	require.NotNil(t, release1)
	defer release1()

	// Try to acquire - should block
	done := make(chan bool)
	go func() {
		release, err := ms.Acquire(ctx, modelCfgV1)
		if err != nil {
			t.Error(err)
			return
		}
		release()
		done <- true
	}()

	// Verify it's blocked
	select {
	case <-done:
		t.Fatal("Should have blocked")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}

	// Change to limit=10 (simulated config change)
	modelCfgV2 := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 10,
	}

	// New acquire with higher limit should succeed immediately
	release2, err := ms.Acquire(ctx, modelCfgV2)
	require.NoError(t, err)
	require.NotNil(t, release2)
	defer release2()

	// Release old semaphore to unblock goroutine waiting on V1
	release1()
	<-done // Wait for goroutine to complete
}

func TestGeneration_MultipleConfigChanges(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore().(*modelSemaphore)
	ctx := context.Background()

	// Cycle through different limits
	limits := []int64{2, 5, 1, 10, 3}

	for i, limit := range limits {
		modelCfg := config.SelectedModel{
			Provider:         "test",
			Model:            "test-model",
			ConcurrencyLevel: limit,
		}

		release, err := ms.Acquire(ctx, modelCfg)
		require.NoError(t, err)
		require.NotNil(t, release)

		// Verify current level
		key := ms.modelKey(modelCfg)
		ms.mu.Lock()
		ref := ms.semaphores[key]
		ms.mu.Unlock()
		require.Equal(t, limit, ref.level, "Iteration %d: level mismatch", i)

		release()
	}
}

func TestDoubleRelease_Protected(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 1,
	}

	// Acquire
	release, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release)

	// Release once - should work
	release()

	// Release again - should be no-op (not panic)
	release()

	// Should still be able to acquire
	release2, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release2)
	release2()
}

func TestDoubleRelease_Concurrent(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 1,
	}

	// Acquire
	release, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release)

	// Call release from multiple goroutines concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release()
		}()
	}
	wg.Wait()

	// Should still be able to acquire (count should be 1, not negative)
	release2, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release2)
	release2()
}

func TestDoubleRelease_ConcurrencyLimitPreserved(t *testing.T) {
	t.Parallel()

	ms := NewModelSemaphore()
	ctx := context.Background()
	modelCfg := config.SelectedModel{
		Provider:         "test",
		Model:            "test-model",
		ConcurrencyLevel: 2,
	}

	// Acquire both slots
	release1, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release1)

	release2, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release2)

	// Double-release both
	release1()
	release1() // Extra - should be no-op
	release2()
	release2() // Extra - should be no-op

	// Now we should only be able to acquire 2 more (not 4)
	release3, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release3)

	release4, err := ms.Acquire(ctx, modelCfg)
	require.NoError(t, err)
	require.NotNil(t, release4)

	// Fifth acquire should block
	done := make(chan bool)
	go func() {
		release, err := ms.Acquire(ctx, modelCfg)
		if err != nil {
			t.Error(err)
			return
		}
		release()
		done <- true
	}()

	select {
	case <-done:
		t.Fatal("Should have blocked - double release broke limit")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}

	release3()
	<-done // Should complete now
	release4()
}
