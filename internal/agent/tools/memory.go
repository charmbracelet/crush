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
	Action   string   `json:"action" description:"Action to perform: store, get, delete, search, or list"`
	Key      string   `json:"key,omitempty" description:"Memory key for store/get/delete actions"`
	Value    string   `json:"value,omitempty" description:"Memory value for store action"`
	Scope    string   `json:"scope,omitempty" description:"Optional scope for store/search/list actions, such as session or project"`
	Category string   `json:"category,omitempty" description:"Optional memory category for store/search/list actions"`
	Type     string   `json:"type,omitempty" description:"Optional memory type for store/search/list actions"`
	Tags     []string `json:"tags,omitempty" description:"Optional tags for store/search/list actions"`
	Query    string   `json:"query,omitempty" description:"Search query for search action"`
	Limit    int      `json:"limit,omitempty" description:"Maximum number of items to return"`
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

				if err := memorySvc.Store(ctx, memory.StoreParams{
					Key:      params.Key,
					Value:    params.Value,
					Scope:    params.Scope,
					Category: params.Category,
					Type:     params.Type,
					Tags:     params.Tags,
				}); err != nil {
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
				entries, err := memorySvc.Search(ctx, memory.SearchParams{
					Query:    params.Query,
					Scope:    params.Scope,
					Category: params.Category,
					Type:     params.Type,
					Tags:     params.Tags,
					Limit:    params.Limit,
				})
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return fantasy.NewTextResponse(formatMemoryEntries(entries)), nil
			case "list":
				entries, err := memorySvc.List(ctx, memory.ListParams{
					Scope:    params.Scope,
					Category: params.Category,
					Type:     params.Type,
					Tags:     params.Tags,
					Limit:    params.Limit,
				})
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
	lines := []string{fmt.Sprintf("key=%s", entry.Key)}
	lines = append(lines, formatMemoryMetadataLines(entry)...)
	lines = append(lines,
		fmt.Sprintf("updated_at=%s", time.Unix(0, entry.UpdatedAt).Format(time.RFC3339)),
		fmt.Sprintf("value=%s", strings.TrimSpace(entry.Value)),
	)
	return strings.Join(lines, "\n")
}

func formatMemoryEntries(entries []memory.Entry) string {
	if len(entries) == 0 {
		return "No long-term memory entries found."
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Found %d long-term memory entries:\n\n", len(entries))
	for index, entry := range entries {
		value := strings.TrimSpace(entry.Value)
		if len([]rune(value)) > 160 {
			value = string([]rune(value)[:160]) + "…"
		}
		line := []string{fmt.Sprintf("%d. key=%s", index+1, entry.Key)}
		line = append(line, formatMemoryMetadataLines(entry)...)
		line = append(line, fmt.Sprintf("updated_at=%s", time.Unix(0, entry.UpdatedAt).Format(time.RFC3339)))
		fmt.Fprintf(&builder, "%s\n   %s\n", strings.Join(line, " "), value)
	}
	return strings.TrimSpace(builder.String())
}

func formatMemoryMetadataLines(entry memory.Entry) []string {
	parts := make([]string, 0, 4)
	if scope := strings.TrimSpace(entry.Scope); scope != "" {
		parts = append(parts, fmt.Sprintf("scope=%s", scope))
	}
	if category := strings.TrimSpace(entry.Category); category != "" {
		parts = append(parts, fmt.Sprintf("category=%s", category))
	}
	if kind := strings.TrimSpace(entry.Type); kind != "" {
		parts = append(parts, fmt.Sprintf("type=%s", kind))
	}
	if len(entry.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags=%s", strings.Join(entry.Tags, ", ")))
	}
	return parts
}
