package model

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// channelsInfo renders the channel status section for items, as built by
// channelStatusItems. Callers build the list once and pass it in, since they
// also use it to size the section or decide whether to show it at all.
func (m *UI) channelsInfo(items []channelStatusItem, width, maxItems int, isSection bool) string {
	t := m.com.Styles

	title := t.Resource.Heading.Render("Channels")
	if isSection {
		title = common.Section(t, title, width)
	}

	if len(items) == 0 {
		list := t.Resource.AdditionalText.Render("None")
		return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
	}

	list := channelList(t, items, width, maxItems)
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

// channelStatusItem holds the display data for a single channel entry.
type channelStatusItem struct {
	name        string
	icon        string
	title       string
	description string
}

// channelStatusItems collects the MCP servers opted in as channels (via
// --channels or channel_enabled) in whatever state they are in, so a channel
// that is starting, has crashed, or needs auth stays listed rather than
// dropping out. Items come back in name order, as MCP.Sorted returns them.
func (m *UI) channelStatusItems() []channelStatusItem {
	t := m.com.Styles
	var items []channelStatusItem

	for _, mcpCfg := range m.com.Config().MCP.Sorted() {
		state, ok := m.mcpStates[mcpCfg.Name]
		if !ok || (!state.ChannelOptIn && !state.Channel) {
			continue
		}

		var icon string
		var description string
		switch state.State {
		case mcp.StateStarting:
			icon = t.Resource.BusyIcon.String()
			description = t.Resource.StatusText.Render("starting...")
		case mcp.StateConnected:
			if state.Channel {
				icon = t.Resource.OnlineIcon.String()
				description = t.Resource.StatusText.Render("connected")
			} else {
				// Opted in, but the server never declared claude/channel,
				// so its pushes are ignored.
				icon = t.Resource.ErrorIcon.String()
				description = t.Resource.StatusText.Render("no channel capability")
			}
		case mcp.StateError:
			icon = t.Resource.ErrorIcon.String()
			description = t.Resource.StatusText.Render("error")
			if state.Error != nil {
				description = t.Resource.StatusText.Render(fmt.Sprintf("error: %s", state.Error.Error()))
			}
		case mcp.StateNeedsAuth:
			icon = t.Resource.NeedsAuthIcon.String()
			description = t.Resource.StatusText.Render("needs authentication")
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
