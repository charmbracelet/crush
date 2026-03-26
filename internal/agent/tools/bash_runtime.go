package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
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

var ansiControlSequenceRE = regexp.MustCompile(`(?:\x1b\][^\x07\x1b]*(?:\x07|\x1b\\))|(?:\x1b\[[0-?]*[ -/]*[@-~])|(?:\x1b[@-_])`)

func sanitizeTerminalText(content string) string {
	if content == "" {
		return ""
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = ansiControlSequenceRE.ReplaceAllString(content, "")

	var sanitized strings.Builder
	line := make([]rune, 0, len(content))
	lineNeedsReset := false
	flushLine := func() {
		sanitized.WriteString(string(line))
		line = line[:0]
		lineNeedsReset = false
	}
	resetLineIfNeeded := func() {
		if lineNeedsReset {
			line = line[:0]
			lineNeedsReset = false
		}
	}
	writeRune := func(r rune) {
		resetLineIfNeeded()
		line = append(line, r)
	}
	writeString := func(value string) {
		for _, r := range value {
			writeRune(r)
		}
	}

	for _, r := range content {
		switch {
		case r == '\r':
			lineNeedsReset = true
		case r == '\n':
			flushLine()
			sanitized.WriteByte('\n')
		case r == '\b':
			resetLineIfNeeded()
			if len(line) > 0 {
				line = line[:len(line)-1]
			}
		case r == '\t':
			writeString("    ")
		case r < 0x20 || r == 0x7f:
			continue
		default:
			writeRune(r)
		}
	}
	flushLine()

	return strings.TrimSpace(sanitized.String())
}

func combinedOutputSnapshot(stdout, stderr string) string {
	stdout = sanitizeTerminalText(stdout)
	stderr = sanitizeTerminalText(stderr)
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
