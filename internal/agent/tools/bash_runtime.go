package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/toolruntime"
)

const (
	defaultBashTimeoutSeconds = 120
	maxBashTimeoutSeconds     = 600
	bashStreamThrottle        = 150 * time.Millisecond
)

func effectiveBashTimeout(params BashParams) (int, []string) {
	notes := deprecationNotesForAutoBackground(params)
	if params.TimeoutSeconds != nil {
		if *params.TimeoutSeconds == 0 {
			return 0, notes
		}
		if *params.TimeoutSeconds > 0 {
			return min(*params.TimeoutSeconds, maxBashTimeoutSeconds), notes
		}
		return defaultBashTimeoutSeconds, notes
	}
	if params.AutoBackgroundAfter > 0 {
		return min(params.AutoBackgroundAfter, maxBashTimeoutSeconds), notes
	}
	return defaultBashTimeoutSeconds, notes
}

func deprecationNotesForAutoBackground(params BashParams) []string {
	if params.AutoBackgroundAfter == 0 {
		return nil
	}
	return []string{
		"`auto_background_after` is deprecated. Automatic backgrounding is disabled; this value is only interpreted as `timeout_seconds` when `timeout_seconds` is omitted.",
	}
}

func appendDeprecationNotes(metadata *BashResponseMetadata, notes []string) {
	if metadata == nil || len(notes) == 0 {
		return
	}
	metadata.DeprecationNotes = append(metadata.DeprecationNotes, notes...)
}

func publishBashRuntime(ctx context.Context, toolCallID string, status toolruntime.Status, snapshot string, meta map[string]any) {
	reportToolRuntime(ctx, toolCallID, BashToolName, status, snapshot, meta)
}

func publishShellRuntime(ctx context.Context, bgShell *shell.BackgroundShell, status toolruntime.Status, snapshot string) {
	if bgShell == nil || bgShell.SessionID == "" || bgShell.ToolCallID == "" || bgShell.ToolName == "" {
		return
	}
	toolruntime.Report(ctx, toolruntime.State{
		SessionID:    bgShell.SessionID,
		ToolCallID:   bgShell.ToolCallID,
		ToolName:     bgShell.ToolName,
		Status:       status,
		SnapshotText: snapshot,
		ClientMetadata: map[string]any{
			"shell_id":   bgShell.ID,
			"background": true,
		},
	})
}

func combinedOutputSnapshot(stdout, stderr string) string {
	stdout = strings.TrimSuffix(stdout, "\n")
	stderr = strings.TrimSuffix(stderr, "\n")
	switch {
	case stdout != "" && stderr != "":
		return stdout + "\n" + stderr
	case stdout != "":
		return stdout
	default:
		return stderr
	}
}

func finalShellOutput(stdout, stderr string, execErr error) string {
	return truncateOutput(formatOutput(stdout, stderr, execErr))
}

func buildBashResponseText(output string, workingDir string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return BashNoOutput
	}
	return fmt.Sprintf("%s\n\n<cwd>%s</cwd>", output, normalizeWorkingDir(workingDir))
}

func normalizeWorkingDir(path string) string {
	return filepath.ToSlash(path)
}

func jobStatusText(done bool) string {
	if done {
		return "completed"
	}
	return "running"
}

func formatJobOutput(stdout, stderr string, execErr error, done bool) string {
	if done {
		return finalShellOutput(stdout, stderr, execErr)
	}
	return truncateOutput(combinedOutputSnapshot(stdout, stderr))
}

