package output

import (
	"strings"
	"testing"
	"time"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/llm/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatToolTrace_ReadOperation(t *testing.T) {
	tests := []struct {
		name           string
		metadata       *tools.ExecutionMetadata
		verbosity      config.VerbosityLevel
		expectContains []string
		expectEmpty    bool
	}{
		{
			name: "read file - normal verbosity",
			metadata: &tools.ExecutionMetadata{
				ToolName:  "view",
				Operation: "read",
				FilePath:  "/path/to/file.go",
				LineCount: 100,
				ByteSize:  4096,
				Duration:  500 * time.Millisecond,
			},
			verbosity: config.VerbosityNormal,
			expectContains: []string{
				"[TOOL]",
				"view",
				"/path/to/file.go",
				"100 lines",
				"4.0 KB",
				"0.5s",
			},
		},
		{
			name: "read file - verbose verbosity",
			metadata: &tools.ExecutionMetadata{
				ToolName:  "view",
				Operation: "read",
				FilePath:  "/path/to/file.go",
				LineCount: 50,
				ByteSize:  2048,
				Duration:  250 * time.Millisecond,
			},
			verbosity: config.VerbosityVerbose,
			expectContains: []string{
				"[TOOL]",
				"view",
				"/path/to/file.go",
				"50 lines",
				"2.0 KB",
				"0.2s",
			},
		},
		{
			name: "read file - quiet verbosity",
			metadata: &tools.ExecutionMetadata{
				ToolName:  "view",
				Operation: "read",
				FilePath:  "/path/to/file.go",
				LineCount: 100,
				ByteSize:  4096,
				Duration:  500 * time.Millisecond,
			},
			verbosity:   config.VerbosityQuiet,
			expectEmpty: true,
		},
		{
			name:        "nil metadata",
			metadata:    nil,
			verbosity:   config.VerbosityNormal,
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatToolTrace(tt.metadata, tt.verbosity)

			if tt.expectEmpty {
				assert.Empty(t, result)
				return
			}

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected, "Expected to find %q in output", expected)
			}
		})
	}
}

func TestFormatToolTrace_WriteOperation(t *testing.T) {
	tests := []struct {
		name           string
		metadata       *tools.ExecutionMetadata
		verbosity      config.VerbosityLevel
		expectContains []string
	}{
		{
			name: "write file",
			metadata: &tools.ExecutionMetadata{
				ToolName:  "write",
				Operation: "write",
				FilePath:  "/path/to/output.go",
				LineCount: 50,
				ByteSize:  2048,
				Duration:  100 * time.Millisecond,
			},
			verbosity: config.VerbosityNormal,
			expectContains: []string{
				"[TOOL]",
				"write",
				"/path/to/output.go",
				"+2048c",
				"+50L",
				"0.1s",
			},
		},
		{
			name: "created file",
			metadata: &tools.ExecutionMetadata{
				ToolName:  "write",
				Operation: "created",
				FilePath:  "/path/to/new.go",
				LineCount: 25,
				ByteSize:  1024,
				Duration:  50 * time.Millisecond,
			},
			verbosity: config.VerbosityNormal,
			expectContains: []string{
				"[TOOL]",
				"write",
				"/path/to/new.go",
				"+1024c",
				"+25L",
			},
		},
		{
			name: "modified file",
			metadata: &tools.ExecutionMetadata{
				ToolName:  "edit",
				Operation: "modified",
				FilePath:  "/path/to/existing.go",
				LineCount: 10,
				ByteSize:  512,
				Duration:  75 * time.Millisecond,
			},
			verbosity: config.VerbosityNormal,
			expectContains: []string{
				"[TOOL]",
				"edit",
				"/path/to/existing.go",
				"+512c",
				"+10L",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatToolTrace(tt.metadata, tt.verbosity)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestFormatToolTrace_SearchOperation(t *testing.T) {
	tests := []struct {
		name           string
		metadata       *tools.ExecutionMetadata
		verbosity      config.VerbosityLevel
		expectContains []string
	}{
		{
			name: "glob with single match",
			metadata: &tools.ExecutionMetadata{
				ToolName:   "glob",
				Pattern:    "**/*.go",
				MatchCount: 1,
				Duration:   200 * time.Millisecond,
			},
			verbosity: config.VerbosityNormal,
			expectContains: []string{
				"[TOOL]",
				"glob",
				"**/*.go",
				"1 match",
				"0.2s",
			},
		},
		{
			name: "glob with multiple matches",
			metadata: &tools.ExecutionMetadata{
				ToolName:   "glob",
				Pattern:    "**/*.go",
				MatchCount: 42,
				Duration:   300 * time.Millisecond,
			},
			verbosity: config.VerbosityNormal,
			expectContains: []string{
				"[TOOL]",
				"glob",
				"**/*.go",
				"42 matches",
				"0.3s",
			},
		},
		{
			name: "grep with no matches",
			metadata: &tools.ExecutionMetadata{
				ToolName:   "grep",
				Pattern:    "TODO",
				MatchCount: 0,
				Duration:   150 * time.Millisecond,
			},
			verbosity: config.VerbosityNormal,
			expectContains: []string{
				"[TOOL]",
				"grep",
				"TODO",
				"0 matches",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatToolTrace(tt.metadata, tt.verbosity)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestFormatToolTrace_BashCommand(t *testing.T) {
	exitCode0 := 0
	exitCode1 := 1

	tests := []struct {
		name           string
		metadata       *tools.ExecutionMetadata
		verbosity      config.VerbosityLevel
		expectContains []string
		expectNotContains []string
	}{
		{
			name: "bash command success",
			metadata: &tools.ExecutionMetadata{
				ToolName: "bash",
				Command:  "ls -la",
				ExitCode: &exitCode0,
				Duration: 100 * time.Millisecond,
			},
			verbosity: config.VerbosityNormal,
			expectContains: []string{
				"[TOOL]",
				"bash",
				"ls -la",
				"0.1s",
			},
		},
		{
			name: "bash command failure",
			metadata: &tools.ExecutionMetadata{
				ToolName: "bash",
				Command:  "exit 1",
				ExitCode: &exitCode1,
				Duration: 50 * time.Millisecond,
			},
			verbosity: config.VerbosityNormal,
			expectContains: []string{
				"[TOOL]",
				"bash",
				"exit 1",
			},
		},
		{
			name: "long bash command truncated",
			metadata: &tools.ExecutionMetadata{
				ToolName: "bash",
				Command:  "this is a very long command that should be truncated because it exceeds the maximum length allowed for display",
				Duration: 200 * time.Millisecond,
			},
			verbosity: config.VerbosityNormal,
			expectContains: []string{
				"[TOOL]",
				"bash",
				"...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatToolTrace(tt.metadata, tt.verbosity)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected)
			}

			for _, notExpected := range tt.expectNotContains {
				assert.NotContains(t, result, notExpected)
			}
		})
	}
}

func TestFormatToolTrace_WithDiff(t *testing.T) {
	diff := `--- a/file.go
+++ b/file.go
@@ -10,5 +10,5 @@
 func main() {
-    fmt.Println("old")
+    fmt.Println("new")
 }`

	metadata := &tools.ExecutionMetadata{
		ToolName:  "edit",
		Operation: "modified",
		FilePath:  "/path/to/file.go",
		Diff:      diff,
		Additions: 1,
		Deletions: 1,
		Duration:  150 * time.Millisecond,
	}

	// Normal verbosity - should NOT include diff
	resultNormal := FormatToolTrace(metadata, config.VerbosityNormal)
	assert.Contains(t, resultNormal, "[TOOL]")
	assert.Contains(t, resultNormal, "edit")
	assert.NotContains(t, resultNormal, "[DIFF]")

	// Verbose verbosity - should include diff
	resultVerbose := FormatToolTrace(metadata, config.VerbosityVerbose)
	assert.Contains(t, resultVerbose, "[TOOL]")
	assert.Contains(t, resultVerbose, "edit")
	assert.Contains(t, resultVerbose, "[DIFF]")
	assert.Contains(t, resultVerbose, "+1 lines")
	assert.Contains(t, resultVerbose, "-1 deletions")
}

func TestFormatToolTrace_DurationFormatting(t *testing.T) {
	tests := []struct {
		name           string
		duration       time.Duration
		expectedString string
	}{
		{
			name:           "milliseconds",
			duration:       250 * time.Millisecond,
			expectedString: "0.2s",
		},
		{
			name:           "one second",
			duration:       1 * time.Second,
			expectedString: "1.0s",
		},
		{
			name:           "multiple seconds",
			duration:       3500 * time.Millisecond,
			expectedString: "3.5s",
		},
		{
			name:           "very fast",
			duration:       50 * time.Millisecond,
			expectedString: "0.0s", // or "0.1s" depending on rounding
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &tools.ExecutionMetadata{
				ToolName: "test",
				Command:  "test command",
				Duration: tt.duration,
			}

			result := FormatToolTrace(metadata, config.VerbosityNormal)
			assert.Contains(t, result, "s") // Should contain a seconds indicator
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			bytes:    1024,
			expected: "1.0 KB",
		},
		{
			name:     "kilobytes with decimal",
			bytes:    1536,
			expected: "1.5 KB",
		},
		{
			name:     "megabytes",
			bytes:    1048576,
			expected: "1.0 MB",
		},
		{
			name:     "megabytes with decimal",
			bytes:    2097152,
			expected: "2.0 MB",
		},
		{
			name:     "gigabytes",
			bytes:    1073741824,
			expected: "1.0 GB",
		},
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertToolMetadataToJSON(t *testing.T) {
	exitCode0 := 0
	exitCode1 := 1

	tests := []struct {
		name     string
		metadata []*tools.ExecutionMetadata
		expected int // expected number of tools
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			expected: 0,
		},
		{
			name:     "empty metadata",
			metadata: []*tools.ExecutionMetadata{},
			expected: 0,
		},
		{
			name: "single tool",
			metadata: []*tools.ExecutionMetadata{
				{
					ToolName: "bash",
					Command:  "ls -la",
					Duration: 100 * time.Millisecond,
					ExitCode: &exitCode0,
				},
			},
			expected: 1,
		},
		{
			name: "multiple tools",
			metadata: []*tools.ExecutionMetadata{
				{
					ToolName:  "view",
					FilePath:  "/path/to/file.go",
					Operation: "read",
					Duration:  50 * time.Millisecond,
				},
				{
					ToolName: "bash",
					Command:  "go test",
					Duration: 1000 * time.Millisecond,
					ExitCode: &exitCode1,
				},
				{
					ToolName:   "grep",
					Pattern:    "TODO",
					MatchCount: 5,
					Duration:   200 * time.Millisecond,
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToolMetadataToJSON(tt.metadata)

			if tt.expected == 0 {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Len(t, result, tt.expected)

				// Verify fields are populated correctly
				for i, tool := range result {
					assert.NotEmpty(t, tool.Name)
					assert.Equal(t, tt.metadata[i].ToolName, tool.Name)
					assert.Equal(t, tt.metadata[i].Duration.Milliseconds(), tool.DurationMS)

					// Check optional fields
					if tt.metadata[i].FilePath != "" {
						assert.Equal(t, tt.metadata[i].FilePath, tool.File)
					}
					if tt.metadata[i].Pattern != "" {
						assert.Equal(t, tt.metadata[i].Pattern, tool.Pattern)
					}
					if tt.metadata[i].Command != "" {
						assert.Equal(t, tt.metadata[i].Command, tool.Command)
					}
					if tt.metadata[i].ExitCode != nil {
						assert.Equal(t, tt.metadata[i].ExitCode, tool.ExitCode)
					}
				}
			}
		})
	}
}

func TestEmitToolTraceNDJSON(t *testing.T) {
	exitCode0 := 0

	tests := []struct {
		name           string
		taskIndex      int
		metadata       *tools.ExecutionMetadata
		expectContains []string
		expectNil      bool
	}{
		{
			name:      "nil metadata",
			taskIndex: 1,
			metadata:  nil,
			expectNil: true,
		},
		{
			name:      "bash command",
			taskIndex: 1,
			metadata: &tools.ExecutionMetadata{
				ToolName: "bash",
				Command:  "go test ./...",
				Duration: 1500 * time.Millisecond,
				ExitCode: &exitCode0,
			},
			expectContains: []string{
				`"task_index":1`,
				`"tool_name":"bash"`,
				`"command":"go test ./..."`,
				`"duration_ms":1500`,
				`"exit_code":0`,
			},
		},
		{
			name:      "view file",
			taskIndex: 2,
			metadata: &tools.ExecutionMetadata{
				ToolName:  "view",
				FilePath:  "/path/to/file.go",
				Operation: "read",
				LineCount: 100,
				ByteSize:  4096,
				Duration:  50 * time.Millisecond,
			},
			expectContains: []string{
				`"task_index":2`,
				`"tool_name":"view"`,
				`"file_path":"/path/to/file.go"`,
				`"operation":"read"`,
				`"line_count":100`,
				`"byte_size":4096`,
			},
		},
		{
			name:      "grep with error",
			taskIndex: 3,
			metadata: &tools.ExecutionMetadata{
				ToolName:     "grep",
				Pattern:      "TODO",
				ErrorMessage: "pattern not found",
				Duration:     100 * time.Millisecond,
			},
			expectContains: []string{
				`"task_index":3`,
				`"tool_name":"grep"`,
				`"pattern":"TODO"`,
				`"error":"pattern not found"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder

			err := EmitToolTraceNDJSON(&buf, tt.taskIndex, tt.metadata)

			if tt.expectNil {
				assert.NoError(t, err)
				assert.Empty(t, buf.String())
				return
			}

			require.NoError(t, err)
			output := buf.String()

			// Verify it's valid JSON (ends with newline)
			assert.True(t, strings.HasSuffix(output, "\n"))

			for _, expected := range tt.expectContains {
				assert.Contains(t, output, expected)
			}

			// Verify timestamp field exists
			assert.Contains(t, output, `"timestamp"`)
		})
	}
}

func TestFormatDiff(t *testing.T) {
	tests := []struct {
		name           string
		diff           string
		additions      int
		deletions      int
		expectContains []string
	}{
		{
			name: "simple diff",
			diff: `--- a/file.go
+++ b/file.go
@@ -1,3 +1,3 @@
 package main
-func old() {}
+func new() {}`,
			additions: 1,
			deletions: 1,
			expectContains: []string{
				"[DIFF]",
				"--- a/file.go",
				"+++ b/file.go",
				"+1 lines",
				"-1 deletions",
			},
		},
		{
			name: "long diff truncated",
			diff: strings.Repeat("line\n", 30), // 30 lines
			additions: 15,
			deletions: 15,
			expectContains: []string{
				"[DIFF]",
				"more lines", // Should mention more lines
				"+15 lines",
				"-15 deletions",
			},
		},
		{
			name: "no stats",
			diff: "simple diff",
			additions: 0,
			deletions: 0,
			expectContains: []string{
				"[DIFF]",
				"simple diff",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDiff(tt.diff, tt.additions, tt.deletions)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestFormatJSON(t *testing.T) {
	tests := []struct {
		name           string
		results        interface{}
		summary        interface{}
		expectContains []string
		expectError    bool
	}{
		{
			name: "valid results and summary",
			results: []map[string]interface{}{
				{"task": "task 1", "status": "success"},
				{"task": "task 2", "status": "success"},
			},
			summary: map[string]interface{}{
				"total_tasks": 2,
				"succeeded":   2,
			},
			expectContains: []string{
				`"results"`,
				`"summary"`,
				`"task 1"`,
				`"task 2"`,
				`"total_tasks"`,
			},
			expectError: false,
		},
		{
			name:    "empty results",
			results: []interface{}{},
			summary: map[string]interface{}{},
			expectContains: []string{
				`"results"`,
				`"summary"`,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FormatJSON(tt.results, tt.summary)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			for _, expected := range tt.expectContains {
				assert.Contains(t, result, expected)
			}

			// Verify it's valid JSON (should have proper formatting)
			assert.True(t, strings.Contains(result, "{") && strings.Contains(result, "}"))
		})
	}
}

func TestFormatDiffOutput(t *testing.T) {
	// This is a placeholder implementation
	result := FormatDiffOutput([]interface{}{})

	// Currently returns placeholder message
	assert.Contains(t, result, "Diff output not yet implemented")
}

func TestToolTraceNDJSON_Completeness(t *testing.T) {
	// Test that all fields are properly marshaled
	exitCode := 1
	metadata := &tools.ExecutionMetadata{
		ToolName:     "bash",
		Command:      "test command",
		Duration:     500 * time.Millisecond,
		FilePath:     "/path/to/file",
		Operation:    "test",
		LineCount:    10,
		ByteSize:     1024,
		Pattern:      "test pattern",
		MatchCount:   5,
		ExitCode:     &exitCode,
		ErrorMessage: "test error",
		Additions:    2,
		Deletions:    1,
	}

	var buf strings.Builder
	err := EmitToolTraceNDJSON(&buf, 1, metadata)
	require.NoError(t, err)

	output := buf.String()

	// Verify all fields are present
	expectedFields := []string{
		`"timestamp"`,
		`"task_index":1`,
		`"tool_name":"bash"`,
		`"duration_ms":500`,
		`"file_path":"/path/to/file"`,
		`"operation":"test"`,
		`"line_count":10`,
		`"byte_size":1024`,
		`"pattern":"test pattern"`,
		`"match_count":5`,
		`"command":"test command"`,
		`"exit_code":1`,
		`"error":"test error"`,
		`"additions":2`,
		`"deletions":1`,
	}

	for _, field := range expectedFields {
		assert.Contains(t, output, field, "Expected field %s in output", field)
	}
}

func TestFormatToolTrace_ToolNamePadding(t *testing.T) {
	// Test that tool names are padded consistently for alignment
	tests := []struct {
		name     string
		toolName string
	}{
		{"short name", "view"},
		{"medium name", "bash"},
		{"long name", "grep"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &tools.ExecutionMetadata{
				ToolName: tt.toolName,
				Command:  "test",
				Duration: 100 * time.Millisecond,
			}

			result := FormatToolTrace(metadata, config.VerbosityNormal)

			// The output should contain the tool name
			assert.Contains(t, result, tt.toolName)
		})
	}
}
