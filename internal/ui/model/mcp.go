package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

type MCPInfo struct {
	mcp.ClientInfo
}

func (m *UI) mcpInfo(t *styles.Styles, width, height int) string {
	var mcps []MCPInfo

	for _, state := range m.mcpStates {
		mcps = append(mcps, MCPInfo{ClientInfo: state})
	}

	title := t.Subtle.Render("MCPs")
	list := t.Subtle.Render("None")
	if len(mcps) > 0 {
		height = max(0, height-2) // remove title and space
		list = mcpList(t, mcps, width, height)
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

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

func mcpList(t *styles.Styles, mcps []MCPInfo, width, height int) string {
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

	if len(renderedMcps) > height {
		visibleItems := renderedMcps[:height-1]
		remaining := len(renderedMcps) - (height - 1)
		visibleItems = append(visibleItems, t.Subtle.Render(fmt.Sprintf("…and %d more", remaining)))
		return lipgloss.JoinVertical(lipgloss.Left, visibleItems...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, renderedMcps...)
}
