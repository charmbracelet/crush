package narrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Context represents the context for generating explanations
type Context struct {
	Action      string            // Type of action (file_changed, command_executed, etc.)
	Files       []string          // Affected files
	DiffSummary string            // Summary of changes
	Timestamp   time.Time         // When the event occurred
	Metadata    map[string]string // Additional context
}

// Explainer builds context from various sources
type Explainer struct {
	workspaceRoot string
	gitEnabled    bool
}

// NewExplainer creates a new explainer
func NewExplainer(workspaceRoot string) *Explainer {
	return &Explainer{
		workspaceRoot: workspaceRoot,
		gitEnabled:    true,
	}
}

// BuildContextFromFileChange creates context from file system events
func (e *Explainer) BuildContextFromFileChange(filePath string, eventType string) *Context {
	relPath, err := filepath.Rel(e.workspaceRoot, filePath)
	if err != nil {
		relPath = filePath
	}

	context := &Context{
		Action:    eventType,
		Files:     []string{relPath},
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"file_type": filepath.Ext(relPath),
			"size_str":  formatFileSize(filePath),
		},
	}

	// Try to get git diff if available
	if e.gitEnabled {
		if diff := e.getGitDiffSummary(filePath); diff != "" {
			context.DiffSummary = diff
		}
	}

	return context
}

// BuildContextFromCommand creates context from command execution
func (e *Explainer) BuildContextFromCommand(command []string, output string, exitCode int) *Context {
	context := &Context{
		Action:    "command_executed",
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"command":   strings.Join(command, " "),
			"exit_code": fmt.Sprintf("%d", exitCode),
		},
	}

	// Extract file references from command
	for _, arg := range command {
		if strings.Contains(arg, ".") || strings.Contains(arg, "/") {
			context.Files = append(context.Files, arg)
		}
	}

	// Include output summary if available
	if len(output) > 0 {
		context.DiffSummary = e.summarizeOutput(output)
	}

	return context
}

// BuildContextFromPlanStep creates context from plan execution
func (e *Explainer) BuildContextFromPlanStep(stepID int, description string, status string) *Context {
	context := &Context{
		Action:    "plan_step_executed",
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"step_id":     fmt.Sprintf("%d", stepID),
			"status":      status,
			"description": description,
		},
	}

	return context
}

// getGitDiffSummary gets a summary of git changes for a file
func (e *Explainer) getGitDiffSummary(filePath string) string {
	// Try to get git diff for the specific file
	cmd := exec.Command("git", "diff", "--no-index", filePath)
	cmd.Dir = e.workspaceRoot

	output, err := cmd.Output()
	if err != nil {
		// Try getting diff for the directory
		dir := filepath.Dir(filePath)
		if dir != "." {
			cmd = exec.Command("git", "diff", "--no-index", dir)
			cmd.Dir = e.workspaceRoot
			output, err = cmd.Output()
			if err != nil {
				return ""
			}
		} else {
			return ""
		}
	}

	// Parse and summarize the diff
	lines := strings.Split(string(output), "\n")
	var additions, deletions int
	var changedFiles []string

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				changedFiles = append(changedFiles, parts[3])
			}
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}

	if len(changedFiles) > 0 || additions > 0 || deletions > 0 {
		return fmt.Sprintf("Modified %s (+%d -%d)", strings.Join(changedFiles, ", "), additions, deletions)
	}

	return ""
}

// summarizeOutput creates a brief summary of command output
func (e *Explainer) summarizeOutput(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return ""
	}

	// Take first and last lines for context
	summary := lines[0]
	if len(lines) > 1 {
		summary += "..."
		summary += lines[len(lines)-1]
	}

	// Truncate if too long
	if len(summary) > 100 {
		summary = summary[:97] + "..."
	}

	return summary
}

// formatFileSize returns a human-readable file size
func formatFileSize(filePath string) string {
	info, err := os.Stat(filePath)
	if err != nil {
		return "unknown"
	}

	size := info.Size()
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	} else {
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	}
}

// BuildPrompt creates a prompt for the AI narrator
func (e *Explainer) BuildPrompt(context *Context) string {
	prompt := fmt.Sprintf(`You are an AI assistant helping explain what just happened in a coding session.

Context:
- Action: %s
- Time: %s
- Files: %s`,
		context.Action,
		context.Timestamp.Format("2006-01-02 15:04:05"),
		strings.Join(context.Files, ", "),
	)

	if context.DiffSummary != "" {
		prompt += fmt.Sprintf("\n- Changes: %s", context.DiffSummary)
	}

	for key, value := range context.Metadata {
		prompt += fmt.Sprintf("\n- %s: %s", key, value)
	}

	prompt += `

Please provide a brief, clear explanation (2-3 sentences) of what happened and why it matters for the development workflow. Focus on the practical impact and next steps.

Explanation:`

	return prompt
}

// IsGitRepo checks if the workspace is a git repository
func (e *Explainer) IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = e.workspaceRoot
	err := cmd.Run()
	return err == nil
}
