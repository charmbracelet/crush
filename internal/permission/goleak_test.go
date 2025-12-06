package permission

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestPermissionService_GoroutineLeak(t *testing.T) {
	// Verify no goroutine leaks after test completes
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	
	service := NewPermissionService("/tmp", false, []string{})
	
	// Subscribe to consume events (prevents goroutine leaks)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	
	events := service.Subscribe(ctx)
	go func() {
		// Consume events to prevent goroutine leaks
		for range events {
			// Just drain the channel
		}
	}()
	
	// Test basic permission request flow
	result := service.Request(CreatePermissionRequest{
		SessionID:   "test-session",
		ToolName:    "test-tool",
		Action:      "test-action",
		Description: "test permission",
		Path:        "/tmp",
	})
	
	// Should return false (not in allowlist and not auto-approved)
	assert.False(t, result, "Should return false for non-allowlisted tool")
}