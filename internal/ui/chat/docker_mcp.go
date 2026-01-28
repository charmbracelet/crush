package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// DockerMCPToolMessageItem is a message item that represents a Docker MCP tool call.
type DockerMCPToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*DockerMCPToolMessageItem)(nil)

// NewDockerMCPToolMessageItem creates a new [DockerMCPToolMessageItem].
func NewDockerMCPToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &DockerMCPToolRenderContext{}, canceled)
}

// DockerMCPToolRenderContext renders Docker MCP tool messages.
type DockerMCPToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (d *DockerMCPToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)

	var params map[string]any
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		params = make(map[string]any)
	}

	tool := strings.TrimPrefix(opts.ToolCall.Name, "mcp_"+config.DockerMCPName+"_")

	mainParam := opts.ToolCall.Input
	extraArgs := map[string]string{}
	switch tool {
	case "mcp-find":
		if query, ok := params["query"]; ok {
			if qStr, ok := query.(string); ok {
				mainParam = qStr
			}
		}
		for k, v := range params {
			if k == "query" {
				continue
			}
			data, _ := json.Marshal(v)
			extraArgs[k] = string(data)
		}
	case "mcp-add":
		if name, ok := params["name"]; ok {
			if nStr, ok := name.(string); ok {
				mainParam = nStr
			}
		}
		for k, v := range params {
			if k == "name" {
				continue
			}
			data, _ := json.Marshal(v)
			extraArgs[k] = string(data)
		}
	case "mcp-remove":
		if name, ok := params["name"]; ok {
			if nStr, ok := name.(string); ok {
				mainParam = nStr
			}
		}
		for k, v := range params {
			if k == "name" {
				continue
			}
			data, _ := json.Marshal(v)
			extraArgs[k] = string(data)
		}
	case "mcp-exec":
		if name, ok := params["name"]; ok {
			if nStr, ok := name.(string); ok {
				mainParam = nStr
			}
		}
	case "mcp-config-set":
		if server, ok := params["server"]; ok {
			if sStr, ok := server.(string); ok {
				mainParam = sStr
			}
		}
	}

	var toolParams []string
	toolParams = append(toolParams, mainParam)
	for k, v := range extraArgs {
		toolParams = append(toolParams, k, v)
	}

	header := d.makeHeader(sty, tool, cappedWidth, opts, toolParams...)
	if opts.Compact {
		return header
	}

	if opts.IsPending() {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if tool == "mcp-find" {
		return joinToolParts(header, d.renderMCPServers(sty, opts, cappedWidth))
	}

	if !opts.HasResult() || opts.Result.Content == "" {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	body := sty.Tool.Body.Render(toolOutputPlainContent(sty, opts.Result.Content, bodyWidth, opts.ExpandedContent))
	return joinToolParts(header, body)
}

// FindMCPResponse represents the response from mcp-find.
type FindMCPResponse struct {
	Servers []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"servers"`
}

func (d *DockerMCPToolRenderContext) renderMCPServers(sty *styles.Styles, opts *ToolRenderOpts, width int) string {
	if !opts.HasResult() || opts.Result.Content == "" {
		return ""
	}

	var result FindMCPResponse
	if err := json.Unmarshal([]byte(opts.Result.Content), &result); err != nil {
		return toolOutputPlainContent(sty, opts.Result.Content, width-toolBodyLeftPaddingTotal, opts.ExpandedContent)
	}

	if len(result.Servers) == 0 {
		return sty.Subtle.Render("No MCP servers found.")
	}

	bodyWidth := min(120, width) - toolBodyLeftPaddingTotal
	rows := [][]string{}
	moreServers := ""
	for i, server := range result.Servers {
		if i > 9 {
			moreServers = sty.Subtle.Render(fmt.Sprintf("... and %d more", len(result.Servers)-10))
			break
		}
		rows = append(rows, []string{sty.Base.Render(server.Name), sty.Subtle.Render(server.Description)})
	}
	serverTable := table.New().
		Wrap(false).
		BorderTop(false).
		BorderBottom(false).
		BorderRight(false).
		BorderLeft(false).
		BorderColumn(false).
		BorderRow(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle()
			}
			switch col {
			case 0:
				return lipgloss.NewStyle().PaddingRight(1)
			}
			return lipgloss.NewStyle()
		}).Rows(rows...).Width(bodyWidth)
	if moreServers != "" {
		return sty.Tool.Body.Render(serverTable.Render() + "\n" + moreServers)
	}
	return sty.Tool.Body.Render(serverTable.Render())
}

func (d *DockerMCPToolRenderContext) makeHeader(sty *styles.Styles, tool string, width int, opts *ToolRenderOpts, params ...string) string {
	mainTool := "Docker MCP"
	action := tool
	actionStyle := sty.Tool.MCPToolName
	switch tool {
	case "mcp-exec":
		action = "Exec"
	case "mcp-config-set":
		action = "Config Set"
	case "mcp-find":
		action = "Find"
	case "mcp-add":
		action = "Add"
		actionStyle = sty.Tool.DockerMCPActionAdd
	case "mcp-remove":
		action = "Remove"
		actionStyle = sty.Tool.DockerMCPActionDel
	case "code-mode":
		action = "Code Mode"
	default:
		action = strings.ReplaceAll(tool, "-", " ")
		action = strings.ReplaceAll(action, "_", " ")
		action = stringext.Capitalize(action)
	}

	if opts.Compact {
		return d.makeCompactHeader(sty, tool, width, params...)
	}

	icon := toolIcon(sty, opts.Status)
	if opts.IsPending() {
		icon = sty.Tool.IconPending.Render()
	}
	toolNameStyled := sty.Tool.MCPName.Render(mainTool)
	arrow := sty.Tool.MCPArrow.String()
	prefix := fmt.Sprintf("%s %s %s %s ", icon, toolNameStyled, arrow, actionStyle.Render(action))
	return prefix + toolParamList(sty, params, width-lipgloss.Width(prefix))
}

func (d *DockerMCPToolRenderContext) makeCompactHeader(sty *styles.Styles, tool string, width int, params ...string) string {
	action := tool
	switch tool {
	case "mcp-exec":
		action = "exec"
	case "mcp-config-set":
		action = "config-set"
	case "mcp-find":
		action = "find"
	case "mcp-add":
		action = "add"
	case "mcp-remove":
		action = "remove"
	case "code-mode":
		action = "code-mode"
	default:
		action = strings.ReplaceAll(tool, "-", " ")
		action = strings.ReplaceAll(action, "_", " ")
	}

	name := fmt.Sprintf("Docker MCP: %s", action)
	return toolHeader(sty, ToolStatusSuccess, name, width, true, params...)
}

// IsDockerMCPTool returns true if the tool name is a Docker MCP tool.
func IsDockerMCPTool(name string) bool {
	return strings.HasPrefix(name, "mcp_"+config.DockerMCPName+"_")
}
