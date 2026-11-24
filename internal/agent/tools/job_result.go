package tools

import (
	"strings"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/x/ansi"
)

// jobLabelWidth bounds the one-line name a job goes by in a report.
const jobLabelWidth = 60

// JobResult is a background job's output and, once it has finished, how
// it ended, read in one snapshot.
type JobResult struct {
	// Text is the output, truncated with the rest spilled to a file, and
	// for a finished job any exit code or abort note.
	Text   string
	Done   bool
	Failed bool
}

// ReadJobResult reads job's output as job_output and job reports show it.
func ReadJobResult(job *shell.BackgroundShell, spillDir string) JobResult {
	stdout, stderr, done, err := job.GetOutput()

	var parts []string
	if stdout != "" {
		parts = append(parts, stdout)
	}
	if stderr != "" {
		parts = append(parts, stderr)
	}
	msg := ""
	if done {
		msg = shell.Describe(err)
	}
	if msg != "" {
		parts = append(parts, msg)
	}

	text := TruncateOutput(strings.Join(parts, "\n"), spillDir)
	if text == "" {
		text = BashNoOutput
	}
	return JobResult{Text: text, Done: done, Failed: msg != ""}
}

// JobLabel names a job by its description, or else its command, on one
// short line.
func JobLabel(job *shell.BackgroundShell) string {
	label := job.Description
	if label == "" {
		label = job.Command
	}
	return ansi.Truncate(strings.Join(strings.Fields(label), " "), jobLabelWidth, "…")
}
