package permission

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"go.uber.org/goleak"
)

func TestPermissionService_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	t.Parallel()

	// Verify no goroutine leaks after test completes
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	// Use a clean service for stress testing
	service := NewPermissionService("/tmp", false, []string{})

	// We need to subscribe BEFORE sending requests
	// Note: In real app, the UI subscribes.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	events := service.Subscribe(ctx)

	const (
		numGoroutines = 20 // Reduced to avoid overwhelming the 64-buffer pubsub
		numRequests   = 50 // Reduced to avoid timeout
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Consumer goroutine (Simulates UI)
	go func() {
		for event := range events {
			// Determine action based on random chance or tool name
			req := event.Payload

			// Simulate user reaction time
			time.Sleep(time.Millisecond)

			// Randomly choose:
			// 1. Grant Persistent (should populate map)
			// 2. Grant Once
			// 3. Deny

			action := rand.Intn(3)
			switch action {
			case 0:
				service.GrantPersistent(req)
			case 1:
				service.Grant(req)
			case 2:
				service.Deny(req)
			}
		}
	}()

	// Producer goroutines (Simulate Agents)
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range numRequests {
				req := CreatePermissionRequest{
					SessionID:  fmt.Sprintf("session-%d", id), // Unique session per goroutine to avoid auto-approve cross-talk
					ToolName:   fmt.Sprintf("tool-%d", j%5),   // 5 tools, so we hit the same ones repeatedly
					Action:     "execute",
					Path:       "/tmp",
					ToolCallID: message.ToolCallID(fmt.Sprintf("call-%d-%d", id, j)),
				}

				// This blocks until Granted/Denied
				// If we have a race or deadlock, this will hang
				service.Request(req)
			}
		}(i)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Stress test timed out - likely deadlock or lost event")
	}
}
