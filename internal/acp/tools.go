package acp

import (
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/coder/acp-go-sdk"
	"log/slog"
	"strings"
)

type ToolCall message.ToolCall

func (t ToolCall) Kind() acp.ToolKind {
	switch t.Name {
	case tools.BashToolName, tools.BashNoOutput:
		return acp.ToolKindExecute
	case tools.DownloadToolName, tools.FetchToolName:
		return acp.ToolKindFetch
	case tools.GlobToolName, tools.LSToolName, tools.GrepToolName:
		return acp.ToolKindSearch
	case tools.EditToolName, tools.MultiEditToolName, tools.WriteToolName:
		return acp.ToolKindEdit
	case tools.ViewToolName:
		return acp.ToolKindRead
	}

	return acp.ToolKindOther
}

func (t ToolCall) StartToolCall() *acp.SessionUpdateToolCall {
	result := &acp.SessionUpdateToolCall{
		ToolCallId: acp.ToolCallId(t.ID),
		Kind:       t.Kind(),
		Title:      t.Name,
		Status:     acp.ToolCallStatusPending,
	}

	return result
}

func (t ToolCall) UpdateToolCall() *acp.SessionUpdateToolCallUpdate {
	input := map[string]any{}
	if err := json.Unmarshal([]byte(t.Input), &input); err != nil {
		slog.Warn("Error decoding input data", "err", err)
	}

	result := &acp.SessionUpdateToolCallUpdate{
		ToolCallId: acp.ToolCallId(t.ID),
		Status:     acp.Ptr(acp.ToolCallStatusInProgress),
	}

	filePath, _ := input["file_path"].(string)
	offset, _ := input["offset"].(int)
	limit, _ := input["limit"].(int)
	oldText, _ := input["old_string"].(string)
	newText, _ := input["new_string"].(string)
	content, _ := input["content"].(string)

	var locations []acp.ToolCallLocation
	if filePath != "" {
		locations = append(locations, acp.ToolCallLocation{
			Path: filePath,
			Line: acp.Ptr(offset),
		})
	}

	switch t.Name {
	case tools.EditToolName:
		{
			var title strings.Builder
			title.WriteString("Edit ")
			title.WriteString(filePath)
			result.Title = acp.Ptr(title.String())
			result.Content = []acp.ToolCallContent{acp.ToolDiffContent(filePath, newText, oldText)}
		}
	case tools.WriteToolName:
		{
			var title strings.Builder
			title.WriteString("Edit ")
			title.WriteString(filePath)
			result.Title = acp.Ptr(title.String())

			if filePath != "" {
				result.Content = []acp.ToolCallContent{acp.ToolDiffContent(filePath, newText, "")}
			} else {
				result.Content = []acp.ToolCallContent{
					acp.ToolContent(acp.ContentBlock{
						Text: acp.Ptr(acp.ContentBlockText{Text: content}),
					}),
				}
			}
		}
	case tools.ViewToolName:
		{
			var title strings.Builder
			title.WriteString("Read ")
			title.WriteString(filePath)
			switch {
			case limit > 0:
				fmt.Fprintf(&title, " (%d - %d)", offset, offset+limit)
			case offset > 0:
				fmt.Fprintf(&title, " (from line %d)", offset)
			default:
				title.WriteString(" File")
			}

			result.Title = acp.Ptr(title.String())
			result.Locations = locations
		}
	}

	return result
}
