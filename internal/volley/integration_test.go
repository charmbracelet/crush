package volley

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/llm/agent"
	"github.com/bwl/cliffy/internal/message"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
)

// mockAgent simulates an agent for testing
type mockAgent struct {
	delay        time.Duration
	failureRate  float64 // 0.0 to 1.0
	callCount    atomic.Int32
	concurrency  atomic.Int32
	maxConcurrent atomic.Int32
}

func newMockAgent(delay time.Duration, failureRate float64) *mockAgent {
	return &mockAgent{
		delay:       delay,
		failureRate: failureRate,
	}
}

func (m *mockAgent) Model() catwalk.Model {
	return catwalk.Model{
		ID:            "mock-model",
		CostPer1MIn:   1.0,
		CostPer1MOut:  3.0,
	}
}

func (m *mockAgent) Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan agent.AgentEvent, error) {
	// Track concurrency
	current := m.concurrency.Add(1)
	m.callCount.Add(1)

	// Update max concurrency seen
	for {
		max := m.maxConcurrent.Load()
		if current <= max || m.maxConcurrent.CompareAndSwap(max, current) {
			break
		}
	}

	events := make(chan agent.AgentEvent, 1)

	go func() {
		defer m.concurrency.Add(-1)
		defer close(events)

		// Simulate work
		select {
		case <-time.After(m.delay):
			// Simulate random failures
			if m.failureRate > 0 && float64(m.callCount.Load()%100)/100.0 < m.failureRate {
				events <- agent.AgentEvent{
					Type:  agent.AgentEventTypeError,
					Error: fmt.Errorf("simulated failure"),
				}
				return
			}

			// Success
			events <- agent.AgentEvent{
				Type: agent.AgentEventTypeResponse,
				Message: message.Message{
					ID:        "test-msg",
					SessionID: sessionID,
					Role:      message.Assistant,
					Parts: []message.ContentPart{
						message.TextContent{Text: fmt.Sprintf("Response to: %s", content)},
					},
				},
			}
		case <-ctx.Done():
			events <- agent.AgentEvent{
				Type:  agent.AgentEventTypeError,
				Error: ctx.Err(),
			}
		}
	}()

	return events, nil
}

func (m *mockAgent) Cancel(sessionID string) {}
func (m *mockAgent) CancelAll() {}
func (m *mockAgent) IsSessionBusy(sessionID string) bool { return false }
func (m *mockAgent) IsBusy() bool { return false }
func (m *mockAgent) Summarize(ctx context.Context, sessionID string) error { return nil }
func (m *mockAgent) UpdateModel() error { return nil }
func (m *mockAgent) QueuedPrompts(sessionID string) int { return 0 }
func (m *mockAgent) ClearQueue(sessionID string) {}

// TestSchedulerConcurrency verifies that the scheduler respects max concurrency
func TestSchedulerConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create mock agent with 100ms delay
	mockAg := newMockAgent(100*time.Millisecond, 0.0)

	// Create config and message store
	cfg := &config.Config{}
	store := message.NewStore()

	// Create scheduler with max 3 concurrent
	opts := VolleyOptions{
		MaxConcurrent: 3,
		MaxRetries:    0,
		ShowProgress:  false,
	}
	scheduler := NewScheduler(cfg, mockAg, store, opts)

	// Create 10 tasks
	tasks := make([]Task, 10)
	for i := range tasks {
		tasks[i] = Task{
			Index:  i + 1,
			Prompt: fmt.Sprintf("task %d", i+1),
		}
	}

	// Execute
	results, summary, err := scheduler.Execute(ctx, tasks)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify all tasks completed
	if len(results) != 10 {
		t.Errorf("got %d results, want 10", len(results))
	}

	// Verify concurrency was respected
	maxConcurrent := mockAg.maxConcurrent.Load()
	if maxConcurrent > 3 {
		t.Errorf("max concurrent was %d, want <= 3", maxConcurrent)
	}

	// Verify all succeeded
	if summary.SucceededTasks != 10 {
		t.Errorf("got %d succeeded, want 10", summary.SucceededTasks)
	}

	// Verify total calls
	totalCalls := mockAg.callCount.Load()
	if totalCalls != 10 {
		t.Errorf("got %d total calls, want 10", totalCalls)
	}
}

// TestSchedulerRetries verifies retry logic
func TestSchedulerRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create mock agent with 50% failure rate
	mockAg := newMockAgent(10*time.Millisecond, 0.5)

	cfg := &config.Config{}
	store := message.NewStore()

	opts := VolleyOptions{
		MaxConcurrent: 2,
		MaxRetries:    3,
		ShowProgress:  false,
	}
	scheduler := NewScheduler(cfg, mockAg, store, opts)

	// Create 5 tasks
	tasks := make([]Task, 5)
	for i := range tasks {
		tasks[i] = Task{
			Index:  i + 1,
			Prompt: fmt.Sprintf("task %d", i+1),
		}
	}

	// Execute
	results, summary, err := scheduler.Execute(ctx, tasks)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify we got results
	if len(results) != 5 {
		t.Errorf("got %d results, want 5", len(results))
	}

	// With 50% failure rate and 3 retries, we should have some retries
	if summary.TotalRetries == 0 {
		t.Log("Warning: Expected some retries with 50% failure rate")
	}

	t.Logf("Summary: %d succeeded, %d failed, %d total retries",
		summary.SucceededTasks, summary.FailedTasks, summary.TotalRetries)
}

// TestSchedulerCancellation verifies context cancellation works
func TestSchedulerCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create mock agent with long delay
	mockAg := newMockAgent(1*time.Second, 0.0)

	cfg := &config.Config{}
	store := message.NewStore()

	opts := VolleyOptions{
		MaxConcurrent: 2,
		MaxRetries:    0,
		ShowProgress:  false,
	}
	scheduler := NewScheduler(cfg, mockAg, store, opts)

	// Create 10 tasks
	tasks := make([]Task, 10)
	for i := range tasks {
		tasks[i] = Task{
			Index:  i + 1,
			Prompt: fmt.Sprintf("task %d", i+1),
		}
	}

	// Cancel after 200ms
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	// Execute
	results, summary, err := scheduler.Execute(ctx, tasks)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should complete fewer than 10 tasks
	completed := summary.SucceededTasks + summary.FailedTasks
	if completed >= 10 {
		t.Errorf("expected < 10 completed tasks after cancellation, got %d", completed)
	}

	// Verify we got partial results
	if len(results) != 10 {
		t.Errorf("got %d results, want 10 (with some incomplete)", len(results))
	}

	t.Logf("Cancelled after %d completed tasks", completed)
}

// TestSchedulerFailFast verifies fail-fast behavior
func TestSchedulerFailFast(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create mock agent that always fails
	mockAg := newMockAgent(10*time.Millisecond, 1.0)

	cfg := &config.Config{}
	store := message.NewStore()

	opts := VolleyOptions{
		MaxConcurrent: 2,
		MaxRetries:    0,
		ShowProgress:  false,
		FailFast:      true,
	}
	scheduler := NewScheduler(cfg, mockAg, store, opts)

	// Create 10 tasks
	tasks := make([]Task, 10)
	for i := range tasks {
		tasks[i] = Task{
			Index:  i + 1,
			Prompt: fmt.Sprintf("task %d", i+1),
		}
	}

	// Execute
	_, summary, err := scheduler.Execute(ctx, tasks)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should stop early with fail-fast
	completed := summary.SucceededTasks + summary.FailedTasks
	if completed >= 10 {
		t.Errorf("expected < 10 completed tasks with fail-fast, got %d", completed)
	}

	// At least one task should have failed
	if summary.FailedTasks == 0 {
		t.Error("expected at least one failed task")
	}

	t.Logf("Fail-fast stopped after %d completed tasks", completed)
}

// TestSchedulerRateLimitHandling verifies rate limit (429) error handling
func TestSchedulerRateLimitHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Create mock agent that returns 429 errors every 3rd call
	mockAg := newRateLimitedMockAgent(10*time.Millisecond, 3)

	cfg := &config.Config{}
	store := message.NewStore()

	opts := VolleyOptions{
		MaxConcurrent: 2,
		MaxRetries:    3,
		ShowProgress:  false,
	}
	scheduler := NewScheduler(cfg, mockAg, store, opts)

	// Create 10 tasks
	tasks := make([]Task, 10)
	for i := range tasks {
		tasks[i] = Task{
			Index:  i + 1,
			Prompt: fmt.Sprintf("task %d", i+1),
		}
	}

	// Execute
	results, summary, err := scheduler.Execute(ctx, tasks)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify we got results
	if len(results) != 10 {
		t.Errorf("got %d results, want 10", len(results))
	}

	// Should have some retries due to rate limiting
	if summary.TotalRetries == 0 {
		t.Error("expected some retries due to rate limit errors")
	}

	// Most tasks should eventually succeed with retries
	if summary.SucceededTasks < 7 {
		t.Errorf("expected at least 7 successes with retries, got %d", summary.SucceededTasks)
	}

	t.Logf("Rate limit test: %d succeeded, %d failed, %d retries (rate limited every 3rd call)",
		summary.SucceededTasks, summary.FailedTasks, summary.TotalRetries)
}

// rateLimitedMockAgent simulates rate limit errors
type rateLimitedMockAgent struct {
	delay         time.Duration
	failEveryN    int
	callCount     atomic.Int32
	successCount  atomic.Int32
}

func newRateLimitedMockAgent(delay time.Duration, failEveryN int) *rateLimitedMockAgent {
	return &rateLimitedMockAgent{
		delay:      delay,
		failEveryN: failEveryN,
	}
}

func (m *rateLimitedMockAgent) Model() catwalk.Model {
	return catwalk.Model{
		ID:            "mock-model",
		CostPer1MIn:   1.0,
		CostPer1MOut:  3.0,
	}
}

func (m *rateLimitedMockAgent) Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan agent.AgentEvent, error) {
	callNum := m.callCount.Add(1)
	events := make(chan agent.AgentEvent, 1)

	go func() {
		defer close(events)

		// Simulate work
		select {
		case <-time.After(m.delay):
			// Simulate 429 rate limit error every Nth call
			if callNum%int32(m.failEveryN) == 0 {
				events <- agent.AgentEvent{
					Type:  agent.AgentEventTypeError,
					Error: fmt.Errorf("rate limit exceeded (429): too many requests"),
				}
				return
			}

			// Success
			m.successCount.Add(1)
			events <- agent.AgentEvent{
				Type: agent.AgentEventTypeResponse,
				Message: message.Message{
					ID:        "test-msg",
					SessionID: sessionID,
					Role:      message.Assistant,
					Parts: []message.ContentPart{
						message.TextContent{Text: fmt.Sprintf("Response to: %s", content)},
					},
				},
			}
		case <-ctx.Done():
			events <- agent.AgentEvent{
				Type:  agent.AgentEventTypeError,
				Error: ctx.Err(),
			}
		}
	}()

	return events, nil
}

func (m *rateLimitedMockAgent) Cancel(sessionID string) {}
func (m *rateLimitedMockAgent) CancelAll() {}
func (m *rateLimitedMockAgent) IsSessionBusy(sessionID string) bool { return false }
func (m *rateLimitedMockAgent) IsBusy() bool { return false }
func (m *rateLimitedMockAgent) Summarize(ctx context.Context, sessionID string) error { return nil }
func (m *rateLimitedMockAgent) UpdateModel() error { return nil }
func (m *rateLimitedMockAgent) QueuedPrompts(sessionID string) int { return 0 }
func (m *rateLimitedMockAgent) ClearQueue(sessionID string) {}

// BenchmarkScheduler measures scheduler overhead
func BenchmarkScheduler(b *testing.B) {
	ctx := context.Background()

	// Create fast mock agent (1ms delay)
	mockAg := newMockAgent(1*time.Millisecond, 0.0)

	cfg := &config.Config{}

	opts := VolleyOptions{
		MaxConcurrent: 3,
		MaxRetries:    0,
		ShowProgress:  false,
	}

	// Create 10 tasks
	tasks := make([]Task, 10)
	for i := range tasks {
		tasks[i] = Task{
			Index:  i + 1,
			Prompt: fmt.Sprintf("task %d", i+1),
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		store := message.NewStore()
		scheduler := NewScheduler(cfg, mockAg, store, opts)
		_, _, err := scheduler.Execute(ctx, tasks)
		if err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
	}
}
