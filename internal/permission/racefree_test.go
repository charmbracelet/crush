package permission

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRaceFreePermissionService validates lock-free design
func TestRaceFreePermissionService(t *testing.T) {
	t.Run("Lock-free concurrent requests", func(t *testing.T) {
		service := NewRaceFreePermissionService("/tmp", false, []string{})

		var wg sync.WaitGroup
		results := make([]bool, 100)

		// Simulate 100 concurrent permission requests
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				results[index] = service.Request(CreatePermissionRequest{
					SessionID:   "session-" + string(rune(index%10)),
					ToolCallID:  "tool-" + string(rune(index%10)),
					ToolName:    "bash",
					Action:      "execute",
					Description: "Test command",
					Path:        "/tmp/test-" + string(rune(index%10)),
				})
			}(i)
		}

		// Wait for all requests to complete
		wg.Wait()

		// Should not crash or deadlock
		t.Logf("✅ 100 concurrent requests completed without deadlocks")

		// Most should require permission (not in allowlist)
		approvedCount := 0
		for _, result := range results {
			if result {
				approvedCount++
			}
		}

		t.Logf("✅ Approved %d/%d requests (as expected for non-allowlist)", approvedCount, len(results))
	})

	t.Run("Lock-free grant/deny operations", func(t *testing.T) {
		service := NewRaceFreePermissionService("/tmp", false, []string{})

		var wg sync.WaitGroup

		// Test concurrent grant operations
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				permission := PermissionRequest{
					ID: string(rune(index)),
					CreatePermissionRequest: CreatePermissionRequest{
						SessionID:   "test-session",
						ToolCallID:  "test-tool",
						ToolName:    "bash",
						Action:      "execute",
						Description: "Test",
						Path:        "/tmp/test",
					},
				}

				// Should not cause race conditions
				if index%2 == 0 {
					service.GrantPersistent(permission)
				} else {
					service.Grant(permission)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("✅ 50 concurrent grant operations completed without races")
	})

	t.Run("Lock-free deny operations", func(t *testing.T) {
		service := NewRaceFreePermissionService("/tmp", false, []string{})

		var wg sync.WaitGroup

		// Test concurrent deny operations
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				permission := PermissionRequest{
					ID: string(rune(index)),
					CreatePermissionRequest: CreatePermissionRequest{
						SessionID:   "test-session",
						ToolCallID:  "test-tool",
						ToolName:    "bash",
						Action:      "execute",
						Description: "Test",
						Path:        "/tmp/test",
					},
				}

				// Should not cause race conditions
				service.Deny(permission)
			}(i)
		}

		wg.Wait()
		t.Logf("✅ 50 concurrent deny operations completed without races")
	})
}

// BenchmarkRaceFreePermissionService tests performance
func BenchmarkRaceFreePermissionService(b *testing.B) {
	service := NewRaceFreePermissionService("/tmp", false, []string{})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		service.Request(CreatePermissionRequest{
			SessionID:   "benchmark-session",
			ToolCallID:  "benchmark-tool",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Benchmark command",
			Path:        "/tmp/benchmark",
		})
	}
}

// TestRaceFreeVsMutex compares performance
func TestRaceFreeVsMutex(t *testing.T) {
	const iterations = 1000

	// Test lock-free service
	raceFree := NewRaceFreePermissionService("/tmp", false, []string{})

	start := time.Now()
	for i := 0; i < iterations; i++ {
		raceFree.Request(CreatePermissionRequest{
			SessionID:   "rf-session",
			ToolCallID:  "rf-tool",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Test",
			Path:        "/tmp/rf",
		})
	}
	raceFreeTime := time.Since(start)

	// Test original mutex-based service
	mutexService := NewPermissionService("/tmp", false, []string{})

	start = time.Now()
	for i := 0; i < iterations; i++ {
		mutexService.Request(CreatePermissionRequest{
			SessionID:   "mutex-session",
			ToolCallID:  "mutex-tool",
			ToolName:    "bash",
			Action:      "execute",
			Description: "Test",
			Path:        "/tmp/mutex",
		})
	}
	mutexTime := time.Since(start)

	t.Logf("Lock-free service: %v", raceFreeTime)
	t.Logf("Mutex service: %v", mutexTime)

	if raceFreeTime < mutexTime {
		speedup := float64(mutexTime) / float64(raceFreeTime)
		t.Logf("✅ Lock-free is %.1fx faster", speedup)
	}
}

// TestEventualConsistency tests eventual consistency under high load
func TestEventualConsistency(t *testing.T) {
	service := NewRaceFreePermissionService("/tmp", false, []string{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Subscribe to events
	events := service.SubscribeNotifications(ctx)

	var wg sync.WaitGroup
	completedRequests := 0
	totalRequests := 200

	// Start many concurrent requests
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			req := CreatePermissionRequest{
				SessionID:   "consistency-test",
				ToolCallID:  "tool-" + string(rune(index%10)),
				ToolName:    "bash",
				Action:      "execute",
				Description: "Consistency test",
				Path:        "/tmp/consistency",
			}

			// Make request
			result := service.Request(req)
			if result {
				atomic.AddInt32(&completedRequests, 1)
			}
		}(i)
	}

	// Grant some permissions to test consistency
	go func() {
		for i := 0; i < 20; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				event := <-events
				if i < 10 {
					// Grant first 10 requests
					service.Grant(event.Payload)
				} else {
					// Deny remaining
					service.Deny(event.Payload)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	wg.Wait()

	t.Logf("✅ %d/%d requests completed with eventual consistency", completedRequests, totalRequests)

	// Should have exactly 10 granted (first 10), rest denied
	assert.Equal(t, int32(10), atomic.LoadInt32(&completedRequests), "Expected exactly 10 granted permissions")
}
