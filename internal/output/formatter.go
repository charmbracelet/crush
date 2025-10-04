package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/llm/tools"
)

// FormatToolTrace formats tool execution metadata for display
func FormatToolTrace(metadata *tools.ExecutionMetadata, verbosity config.VerbosityLevel) string {
	if verbosity == config.VerbosityQuiet || metadata == nil {
		return ""
	}

	var parts []string

	// Tool name (padded for alignment)
	toolName := fmt.Sprintf("%-6s", metadata.ToolName)
	parts = append(parts, toolName)

	// Format based on tool type
	switch {
	case metadata.Operation == "read" && metadata.FilePath != "":
		sizeStr := formatBytes(metadata.ByteSize)
		parts = append(parts, fmt.Sprintf("◱╼%s (%d lines, %s)", metadata.FilePath, metadata.LineCount, sizeStr))

	case metadata.Operation == "write" || metadata.Operation == "created" || metadata.Operation == "modified":
		if metadata.FilePath != "" {
			// Calculate stats for write operations
			statsStr := ""
			if metadata.ByteSize > 0 {
				statsStr = fmt.Sprintf(" +%dc", metadata.ByteSize)
			}
			if metadata.LineCount > 0 {
				statsStr += fmt.Sprintf(" +%dL", metadata.LineCount)
			}
			parts = append(parts, fmt.Sprintf("◱╼%s%s", metadata.FilePath, statsStr))
		}

	case metadata.Pattern != "" && metadata.MatchCount >= 0:
		// Glob or grep
		if metadata.MatchCount == 1 {
			parts = append(parts, fmt.Sprintf("%s → %d match", metadata.Pattern, metadata.MatchCount))
		} else {
			parts = append(parts, fmt.Sprintf("%s → %d matches", metadata.Pattern, metadata.MatchCount))
		}

	case metadata.Command != "":
		// Bash command - no exit code display (icon shows success/fail)
		cmdPreview := metadata.Command
		if len(cmdPreview) > 60 {
			cmdPreview = cmdPreview[:57] + "..."
		}
		parts = append(parts, cmdPreview)
	}

	// Duration (always show in normal/verbose mode)
	if metadata.Duration > 0 {
		parts = append(parts, fmt.Sprintf("%.1fs", metadata.Duration.Seconds()))
	}

	// Join with space (tree chars and icon added by progress.go)
	result := "[TOOL] " + strings.Join(parts, " ")

	// In verbose mode, add diff if available
	if verbosity == config.VerbosityVerbose && metadata.Diff != "" {
		result += "\n" + formatDiff(metadata.Diff, metadata.Additions, metadata.Deletions)
	}

	return result
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDiff formats a diff for display
func formatDiff(diff string, additions, deletions int) string {
	var result strings.Builder
	result.WriteString("\n[DIFF]\n")

	// Show preview of diff (first 20 lines)
	lines := strings.Split(diff, "\n")
	maxLines := 20
	if len(lines) > maxLines {
		result.WriteString(strings.Join(lines[:maxLines], "\n"))
		result.WriteString(fmt.Sprintf("\n... (%d more lines)", len(lines)-maxLines))
	} else {
		result.WriteString(diff)
	}

	// Show stats
	if additions > 0 || deletions > 0 {
		result.WriteString(fmt.Sprintf("\n\nModified: +%d lines, -%d deletions", additions, deletions))
	}

	return result.String()
}

// JSON Output Schema Types

// TaskJSONOutput represents a single task result in JSON format
type TaskJSONOutput struct {
	Task     string           `json:"task"`
	Status   string           `json:"status"`
	Result   string           `json:"result,omitempty"`
	Error    string           `json:"error,omitempty"`
	Metadata TaskMetadataJSON `json:"metadata"`
}

// TaskMetadataJSON contains metadata about task execution
type TaskMetadataJSON struct {
	DurationMS int64            `json:"duration_ms"`
	Tokens     TokenMetadataJSON `json:"tokens"`
	Cost       float64          `json:"cost"`
	Model      string           `json:"model"`
	ToolsUsed  []ToolUsageJSON  `json:"tools_used,omitempty"`
	Retries    int              `json:"retries"`
}

// TokenMetadataJSON contains token usage information
type TokenMetadataJSON struct {
	Input  int64 `json:"input"`
	Output int64 `json:"output"`
	Total  int64 `json:"total"`
}

// ToolUsageJSON represents a single tool execution
type ToolUsageJSON struct {
	Name       string  `json:"name"`
	File       string  `json:"file,omitempty"`
	Pattern    string  `json:"pattern,omitempty"`
	Command    string  `json:"command,omitempty"`
	DurationMS int64   `json:"duration_ms"`
	ExitCode   *int    `json:"exit_code,omitempty"`
}

// VolleyJSONOutput represents multiple task results with summary
type VolleyJSONOutput struct {
	Tasks   []TaskJSONOutput   `json:"tasks"`
	Summary SummaryJSON        `json:"summary"`
}

// SummaryJSON contains aggregate results from a volley execution
type SummaryJSON struct {
	TotalTasks        int     `json:"total_tasks"`
	SucceededTasks    int     `json:"succeeded_tasks"`
	FailedTasks       int     `json:"failed_tasks"`
	CanceledTasks     int     `json:"canceled_tasks"`
	TotalDurationMS   int64   `json:"total_duration_ms"`
	TotalTokens       int64   `json:"total_tokens"`
	TotalCost         float64 `json:"total_cost"`
	MaxConcurrentUsed int     `json:"max_concurrent_used"`
	TotalRetries      int     `json:"total_retries"`
}

// ConvertToolMetadataToJSON converts tool execution metadata to JSON format
func ConvertToolMetadataToJSON(metadata []*tools.ExecutionMetadata) []ToolUsageJSON {
	if len(metadata) == 0 {
		return nil
	}

	result := make([]ToolUsageJSON, 0, len(metadata))
	for _, m := range metadata {
		tool := ToolUsageJSON{
			Name:       m.ToolName,
			DurationMS: m.Duration.Milliseconds(),
		}

		// Add file path if present
		if m.FilePath != "" {
			tool.File = m.FilePath
		}

		// Add pattern if present
		if m.Pattern != "" {
			tool.Pattern = m.Pattern
		}

		// Add command if present
		if m.Command != "" {
			tool.Command = m.Command
		}

		// Add exit code if present
		if m.ExitCode != nil {
			tool.ExitCode = m.ExitCode
		}

		result = append(result, tool)
	}

	return result
}

// ToolTraceNDJSON represents a tool execution in NDJSON format
type ToolTraceNDJSON struct {
	Timestamp  string  `json:"timestamp"`
	TaskIndex  int     `json:"task_index"`
	ToolName   string  `json:"tool_name"`
	DurationMS int64   `json:"duration_ms"`
	FilePath   string  `json:"file_path,omitempty"`
	Operation  string  `json:"operation,omitempty"`
	LineCount  int     `json:"line_count,omitempty"`
	ByteSize   int64   `json:"byte_size,omitempty"`
	Pattern    string  `json:"pattern,omitempty"`
	MatchCount int     `json:"match_count,omitempty"`
	Command    string  `json:"command,omitempty"`
	ExitCode   *int    `json:"exit_code,omitempty"`
	Error      string  `json:"error,omitempty"`
	Additions  int     `json:"additions,omitempty"`
	Deletions  int     `json:"deletions,omitempty"`
}

// EmitToolTraceNDJSON writes tool execution metadata as NDJSON to the given writer
func EmitToolTraceNDJSON(w io.Writer, taskIndex int, metadata *tools.ExecutionMetadata) error {
	if metadata == nil {
		return nil
	}

	trace := ToolTraceNDJSON{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		TaskIndex:  taskIndex,
		ToolName:   metadata.ToolName,
		DurationMS: metadata.Duration.Milliseconds(),
		FilePath:   metadata.FilePath,
		Operation:  metadata.Operation,
		LineCount:  metadata.LineCount,
		ByteSize:   metadata.ByteSize,
		Pattern:    metadata.Pattern,
		MatchCount: metadata.MatchCount,
		Command:    metadata.Command,
		ExitCode:   metadata.ExitCode,
		Additions:  metadata.Additions,
		Deletions:  metadata.Deletions,
	}

	if metadata.ErrorMessage != "" {
		trace.Error = metadata.ErrorMessage
	}

	// Marshal to single-line JSON
	jsonBytes, err := json.Marshal(trace)
	if err != nil {
		return fmt.Errorf("failed to marshal tool trace: %w", err)
	}

	// Write NDJSON line
	_, err = fmt.Fprintf(w, "%s\n", jsonBytes)
	return err
}

// FormatJSON converts volley results and summary to JSON format
// This is a placeholder implementation for improvement #2
func FormatJSON(results interface{}, summary interface{}) (string, error) {
	output := struct {
		Results interface{} `json:"results"`
		Summary interface{} `json:"summary"`
	}{
		Results: results,
		Summary: summary,
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// FormatDiffOutput extracts and formats diffs from task results
// This is a placeholder implementation for improvement #2
func FormatDiffOutput(results interface{}) string {
	// TODO: Implement in improvement #2
	return "Diff output not yet implemented\n"
}
