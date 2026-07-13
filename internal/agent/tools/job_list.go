package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/shell"
)

const (
	JobListToolName = "job_list"
)

//go:embed job_list.md
var jobListDescription string

type JobListParams struct{}

type JobListEntry struct {
	ShellID          string `json:"shell_id"`
	Command          string `json:"command"`
	Description      string `json:"description"`
	Status           string `json:"status"`
	WorkingDirectory string `json:"working_directory"`
}

type JobListResponseMetadata struct {
	Jobs []JobListEntry `json:"jobs"`
}

func NewJobListTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		JobListToolName,
		jobListDescription,
		func(ctx context.Context, params JobListParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			bgManager := shell.GetBackgroundShellManager()
			ids := bgManager.List()
			if len(ids) == 0 {
				return fantasy.WithResponseMetadata(
					fantasy.NewTextResponse("No background shells."),
					JobListResponseMetadata{},
				), nil
			}

			jobs := make([]JobListEntry, 0, len(ids))
			var lines []string
			lines = append(lines, "Background shells:")
			for _, id := range ids {
				bgShell, ok := bgManager.Get(id)
				if !ok {
					continue
				}
				status := "running"
				if bgShell.IsDone() {
					status = "completed"
				}
				entry := JobListEntry{
					ShellID:          bgShell.ID,
					Command:          bgShell.Command,
					Description:      bgShell.Description,
					Status:           status,
					WorkingDirectory: bgShell.WorkingDir,
				}
				jobs = append(jobs, entry)
				lines = append(lines, fmt.Sprintf("- %s [%s] %s", entry.ShellID, entry.Status, entry.Command))
				if entry.Description != "" {
					lines = append(lines, "  description: "+entry.Description)
				}
				if entry.WorkingDirectory != "" {
					lines = append(lines, "  cwd: "+entry.WorkingDirectory)
				}
			}

			if len(jobs) == 0 {
				return fantasy.WithResponseMetadata(
					fantasy.NewTextResponse("No background shells."),
					JobListResponseMetadata{},
				), nil
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(strings.Join(lines, "\n")),
				JobListResponseMetadata{Jobs: jobs},
			), nil
		},
	)
}
