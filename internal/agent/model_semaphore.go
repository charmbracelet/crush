package agent

import (
	"context"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
	"golang.org/x/sync/semaphore"
)

// ModelSemaphore manages concurrency limits per model
type ModelSemaphore interface {
	// Acquire gets a token for given model configuration.
	// Returns (release function, error).
	//
	// Success: returns (releaseFunc, nil) - caller MUST call release()
	// Error: returns (nil, err) - caller MUST NOT call release (it's nil)
	//
	// Error cases:
	//   - context cancelled: returns (nil, ctx.Err())
	//   - context deadline: returns (nil, context.DeadlineExceeded)
	//
	// Fast path: if ConcurrencyLevel == 0 (unlimited),
	// returns (noopRelease, nil) without allocating any resources.
	Acquire(ctx context.Context, modelCfg config.SelectedModel) (func(), error)
}

// Private implementation struct
type modelSemaphore struct {
	mu         sync.Mutex
	semaphores map[string]*semaphoreRef
}

// semaphoreRef wraps a semaphore with its concurrency level for generational updates.
type semaphoreRef struct {
	sem   *semaphore.Weighted
	level int64 // Concurrency level for this generation
}

// Global singleton instance
var (
	defaultModelSemaphore ModelSemaphore
	defaultSemaphoreOnce  sync.Once
)

// DefaultModelSemaphore returns a global default instance
func DefaultModelSemaphore() ModelSemaphore {
	defaultSemaphoreOnce.Do(func() {
		defaultModelSemaphore = &modelSemaphore{
			semaphores: make(map[string]*semaphoreRef),
		}
	})
	return defaultModelSemaphore
}

// NewModelSemaphore creates a fresh instance for testing
func NewModelSemaphore() ModelSemaphore {
	return &modelSemaphore{
		semaphores: make(map[string]*semaphoreRef),
	}
}

// Acquire implementation with fast path for unlimited
func (ms *modelSemaphore) Acquire(ctx context.Context, modelCfg config.SelectedModel) (func(), error) {
	// FAST PATH: Check unlimited FIRST (before any work/allocation)
	// Treat negative or zero values as unlimited
	limit := modelCfg.ConcurrencyLevel
	if limit <= 0 {
		return func() {}, nil // No-op, no allocation, O(1)
	}

	// SLOW PATH: Limited concurrency (1+)
	key := ms.modelKey(modelCfg)

	ms.mu.Lock()
	ref, ok := ms.semaphores[key]
	if !ok || ref.level != limit {
		// Create new generation when level changes
		ref = &semaphoreRef{
			sem:   semaphore.NewWeighted(limit),
			level: limit,
		}
		ms.semaphores[key] = ref // Replace old ref
	}
	ms.mu.Unlock()

	// Acquire token
	if err := ref.sem.Acquire(ctx, 1); err != nil {
		// Context was cancelled or deadline exceeded
		return nil, err // Return nil release function
	}

	// Create sync.Once for this specific acquire (double-release protection)
	once := &sync.Once{}

	return func() {
		once.Do(func() {
			ref.sem.Release(1)
		})
	}, nil
}

// modelKey generates a unique key for the semaphore map.
// Key format: "provider:model" (e.g., "openai:gpt-4")
func (ms *modelSemaphore) modelKey(cfg config.SelectedModel) string {
	return cfg.Provider + ":" + cfg.Model
}
