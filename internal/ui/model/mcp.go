package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// mcpInfo renders the MCP status section showing active MCP clients and their
// tool/prompt counts.
func (m *UI) mcpInfo(width, maxItems int, isSection bool) string {
	var mcps []mcp.ClientInfo
	t := m.com.Styles

	for _, mcp := range m.com.Config().MCP.Sorted() {
		if state, ok := m.mcpStates[mcp.Name]; ok {
			mcps = append(mcps, state)
		}
	}

	title := t.Subtle.Render("MCPs")
	if isSection {
		title = common.Section(t, title, width)
	}
	list := t.Subtle.Render("None")
	if len(mcps) > 0 {
		list = mcpList(t, mcps, width, maxItems)
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

// mcpCounts formats tool and prompt counts for display.
func mcpCounts(t *styles.Styles, counts mcp.Counts) string {
	parts := []string{}
	if counts.Tools > 0 {
		parts = append(parts, t.Subtle.Render(fmt.Sprintf("%d tools", counts.Tools)))
	}
	if counts.Prompts > 0 {
		parts = append(parts, t.Subtle.Render(fmt.Sprintf("%d prompts", counts.Prompts)))
	}
	return strings.Join(parts, " ")
}

// mcpList renders a list of MCP clients with their status and counts,
// truncating to maxItems if needed.
func mcpList(t *styles.Styles, mcps []mcp.ClientInfo, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}
	var renderedMcps []string

	for _, m := range mcps {
		var icon string
		title := m.Name
		var description string
		var extraContent string

		switch m.State {
		case mcp.StateStarting:
			icon = t.ItemBusyIcon.String()
			description = t.Subtle.Render("starting...")
		case mcp.StateConnected:
			icon = t.ItemOnlineIcon.String()
			extraContent = mcpCounts(t, m.Counts)
		case mcp.StateError:
			icon = t.ItemErrorIcon.String()
			description = t.Subtle.Render("error")
			if m.Error != nil {
				description = t.Subtle.Render(fmt.Sprintf("error: %s", m.Error.Error()))
			}
		case mcp.StateDisabled:
			icon = t.ItemOfflineIcon.Foreground(t.Muted.GetBackground()).String()
			description = t.Subtle.Render("disabled")
		default:
			icon = t.ItemOfflineIcon.String()
		}

		renderedMcps = append(renderedMcps, common.Status(t, common.StatusOpts{
			Icon:         icon,
			Title:        title,
			Description:  description,
			ExtraContent: extraContent,
		}, width))
	}

	if len(renderedMcps) > maxItems {
		visibleItems := renderedMcps[:maxItems-1]
		remaining := len(renderedMcps) - maxItems
		visibleItems = append(visibleItems, t.Subtle.Render(fmt.Sprintf("…and %d more", remaining)))
		return lipgloss.JoinVertical(lipgloss.Left, visibleItems...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, renderedMcps...)
}
