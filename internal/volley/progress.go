package volley

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/llm/tools"
	"github.com/bwl/cliffy/internal/output"
)

// taskState tracks the state and tool traces for a single task
type taskState struct {
	task      Task
	status    string // "running", "completed", "failed"
	icon      string // ▶, ✓, ✗
	result    *TaskResult
	tools     []*tools.ExecutionMetadata
	workerID  int
	lineCount int // Number of lines this task occupies (1 for task + N for tools)
}

// ProgressTracker tracks and displays volley execution progress
type ProgressTracker struct {
	enabled bool
	out     io.Writer

	mu         sync.Mutex
	totalTasks int
	maxWorkers int
	started    time.Time

	// Track state per task for in-place updates
	taskStates   map[int]*taskState // key = task.Index
	totalLines   int                // Total lines currently displayed
	taskOrder    []int              // Order of tasks as they appear on screen
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(enabled bool) *ProgressTracker {
	return &ProgressTracker{
		enabled:    enabled,
		out:        os.Stderr,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
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

	// Initialize task state
	p.taskStates[task.Index] = &taskState{
		task:      task,
		status:    "running",
		icon:      "▶",
		workerID:  workerID,
		tools:     []*tools.ExecutionMetadata{},
		lineCount: 1,
	}

	// Add to task order if not already there
	found := false
	for _, idx := range p.taskOrder {
		if idx == task.Index {
			found = true
			break
		}
	}
	if !found {
		p.taskOrder = append(p.taskOrder, task.Index)
	}

	// Render all tasks
	p.renderAll()
}

// TaskCompleted reports that a task completed successfully
func (p *ProgressTracker) TaskCompleted(task Task, result TaskResult) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.taskStates[task.Index]
	if state == nil {
		return
	}

	// Update state
	state.status = "completed"
	state.icon = "✓"
	state.result = &result

	// Re-render all tasks
	p.renderAll()
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

// renderAll renders all tasks and their tool traces, updating in place
func (p *ProgressTracker) renderAll() {
	// Build all lines for all tasks
	var allLines []string

	for _, taskIndex := range p.taskOrder {
		state := p.taskStates[taskIndex]
		if state == nil {
			continue
		}

		// Task status line
		taskLine := p.formatTaskLine(state)
		allLines = append(allLines, taskLine)

		// Tool trace lines with tree characters
		for i, toolMeta := range state.tools {
			var treeChar string
			if i == len(state.tools)-1 {
				treeChar = "╰────" // Last tool
			} else {
				treeChar = "├────" // Not last tool
			}

			toolLine := p.formatToolLine(treeChar, toolMeta)
			allLines = append(allLines, toolLine)
		}
	}

	newTotalLines := len(allLines)

	// If this is a re-render, move cursor up and clear
	if p.totalLines > 0 {
		// Move cursor up to the start
		for i := 0; i < p.totalLines; i++ {
			fmt.Fprintf(p.out, "\033[1A") // Move up one line
		}
		// Move to beginning of line
		fmt.Fprintf(p.out, "\r")
		// Clear from cursor to end of screen
		fmt.Fprintf(p.out, "\033[J")
	}

	// Print all lines
	for _, line := range allLines {
		fmt.Fprintln(p.out, line)
	}

	// Update total line count
	p.totalLines = newTotalLines
}

// formatTaskLine formats the main task status line
func (p *ProgressTracker) formatTaskLine(state *taskState) string {
	if state.status == "running" {
		return fmt.Sprintf("[%d/%d] %s %s (worker %d)",
			state.task.Index, p.totalTasks,
			state.icon,
			truncate(state.task.Prompt, 60),
			state.workerID)
	}

	// Completed or failed
	if state.result != nil {
		return fmt.Sprintf("[%d/%d] %s %s (%.1fs, %s tokens, $%.4f, %s)",
			state.task.Index, p.totalTasks,
			state.icon,
			truncate(state.task.Prompt, 60),
			state.result.Duration.Seconds(),
			formatTokens(state.result.TokensTotal),
			state.result.Cost,
			formatModel(state.result.Model))
	}

	return fmt.Sprintf("[%d/%d] %s %s",
		state.task.Index, p.totalTasks,
		state.icon,
		truncate(state.task.Prompt, 60))
}

// formatToolLine formats a single tool trace line
func (p *ProgressTracker) formatToolLine(treeChar string, metadata *tools.ExecutionMetadata) string {
	trace := output.FormatToolTrace(metadata, config.VerbosityNormal)
	// Remove the [TOOL] prefix since we're using tree characters
	trace = strings.TrimPrefix(trace, "[TOOL] ")
	return fmt.Sprintf("  %s %s", treeChar, trace)
}

// ToolExecuted displays a tool execution trace
func (p *ProgressTracker) ToolExecuted(task Task, metadata *tools.ExecutionMetadata) {
	if !p.enabled || metadata == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.taskStates[task.Index]
	if state == nil {
		return
	}

	// Add tool to state
	state.tools = append(state.tools, metadata)

	// Re-render all tasks
	p.renderAll()
}

// ShowProgress displays a progress update for a running task
func (p *ProgressTracker) ShowProgress(task Task, message string) {
	// Progress events are now handled by renderTask, so this is a no-op
	// The progress message was used for inline updates like "[1/3] ⋯ Running: cmd"
	// but with tree display, we don't need this anymore
}
