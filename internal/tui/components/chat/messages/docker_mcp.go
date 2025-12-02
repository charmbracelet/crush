package messages

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/crush/internal/tui/styles"

	"github.com/charmbracelet/crush/internal/stringext"
)

type dockerMCPRenderer struct {
	baseRenderer
}

// Render displays file content with optional limit and offset parameters
func (dr dockerMCPRenderer) Render(v *toolCallCmp) string {
	var params map[string]any
	if err := dr.unmarshalParams(v.call.Input, &params); err != nil {
		return dr.renderError(v, "Invalid view parameters")
	}

	tool := strings.ReplaceAll(v.call.Name, "mcp_crush_docker_", "")

	main := v.call.Input
	extraArgs := map[string]string{}
	switch tool {
	case "mcp-find":
		if query, ok := params["query"]; ok {
			if qStr, ok := query.(string); ok {
				main = qStr
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
				main = nStr
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
				main = nStr
			}
		}
		for k, v := range params {
			if k == "name" {
				continue
			}

			data, _ := json.Marshal(v)
			extraArgs[k] = string(data)
		}
	}

	args := newParamBuilder().
		addMain(main)

	for k, v := range extraArgs {
		args.addKeyValue(k, v)
	}

	width := v.textWidth()
	if v.isNested {
		width -= 4 // Adjust for nested tool call indentation
	}
	header := dr.makeHeader(v, tool, width, args.build()...)
	if v.isNested {
		return v.style().Render(header)
	}
	if res, done := earlyState(header, v); done {
		return res
	}

	if tool == "mcp-find" {
		return joinHeaderBody(header, dr.renderMCPServers(v))
	}
	return joinHeaderBody(header, renderPlainContent(v, v.result.Content))
}

type FindMCPResponse struct {
	Servers []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"servers"`
}

func (dr dockerMCPRenderer) renderMCPServers(v *toolCallCmp) string {
	t := styles.CurrentTheme()
	var result FindMCPResponse
	if err := dr.unmarshalParams(v.result.Content, &result); err != nil {
		return renderPlainContent(v, v.result.Content)
	}

	if len(result.Servers) == 0 {
		return t.S().Muted.Render("No MCP servers found.")
	}
	width := min(120, v.textWidth()) - 2
	rows := [][]string{}
	moreServers := ""
	for i, server := range result.Servers {
		if i > 9 {
			moreServers = t.S().Subtle.Render(fmt.Sprintf("... and %d mode", len(result.Servers)-10))
			break
		}
		rows = append(rows, []string{t.S().Base.Render(server.Name), t.S().Muted.Render(server.Description)})
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
		}).Rows(rows...).Width(width)
	if moreServers != "" {
		return serverTable.Render() + "\n" + moreServers
	}
	return serverTable.Render()
}

func (dr dockerMCPRenderer) makeHeader(v *toolCallCmp, tool string, width int, params ...string) string {
	t := styles.CurrentTheme()
	mainTool := "Docker MCP"
	action := tool
	actionStyle := t.S().Base.Foreground(t.BlueDark)
	switch tool {
	case "mcp-exec":
		action = "Exec"
	case "mcp-config-set":
		action = "Config Set"
	case "mcp-find":
		action = "Find"
	case "mcp-add":
		action = "Add"
		actionStyle = t.S().Base.Foreground(t.GreenLight)
	case "mcp-remove":
		action = "Remove"
		actionStyle = t.S().Base.Foreground(t.RedLighter)
	default:
		action = strings.ReplaceAll(tool, "-", " ")
		action = strings.ReplaceAll(action, "_", " ")
		action = stringext.Capitalize(action)
	}
	if v.isNested {
		return dr.makeNestedHeader(v, tool, width, params...)
	}
	icon := t.S().Base.Foreground(t.GreenDark).Render(styles.ToolPending)
	if v.result.ToolCallID != "" {
		if v.result.IsError {
			icon = t.S().Base.Foreground(t.RedDark).Render(styles.ToolError)
		} else {
			icon = t.S().Base.Foreground(t.Green).Render(styles.ToolSuccess)
		}
	} else if v.cancelled {
		icon = t.S().Muted.Render(styles.ToolPending)
	}
	tool = t.S().Base.Foreground(t.Blue).Render(mainTool)
	arrow := t.S().Base.Foreground(t.BlueDark).Render(styles.ArrowIcon)
	prefix := fmt.Sprintf("%s %s %s %s ", icon, tool, arrow, actionStyle.Render(action))
	return prefix + renderParamList(false, width-lipgloss.Width(prefix), params...)
}
