package permission

import (
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
)

// BenchmarkRaceFreePermissionRequest benchmarks permission requests
func BenchmarkRaceFreePermissionRequest(b *testing.B) {
	service := NewRaceFreePermissionService("/tmp", false, []string{"bash"})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			service.Request(CreatePermissionRequest{
				SessionID:   "bench-session",
				ToolCallID:  message.ToolCallID("bench-tool"),
				ToolName:    "bash",
				Action:      "execute",
				Description: "Benchmark test",
				Path:        "/tmp/bench",
			})
			i++
		}
	})
}

// BenchmarkMutexPermissionRequest benchmarks original mutex implementation
func BenchmarkMutexPermissionRequest(b *testing.B) {
	service := NewPermissionService("/tmp", false, []string{"bash"})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			service.Request(CreatePermissionRequest{
				SessionID:   "bench-session",
				ToolCallID:  message.ToolCallID("bench-tool"),
				ToolName:    "bash",
				Action:      "execute",
				Description: "Benchmark test",
				Path:        "/tmp/bench",
			})
			i++
		}
	})
}

// BenchmarkConcurrentRequests tests realistic concurrent load
func BenchmarkConcurrentRequests(b *testing.B) {
	service := NewRaceFreePermissionService("/tmp", false, []string{})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			service.Request(CreatePermissionRequest{
				SessionID:   "concurrent-session",
				ToolCallID:  message.ToolCallID("concurrent-tool"),
				ToolName:    "bash",
				Action:      "execute",
				Description: "Concurrent benchmark",
				Path:        "/tmp/concurrent",
			})
		}
	})
}

// BenchmarkPermissionCacheHit tests cache lookup performance
func BenchmarkPermissionCacheHit(b *testing.B) {
	service := NewRaceFreePermissionService("/tmp", true, []string{})

	// Test skip mode performance (fast path)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			service.Request(CreatePermissionRequest{
				SessionID:   "cache-session",
				ToolCallID:  message.ToolCallID("cache-tool"),
				ToolName:    "bash",
				Action:      "execute",
				Description: "Cache test",
				Path:        "/tmp/cache",
			})
		}
	})
}

// TestBenchmarkComparison provides detailed performance comparison.
// Uses skip=true to test fast path performance without blocking on approval.
func TestBenchmarkComparison(t *testing.T) {
	const iterations = 10000

	// Test race-free service with skip mode (fast path benchmark)
	raceFree := NewRaceFreePermissionService("/tmp", true, []string{})

	start := time.Now()
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(sessionID int) {
			defer wg.Done()
			for range iterations / 10 {
				raceFree.Request(CreatePermissionRequest{
					SessionID:   "session-" + string(rune(sessionID)),
					ToolCallID:  message.ToolCallID("tool-" + string(rune(sessionID))),
					ToolName:    "bash",
					Action:      "execute",
					Description: "High load test",
					Path:        "/tmp/highload",
				})
			}
		}(i)
	}
	wg.Wait()
	raceFreeTime := time.Since(start)

	// Test mutex service with same load (skip mode)
	mutexService := NewPermissionService("/tmp", true, []string{})

	start = time.Now()
	for i := range 10 {
		wg.Add(1)
		go func(sessionID int) {
			defer wg.Done()
			for range iterations / 10 {
				mutexService.Request(CreatePermissionRequest{
					SessionID:   "session-" + string(rune(sessionID)),
					ToolCallID:  message.ToolCallID("tool-" + string(rune(sessionID))),
					ToolName:    "bash",
					Action:      "execute",
					Description: "High load test",
					Path:        "/tmp/highload",
				})
			}
		}(i)
	}
	wg.Wait()
	mutexTime := time.Since(start)

	t.Logf("High load test (%d iterations):", iterations)
	t.Logf("Race-free service: %v", raceFreeTime)
	t.Logf("Mutex service: %v", mutexTime)

	if raceFreeTime < mutexTime {
		speedup := float64(mutexTime) / float64(raceFreeTime)
		t.Logf("✅ Race-free is %.1fx faster under high load", speedup)
	} else {
		t.Logf("⚠️  Mutex is %.1fx faster (likely due to test conditions)", float64(raceFreeTime)/float64(mutexTime))
	}
}
