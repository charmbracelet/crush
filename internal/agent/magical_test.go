package agent

import (
	"testing"
	"time"

	"charm.land/fantasy"
)

func TestCircuitBreakerIntegration(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 1*time.Second)
	
	// Simulate failures
	for i := 0; i < 3; i++ {
		delay, shouldTrip := cb.RecordFailure(&fantasy.ProviderError{
			Message:    "test error",
			StatusCode: 500,
		})
		if i < 2 {
			if shouldTrip {
				t.Errorf("Circuit should not trip after %d failures", i+1)
			}
		} else {
			if !shouldTrip {
				t.Errorf("Circuit should trip after %d failures", i+1)
			}
		}
		_ = delay
	}
	
	// Circuit should be open
	if cb.State() != CircuitOpen {
		t.Errorf("Expected circuit to be open, got %v", cb.State())
	}
	
	// Allow should return false when circuit is open
	if cb.Allow() {
		t.Error("Allow should return false when circuit is open")
	}
	
	// Wait for timeout
	time.Sleep(1100 * time.Millisecond)
	
	// Circuit should transition to half-open
	if !cb.Allow() {
		t.Error("Allow should return true after timeout in half-open state")
	}
	
	t.Log("Circuit breaker integration test passed")
}

func TestContextManagerIntegration(t *testing.T) {
	cm := newContextManager()
	
	// Add entries at different layers
	cm.Add(ContextEntry{
		Content:    "test content 1",
		Importance: 0.8,
		Layer:      L1RollingWindow,
	})
	
	cm.Add(ContextEntry{
		Content:    "test content 2",
		Importance: 0.6,
		Layer:      L2TopicSegmentation,
	})
	
	cm.Add(ContextEntry{
		Content:    "error: test error: delay: 1s",
		Importance: 0.8,
		Layer:      L3ImportantMemory,
	})
	
	// Verify entries are stored
	l1Entries := cm.GetContextForLayer(L1RollingWindow)
	if len(l1Entries) != 1 {
		t.Errorf("Expected 1 L1 entry, got %d", len(l1Entries))
	}
	
	// Test OnRetry
	cm.OnRetry(&fantasy.ProviderError{
		Message: "rate limit",
	}, 1*time.Second)
	
	// Verify error was logged
	summary := cm.Summarize()
	if summary == "" {
		t.Error("Summary should not be empty")
	}
	
	t.Logf("Context manager summary: %s", summary)
	t.Log("Context manager integration test passed")
}

func TestStreamingMonitorIntegration(t *testing.T) {
	sm := newStreamingMonitor()
	
	// Start a tool
	sm.StartTool("call-123", "bash", "echo hello")
	
	// Complete the tool
	sm.CompleteTool("call-123", "")
	
	// Verify execution is tracked
	stats := sm.GetExecutionStats()
	if stats["completed"].(int) != 1 {
		t.Errorf("Expected 1 completed execution, got %v", stats["completed"])
	}
	
	t.Log("Streaming monitor integration test passed")
}
