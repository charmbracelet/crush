package format

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

func ToolSummaryWithStatus(sty *styles.Styles, toolCall message.ToolCall, status ToolStatus, width int, workingDir string) string {
	name, params := toolSummaryParameters(toolCall, workingDir)
	return ToolHeader(sty, status, name, width, &ToolHeaderOptions{Compact: true}, params...)
}

func toolSummaryParameters(toolCall message.ToolCall, workingDir string) (string, []string) {
	switch toolCall.Name {
	case tools.ViewToolName:
		var params tools.ViewParams
		if json.Unmarshal([]byte(toolCall.Input), &params) == nil {
			out := []string{fsext.PrettyPath(params.FilePath)}
			if params.Limit != 0 {
				out = append(out, "limit", fmt.Sprintf("%d", params.Limit))
			}
			if params.Offset != 0 {
				out = append(out, "offset", fmt.Sprintf("%d", params.Offset))
			}
			return "View", out
		}
	case tools.GrepToolName:
		var params tools.GrepParams
		if json.Unmarshal([]byte(toolCall.Input), &params) == nil {
			out := []string{params.Pattern}
			if params.Path != "" {
				out = append(out, "path", params.Path)
			}
			if params.Include != "" {
				out = append(out, "include", params.Include)
			}
			if params.LiteralText {
				out = append(out, "literal", "true")
			}
			return "Grep", out
		}
	case tools.GlobToolName:
		var params tools.GlobParams
		if json.Unmarshal([]byte(toolCall.Input), &params) == nil {
			out := []string{params.Pattern}
			if params.Path != "" {
				out = append(out, "path", params.Path)
			}
			return "Glob", out
		}
	case tools.LSToolName:
		var params tools.LSParams
		if json.Unmarshal([]byte(toolCall.Input), &params) == nil {
			path := params.Path
			if path == "" {
				path = "."
			}
			return "List", []string{fsext.PrettyPath(path)}
		}
	case tools.WriteToolName:
		var params tools.WriteParams
		if json.Unmarshal([]byte(toolCall.Input), &params) == nil {
			return "Write", []string{fsext.PrettyPath(params.FilePath)}
		}
	case tools.EditToolName:
		var params tools.EditParams
		if json.Unmarshal([]byte(toolCall.Input), &params) == nil {
			return "Edit", []string{fsext.PrettyPath(params.FilePath)}
		}
	case tools.MultiEditToolName:
		var params tools.MultiEditParams
		if json.Unmarshal([]byte(toolCall.Input), &params) == nil {
			out := []string{fsext.PrettyPath(params.FilePath)}
			if len(params.Edits) > 0 {
				out = append(out, "edits", fmt.Sprintf("%d", len(params.Edits)))
			}
			return "Multi-Edit", out
		}
	case tools.BashToolName:
		var params tools.BashParams
		if json.Unmarshal([]byte(toolCall.Input), &params) == nil {
			command := StripBashDisplayPrefix(params.Command, workingDir)
			return "Bash", []string{strings.ReplaceAll(command, "\n", " ")}
		}
	}

	name := strings.NewReplacer("_", " ", "-", " ").Replace(toolCall.Name)
	name = stringext.Capitalize(name)
	var params map[string]any
	if json.Unmarshal([]byte(toolCall.Input), &params) == nil && len(params) > 0 {
		compact, _ := json.Marshal(params)
		return name, []string{string(compact)}
	}
	return name, []string{toolCall.Input}
}
