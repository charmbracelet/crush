package volley

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/llm/agent"
	"github.com/bwl/cliffy/internal/llm/tools"
	"github.com/bwl/cliffy/internal/message"
	outputpkg "github.com/bwl/cliffy/internal/output"
)

// Scheduler manages parallel execution of tasks with resilience features
//
// Resilience Features:
//   - Smart retry with exponential backoff and jitter to prevent thundering herd
//   - Per-error retry policies (rate limits get longer backoff, network errors retry quickly)
//   - Clean queue draining on fail-fast cancellation
//   - Adaptive concurrency tracking with health metrics
//   - Context-aware cancellation during retry delays
//
// The scheduler maintains success/failure counts for adaptive behavior and tracks
// actual concurrent workers for observability. These metrics enable future
// enhancements like dynamic concurrency adjustment based on error rates.
type Scheduler struct {
	config       *config.Config
	agent        agent.Service
	messageStore *message.Store
	options      VolleyOptions
	progress     *ProgressTracker

	// Concurrency control and health tracking
	mu                sync.Mutex
	currentConcurrent int // Current number of active workers
	successCount      int // Consecutive successful tasks (reset on failure)
	failureCount      int // Consecutive failed tasks (reset on success)

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

	// Set model name on progress tracker
	modelName := formatModel(s.agent.Model().ID)
	s.progress.SetModel(modelName)

	// Start progress tracker
	s.progress.Start(len(tasks), s.options.MaxConcurrent)

	// Initialize all tasks as queued
	s.progress.InitializeTasks(tasks)

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
			// Context canceled - drain remaining tasks as canceled
			s.drainQueue(taskQueue, workerID)
			return
		case task, ok := <-taskQueue:
			if !ok {
				return
			}

			// Track concurrent worker count
			s.mu.Lock()
			s.currentConcurrent++
			s.mu.Unlock()

			result := s.executeTask(ctx, workerID, task)

			// Update concurrent count
			s.mu.Lock()
			s.currentConcurrent--
			s.mu.Unlock()

			s.resultsChan <- result
		}
	}
}

// drainQueue marks all remaining tasks as canceled when context is done
//
// This is called during fail-fast cancellation or context cancellation to ensure
// all queued tasks are properly marked as canceled rather than left in pending state.
// Each worker drains tasks from the queue until it's empty or closed.
//
// Without this, tasks would remain in the queue indefinitely when fail-fast triggers,
// creating incomplete results and confusing output.
func (s *Scheduler) drainQueue(taskQueue <-chan Task, workerID int) {
	for task := range taskQueue {
		result := TaskResult{
			Task:     task,
			Status:   TaskStatusCanceled,
			Error:    fmt.Errorf("task canceled due to fail-fast or context cancellation"),
			WorkerID: workerID,
		}
		s.resultsChan <- result
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
	var lastOutput string
	for attempt := 0; attempt <= s.options.MaxRetries; attempt++ {
		if attempt > 0 {
			result.Retries = attempt
			delay := retryDelayForError(lastErr, attempt)
			s.progress.TaskRetrying(task, attempt, delay)

			// Sleep with context awareness for clean cancellation
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				result.Status = TaskStatusCanceled
				result.Error = ctx.Err()
				result.Duration = time.Since(startTime)
				return result
			}
		}

		// Build prompt with optional context
		prompt := task.Prompt
		if s.options.Context != "" {
			prompt = s.options.Context + "\n\n" + prompt
		}

		// On retry, append error feedback to help LLM self-correct
		// Only feed back non-network errors (e.g., tool failures, validation errors)
		if attempt > 0 && lastErr != nil && !isNetworkError(lastErr) {
			errorFeedback := fmt.Sprintf("\n\n[Previous attempt failed with error: %v", lastErr)
			if lastOutput != "" {
				errorFeedback += fmt.Sprintf("\nPartial output: %s", truncateOutput(lastOutput, 200))
			}
			errorFeedback += "\nPlease try a different approach.]"
			prompt = prompt + errorFeedback
		}

		// Execute via agent
		output, usage, toolMetadata, err := s.executeViaAgent(ctx, prompt, task)
		lastOutput = output // Save for potential error feedback

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
			// Emit NDJSON tool trace if requested
			if s.options.EmitToolTrace {
				_ = outputpkg.EmitToolTraceNDJSON(os.Stderr, task.Index, event.ToolMetadata)
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

			// Extract output and thinking from assistant messages
			for _, msg := range messages {
				if msg.Role == message.Assistant {
					// Show thinking if requested
					if s.options.ShowThinking {
						reasoning := msg.ReasoningContent()
						if reasoning.Thinking != "" {
							s.progress.ShowThinking(task, reasoning.Thinking, s.options.ThinkingFormat)
						}
					}

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

// getHealthMetrics returns current success/failure metrics for adaptive behavior
//
// These metrics track the scheduler's health and can be used for:
//   - Adaptive concurrency scaling (reduce workers during high failure rates)
//   - Circuit breaking (stop sending requests if provider is down)
//   - Telemetry and monitoring
//
// Success/failure counts track consecutive results and reset on state change,
// enabling detection of temporary provider issues vs. permanent errors.
func (s *Scheduler) getHealthMetrics() (successCount, failureCount, currentConcurrent int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.successCount, s.failureCount, s.currentConcurrent
}

// shouldBackoff determines if the scheduler should slow down based on failure rate
//
// Returns true if consecutive failures exceed threshold (3+) with no recent successes.
// This can be used by future adaptive concurrency implementations to:
//   - Reduce concurrent workers when provider is struggling
//   - Implement circuit breaker pattern
//   - Add extra delays before starting new tasks
//
// Currently used for observability; actual backoff logic can be added as needed.
func (s *Scheduler) shouldBackoff() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we have 3+ consecutive failures and no recent successes, back off
	// This can be used by future adaptive concurrency implementations
	return s.failureCount >= 3 && s.successCount == 0
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

// ErrorClass represents the category of error for retry strategy
//
// Different error types require different retry strategies:
//   - Rate limits need long backoff to avoid further throttling
//   - Network errors can retry quickly since they're usually transient
//   - Timeouts need moderate backoff as the service may be degraded
//   - Auth/validation errors are fatal and should not be retried
type ErrorClass int

const (
	ErrorClassUnknown    ErrorClass = iota // Unknown errors use standard backoff
	ErrorClassRateLimit                    // Rate limit (429) - needs longest backoff
	ErrorClassTimeout                      // Timeout errors - moderate backoff
	ErrorClassNetwork                      // Network errors - quick retry
	ErrorClassAuth                         // Auth errors (401/403) - fatal, no retry
	ErrorClassValidation                   // Validation errors (400) - fatal, no retry
)

// classifyError determines the error category for adaptive retry
//
// This function examines error messages to determine the appropriate retry strategy.
// It uses simple string matching since errors come from various providers with
// different formats. More sophisticated error classification could be added in future.
func classifyError(err error) ErrorClass {
	if err == nil {
		return ErrorClassUnknown
	}

	errStr := err.Error()

	// Rate limit errors need longer backoff
	if contains(errStr, "429") || contains(errStr, "rate limit") {
		return ErrorClassRateLimit
	}

	// Auth errors are fatal
	if contains(errStr, "401") || contains(errStr, "403") || contains(errStr, "unauthorized") {
		return ErrorClassAuth
	}

	// Validation errors are fatal
	if contains(errStr, "400") || contains(errStr, "invalid") {
		return ErrorClassValidation
	}

	// Timeout errors need moderate backoff
	if contains(errStr, "timeout") || contains(errStr, "context deadline") {
		return ErrorClassTimeout
	}

	// Network errors need quick retry
	if contains(errStr, "connection") || contains(errStr, "network") {
		return ErrorClassNetwork
	}

	return ErrorClassUnknown
}

// retryDelayForError calculates backoff based on error type and attempt
//
// Retry Strategy by Error Class:
//   - Rate Limit: 5s → 10s → 20s → 40s ... (max 120s)
//   - Timeout:    2s → 4s → 8s → 16s ... (max 60s)
//   - Network:    500ms → 1s → 2s → 4s ... (max 30s)
//   - Unknown:    1s → 2s → 4s → 8s ... (max 60s)
//
// All delays include ±25% jitter to prevent thundering herd when multiple
// workers retry simultaneously (e.g., after a provider outage).
//
// Examples:
//   - Rate limit on 1st attempt: ~5s (3.75s to 6.25s with jitter)
//   - Network error on 1st attempt: ~500ms (375ms to 625ms with jitter)
//   - Timeout on 2nd attempt: ~4s (3s to 5s with jitter)
func retryDelayForError(err error, attempt int) time.Duration {
	class := classifyError(err)

	var baseDelay time.Duration
	var maxDelay time.Duration

	switch class {
	case ErrorClassRateLimit:
		// Rate limits need longer backoff: 5s, 10s, 20s, 40s...
		baseDelay = 5 * time.Second
		maxDelay = 120 * time.Second
	case ErrorClassTimeout:
		// Timeouts get moderate backoff: 2s, 4s, 8s, 16s...
		baseDelay = 2 * time.Second
		maxDelay = 60 * time.Second
	case ErrorClassNetwork:
		// Network errors retry quickly: 500ms, 1s, 2s, 4s...
		baseDelay = 500 * time.Millisecond
		maxDelay = 30 * time.Second
	default:
		// Unknown errors use standard backoff: 1s, 2s, 4s, 8s...
		baseDelay = 1 * time.Second
		maxDelay = 60 * time.Second
	}

	// Exponential backoff
	delay := baseDelay * (1 << attempt)
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter (±25% of delay) to avoid thundering herd
	// This spreads out retries to prevent all workers hitting the API simultaneously
	jitterRange := int64(float64(delay) * 0.25)
	if jitterRange > 0 {
		jitter := time.Duration(rand.Int63n(jitterRange*2) - jitterRange)
		delay += jitter
		// Ensure delay stays positive
		if delay < baseDelay/2 {
			delay = baseDelay
		}
	}

	return delay
}

// retryDelay calculates exponential backoff with jitter (legacy, kept for compatibility)
func retryDelay(attempt int) time.Duration {
	return retryDelayForError(nil, attempt)
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

// isNetworkError determines if an error is network-related (not worth feeding back to LLM)
func isNetworkError(err error) bool {
	errStr := err.Error()

	// Network/infrastructure errors that LLM can't fix
	return contains(errStr, "429") ||
		contains(errStr, "rate limit") ||
		contains(errStr, "timeout") ||
		contains(errStr, "context deadline") ||
		contains(errStr, "connection") ||
		contains(errStr, "network") ||
		contains(errStr, "401") ||
		contains(errStr, "403") ||
		contains(errStr, "unauthorized")
}

// truncateOutput truncates output for error feedback
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "..."
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

// formatModel shortens model ID for display
func formatModel(modelID string) string {
	// For common model patterns, show a shorter version
	// e.g., "anthropic/claude-sonnet-4-20250514" -> "claude-sonnet-4"
	// e.g., "openai/gpt-4o" -> "gpt-4o"
	// e.g., "x-ai/grok-4-fast:free" -> "grok-4-fast:free"

	// Split by / to get the model name without provider prefix
	parts := splitByChar(modelID, '/')
	if len(parts) > 1 {
		modelName := parts[len(parts)-1]

		// Further shorten if it contains a date (YYYYMMDD pattern)
		// e.g., "claude-sonnet-4-20250514" -> "claude-sonnet-4"
		if len(modelName) > 8 {
			// Look for -YYYYMMDD pattern at the end
			if modelName[len(modelName)-9] == '-' && isAllDigits(modelName[len(modelName)-8:]) {
				return modelName[:len(modelName)-9]
			}
		}

		return modelName
	}

	return modelID
}

// splitByChar splits a string by a delimiter character
func splitByChar(s string, delim rune) []string {
	var parts []string
	var current string

	for _, ch := range s {
		if ch == delim {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// isAllDigits checks if a string contains only digits
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
