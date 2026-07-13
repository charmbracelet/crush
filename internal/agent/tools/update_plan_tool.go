package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"charm.land/fantasy"
)

const UpdatePlanToolName = "update_plan"

// PlanAction represents the operation to perform on a plan entry.
type PlanAction string

const (
	PlanActionAdd     PlanAction = "add"
	PlanActionUpdate  PlanAction = "update"
	PlanActionRemove  PlanAction = "remove"
	PlanActionClear   PlanAction = "clear"
	PlanActionExecute PlanAction = "execute"
)

type UpdatePlanParams struct {
	Entries []PlanEntryParam `json:"entries" description:"The plan entries to operate on. For 'clear', this should be empty."`
	Action  PlanAction       `json:"action" description:"Action to perform: add (append entries), update (modify existing), remove (delete by content match), clear (remove all), execute (set to in_progress). Default: update."`
}

type PlanEntryParam struct {
	Content  string `json:"content" description:"The plan task description (imperative form, e.g., 'Run tests')"`
	Status   string `json:"status" description:"Task status: pending, in_progress, completed"`
	Priority string `json:"priority" description:"Task priority: high, medium, low. Default: medium"`
}

// PlanEntry is a single task in the agent's execution plan.
// Kept here to avoid an import cycle with the agent package.
type PlanEntry struct {
	Content  string `json:"content"`
	Priority int    `json:"priority"`
	Status   int    `json:"status"`
}

// PlanObserver is called when the plan state changes.
type PlanObserver func(sessionID string, entries []PlanEntry)

var (
	planMu       sync.Mutex
	planState    = make(map[string][]PlanEntry)
	planObserver PlanObserver
)

// SetPlanObserver sets the callback for plan changes.
func SetPlanObserver(fn PlanObserver) {
	planMu.Lock()
	defer planMu.Unlock()
	planObserver = fn
}

func getPlanEntries(sessionID string) []PlanEntry {
	planMu.Lock()
	defer planMu.Unlock()
	entries, ok := planState[sessionID]
	if !ok {
		return nil
	}
	result := make([]PlanEntry, len(entries))
	copy(result, entries)
	return result
}

func setPlanEntries(sessionID string, entries []PlanEntry) {
	planMu.Lock()
	planState[sessionID] = entries
	obs := planObserver
	planMu.Unlock()
	if obs != nil {
		obs(sessionID, entries)
	}
}

func priorityFromStr(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high":
		return 0
	case "low":
		return 2
	default:
		return 1
	}
}

func statusFromStr(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "in_progress", "in-progress", "inprogress":
		return 1
	case "completed":
		return 2
	default:
		return 0
	}
}

func statusToStr(s int) string {
	switch s {
	case 1:
		return "in_progress"
	case 2:
		return "completed"
	default:
		return "pending"
	}
}

func NewUpdatePlanTool() fantasy.AgentTool {
	return fantasy.NewAgentTool(
		UpdatePlanToolName,
		"Use this tool to create and manage a structured plan for complex tasks. "+
			"This helps track progress, organize work, and demonstrate thoroughness. "+
			"\n\nActions:"+
			"\n- add: Append new entries to the plan"+
			"\n- update: Modify entries matching by content (or replace all if no match)"+
			"\n- remove: Delete entries matching by content"+
			"\n- clear: Remove all entries"+
			"\n- execute: Mark entries as in_progress"+
			"\n\nEach entry has: content (required), status (pending/in_progress/completed), priority (high/medium/low).",
		func(ctx context.Context, params UpdatePlanParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("no session ID in context"), nil
			}

			action := params.Action
			if action == "" {
				action = PlanActionUpdate
			}

			entries := getPlanEntries(sessionID)

			switch action {
			case PlanActionClear:
				setPlanEntries(sessionID, nil)
				return fantasy.NewTextResponse("Plan cleared."), nil

			case PlanActionAdd:
				for _, e := range params.Entries {
					if strings.TrimSpace(e.Content) == "" {
						continue
					}
					entries = append(entries, PlanEntry{
						Content:  e.Content,
						Status:   statusFromStr(e.Status),
						Priority: priorityFromStr(e.Priority),
					})
				}
				setPlanEntries(sessionID, entries)
				return fantasy.NewTextResponse(fmt.Sprintf("Added %d entries to plan.", len(params.Entries))), nil

			case PlanActionRemove:
				removeContents := make(map[string]bool)
				for _, e := range params.Entries {
					removeContents[e.Content] = true
				}
				filtered := make([]PlanEntry, 0, len(entries))
				for _, e := range entries {
					if !removeContents[e.Content] {
						filtered = append(filtered, e)
					}
				}
				removed := len(entries) - len(filtered)
				setPlanEntries(sessionID, filtered)
				return fantasy.NewTextResponse(fmt.Sprintf("Removed %d entries from plan.", removed)), nil

			case PlanActionExecute:
				for _, p := range params.Entries {
					for i := range entries {
						if entries[i].Content == p.Content {
							entries[i].Status = 1 // in_progress
						}
					}
				}
				setPlanEntries(sessionID, entries)
				return fantasy.NewTextResponse("Plan entries marked as in_progress."), nil

			case PlanActionUpdate:
				if len(params.Entries) == 0 {
					return fantasy.NewTextResponse(formatPlanStatus(entries)), nil
				}
				// If entries have content that matches existing entries, update them.
				// Otherwise, replace the entire plan.
				replace := true
				if len(entries) > 0 {
					existing := make(map[string]bool)
					for _, e := range entries {
						existing[e.Content] = true
					}
					for _, p := range params.Entries {
						if existing[p.Content] {
							replace = false
							break
						}
					}
				}
				if replace {
					newEntries := make([]PlanEntry, 0, len(params.Entries))
					for _, p := range params.Entries {
						if strings.TrimSpace(p.Content) == "" {
							continue
						}
						newEntries = append(newEntries, PlanEntry{
							Content:  p.Content,
							Status:   statusFromStr(p.Status),
							Priority: priorityFromStr(p.Priority),
						})
					}
					setPlanEntries(sessionID, newEntries)
					return fantasy.NewTextResponse(fmt.Sprintf("Plan replaced with %d entries.", len(newEntries))), nil
				}
				// Update matching entries
				for _, p := range params.Entries {
					for i := range entries {
						if entries[i].Content == p.Content {
							if p.Status != "" {
								entries[i].Status = statusFromStr(p.Status)
							}
							if p.Priority != "" {
								entries[i].Priority = priorityFromStr(p.Priority)
							}
						}
					}
				}
				setPlanEntries(sessionID, entries)
				return fantasy.NewTextResponse(fmt.Sprintf("Updated %d entries in plan.", len(params.Entries))), nil

			default:
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Unknown action: %s", action)), nil
			}
		},
	)
}

func formatPlanStatus(entries []PlanEntry) string {
	if len(entries) == 0 {
		return "Plan is empty."
	}
	pending, inProgress, completed := 0, 0, 0
	for _, e := range entries {
		switch e.Status {
		case 0:
			pending++
		case 1:
			inProgress++
		case 2:
			completed++
		}
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Plan (%d entries: %d pending, %d in_progress, %d completed):\n",
		len(entries), pending, inProgress, completed))
	for i, e := range entries {
		b.WriteString(fmt.Sprintf("  %d. [%s] %s (priority: %d)\n", i+1, statusToStr(e.Status), e.Content, e.Priority))
	}
	return b.String()
}

// ClearPlanState removes plan state for a session.
func ClearPlanState(sessionID string) {
	planMu.Lock()
	defer planMu.Unlock()
	delete(planState, sessionID)
}

// Ensure slog import is used.
var _ = slog.Info
