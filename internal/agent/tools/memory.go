package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/permission"
)

//go:embed memory.md
var longTermMemoryDescription []byte

const LongTermMemoryToolName = "long_term_memory"

type LongTermMemoryParams struct {
	Action string `json:"action" description:"Action to perform: store, get, delete, search, or list"`
	Key    string `json:"key,omitempty" description:"Memory key for store/get/delete actions"`
	Value  string `json:"value,omitempty" description:"Memory value for store action"`
	Query  string `json:"query,omitempty" description:"Search query for search action"`
	Limit  int    `json:"limit,omitempty" description:"Maximum number of items to return"`
}

func NewLongTermMemoryTool(memorySvc memory.Service, permissions permission.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LongTermMemoryToolName,
		string(longTermMemoryDescription),
		func(ctx context.Context, params LongTermMemoryParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if memorySvc == nil {
				return fantasy.NewTextErrorResponse("long-term memory service is unavailable"), nil
			}

			action := strings.ToLower(strings.TrimSpace(params.Action))
			switch action {
			case "store":
				sessionID := GetSessionFromContext(ctx)
				if sessionID == "" {
					return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for long_term_memory store")
				}

				permissionResponse, err := RequestPermission(ctx, permissions,
					permission.CreatePermissionRequest{
						SessionID:   sessionID,
						Path:        workingDir,
						ToolCallID:  call.ID,
						ToolName:    LongTermMemoryToolName,
						Action:      "write",
						Description: fmt.Sprintf("Store long-term memory key %q", strings.TrimSpace(params.Key)),
						Params:      params,
					},
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if permissionResponse != nil {
					return *permissionResponse, nil
				}

				if err := memorySvc.Store(ctx, params.Key, params.Value); err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return fantasy.NewTextResponse(fmt.Sprintf("Stored long-term memory key %q.", strings.TrimSpace(params.Key))), nil
			case "get":
				entry, err := memorySvc.Get(ctx, params.Key)
				if err != nil {
					if err == memory.ErrNotFound {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("No long-term memory found for key %q.", strings.TrimSpace(params.Key))), nil
					}
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return fantasy.NewTextResponse(formatMemoryEntry(entry)), nil
			case "delete":
				sessionID := GetSessionFromContext(ctx)
				if sessionID == "" {
					return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for long_term_memory delete")
				}

				permissionResponse, err := RequestPermission(ctx, permissions,
					permission.CreatePermissionRequest{
						SessionID:   sessionID,
						Path:        workingDir,
						ToolCallID:  call.ID,
						ToolName:    LongTermMemoryToolName,
						Action:      "delete",
						Description: fmt.Sprintf("Delete long-term memory key %q", strings.TrimSpace(params.Key)),
						Params:      params,
					},
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if permissionResponse != nil {
					return *permissionResponse, nil
				}

				if err := memorySvc.Delete(ctx, params.Key); err != nil {
					if err == memory.ErrNotFound {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("No long-term memory found for key %q.", strings.TrimSpace(params.Key))), nil
					}
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return fantasy.NewTextResponse(fmt.Sprintf("Deleted long-term memory key %q.", strings.TrimSpace(params.Key))), nil
			case "search":
				entries, err := memorySvc.Search(ctx, params.Query, params.Limit)
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return fantasy.NewTextResponse(formatMemoryEntries(entries)), nil
			case "list":
				entries, err := memorySvc.List(ctx, params.Limit)
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return fantasy.NewTextResponse(formatMemoryEntries(entries)), nil
			default:
				return fantasy.NewTextErrorResponse("action must be one of: store, get, delete, search, list"), nil
			}
		},
	)
}

func formatMemoryEntry(entry memory.Entry) string {
	timestamp := time.Unix(0, entry.UpdatedAt).Format(time.RFC3339)
	return fmt.Sprintf("key=%s\nupdated_at=%s\nvalue=%s", entry.Key, timestamp, strings.TrimSpace(entry.Value))
}

func formatMemoryEntries(entries []memory.Entry) string {
	if len(entries) == 0 {
		return "No long-term memory entries found."
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Found %d long-term memory entries:\n\n", len(entries))
	for index, entry := range entries {
		timestamp := time.Unix(0, entry.UpdatedAt).Format(time.RFC3339)
		value := strings.TrimSpace(entry.Value)
		if len([]rune(value)) > 160 {
			value = string([]rune(value)[:160]) + "…"
		}
		fmt.Fprintf(&builder, "%d. key=%s updated_at=%s\n   %s\n", index+1, entry.Key, timestamp, value)
	}
	return strings.TrimSpace(builder.String())
}
