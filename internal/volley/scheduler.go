package volley

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/llm/agent"
	"github.com/bwl/cliffy/internal/llm/tools"
	"github.com/bwl/cliffy/internal/message"
)

// Scheduler manages parallel execution of tasks with rate limiting
type Scheduler struct {
	config       *config.Config
	agent        agent.Service
	messageStore *message.Store
	options      VolleyOptions
	progress     *ProgressTracker

	// Concurrency control
	mu                sync.Mutex
	currentConcurrent int
	successCount      int
	failureCount      int

	// Results tracking
	results     []TaskResult
	resultsChan chan TaskResult
}

// NewScheduler creates a new scheduler
func NewScheduler(cfg *config.Config, ag agent.Service, store *message.Store, opts VolleyOptions) *Scheduler {
	return &Scheduler{
		config:       cfg,
		agent:        ag,
		messageStore: store,
		options:      opts,
		progress:     NewProgressTracker(opts.ShowProgress),
		results:      make([]TaskResult, 0),
		resultsChan:  make(chan TaskResult, opts.MaxConcurrent*2),
	}
}

// Execute runs all tasks in the volley with smart concurrency management
func (s *Scheduler) Execute(ctx context.Context, tasks []Task) ([]TaskResult, VolleySummary, error) {
	if len(tasks) == 0 {
		return nil, VolleySummary{}, fmt.Errorf("no tasks to execute")
	}

	startTime := time.Now()

	// Initialize results slice
	s.results = make([]TaskResult, len(tasks))
	for i := range s.results {
		s.results[i] = TaskResult{
			Task:   tasks[i],
			Status: TaskStatusPending,
		}
	}

	// Start progress tracker
	s.progress.Start(len(tasks), s.options.MaxConcurrent)

	// Create task queue
	taskQueue := make(chan Task, len(tasks))
	for _, task := range tasks {
		taskQueue <- task
	}
	close(taskQueue)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < s.options.MaxConcurrent; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			s.worker(ctx, workerID, taskQueue)
		}(i + 1)
	}

	// Collect results in a goroutine
	go func() {
		wg.Wait()
		close(s.resultsChan)
	}()

	// Process results as they come in
	for result := range s.resultsChan {
		s.handleResult(ctx, result, cancel)
	}

	// Calculate summary
	summary := s.calculateSummary(time.Since(startTime))

	// Only show summary if enabled
	if s.options.ShowSummary {
		s.progress.Finish(summary)
	}

	return s.results, summary, nil
}

// worker processes tasks from the queue
func (s *Scheduler) worker(ctx context.Context, workerID int, taskQueue <-chan Task) {
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-taskQueue:
			if !ok {
				return
			}

			result := s.executeTask(ctx, workerID, task)
			s.resultsChan <- result
		}
	}
}

// executeTask runs a single task with retries
func (s *Scheduler) executeTask(ctx context.Context, workerID int, task Task) TaskResult {
	result := TaskResult{
		Task:     task,
		Status:   TaskStatusRunning,
		WorkerID: workerID,
	}

	s.progress.TaskStarted(task, workerID)

	startTime := time.Now()

	// Try executing with retries
	var lastErr error
	for attempt := 0; attempt <= s.options.MaxRetries; attempt++ {
		if attempt > 0 {
			result.Retries = attempt
			delay := retryDelay(attempt)
			s.progress.TaskRetrying(task, attempt, delay)
			time.Sleep(delay)
		}

		// Build prompt with optional context
		prompt := task.Prompt
		if s.options.Context != "" {
			prompt = s.options.Context + "\n\n" + prompt
		}

		// Execute via agent
		output, usage, toolMetadata, err := s.executeViaAgent(ctx, prompt, task)

		if err != nil {
			lastErr = err

			// Always log errors for debugging
			s.progress.TaskError(task, attempt, err)

			// Check if we should retry
			if shouldRetry(err, attempt, s.options.MaxRetries) {
				s.mu.Lock()
				s.failureCount++
				s.mu.Unlock()
				continue
			}

			// Fatal error or max retries exceeded
			break
		}

		// Success
		s.mu.Lock()
		s.successCount++
		s.failureCount = 0 // Reset failure count on success
		s.mu.Unlock()

		result.Status = TaskStatusSuccess
		result.Output = output
		result.TokensInput = usage.InputTokens
		result.TokensOutput = usage.OutputTokens
		result.TokensTotal = usage.TotalTokens
		result.Cost = s.calculateCost(usage)
		result.Duration = time.Since(startTime)
		result.Error = nil
		result.Model = s.agent.Model().ID
		result.ToolMetadata = toolMetadata

		s.progress.TaskCompleted(task, result)

		return result
	}

	// Failed after all retries
	result.Status = TaskStatusFailed
	result.Error = lastErr
	result.Duration = time.Since(startTime)

	s.progress.TaskFailed(task, lastErr, result.Retries)

	return result
}

// executeViaAgent runs the task through the agent
func (s *Scheduler) executeViaAgent(ctx context.Context, prompt string, task Task) (string, Usage, []*tools.ExecutionMetadata, error) {
	// Generate unique session ID for this task
	sessionID := fmt.Sprintf("volley-%d", time.Now().UnixNano())

	// Run agent
	events, err := s.agent.Run(ctx, sessionID, prompt)
	if err != nil {
		return "", Usage{}, nil, fmt.Errorf("failed to run agent: %w", err)
	}

	if events == nil {
		return "", Usage{}, nil, fmt.Errorf("request was queued unexpectedly")
	}

	// Process events
	var output string
	var usage Usage
	var toolMetadata []*tools.ExecutionMetadata

	for event := range events {
		switch event.Type {
		case agent.AgentEventTypeToolTrace:
			// Real-time tool trace display
			if s.options.Verbosity != config.VerbosityQuiet {
				s.progress.ToolExecuted(task, event.ToolMetadata)
			}
			// Collect metadata for result
			toolMetadata = append(toolMetadata, event.ToolMetadata)

		case agent.AgentEventTypeProgress:
			// Progress updates (for future Phase 4)
			if s.options.Verbosity != config.VerbosityQuiet {
				s.progress.ShowProgress(task, event.Progress)
			}

		case agent.AgentEventTypeError:
			return "", Usage{}, toolMetadata, event.Error

		case agent.AgentEventTypeResponse:
			// Get final message
			messages, err := s.messageStore.List(ctx, sessionID)
			if err != nil {
				return "", Usage{}, toolMetadata, fmt.Errorf("failed to list messages: %w", err)
			}

			// Extract output from assistant messages
			for _, msg := range messages {
				if msg.Role == message.Assistant {
					output += msg.Content().Text
				}
			}

			// Extract token usage from event
			usage = Usage{
				InputTokens:  event.TokenUsage.InputTokens,
				OutputTokens: event.TokenUsage.OutputTokens,
				TotalTokens:  event.TokenUsage.InputTokens + event.TokenUsage.OutputTokens,
			}

			return output, usage, toolMetadata, nil
		}
	}

	return output, usage, toolMetadata, nil
}

// handleResult processes a completed task result
func (s *Scheduler) handleResult(ctx context.Context, result TaskResult, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store result
	s.results[result.Task.Index-1] = result

	// Check fail-fast
	if s.options.FailFast && result.Status == TaskStatusFailed {
		cancel()
	}
}

// calculateSummary generates aggregate statistics
func (s *Scheduler) calculateSummary(duration time.Duration) VolleySummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	summary := VolleySummary{
		TotalTasks: len(s.results),
		Duration:   duration,
	}

	var totalRetries int
	var totalTokens int64
	var totalCost float64

	for _, result := range s.results {
		switch result.Status {
		case TaskStatusSuccess:
			summary.SucceededTasks++
		case TaskStatusFailed:
			summary.FailedTasks++
		case TaskStatusCanceled:
			summary.CanceledTasks++
		}

		totalRetries += result.Retries
		totalTokens += result.TokensTotal
		totalCost += result.Cost
	}

	summary.TotalRetries = totalRetries
	summary.TotalTokens = totalTokens
	summary.TotalCost = totalCost

	if summary.SucceededTasks > 0 {
		summary.AvgTokensPerTask = totalTokens / int64(summary.SucceededTasks)
	}

	summary.MaxConcurrentUsed = s.options.MaxConcurrent

	return summary
}

// calculateCost estimates cost based on token usage and model pricing
func (s *Scheduler) calculateCost(usage Usage) float64 {
	model := s.agent.Model()

	inputCost := model.CostPer1MIn / 1e6 * float64(usage.InputTokens)
	outputCost := model.CostPer1MOut / 1e6 * float64(usage.OutputTokens)

	return inputCost + outputCost
}

// retryDelay calculates exponential backoff with jitter
func retryDelay(attempt int) time.Duration {
	base := 1 * time.Second
	maxDelay := 60 * time.Second

	// Exponential: 1s, 2s, 4s, 8s, 16s, 32s, 60s (capped)
	delay := base * (1 << attempt)
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter (0-1000ms) to avoid thundering herd
	// jitter := time.Duration(rand.Intn(1000)) * time.Millisecond

	return delay
}

// shouldRetry determines if a task should be retried based on the error
func shouldRetry(err error, attempt, maxRetries int) bool {
	if attempt >= maxRetries {
		return false
	}

	// Check for retryable errors
	errStr := err.Error()

	// Rate limit errors (429)
	if contains(errStr, "429") || contains(errStr, "rate limit") {
		return true
	}

	// Timeout errors
	if contains(errStr, "timeout") || contains(errStr, "context deadline") {
		return true
	}

	// Network errors
	if contains(errStr, "connection") || contains(errStr, "network") {
		return true
	}

	// Don't retry auth errors
	if contains(errStr, "401") || contains(errStr, "403") || contains(errStr, "unauthorized") {
		return false
	}

	// Don't retry bad request errors
	if contains(errStr, "400") || contains(errStr, "invalid") {
		return false
	}

	// Default: retry for unknown errors
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
