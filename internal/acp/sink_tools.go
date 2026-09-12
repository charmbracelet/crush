package acp

import (
	"encoding/json"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/coder/acp-go-sdk"
)

func (s *Sink) translateToolCall(tc message.ToolCall) *acp.SessionUpdate {
	if !tc.Finished {
		opts := []acp.ToolCallStartOpt{
			acp.WithStartStatus(acp.ToolCallStatusPending),
			acp.WithStartKind(toolKind(tc.Name)),
		}

		// Parse input to extract path, title, and raw input.
		title := tc.Name
		if input := parseToolInput(tc.Input); input != nil {
			if input.Path != "" {
				opts = append(opts, acp.WithStartLocations([]acp.ToolCallLocation{{Path: input.Path}}))
			}
			if input.Title != "" {
				title = input.Title
			}
			opts = append(opts, acp.WithStartRawInput(input.Raw))
		}

		update := acp.StartToolCall(acp.ToolCallId(tc.ID), title, opts...)
		return &update
	}

	// Tool finished streaming - update with title and input now available.
	opts := []acp.ToolCallUpdateOpt{
		acp.WithUpdateStatus(acp.ToolCallStatusInProgress),
	}
	if input := parseToolInput(tc.Input); input != nil {
		if input.Title != "" {
			opts = append(opts, acp.WithUpdateTitle(input.Title))
		}
		if input.Path != "" {
			opts = append(opts, acp.WithUpdateLocations([]acp.ToolCallLocation{{Path: input.Path}}))
		}
		opts = append(opts, acp.WithUpdateRawInput(input.Raw))
	}

	update := acp.UpdateToolCall(acp.ToolCallId(tc.ID), opts...)
	return &update
}

// toolInput holds parsed tool call input.
type toolInput struct {
	Path  string
	Title string
	Raw   map[string]any
}

// parseToolInput extracts path and raw input from JSON tool input.
func parseToolInput(input string) *toolInput {
	if input == "" {
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil
	}

	ti := &toolInput{Raw: raw}

	// Extract path from common field names.
	if path, ok := raw["file_path"].(string); ok {
		ti.Path = path
	} else if path, ok := raw["path"].(string); ok {
		ti.Path = path
	}

	// Extract title/description for display.
	if desc, ok := raw["description"].(string); ok {
		ti.Title = desc
	}

	return ti
}

// toolKind maps Crush tool names to ACP tool kinds.
func toolKind(name string) acp.ToolKind {
	switch name {
	case "view", "ls", "job_output", "lsp_diagnostics":
		return acp.ToolKindRead
	case "edit", "multiedit", "write":
		return acp.ToolKindEdit
	case "bash", "job_kill":
		return acp.ToolKindExecute
	case "grep", "glob", "lsp_references", "sourcegraph", "web_search":
		return acp.ToolKindSearch
	case "fetch", "agentic_fetch", "web_fetch", "download":
		return acp.ToolKindFetch
	default:
		return acp.ToolKindOther
	}
}

// diffMetadata holds fields common to edit tool response metadata.
type diffMetadata struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
}

func (s *Sink) translateToolResult(tr message.ToolResult) *acp.SessionUpdate {
	status := acp.ToolCallStatusCompleted
	if tr.IsError {
		status = acp.ToolCallStatusFailed
	}

	// For edit tools with metadata, emit diff content.
	content := []acp.ToolCallContent{acp.ToolContent(acp.TextBlock(tr.Content))}
	var locations []acp.ToolCallLocation

	if !tr.IsError && tr.Metadata != "" {
		switch tr.Name {
		case "edit", "multiedit", "write":
			var meta diffMetadata
			if err := json.Unmarshal([]byte(tr.Metadata), &meta); err == nil && meta.FilePath != "" {
				content = []acp.ToolCallContent{
					acp.ToolDiffContent(meta.FilePath, meta.NewContent, meta.OldContent),
				}
			}
		case "view":
			var meta struct {
				FilePath string `json:"file_path"`
			}
			if err := json.Unmarshal([]byte(tr.Metadata), &meta); err == nil && meta.FilePath != "" {
				locations = []acp.ToolCallLocation{{Path: meta.FilePath}}
			}
		case "ls":
			var meta struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(tr.Metadata), &meta); err == nil && meta.Path != "" {
				locations = []acp.ToolCallLocation{{Path: meta.Path}}
			}
		}
	}

	opts := []acp.ToolCallUpdateOpt{
		acp.WithUpdateStatus(status),
		acp.WithUpdateContent(content),
	}
	if len(locations) > 0 {
		opts = append(opts, acp.WithUpdateLocations(locations))
	}

	update := acp.UpdateToolCall(acp.ToolCallId(tr.ToolCallID), opts...)
	return &update
}
