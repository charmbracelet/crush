package coordination

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTaskClassifierImpl_NewTaskClassifierImpl tests creating a new TaskClassifierImpl
func TestTaskClassifierImpl_NewTaskClassifierImpl(t *testing.T) {
	tc := NewTaskClassifierImpl()
	assert.NotNil(t, tc)
}

// TestTaskClassifierImpl_Classify_QuickLookup tests quick lookup task classification
func TestTaskClassifierImpl_Classify_QuickLookup(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name     string
		prompt   string
		wantType TaskType
	}{
		{
			name:     "What is command",
			prompt:   "What is the ls command?",
			wantType: TaskQuickLookup,
		},
		{
			name:     "How to do something",
			prompt:   "How to delete a file in Linux?",
			wantType: TaskQuickLookup,
		},
		{
			name:     "Explain concept",
			prompt:   "Explain what Git is",
			wantType: TaskQuickLookup,
		},
		{
			name:     "Definition request",
			prompt:   "What does docker do?",
			wantType: TaskQuickLookup,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.wantType, result.TaskType, "Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_Classify_FileOperation tests file operation task classification
func TestTaskClassifierImpl_Classify_FileOperation(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name     string
		prompt   string
		wantType TaskType
	}{
		{
			name:     "Read file",
			prompt:   "Show me the content of config.txt",
			wantType: TaskFileOperation,
		},
		{
			name:     "Write file",
			prompt:   "Create a new file called hello.txt",
			wantType: TaskFileOperation,
		},
		{
			name:     "Edit file",
			prompt:   "Update the config file",
			wantType: TaskFileOperation,
		},
		{
			name:     "Delete file",
			prompt:   "Remove the old logs",
			wantType: TaskFileOperation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.wantType, result.TaskType, "Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_Classify_GitHub tests GitHub task classification
func TestTaskClassifierImpl_Classify_GitHub(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name     string
		prompt   string
		wantType TaskType
	}{
		{
			name:     "Git status",
			prompt:   "Check the git status",
			wantType: TaskGitHub,
		},
		{
			name:     "Git commit",
			prompt:   "Commit the changes",
			wantType: TaskGitHub,
		},
		{
			name:     "Git push",
			prompt:   "Push to remote",
			wantType: TaskGitHub,
		},
		{
			name:     "Git PR",
			prompt:   "Create a pull request",
			wantType: TaskGitHub,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.wantType, result.TaskType, "Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_Classify_BugHunt tests bug hunt task classification
func TestTaskClassifierImpl_Classify_BugHunt(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name     string
		prompt   string
		wantType TaskType
	}{
		{
			name:     "Find bug",
			prompt:   "Find the bug in this code",
			wantType: TaskBugHunt,
		},
		{
			name:     "Debug issue",
			prompt:   "Debug why this is crashing",
			wantType: TaskBugHunt,
		},
		{
			name:     "Fix error",
			prompt:   "Fix the null pointer exception",
			wantType: TaskBugHunt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.wantType, result.TaskType, "Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_Classify_CodeReview tests code review task classification
func TestTaskClassifierImpl_Classify_CodeReview(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name     string
		prompt   string
		wantType TaskType
	}{
		{
			name:     "Review code",
			prompt:   "Review this code for best practices",
			wantType: TaskCodeReview,
		},
		{
			name:     "Check quality",
			prompt:   "Check code quality",
			wantType: TaskCodeReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.wantType, result.TaskType, "Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_Classify_ComplexRefactor tests complex refactor task classification
func TestTaskClassifierImpl_Classify_ComplexRefactor(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name     string
		prompt   string
		wantType TaskType
	}{
		{
			name:     "Refactor module",
			prompt:   "Refactor the entire auth module",
			wantType: TaskComplexRefactor,
		},
		{
			name:     "Rewrite system",
			prompt:   "Rewrite the caching layer",
			wantType: TaskComplexRefactor,
		},
		{
			name:     "Large refactor",
			prompt:   "Migrate from REST to GraphQL",
			wantType: TaskComplexRefactor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.wantType, result.TaskType, "Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_Classify_Creative tests creative task classification
func TestTaskClassifierImpl_Classify_Creative(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name     string
		prompt   string
		wantType TaskType
	}{
		{
			name:     "Write story",
			prompt:   "Write me a short story",
			wantType: TaskCreative,
		},
		{
			name:     "Generate ideas",
			prompt:   "Give me ideas for a new app",
			wantType: TaskCreative,
		},
		{
			name:     "Brainstorm",
			prompt:   "Brainstorm creative solutions",
			wantType: TaskCreative,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.wantType, result.TaskType, "Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_Classify_MCPTask tests MCP task classification
func TestTaskClassifierImpl_Classify_MCPTask(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name     string
		prompt   string
		wantType TaskType
	}{
		{
			name:     "MCP tool use",
			prompt:   "Use the filesystem tool to read a file",
			wantType: TaskMCPTask,
		},
		{
			name:     "MCP search",
			prompt:   "Search the web for latest news",
			wantType: TaskMCPTask,
		},
		{
			name:     "MCP database",
			prompt:   "Query the database for users",
			wantType: TaskMCPTask,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.wantType, result.TaskType, "Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_Classify_DataProcessing tests data processing classification
func TestTaskClassifierImpl_Classify_DataProcessing(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name     string
		prompt   string
		wantType TaskType
	}{
		{
			name:     "Process data",
			prompt:   "Process this CSV file",
			wantType: TaskDataProcessing,
		},
		{
			name:     "Transform data",
			prompt:   "Transform the JSON data",
			wantType: TaskDataProcessing,
		},
		{
			name:     "Parse logs",
			prompt:   "Parse these log files",
			wantType: TaskDataProcessing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.wantType, result.TaskType, "Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_Classify_DefaultCase tests default/unknown task classification
func TestTaskClassifierImpl_Classify_DefaultCase(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name     string
		prompt   string
		wantType TaskType
	}{
		{
			name:     "Generic task",
			prompt:   "Do something for me",
			wantType: TaskUnknown,
		},
		{
			name:     "Ambiguous task",
			prompt:   "Handle this",
			wantType: TaskUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.wantType, result.TaskType, "Unexpected type for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_Classify_EmptyPrompt tests with empty prompt
func TestTaskClassifierImpl_Classify_EmptyPrompt(t *testing.T) {
	tc := NewTaskClassifierImpl()

	result := tc.Classify("")
	// Should return unknown for empty
	assert.Equal(t, TaskUnknown, result.TaskType)
}

// TestTaskClassifierImpl_Confidence tests confidence score calculation
func TestTaskClassifierImpl_Confidence(t *testing.T) {
	tc := NewTaskClassifierImpl()

	// Clear match should have some confidence
	result := tc.Classify("What is the ls command?")
	assert.Greater(t, result.Confidence, 0.0)
}

// TestTaskClassifierImpl_ExecutorSelection tests executor selection
func TestTaskClassifierImpl_ExecutorSelection(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name             string
		prompt           string
		expectedExecutor ExecutorType
	}{
		{
			name:             "Quick lookup - CL",
			prompt:           "What is git?",
			expectedExecutor: ExecutorCL,
		},
		{
			name:             "File operation - CL",
			prompt:           "Read the file",
			expectedExecutor: ExecutorCL,
		},
		{
			name:             "GitHub - Claude Code",
			prompt:           "Create a pull request",
			expectedExecutor: ExecutorClaudeCode,
		},
		{
			name:             "Bug hunt - Claude Code",
			prompt:           "Fix the bug",
			expectedExecutor: ExecutorClaudeCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.expectedExecutor, result.Executor,
				"Unexpected executor for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_ToolsAssignment tests tools assignment
func TestTaskClassifierImpl_ToolsAssignment(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name        string
		prompt      string
		expectTools []string
	}{
		{
			name:        "GitHub task",
			prompt:      "Create a pull request",
			expectTools: []string{"Bash", "Read", "Grep"},
		},
		{
			name:        "File operation",
			prompt:      "Read the file",
			expectTools: []string{"Read", "Write", "Edit", "Bash"},
		},
		{
			name:        "Code review",
			prompt:      "Review this code",
			expectTools: []string{"Read", "Grep", "Glob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.Equal(t, tt.expectTools, result.Tools,
				"Unexpected tools for prompt: %s", tt.prompt)
		})
	}
}

// TestTaskClassifierImpl_CostEstimate tests cost estimation
func TestTaskClassifierImpl_CostEstimate(t *testing.T) {
	tc := NewTaskClassifierImpl()

	tests := []struct {
		name    string
		prompt  string
		minCost float64
		maxCost float64
	}{
		{
			name:    "Quick lookup cheap",
			prompt:  "What is git?",
			minCost: 0.0,
			maxCost: 0.002,
		},
		{
			name:    "Complex refactor expensive",
			prompt:  "Refactor the entire auth module",
			minCost: 0.05,
			maxCost: 0.15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.Classify(tt.prompt)
			assert.GreaterOrEqual(t, result.CostEstimate, tt.minCost)
			assert.LessOrEqual(t, result.CostEstimate, tt.maxCost)
		})
	}
}

// TestTaskClassifierImpl_Reason tests reason formatting
func TestTaskClassifierImpl_Reason(t *testing.T) {
	tc := NewTaskClassifierImpl()

	result := tc.Classify("What is git?")
	assert.NotEmpty(t, result.Reason)
}

// TestTaskClassifierImpl_TaskTypeString tests string representation
func TestTaskClassifierImpl_TaskTypeString(t *testing.T) {
	tests := []struct {
		taskType TaskType
		expected string
	}{
		{TaskUnknown, "unknown"},
		{TaskQuickLookup, "quick_lookup"},
		{TaskFileOperation, "file_operation"},
		{TaskDataProcessing, "data_processing"},
		{TaskGitHub, "github"},
		{TaskComplexRefactor, "complex_refactor"},
		{TaskCodeReview, "code_review"},
		{TaskBugHunt, "bug_hunt"},
		{TaskCreative, "creative"},
		{TaskMCPTask, "mcp_task"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.taskType.String())
		})
	}
}
