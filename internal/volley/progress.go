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
	"golang.org/x/term"
)

// taskState tracks the state and tool traces for a single task
type taskState struct {
	task         Task
	status       string // "running", "completed", "failed"
	result       *TaskResult
	tools        []*tools.ExecutionMetadata
	workerID     int
	lineCount    int  // Number of lines this task occupies (1 for task + N for tools)
	spinnerFrame int  // Current spinner animation frame (0-3)
	collapsed    bool // Whether completed task tools are hidden

	// Error tracking for collapsing repeated errors
	lastError      *ProviderError
	errorCount     int
	lastErrorClass ErrorClass
}

// ProgressTracker tracks and displays volley execution progress
type ProgressTracker struct {
	enabled bool
	out     io.Writer

	mu         sync.Mutex
	totalTasks int
	maxWorkers int
	started    time.Time
	modelName  string // Model name for header

	// Track state per task for in-place updates
	taskStates   map[int]*taskState // key = task.Index
	totalLines   int                // Total lines currently displayed
	taskOrder    []int              // Order of tasks as they appear on screen
	spinnerTick  chan struct{}      // Ticker for spinner animation
	stopSpinner  chan struct{}      // Stop signal for spinner
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(enabled bool) *ProgressTracker {
	// Disable progress if not a TTY (e.g., output is redirected or piped)
	if enabled && !term.IsTerminal(int(os.Stderr.Fd())) {
		enabled = false
	}

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

	// Print tennis racket header
	fmt.Fprintf(p.out, "%s═╕   %d tasks volleyed\n", tools.AsciiTennisRacketHead, totalTasks)
	fmt.Fprintf(p.out, "  ╰-╮ Using %s\n", p.modelName)

	// Start spinner animation
	p.spinnerTick = make(chan struct{})
	p.stopSpinner = make(chan struct{})
	go p.spinnerLoop()
}

// InitializeTasks sets up all tasks as queued at the start
func (p *ProgressTracker) InitializeTasks(tasks []Task) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Initialize all tasks as queued in submission order
	for _, task := range tasks {
		p.taskStates[task.Index] = &taskState{
			task:         task,
			status:       "queued",
			workerID:     0,
			tools:        []*tools.ExecutionMetadata{},
			lineCount:    1,
			spinnerFrame: 0,
		}
		p.taskOrder = append(p.taskOrder, task.Index)
	}

	// Initial render showing all queued tasks
	p.renderAll()
}

// TaskStarted reports that a task has started
func (p *ProgressTracker) TaskStarted(task Task, workerID int) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.taskStates[task.Index]
	if state == nil {
		// Fallback: create state if not initialized
		state = &taskState{
			task:         task,
			status:       "running",
			workerID:     workerID,
			tools:        []*tools.ExecutionMetadata{},
			lineCount:    1,
			spinnerFrame: 0,
		}
		p.taskStates[task.Index] = state
		p.taskOrder = append(p.taskOrder, task.Index)
	} else {
		// Update existing queued task to running
		state.status = "running"
		state.workerID = workerID
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
	state.result = &result
	// Auto-collapse completed tasks to save screen space
	state.collapsed = true

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

// TaskError reports an error during task execution (deprecated - use TaskProviderError)
func (p *ProgressTracker) TaskError(task Task, attempt int, err error) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Fprintf(p.out, "[%d/%d] ✗ %s error (attempt %d): %v\n",
		task.Index, p.totalTasks, truncate(task.Prompt, 40), attempt, err)
}

// TaskProviderError reports a structured provider error with error collapsing
func (p *ProgressTracker) TaskProviderError(providerErr ProviderError) {
	if !p.enabled {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.taskStates[providerErr.Task.Index]
	if state == nil {
		return
	}

	// Check if this is the same error as last time
	isSameError := false
	if state.lastError != nil &&
		state.lastErrorClass == providerErr.ErrorClass &&
		state.lastError.Message == providerErr.Message {
		isSameError = true
		state.errorCount++
	} else {
		// New error type, reset counter
		state.errorCount = 1
		state.lastErrorClass = providerErr.ErrorClass
	}

	// Store the error
	state.lastError = &providerErr

	// Update task status
	if providerErr.IsRetrying {
		state.status = "retrying"
	} else {
		state.status = "failed"
	}

	// Don't spam the display with identical errors
	// Only show the first occurrence and updates every 3rd repeat
	if isSameError && state.errorCount > 1 && state.errorCount%3 != 0 {
		return
	}

	// Re-render to show updated status
	p.renderAll()
}

// Finish displays the final summary
func (p *ProgressTracker) Finish(summary VolleySummary) {
	if !p.enabled {
		return
	}

	// Stop spinner animation
	if p.stopSpinner != nil {
		close(p.stopSpinner)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Fprintf(p.out, "\n")

	if summary.FailedTasks == 0 && summary.CanceledTasks == 0 {
		fmt.Fprintf(p.out, "%s %d/%d tasks succeeded in %.1fs\n",
			tools.AsciiTennisRacketHead,
			summary.SucceededTasks, summary.TotalTasks, summary.Duration.Seconds())
	} else {
		fmt.Fprintf(p.out, "%s %d/%d succeeded, %d failed",
			tools.AsciiTennisRacketHead,
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


// renderAll renders all tasks and their tool traces, updating in place
func (p *ProgressTracker) renderAll() {
	// Build all lines for all tasks
	var allLines []string

	for displayNum, taskIndex := range p.taskOrder {
		state := p.taskStates[taskIndex]
		if state == nil {
			continue
		}

		// Task status line (displayNum is 0-indexed, so add 1)
		taskLine := p.formatTaskLine(displayNum+1, state)
		allLines = append(allLines, taskLine)

		// Tool trace lines with tree characters (skip if collapsed)
		if !state.collapsed {
			// Only show the most recent 4 tool calls
			visibleTools := state.tools
			startIdx := 0
			if len(state.tools) > 4 {
				startIdx = len(state.tools) - 4
				visibleTools = state.tools[startIdx:]
			}

			for i, toolMeta := range visibleTools {
				var treeChar string
				if i == len(visibleTools)-1 {
					treeChar = tools.AsciiTreeLast // ╰
				} else {
					treeChar = tools.AsciiTreeMid // ├
				}

				toolLine := p.formatToolLine(treeChar, toolMeta)
				allLines = append(allLines, toolLine)
			}
		}
	}

	newTotalLines := len(allLines)

	// If this is a re-render, move cursor up and clear
	if p.totalLines > 0 {
		// Always move up by the actual number of lines we previously printed
		// This handles the case where visible lines decrease (e.g., tools scrolling off)
		linesToClear := p.totalLines

		// Move cursor up to the start
		for i := 0; i < linesToClear; i++ {
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

	// Update total line count to reflect what we actually printed
	p.totalLines = newTotalLines
}

// formatTaskLine formats the main task status line
func (p *ProgressTracker) formatTaskLine(displayNum int, state *taskState) string {
	// Get icon based on status
	var icon string
	switch state.status {
	case "running":
		// Use spinner frames
		spinnerFrames := []string{
			tools.AsciiTaskSpinner0,
			tools.AsciiTaskSpinner1,
			tools.AsciiTaskSpinner2,
			tools.AsciiTaskSpinner3,
		}
		icon = spinnerFrames[state.spinnerFrame%4]
	case "completed":
		icon = tools.AsciiTaskComplete
	case "failed":
		icon = "✗" // Failed icon
	case "retrying":
		icon = "⚠" // Warning icon for retrying
	default:
		icon = tools.AsciiTaskQueued
	}

	// Format task description
	taskDesc := truncate(state.task.Prompt, 60)

	// Determine if task has tools (will show branching)
	hasBranch := len(state.tools) > 0

	// Show error information for retrying/failed tasks
	if state.status == "retrying" && state.lastError != nil {
		errorSuffix := state.lastError.Message
		if state.errorCount > 1 {
			errorSuffix = fmt.Sprintf("%s (×%d)", errorSuffix, state.errorCount)
		}
		if state.lastError.IsRetrying {
			errorSuffix = fmt.Sprintf("%s - retrying", errorSuffix)
		}
		return fmt.Sprintf("%d   %s %s - %s",
			displayNum, icon, taskDesc, errorSuffix)
	}

	if state.status == "failed" && state.lastError != nil {
		errorSuffix := state.lastError.Message
		if state.errorCount > 1 {
			errorSuffix = fmt.Sprintf("%s (×%d)", errorSuffix, state.errorCount)
		}
		return fmt.Sprintf("%d   %s %s - %s",
			displayNum, icon, taskDesc, errorSuffix)
	}

	if state.status == "running" {
		// Running task: show worker label
		if hasBranch {
			return fmt.Sprintf("%d %s %s %s (worker %d)",
				displayNum, tools.AsciiTreeBranch, icon, taskDesc, state.workerID)
		}
		return fmt.Sprintf("%d   %s %s (worker %d)",
			displayNum, icon, taskDesc, state.workerID)
	}

	// Completed task
	if state.result != nil {
		if hasBranch {
			// If collapsed, show summary with tool count
			if state.collapsed {
				toolSummary := p.formatToolSummary(state.tools)
				return fmt.Sprintf("%d %s %s %s %s  %s tokens $%.4f  %.1fs",
					displayNum, tools.AsciiTreeBranch, icon, taskDesc, toolSummary,
					formatTokens(state.result.TokensTotal),
					state.result.Cost,
					state.result.Duration.Seconds())
			}
			// Tasks with tools show metrics on same line
			return fmt.Sprintf("%d %s %s %s %s tokens $%.4f  %.1fs",
				displayNum, tools.AsciiTreeBranch, icon, taskDesc,
				formatTokens(state.result.TokensTotal),
				state.result.Cost,
				state.result.Duration.Seconds())
		}
		// Tasks without tools show metrics in parens
		return fmt.Sprintf("%d   %s %s (%.1fs, %s tokens)",
			displayNum, icon, taskDesc,
			state.result.Duration.Seconds(),
			formatTokens(state.result.TokensTotal))
	}

	// Fallback for queued or unknown
	return fmt.Sprintf("%d   %s %s",
		displayNum, icon, taskDesc)
}

// formatToolLine formats a single tool trace line
func (p *ProgressTracker) formatToolLine(treeChar string, metadata *tools.ExecutionMetadata) string {
	// Determine tool icon based on exit code (for bash) or success
	toolIcon := tools.AsciiToolSuccess // Default: success
	if metadata.ExitCode != nil && *metadata.ExitCode != 0 {
		toolIcon = tools.AsciiToolFailed
	}

	// Get formatted trace without [TOOL] prefix
	trace := output.FormatToolTrace(metadata, config.VerbosityNormal)
	trace = strings.TrimPrefix(trace, "[TOOL] ")

	// Format: "  ├───▣ bash   cd crates && cargo new 0.5s"
	return fmt.Sprintf("  %s%s%s %s",
		treeChar, tools.AsciiTreeLine, toolIcon, trace)
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

// ShowThinking displays thinking/reasoning content from the LLM
func (p *ProgressTracker) ShowThinking(task Task, thinking string, format string) {
	if !p.enabled {
		// When progress is disabled, output to stderr for verbosity
		if format == "json" {
			fmt.Fprintf(p.out, `{"type":"thinking","task":%d,"content":%q}`+"\n", task.Index, thinking)
		} else {
			fmt.Fprintf(p.out, "\n[THINKING - Task %d]\n%s\n[/THINKING]\n\n", task.Index, thinking)
		}
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// When progress is enabled, print thinking separately (not in the progress display)
	// This is because thinking can be quite long and would interfere with the live updates

	// Temporarily clear the current display
	if p.totalLines > 0 {
		for i := 0; i < p.totalLines; i++ {
			fmt.Fprintf(p.out, "\033[1A") // Move up one line
		}
		fmt.Fprintf(p.out, "\r")
		fmt.Fprintf(p.out, "\033[J") // Clear from cursor to end of screen
	}

	// Print thinking
	if format == "json" {
		fmt.Fprintf(p.out, `{"type":"thinking","task":%d,"content":%q}`+"\n", task.Index, thinking)
	} else {
		fmt.Fprintf(p.out, "\n[THINKING - Task %d]\n%s\n[/THINKING]\n\n", task.Index, thinking)
	}

	// Re-render the progress display
	p.renderAll()
}

// spinnerLoop runs the spinner animation in a goroutine
func (p *ProgressTracker) spinnerLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopSpinner:
			return
		case <-ticker.C:
			p.mu.Lock()
			// Update spinner frame for all running tasks
			for _, state := range p.taskStates {
				if state.status == "running" {
					state.spinnerFrame = (state.spinnerFrame + 1) % 4
				}
			}
			// Re-render
			p.renderAll()
			p.mu.Unlock()
		}
	}
}

// SetModel sets the model name for display in the header
func (p *ProgressTracker) SetModel(modelName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.modelName = modelName
}

// formatToolSummary creates a compact summary of tools executed
func (p *ProgressTracker) formatToolSummary(toolMetas []*tools.ExecutionMetadata) string {
	if len(toolMetas) == 0 {
		return ""
	}

	// Count tools by name (preserve order)
	type toolCount struct {
		name  string
		count int
	}
	seen := make(map[string]int)
	var order []string

	for _, meta := range toolMetas {
		if _, exists := seen[meta.ToolName]; !exists {
			order = append(order, meta.ToolName)
		}
		seen[meta.ToolName]++
	}

	// Build summary string with counts
	var parts []string
	for _, name := range order {
		count := seen[name]
		if count > 1 {
			parts = append(parts, fmt.Sprintf("%d×%s", count, name))
		} else {
			parts = append(parts, name)
		}
	}

	// Show first 3 tools, then indicate more
	if len(parts) > 3 {
		parts = append(parts[:3], fmt.Sprintf("+%d more", len(parts)-3))
	}

	return fmt.Sprintf("[%s]", strings.Join(parts, " "))
}
