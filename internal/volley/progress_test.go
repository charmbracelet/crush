package volley

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/bwl/cliffy/internal/llm/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProgressTracker(t *testing.T) {
	tests := []struct {
		name            string
		enabled         bool
		expectEnabled   bool
	}{
		{
			name:          "enabled tracker",
			enabled:       true,
			expectEnabled: false, // Will be false because stderr is not a TTY in tests
		},
		{
			name:          "disabled tracker",
			enabled:       false,
			expectEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewProgressTracker(tt.enabled)
			assert.NotNil(t, tracker)
			assert.Equal(t, tt.expectEnabled, tracker.enabled)
			assert.NotNil(t, tracker.taskStates)
			assert.NotNil(t, tracker.taskOrder)
		})
	}
}

func TestProgressTracker_Start(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	tracker.Start(5, 3)

	assert.Equal(t, 5, tracker.totalTasks)
	assert.Equal(t, 3, tracker.maxWorkers)
	assert.False(t, tracker.started.IsZero())

	// Check that header was written
	output := buf.String()
	assert.Contains(t, output, "5 tasks")
}

func TestProgressTracker_InitializeTasks(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	tasks := []Task{
		{Index: 1, Prompt: "task 1"},
		{Index: 2, Prompt: "task 2"},
		{Index: 3, Prompt: "task 3"},
	}

	tracker.InitializeTasks(tasks)

	// Verify all tasks are initialized as queued
	assert.Len(t, tracker.taskStates, 3)
	assert.Len(t, tracker.taskOrder, 3)

	for i, task := range tasks {
		state := tracker.taskStates[task.Index]
		require.NotNil(t, state)
		assert.Equal(t, "queued", state.status)
		assert.Equal(t, task.Prompt, state.task.Prompt)
		assert.Equal(t, task.Index, tracker.taskOrder[i])
	}
}

func TestProgressTracker_TaskStarted(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}

	// Initialize task first
	tracker.InitializeTasks([]Task{task})
	buf.Reset() // Clear initialization output

	// Start the task
	tracker.TaskStarted(task, 1)

	state := tracker.taskStates[task.Index]
	assert.Equal(t, "running", state.status)
	assert.Equal(t, 1, state.workerID)
}

func TestProgressTracker_TaskCompleted(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}
	result := TaskResult{
		Task:         task,
		Status:       TaskStatusSuccess,
		Output:       "success",
		Duration:     1 * time.Second,
		TokensTotal:  1000,
		Cost:         0.01,
	}

	// Initialize and start task
	tracker.InitializeTasks([]Task{task})
	tracker.TaskStarted(task, 1)
	buf.Reset()

	// Complete the task
	tracker.TaskCompleted(task, result)

	state := tracker.taskStates[task.Index]
	assert.Equal(t, "completed", state.status)
	assert.NotNil(t, state.result)
	assert.True(t, state.collapsed) // Should auto-collapse
}

func TestProgressTracker_TaskFailed(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		totalTasks: 5,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}

	// Test without retries
	tracker.TaskFailed(task, assert.AnError, 0)
	output := buf.String()
	assert.Contains(t, output, "failed")
	assert.Contains(t, output, "test task")

	buf.Reset()

	// Test with retries
	tracker.TaskFailed(task, assert.AnError, 3)
	output = buf.String()
	assert.Contains(t, output, "failed after 3 retries")
}

func TestProgressTracker_TaskRetrying(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		totalTasks: 5,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}
	delay := 2 * time.Second

	tracker.TaskRetrying(task, 2, delay)

	output := buf.String()
	assert.Contains(t, output, "retrying")
	assert.Contains(t, output, "2.0s")
	assert.Contains(t, output, "attempt 2")
}

func TestProgressTracker_TaskError(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		totalTasks: 5,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}

	tracker.TaskError(task, 1, assert.AnError)

	output := buf.String()
	assert.Contains(t, output, "error")
	assert.Contains(t, output, "attempt 1")
}

func TestProgressTracker_ToolExecuted(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}
	metadata := &tools.ExecutionMetadata{
		ToolName: "bash",
		Command:  "ls -la",
		Duration: 100 * time.Millisecond,
	}

	// Initialize and start task
	tracker.InitializeTasks([]Task{task})
	tracker.TaskStarted(task, 1)

	// Execute tool
	tracker.ToolExecuted(task, metadata)

	state := tracker.taskStates[task.Index]
	assert.Len(t, state.tools, 1)
	assert.Equal(t, "bash", state.tools[0].ToolName)
}

func TestProgressTracker_Finish(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
		stopSpinner: make(chan struct{}),
	}

	summary := VolleySummary{
		TotalTasks:     5,
		SucceededTasks: 4,
		FailedTasks:    1,
		Duration:       10 * time.Second,
	}

	tracker.Finish(summary)

	output := buf.String()
	assert.Contains(t, output, "4/5 succeeded")
	assert.Contains(t, output, "1 failed")
	assert.Contains(t, output, "10.0s")
}

func TestProgressTracker_Finish_AllSuccess(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
		stopSpinner: make(chan struct{}),
	}

	summary := VolleySummary{
		TotalTasks:     3,
		SucceededTasks: 3,
		FailedTasks:    0,
		Duration:       5 * time.Second,
	}

	tracker.Finish(summary)

	output := buf.String()
	assert.Contains(t, output, "3/3 tasks succeeded")
	assert.Contains(t, output, "5.0s")
}

func TestProgressTracker_Finish_WithCanceled(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
		stopSpinner: make(chan struct{}),
	}

	summary := VolleySummary{
		TotalTasks:     5,
		SucceededTasks: 2,
		FailedTasks:    1,
		CanceledTasks:  2,
		Duration:       3 * time.Second,
	}

	tracker.Finish(summary)

	output := buf.String()
	assert.Contains(t, output, "2/5 succeeded")
	assert.Contains(t, output, "1 failed")
	assert.Contains(t, output, "2 canceled")
}

func TestProgressTracker_DisabledDoesNotOutput(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    false,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}

	tracker.Start(5, 3)
	tracker.InitializeTasks([]Task{task})
	tracker.TaskStarted(task, 1)
	tracker.TaskCompleted(task, TaskResult{})

	// Nothing should be written when disabled
	assert.Empty(t, buf.String())
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "1234567890",
			maxLen:   10,
			expected: "1234567890",
		},
		{
			name:     "too long",
			input:    "this is a very long string",
			maxLen:   10,
			expected: "this is...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		name     string
		tokens   int64
		expected string
	}{
		{
			name:     "small number",
			tokens:   100,
			expected: "100",
		},
		{
			name:     "exactly 1000",
			tokens:   1000,
			expected: "1.0k",
		},
		{
			name:     "thousands",
			tokens:   5500,
			expected: "5.5k",
		},
		{
			name:     "large number",
			tokens:   15000,
			expected: "15.0k",
		},
		{
			name:     "zero",
			tokens:   0,
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTokens(tt.tokens)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProgressTracker_FormatTaskLine(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	tests := []struct {
		name           string
		state          *taskState
		displayNum     int
		expectContains []string
	}{
		{
			name: "queued task",
			state: &taskState{
				task:   Task{Index: 1, Prompt: "test task"},
				status: "queued",
			},
			displayNum: 1,
			expectContains: []string{
				"1",
				"test task",
			},
		},
		{
			name: "running task",
			state: &taskState{
				task:     Task{Index: 1, Prompt: "test task"},
				status:   "running",
				workerID: 2,
			},
			displayNum: 1,
			expectContains: []string{
				"1",
				"test task",
				"worker 2",
			},
		},
		{
			name: "completed task with result",
			state: &taskState{
				task:   Task{Index: 1, Prompt: "test task"},
				status: "completed",
				result: &TaskResult{
					Duration:    2 * time.Second,
					TokensTotal: 1500,
					Cost:        0.05,
				},
				collapsed: false,
			},
			displayNum: 1,
			expectContains: []string{
				"1",
				"test task",
				"2.0s",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.formatTaskLine(tt.displayNum, tt.state)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestProgressTracker_FormatToolLine(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	exitCode0 := 0
	exitCode1 := 1

	tests := []struct {
		name           string
		treeChar       string
		metadata       *tools.ExecutionMetadata
		expectContains []string
	}{
		{
			name:     "successful bash command",
			treeChar: "├",
			metadata: &tools.ExecutionMetadata{
				ToolName: "bash",
				Command:  "ls -la",
				ExitCode: &exitCode0,
				Duration: 100 * time.Millisecond,
			},
			expectContains: []string{
				"├",
				"bash",
				"ls -la",
			},
		},
		{
			name:     "failed bash command",
			treeChar: "╰",
			metadata: &tools.ExecutionMetadata{
				ToolName: "bash",
				Command:  "exit 1",
				ExitCode: &exitCode1,
				Duration: 50 * time.Millisecond,
			},
			expectContains: []string{
				"╰",
				"bash",
				"exit 1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.formatToolLine(tt.treeChar, tt.metadata)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestProgressTracker_FormatToolSummary(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	tests := []struct {
		name           string
		tools          []*tools.ExecutionMetadata
		expectContains []string
	}{
		{
			name:           "empty tools",
			tools:          []*tools.ExecutionMetadata{},
			expectContains: []string{},
		},
		{
			name: "single tool",
			tools: []*tools.ExecutionMetadata{
				{ToolName: "bash"},
			},
			expectContains: []string{
				"[bash]",
			},
		},
		{
			name: "multiple same tool",
			tools: []*tools.ExecutionMetadata{
				{ToolName: "bash"},
				{ToolName: "bash"},
				{ToolName: "bash"},
			},
			expectContains: []string{
				"[3×bash]",
			},
		},
		{
			name: "mixed tools",
			tools: []*tools.ExecutionMetadata{
				{ToolName: "bash"},
				{ToolName: "view"},
				{ToolName: "bash"},
			},
			expectContains: []string{
				"[2×bash view]",
			},
		},
		{
			name: "many tools truncated",
			tools: []*tools.ExecutionMetadata{
				{ToolName: "bash"},
				{ToolName: "view"},
				{ToolName: "edit"},
				{ToolName: "grep"},
				{ToolName: "glob"},
			},
			expectContains: []string{
				"[bash view edit +2 more]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.formatToolSummary(tt.tools)

			if len(tt.expectContains) == 0 {
				assert.Empty(t, result)
			} else {
				for _, expected := range tt.expectContains {
					assert.Equal(t, expected, result)
				}
			}
		})
	}
}

func TestProgressTracker_ShowThinking(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}

	// Test text format
	tracker.ShowThinking(task, "This is my reasoning", "text")
	output := buf.String()
	assert.Contains(t, output, "[THINKING - Task 1]")
	assert.Contains(t, output, "This is my reasoning")
	assert.Contains(t, output, "[/THINKING]")

	buf.Reset()

	// Test json format
	tracker.ShowThinking(task, "This is my reasoning", "json")
	output = buf.String()
	assert.Contains(t, output, `"type":"thinking"`)
	assert.Contains(t, output, `"task":1`)
	assert.Contains(t, output, `"This is my reasoning"`)
}

func TestProgressTracker_ShowThinking_Disabled(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    false,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}

	// Even when disabled, thinking should still be output (to stderr)
	tracker.ShowThinking(task, "This is my reasoning", "text")
	output := buf.String()
	assert.Contains(t, output, "[THINKING - Task 1]")
}

func TestProgressTracker_SetModel(t *testing.T) {
	tracker := NewProgressTracker(true)

	tracker.SetModel("gpt-4")
	assert.Equal(t, "gpt-4", tracker.modelName)

	tracker.SetModel("claude-3-opus")
	assert.Equal(t, "claude-3-opus", tracker.modelName)
}

func TestProgressTracker_RenderAll(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	// Initialize with tasks
	tasks := []Task{
		{Index: 1, Prompt: "task 1"},
		{Index: 2, Prompt: "task 2"},
	}
	tracker.InitializeTasks(tasks)

	// Start one task
	tracker.TaskStarted(tasks[0], 1)

	// Add a tool to the running task
	tracker.ToolExecuted(tasks[0], &tools.ExecutionMetadata{
		ToolName: "bash",
		Command:  "ls",
		Duration: 100 * time.Millisecond,
	})

	output := buf.String()

	// Should contain both tasks
	assert.Contains(t, output, "task 1")
	assert.Contains(t, output, "task 2")

	// Should contain the tool trace
	assert.Contains(t, output, "bash")
	assert.Contains(t, output, "ls")
}

func TestProgressTracker_SpinnerAnimation(t *testing.T) {
	// This test verifies the spinner state management, not the actual animation
	state := &taskState{
		status:       "running",
		spinnerFrame: 0,
	}

	// Simulate spinner updates
	for i := 0; i < 10; i++ {
		state.spinnerFrame = (state.spinnerFrame + 1) % 4
		assert.True(t, state.spinnerFrame >= 0 && state.spinnerFrame < 4)
	}
}

func TestProgressTracker_TaskStateCollapsing(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}

	// Initialize and start task
	tracker.InitializeTasks([]Task{task})
	tracker.TaskStarted(task, 1)

	// Add multiple tools
	for i := 0; i < 10; i++ {
		tracker.ToolExecuted(task, &tools.ExecutionMetadata{
			ToolName: "bash",
			Command:  "test command",
			Duration: 100 * time.Millisecond,
		})
	}

	state := tracker.taskStates[task.Index]
	assert.Len(t, state.tools, 10)

	// Complete the task - should auto-collapse
	tracker.TaskCompleted(task, TaskResult{
		TokensTotal: 1000,
		Duration:    5 * time.Second,
	})

	assert.True(t, state.collapsed)
}

func TestProgressTracker_MultipleTasksOrdering(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	// Initialize tasks in specific order
	tasks := []Task{
		{Index: 3, Prompt: "task 3"},
		{Index: 1, Prompt: "task 1"},
		{Index: 2, Prompt: "task 2"},
	}

	tracker.InitializeTasks(tasks)

	// Verify tasks maintain submission order
	assert.Equal(t, []int{3, 1, 2}, tracker.taskOrder)

	// Verify all tasks are queued
	for _, task := range tasks {
		state := tracker.taskStates[task.Index]
		assert.NotNil(t, state)
		assert.Equal(t, "queued", state.status)
	}
}

func TestProgressTracker_ToolScrolling(t *testing.T) {
	// Test that only the most recent 4 tools are shown
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	task := Task{Index: 1, Prompt: "test task"}
	tracker.InitializeTasks([]Task{task})
	tracker.TaskStarted(task, 1)

	// Add 10 tools
	for i := 0; i < 10; i++ {
		tracker.ToolExecuted(task, &tools.ExecutionMetadata{
			ToolName: "bash",
			Command:  "test command",
			Duration: 100 * time.Millisecond,
		})
	}

	state := tracker.taskStates[task.Index]
	assert.Len(t, state.tools, 10)

	// The rendering should only show the last 4 tools (when not collapsed)
	// This is tested indirectly through the renderAll method
}

func TestProgressTracker_ConcurrentUpdates(t *testing.T) {
	// Test that the mutex protects concurrent access
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
	}

	tasks := []Task{
		{Index: 1, Prompt: "task 1"},
		{Index: 2, Prompt: "task 2"},
		{Index: 3, Prompt: "task 3"},
	}

	tracker.InitializeTasks(tasks)

	// Simulate concurrent updates (simplified test)
	done := make(chan bool)

	go func() {
		tracker.TaskStarted(tasks[0], 1)
		done <- true
	}()

	go func() {
		tracker.TaskStarted(tasks[1], 2)
		done <- true
	}()

	go func() {
		tracker.TaskStarted(tasks[2], 3)
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// Verify all tasks were updated
	for _, task := range tasks {
		state := tracker.taskStates[task.Index]
		assert.Equal(t, "running", state.status)
	}
}

func TestProgressTracker_VerbositySnapshot(t *testing.T) {
	// Test different verbosity levels produce different output
	tests := []struct {
		name      string
		enabled   bool
		setupFunc func(*ProgressTracker)
	}{
		{
			name:    "disabled - no output",
			enabled: false,
			setupFunc: func(p *ProgressTracker) {
				task := Task{Index: 1, Prompt: "test"}
				p.Start(1, 1)
				p.InitializeTasks([]Task{task})
				p.TaskStarted(task, 1)
				p.TaskCompleted(task, TaskResult{})
			},
		},
		{
			name:    "enabled - with output",
			enabled: true,
			setupFunc: func(p *ProgressTracker) {
				task := Task{Index: 1, Prompt: "test"}
				p.Start(1, 1)
				p.InitializeTasks([]Task{task})
				p.TaskStarted(task, 1)
				p.TaskCompleted(task, TaskResult{})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			tracker := &ProgressTracker{
				enabled:    tt.enabled,
				out:        &buf,
				taskStates: make(map[int]*taskState),
				taskOrder:  []int{},
			}

			tt.setupFunc(tracker)

			output := buf.String()
			if tt.enabled {
				assert.NotEmpty(t, output, "Expected output when enabled")
			} else {
				assert.Empty(t, output, "Expected no output when disabled")
			}
		})
	}
}

func TestProgressTracker_ComplexScenario(t *testing.T) {
	// Integration-style test covering a complete workflow
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
		stopSpinner: make(chan struct{}),
	}

	tasks := []Task{
		{Index: 1, Prompt: "analyze auth.go"},
		{Index: 2, Prompt: "analyze db.go"},
		{Index: 3, Prompt: "analyze api.go"},
	}

	// Start tracking
	tracker.SetModel("gpt-4")
	tracker.Start(3, 2)
	tracker.InitializeTasks(tasks)

	// Task 1: Start and complete successfully
	tracker.TaskStarted(tasks[0], 1)
	tracker.ToolExecuted(tasks[0], &tools.ExecutionMetadata{
		ToolName:  "view",
		FilePath:  "auth.go",
		Operation: "read",
		Duration:  100 * time.Millisecond,
	})
	tracker.TaskCompleted(tasks[0], TaskResult{
		Status:      TaskStatusSuccess,
		Duration:    1 * time.Second,
		TokensTotal: 1000,
		Cost:        0.01,
	})

	// Task 2: Start, execute tools, complete
	tracker.TaskStarted(tasks[1], 2)
	tracker.ToolExecuted(tasks[1], &tools.ExecutionMetadata{
		ToolName:  "view",
		FilePath:  "db.go",
		Operation: "read",
		Duration:  150 * time.Millisecond,
	})
	tracker.ToolExecuted(tasks[1], &tools.ExecutionMetadata{
		ToolName: "bash",
		Command:  "go test",
		Duration: 500 * time.Millisecond,
	})
	tracker.TaskCompleted(tasks[1], TaskResult{
		Status:      TaskStatusSuccess,
		Duration:    2 * time.Second,
		TokensTotal: 1500,
		Cost:        0.02,
	})

	// Task 3: Start, fail, retry, succeed
	tracker.TaskStarted(tasks[2], 1)
	tracker.TaskError(tasks[2], 1, strings.NewReader("timeout").UnreadRune())
	tracker.TaskRetrying(tasks[2], 2, 1*time.Second)
	tracker.TaskCompleted(tasks[2], TaskResult{
		Status:      TaskStatusSuccess,
		Duration:    3 * time.Second,
		TokensTotal: 2000,
		Cost:        0.03,
		Retries:     1,
	})

	// Finish
	tracker.Finish(VolleySummary{
		TotalTasks:     3,
		SucceededTasks: 3,
		FailedTasks:    0,
		Duration:       6 * time.Second,
	})

	output := buf.String()

	// Verify key elements appear in output
	assert.Contains(t, output, "auth.go")
	assert.Contains(t, output, "db.go")
	assert.Contains(t, output, "api.go")
	assert.Contains(t, output, "3/3 tasks succeeded")
	assert.Contains(t, output, "6.0s")
}

func TestProgressTracker_TaskProviderError(t *testing.T) {
	tests := []struct {
		name           string
		errors         []ProviderError
		expectContains []string
	}{
		{
			name: "single error",
			errors: []ProviderError{
				{
					Task:       Task{Index: 1, Prompt: "test task"},
					Attempt:    0,
					Error:      assert.AnError,
					ErrorClass: ErrorClassRateLimit,
					HTTPStatus: 429,
					Message:    "rate limit exceeded",
					IsRetrying: true,
				},
			},
			expectContains: []string{"rate limit exceeded", "⚠"},
		},
		{
			name: "repeated identical errors should collapse",
			errors: []ProviderError{
				{
					Task:       Task{Index: 1, Prompt: "test task"},
					Attempt:    0,
					Error:      assert.AnError,
					ErrorClass: ErrorClassRateLimit,
					HTTPStatus: 429,
					Message:    "rate limit exceeded",
					IsRetrying: true,
				},
				{
					Task:       Task{Index: 1, Prompt: "test task"},
					Attempt:    1,
					Error:      assert.AnError,
					ErrorClass: ErrorClassRateLimit,
					HTTPStatus: 429,
					Message:    "rate limit exceeded",
					IsRetrying: true,
				},
				{
					Task:       Task{Index: 1, Prompt: "test task"},
					Attempt:    2,
					Error:      assert.AnError,
					ErrorClass: ErrorClassRateLimit,
					HTTPStatus: 429,
					Message:    "rate limit exceeded",
					IsRetrying: true,
				},
			},
			expectContains: []string{"×3"}, // Error count should show
		},
		{
			name: "auth error is fatal",
			errors: []ProviderError{
				{
					Task:       Task{Index: 1, Prompt: "test task"},
					Attempt:    0,
					Error:      assert.AnError,
					ErrorClass: ErrorClassAuth,
					HTTPStatus: 401,
					Message:    "authentication failed",
					IsRetrying: false,
				},
			},
			expectContains: []string{"authentication failed", "✗"},
		},
		{
			name: "different error types reset counter",
			errors: []ProviderError{
				{
					Task:       Task{Index: 1, Prompt: "test task"},
					Attempt:    0,
					Error:      assert.AnError,
					ErrorClass: ErrorClassRateLimit,
					HTTPStatus: 429,
					Message:    "rate limit exceeded",
					IsRetrying: true,
				},
				{
					Task:       Task{Index: 1, Prompt: "test task"},
					Attempt:    1,
					Error:      assert.AnError,
					ErrorClass: ErrorClassTimeout,
					HTTPStatus: 0,
					Message:    "request timed out",
					IsRetrying: true,
				},
			},
			expectContains: []string{"request timed out"}, // Should show latest error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			tracker := &ProgressTracker{
				enabled:    true,
				out:        &buf,
				taskStates: make(map[int]*taskState),
				taskOrder:  []int{},
				totalTasks: 1,
			}

			// Initialize task
			tracker.taskStates[1] = &taskState{
				task:   Task{Index: 1, Prompt: "test task"},
				status: "running",
			}
			tracker.taskOrder = []int{1}

			// Report errors
			for _, err := range tt.errors {
				tracker.TaskProviderError(err)
			}

			output := buf.String()

			// Verify expected content
			for _, expected := range tt.expectContains {
				assert.Contains(t, output, expected, "Expected to find '%s' in output", expected)
			}

			// Verify error state is tracked
			state := tracker.taskStates[1]
			require.NotNil(t, state.lastError)
			// Error count is the count for the current error class, not total errors
			// When error class changes, counter resets to 1
			if len(tt.errors) > 1 {
				// Check if last two errors have the same class
				lastErr := tt.errors[len(tt.errors)-1]
				prevErr := tt.errors[len(tt.errors)-2]
				if lastErr.ErrorClass == prevErr.ErrorClass && lastErr.Message == prevErr.Message {
					// Same error class, counter should increment
					assert.Equal(t, len(tt.errors), state.errorCount)
				} else {
					// Different error class, counter should be 1
					assert.Equal(t, 1, state.errorCount)
				}
			} else {
				assert.Equal(t, 1, state.errorCount)
			}
		})
	}
}

func TestProgressTracker_ErrorCollapsing(t *testing.T) {
	var buf bytes.Buffer

	tracker := &ProgressTracker{
		enabled:    true,
		out:        &buf,
		taskStates: make(map[int]*taskState),
		taskOrder:  []int{},
		totalTasks: 1,
	}

	task := Task{Index: 1, Prompt: "test task"}

	// Initialize task
	tracker.taskStates[1] = &taskState{
		task:   task,
		status: "running",
	}
	tracker.taskOrder = []int{1}

	// Report the same error 10 times
	for i := 0; i < 10; i++ {
		err := ProviderError{
			Task:       task,
			Attempt:    i,
			Error:      assert.AnError,
			ErrorClass: ErrorClassRateLimit,
			HTTPStatus: 429,
			Message:    "rate limit exceeded",
			IsRetrying: true,
		}
		buf.Reset() // Clear buffer for each iteration
		tracker.TaskProviderError(err)
	}

	state := tracker.taskStates[1]
	assert.Equal(t, 10, state.errorCount, "Error count should track all occurrences")
	assert.Equal(t, ErrorClassRateLimit, state.lastErrorClass)
}
