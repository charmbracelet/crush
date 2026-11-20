package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/x/ansi"
)

const (
	JobListToolName = "job_list"
)

//go:embed job_list.md
var jobListDescription string

// maxJobListCommandWidth bounds how much of each command job_list shows.
const maxJobListCommandWidth = 80

type JobListParams struct{}

// JobListEntry describes one background job in a job_list result.
type JobListEntry struct {
	ShellID     string `json:"shell_id"`
	Command     string `json:"command"`
	Description string `json:"description"`
	Running     bool   `json:"running"`
	ExitCode    int    `json:"exit_code"`
}

type JobListResponseMetadata struct {
	Jobs []JobListEntry `json:"jobs"`
}

func NewJobListTool(jobs BackgroundJobs) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		JobListToolName,
		jobListDescription,
		func(ctx context.Context, _ JobListParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			shells := shell.GetBackgroundShellManager().Jobs(jobs.Scope(ctx))
			if len(shells) == 0 {
				return fantasy.NewTextResponse("No background jobs in this session."), nil
			}

			now := time.Now()
			var metadata JobListResponseMetadata
			var b strings.Builder
			for _, bgShell := range shells {
				entry := jobListEntry(bgShell)
				metadata.Jobs = append(metadata.Jobs, entry)
				fmt.Fprintf(&b, "%s  %s  %s\n", entry.ShellID, jobListStatus(bgShell, entry, now), jobListCommand(entry))
			}
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(strings.TrimRight(b.String(), "\n")), metadata), nil
		},
	)
}

func jobListEntry(bgShell *shell.BackgroundShell) JobListEntry {
	_, _, done, err := bgShell.GetOutput()
	return JobListEntry{
		ShellID:     bgShell.ID,
		Command:     bgShell.Command,
		Description: bgShell.Description,
		Running:     !done,
		ExitCode:    shell.ExitCode(err),
	}
}

func jobListStatus(bgShell *shell.BackgroundShell, entry JobListEntry, now time.Time) string {
	if entry.Running {
		return "running for " + now.Sub(bgShell.StartedAt).Round(time.Second).String()
	}
	took := bgShell.CompletedAt().Sub(bgShell.StartedAt).Round(time.Second)
	return fmt.Sprintf("exited %d after %s", entry.ExitCode, took)
}

func jobListCommand(entry JobListEntry) string {
	command, _, multiline := strings.Cut(entry.Command, "\n")
	if multiline {
		command += " …"
	}
	command = ansi.Truncate(command, maxJobListCommandWidth, "…")
	if entry.Description != "" {
		command += " (" + entry.Description + ")"
	}
	return command
}
