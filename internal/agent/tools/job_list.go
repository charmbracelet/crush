package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/shell"
)

const (
	JobListToolName = "job_list"
)

//go:embed job_list.md
var jobListDescription string

type JobListParams struct {
	RunningOnly bool `json:"running_only" description:"Only list jobs that are still running. Defaults to false, which lists finished jobs too."`
}

type JobListEntry struct {
	ShellID     string `json:"shell_id"`
	Command     string `json:"command"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
	ExitCode    int    `json:"exit_code"`
	ElapsedMS   int64  `json:"elapsed_ms"`
	LogPath     string `json:"log_path,omitempty"`
}

type JobListResponseMetadata struct {
	Jobs []JobListEntry `json:"jobs"`
}

func NewJobListTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		JobListToolName,
		jobListDescription,
		func(ctx context.Context, params JobListParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)

			var (
				entries []JobListEntry
				lines   []string
			)
			for _, info := range shell.GetBackgroundShellManager().Jobs() {
				// Jobs are tracked process-wide, so keep each conversation to
				// the ones it started.
				if info.SessionID != sessionID {
					continue
				}
				if params.RunningOnly && info.Done {
					continue
				}

				elapsed := time.Since(info.StartedAt)
				status := fmt.Sprintf("running for %s", elapsed.Round(time.Second))
				if info.Done {
					status = "completed"
					if info.ExitCode != 0 {
						status = fmt.Sprintf("failed (exit %d)", info.ExitCode)
					}
				}

				label := info.Description
				if label == "" {
					label = firstLine(info.Command)
				}

				entries = append(entries, JobListEntry{
					ShellID:     info.ID,
					Command:     info.Command,
					Description: info.Description,
					Done:        info.Done,
					ExitCode:    info.ExitCode,
					ElapsedMS:   elapsed.Milliseconds(),
					LogPath:     info.LogPath,
				})
				lines = append(lines, fmt.Sprintf("%s  %s  %s", info.ID, status, label))
			}

			if len(entries) == 0 {
				return fantasy.WithResponseMetadata(
					fantasy.NewTextResponse("No background jobs."),
					JobListResponseMetadata{},
				), nil
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(strings.Join(lines, "\n")),
				JobListResponseMetadata{Jobs: entries},
			), nil
		},
	)
}

// firstLine trims a command down to something that fits on a list row.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + " …"
	}
	return strings.TrimSpace(s)
}
