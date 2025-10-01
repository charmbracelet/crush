package output

import (
	"fmt"
	"strings"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/llm/tools"
)

// FormatToolTrace formats tool execution metadata for display
func FormatToolTrace(metadata *tools.ExecutionMetadata, verbosity config.VerbosityLevel) string {
	if verbosity == config.VerbosityQuiet || metadata == nil {
		return ""
	}

	var parts []string

	// Tool name
	parts = append(parts, metadata.ToolName)

	// Format based on tool type
	switch {
	case metadata.Operation == "read" && metadata.FilePath != "":
		sizeStr := formatBytes(metadata.ByteSize)
		parts = append(parts, fmt.Sprintf("%s (%d lines, %s)", metadata.FilePath, metadata.LineCount, sizeStr))

	case metadata.Operation == "write" || metadata.Operation == "created" || metadata.Operation == "modified":
		if metadata.FilePath != "" {
			parts = append(parts, fmt.Sprintf("%s (%s, %d lines)", metadata.FilePath, metadata.Operation, metadata.LineCount))
		}

	case metadata.Pattern != "" && metadata.MatchCount >= 0:
		// Glob or grep
		if metadata.MatchCount == 1 {
			parts = append(parts, fmt.Sprintf("%s → %d match", metadata.Pattern, metadata.MatchCount))
		} else {
			parts = append(parts, fmt.Sprintf("%s → %d matches", metadata.Pattern, metadata.MatchCount))
		}

	case metadata.Command != "":
		// Bash command
		cmdPreview := metadata.Command
		if len(cmdPreview) > 50 {
			cmdPreview = cmdPreview[:47] + "..."
		}
		exitInfo := ""
		if metadata.ExitCode != nil {
			exitInfo = fmt.Sprintf(" (exit %d)", *metadata.ExitCode)
		}
		parts = append(parts, fmt.Sprintf("%s%s", cmdPreview, exitInfo))
	}

	// Duration (always show in normal/verbose mode)
	if metadata.Duration > 0 {
		parts = append(parts, fmt.Sprintf("%.1fs", metadata.Duration.Seconds()))
	}

	// Join with " — "
	result := "[TOOL] " + strings.Join(parts, " — ")

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
