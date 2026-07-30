package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/scheduler"
)

const (
	// CronCreateToolName is the name of the scheduled-task creation tool,
	// matching the tool name in Claude Code and Codex.
	CronCreateToolName = "CronCreate"
	// CronListToolName is the name of the scheduled-task listing tool.
	CronListToolName = "CronList"
	// CronDeleteToolName is the name of the scheduled-task deletion tool.
	CronDeleteToolName = "CronDelete"
)

//go:embed croncreate.md
var cronCreateDescription string

//go:embed cronlist.md
var cronListDescription string

//go:embed crondelete.md
var cronDeleteDescription string

type CronCreateParams struct {
	Cron      string `json:"cron" description:"Required. A 5-field local-time cron expression: minute hour day-of-month month day-of-week."`
	Prompt    string `json:"prompt" description:"Required. The prompt to run when the task fires."`
	Recurring *bool  `json:"recurring,omitempty" description:"Whether the task reschedules itself after it fires. Defaults to true; false makes a one-shot task that fires once and deletes itself."`
	Durable   bool   `json:"durable,omitempty" description:"Whether to persist the task to disk so it survives restarts. Defaults to false (session-only, in-memory)."`
}

type CronDeleteParams struct {
	ID string `json:"id" description:"Required. The scheduled task ID to cancel."`
}

// CronTasksResponseMetadata carries the affected task(s) back to the UI.
type CronTasksResponseMetadata struct {
	Tasks []scheduler.Task `json:"tasks"`
}

func cronSessionID(ctx context.Context) (string, error) {
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return "", fmt.Errorf("session ID is required for managing scheduled tasks")
	}
	// Sub-agent (agent tool) sessions use the "messageID$$toolCallID"
	// format. Scheduled tasks can only be managed from the root session,
	// matching the root-thread restriction in the Codex prototype.
	if strings.Contains(sessionID, "$$") {
		return "", fmt.Errorf("scheduled tasks can only be managed from the root session, not from a sub-agent")
	}
	return sessionID, nil
}

func formatTaskTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05 MST")
}

// NewCronCreateTool creates the CronCreate tool.
func NewCronCreateTool(store *scheduler.Store) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CronCreateToolName,
		cronCreateDescription,
		func(ctx context.Context, params CronCreateParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID, err := cronSessionID(ctx)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			recurring := true
			if params.Recurring != nil {
				recurring = *params.Recurring
			}

			task, err := store.Create(sessionID, params.Cron, params.Prompt, recurring, params.Durable)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			kind := "recurring"
			if !task.Recurring {
				kind = "one-shot"
			}
			storage := "session-only (in-memory)"
			if task.Durable {
				storage = "durable (persisted to disk)"
			}

			response := fmt.Sprintf(
				"Scheduled task created.\n\nID: %s\nSchedule: %s (local time)\nNext run: %s\nType: %s, %s\nPrompt: %s",
				task.ID, task.Cron, formatTaskTime(task.NextRunAt), kind, storage, task.Prompt,
			)
			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(response),
				CronTasksResponseMetadata{Tasks: []scheduler.Task{task}},
			), nil
		},
	)
}

// NewCronListTool creates the CronList tool.
func NewCronListTool(store *scheduler.Store) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CronListToolName,
		cronListDescription,
		func(ctx context.Context, params struct{}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID, err := cronSessionID(ctx)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			tasks := store.List(sessionID)
			if len(tasks) == 0 {
				return fantasy.NewTextResponse("No scheduled tasks in this session."), nil
			}

			response := fmt.Sprintf("%d scheduled task(s) in this session:\n", len(tasks))
			for _, task := range tasks {
				kind := "recurring"
				if !task.Recurring {
					kind = "one-shot"
				}
				durable := ""
				if task.Durable {
					durable = ", durable"
				}
				response += fmt.Sprintf(
					"\n- ID: %s\n  Schedule: %s (local time)\n  Next run: %s\n  Type: %s%s, %d run(s)\n  Prompt: %s",
					task.ID, task.Cron, formatTaskTime(task.NextRunAt), kind, durable, task.RunCount, task.Prompt,
				)
				if task.LastError != "" {
					response += fmt.Sprintf("\n  Last error: %s", task.LastError)
				}
			}
			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(response),
				CronTasksResponseMetadata{Tasks: tasks},
			), nil
		},
	)
}

// NewCronDeleteTool creates the CronDelete tool.
func NewCronDeleteTool(store *scheduler.Store) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CronDeleteToolName,
		cronDeleteDescription,
		func(ctx context.Context, params CronDeleteParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID, err := cronSessionID(ctx)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if params.ID == "" {
				return fantasy.ToolResponse{}, errors.New("id is required")
			}

			task, err := store.Delete(sessionID, params.ID)
			if errors.Is(err, scheduler.ErrTaskNotFound) {
				return fantasy.ToolResponse{}, fmt.Errorf("no scheduled task with ID %q in this session", params.ID)
			}
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			response := fmt.Sprintf("Scheduled task %s deleted.\n\nSchedule: %s\nPrompt: %s", task.ID, task.Cron, task.Prompt)
			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(response),
				CronTasksResponseMetadata{Tasks: []scheduler.Task{task}},
			), nil
		},
	)
}
