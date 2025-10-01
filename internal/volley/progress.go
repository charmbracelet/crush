package volley

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// ProgressTracker tracks and displays volley execution progress
type ProgressTracker struct {
	enabled bool
	out     io.Writer

	mu         sync.Mutex
	totalTasks int
	maxWorkers int
	started    time.Time
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(enabled bool) *ProgressTracker {
	return &ProgressTracker{
		enabled: enabled,
		out:     os.Stderr,
	}
}

// Start initializes the progress tracker
func (p *ProgressTracker) Start(totalTasks, maxWorkers int) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalTasks = totalTasks
	p.maxWorkers = maxWorkers
	p.started = time.Now()

	fmt.Fprintf(p.out, "Volley: %d tasks queued, max %d concurrent\n\n", totalTasks, maxWorkers)
}

// TaskStarted reports that a task has started
func (p *ProgressTracker) TaskStarted(task Task, workerID int) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Fprintf(p.out, "[%d/%d] ▶ %s (worker %d)\n",
		task.Index, p.totalTasks, truncate(task.Prompt, 60), workerID)
}

// TaskCompleted reports that a task completed successfully
func (p *ProgressTracker) TaskCompleted(task Task, result TaskResult) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Fprintf(p.out, "[%d/%d] ✓ %s (%.1fs, %s tokens, $%.4f, %s)\n",
		task.Index, p.totalTasks, truncate(task.Prompt, 60),
		result.Duration.Seconds(),
		formatTokens(result.TokensTotal),
		result.Cost,
		formatModel(result.Model))
}

// TaskFailed reports that a task failed
func (p *ProgressTracker) TaskFailed(task Task, err error, retries int) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if retries > 0 {
		fmt.Fprintf(p.out, "[%d/%d] ✗ %s failed after %d retries: %v\n",
			task.Index, p.totalTasks, truncate(task.Prompt, 60), retries, err)
	} else {
		fmt.Fprintf(p.out, "[%d/%d] ✗ %s failed: %v\n",
			task.Index, p.totalTasks, truncate(task.Prompt, 60), err)
	}
}

// TaskRetrying reports that a task is being retried
func (p *ProgressTracker) TaskRetrying(task Task, attempt int, delay time.Duration) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Fprintf(p.out, "[%d/%d] ⚠ %s retrying in %.1fs... (attempt %d)\n",
		task.Index, p.totalTasks, truncate(task.Prompt, 60), delay.Seconds(), attempt)
}

// TaskError reports an error during task execution
func (p *ProgressTracker) TaskError(task Task, attempt int, err error) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Fprintf(p.out, "[%d/%d] ✗ %s error (attempt %d): %v\n",
		task.Index, p.totalTasks, truncate(task.Prompt, 40), attempt, err)
}

// Finish displays the final summary
func (p *ProgressTracker) Finish(summary VolleySummary) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Fprintf(p.out, "\n")

	if summary.FailedTasks == 0 && summary.CanceledTasks == 0 {
		fmt.Fprintf(p.out, "Volley complete: %d/%d tasks succeeded in %.1fs\n",
			summary.SucceededTasks, summary.TotalTasks, summary.Duration.Seconds())
	} else {
		fmt.Fprintf(p.out, "Volley complete: %d/%d succeeded, %d failed",
			summary.SucceededTasks, summary.TotalTasks, summary.FailedTasks)

		if summary.CanceledTasks > 0 {
			fmt.Fprintf(p.out, ", %d canceled", summary.CanceledTasks)
		}

		fmt.Fprintf(p.out, " in %.1fs\n", summary.Duration.Seconds())
	}
}

// truncate shortens a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatTokens formats token count with k suffix for thousands
func formatTokens(tokens int64) string {
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000.0)
	}
	return fmt.Sprintf("%d", tokens)
}

// formatModel shortens model ID for display
func formatModel(modelID string) string {
	// For common model patterns, show a shorter version
	// e.g., "anthropic/claude-sonnet-4-20250514" -> "claude-sonnet-4"
	// e.g., "openai/gpt-4o" -> "gpt-4o"

	// Split by / to get the model name without provider prefix
	parts := splitString(modelID, '/')
	if len(parts) > 1 {
		modelName := parts[len(parts)-1]

		// Further shorten if it contains a date (YYYYMMDD pattern)
		// e.g., "claude-sonnet-4-20250514" -> "claude-sonnet-4"
		if len(modelName) > 8 {
			// Look for -YYYYMMDD pattern at the end
			if modelName[len(modelName)-9] == '-' && isDigits(modelName[len(modelName)-8:]) {
				return modelName[:len(modelName)-9]
			}
		}

		return modelName
	}

	return modelID
}

// splitString splits a string by a delimiter
func splitString(s string, delim rune) []string {
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

// isDigits checks if a string contains only digits
func isDigits(s string) bool {
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
