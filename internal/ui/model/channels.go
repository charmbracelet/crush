package model

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// channelsInfo renders the channel status section showing MCP servers that are
// connected as channels (via the claude/channel capability + --channels opt-in).
func (m *UI) channelsInfo(width, maxItems int, isSection bool) string {
	t := m.com.Styles

	title := t.Resource.Heading.Render("Channels")
	if isSection {
		title = common.Section(t, title, width)
	}

	channels := m.channelStatusItems()
	if len(channels) == 0 {
		list := t.Resource.AdditionalText.Render("None")
		return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
	}

	list := channelList(t, channels, width, maxItems)
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

// channelStatusItem holds the display data for a single channel entry.
type channelStatusItem struct {
	name        string
	icon        string
	title       string
	description string
}

// channelStatusItems collects all MCP servers that are active channels and
// returns them sorted by name.
func (m *UI) channelStatusItems() []channelStatusItem {
	t := m.com.Styles
	var items []channelStatusItem

	for _, mcpCfg := range m.com.Config().MCP.Sorted() {
		state, ok := m.mcpStates[mcpCfg.Name]
		if !ok || !state.Channel {
			continue
		}

		var icon string
		var description string
		switch state.State {
		case mcp.StateStarting:
			icon = t.Resource.BusyIcon.String()
			description = t.Resource.StatusText.Render("starting...")
		case mcp.StateConnected:
			icon = t.Resource.OnlineIcon.String()
			description = t.Resource.StatusText.Render("connected")
		case mcp.StateError:
			icon = t.Resource.ErrorIcon.String()
			description = t.Resource.StatusText.Render("error")
			if state.Error != nil {
				description = t.Resource.StatusText.Render(fmt.Sprintf("error: %s", state.Error.Error()))
			}
		case mcp.StateDisabled:
			icon = t.Resource.DisabledIcon.String()
			description = t.Resource.StatusText.Render("disabled")
		default:
			icon = t.Resource.OfflineIcon.String()
			description = t.Resource.StatusText.Render("offline")
		}

		items = append(items, channelStatusItem{
			name:        mcpCfg.Name,
			icon:        icon,
			title:       t.Resource.Name.Render(mcpCfg.Name),
			description: description,
		})
	}

	slices.SortStableFunc(items, func(a, b channelStatusItem) int {
		return strings.Compare(a.name, b.name)
	})

	return items
}

func channelList(t *styles.Styles, items []channelStatusItem, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}

	if len(items) > maxItems {
		visibleItems := items[:maxItems-1]
		remaining := len(items) - (maxItems - 1)
		items = append(visibleItems, channelStatusItem{
			title: t.Resource.AdditionalText.Render(fmt.Sprintf("…and %d more", remaining)),
		})
	}

	renderedItems := make([]string, 0, len(items))
	for _, item := range items {
		renderedItems = append(renderedItems, common.Status(t, common.StatusOpts{
			Icon:        item.icon,
			Title:       item.title,
			Description: item.description,
		}, width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, renderedItems...)
}
