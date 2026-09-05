package model

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
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

	title := t.Resource.Heading.Render("MCPs")
	if isSection {
		title = common.Section(t, title, width)
	}
	list := t.Resource.AdditionalText.Render("None")
	if len(mcps) > 0 {
		list = mcpList(t, mcps, width, maxItems)
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

// mcpCounts formats tool, prompt, and resource counts for display.
func mcpCounts(t *styles.Styles, counts mcp.Counts) string {
	var parts []string
	if counts.Tools > 0 {
		parts = append(parts, t.Resource.CapabilityCount.Render(fmt.Sprintf("%d tools", counts.Tools)))
	}
	if counts.Prompts > 0 {
		parts = append(parts, t.Resource.CapabilityCount.Render(fmt.Sprintf("%d prompts", counts.Prompts)))
	}
	if counts.Resources > 0 {
		parts = append(parts, t.Resource.CapabilityCount.Render(fmt.Sprintf("%d resources", counts.Resources)))
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
		// Show "Docker MCP" instead of the config name for Docker MCP.
		if m.Name == config.DockerMCPName {
			title = "Docker MCP"
		}
		title = t.Resource.Name.Render(title)
		var description string
		var extraContent string

		switch m.State {
		case mcp.StateStarting:
			icon = t.Resource.BusyIcon.String()
			description = t.Resource.StatusText.Render("starting...")
		case mcp.StateConnected:
			icon = t.Resource.OnlineIcon.String()
			extraContent = mcpCounts(t, m.Counts)
		case mcp.StateError:
			icon = t.Resource.ErrorIcon.String()
			description = t.Resource.StatusText.Render("error")
			if m.Error != nil {
				description = t.Resource.StatusText.Render(fmt.Sprintf("error: %s", m.Error.Error()))
			}
		case mcp.StateNeedsAuth:
			icon = t.Resource.NeedsAuthIcon.String()
			description = t.Resource.StatusText.Render("needs authentication")
		case mcp.StateDisabled:
			icon = t.Resource.DisabledIcon.String()
			description = t.Resource.StatusText.Render("disabled")
		default:
			icon = t.Resource.OfflineIcon.String()
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
		visibleItems = append(visibleItems, t.Resource.AdditionalText.Render(fmt.Sprintf("…and %d more", remaining)))
		return lipgloss.JoinVertical(lipgloss.Left, visibleItems...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, renderedMcps...)
}

// openMCPTogglesDialog opens the repository-scoped MCP toggles dialog.
func (m *UI) openMCPTogglesDialog() tea.Cmd {
	if m.dialog.ContainsDialog(dialog.MCPTogglesID) {
		m.dialog.BringToFront(dialog.MCPTogglesID)
		return nil
	}
	items, err := m.mcpToggleItems()
	if err != nil {
		return util.ReportError(err)
	}
	m.dialog.OpenDialog(dialog.NewMCPToggles(m.com, items))
	return nil
}

// mcpToggleItems builds the dialog items from the configured servers, the
// live connection states, and the repository's disabled overrides.
func (m *UI) mcpToggleItems() ([]dialog.MCPToggleItem, error) {
	disabledServers, err := m.com.Workspace.MCPServersDisabled(context.Background())
	if err != nil {
		return nil, err
	}
	disabled := make(map[string]struct{}, len(disabledServers))
	for _, name := range disabledServers {
		disabled[name] = struct{}{}
	}
	// A repository-scoped enabled override turns a config-disabled server
	// back on for this repository; it is force-started at startup.
	enabledServers, err := m.com.Workspace.MCPServersEnabled(context.Background())
	if err != nil {
		return nil, err
	}
	enabled := make(map[string]struct{}, len(enabledServers))
	for _, name := range enabledServers {
		enabled[name] = struct{}{}
	}
	items := make([]dialog.MCPToggleItem, 0, len(m.com.Config().MCP))
	for _, configured := range m.com.Config().MCP.Sorted() {
		_, enabledOverride := enabled[configured.Name]
		item := dialog.MCPToggleItem{
			Name:           configured.Name,
			ConfigDisabled: configured.MCP.Disabled && !enabledOverride,
			Status:         mcpStatusText(m.mcpStates[configured.Name]),
		}
		if _, off := disabled[configured.Name]; off {
			item.Disabled = true
		}
		items = append(items, item)
	}
	return items, nil
}

// applyMCPToggle persists a repository-scoped MCP toggle. Enabling a
// config-disabled server also starts it now and records an enabled
// override, so it stays enabled across restarts. The dialog keeps its own
// optimistic state; failures surface as an error toast.
func (m *UI) applyMCPToggle(msg dialog.ActionToggleMCP) tea.Cmd {
	name := msg.Name
	disable := msg.Disabled
	return func() tea.Msg {
		if !disable {
			if configured, ok := m.com.Config().MCP[name]; ok && configured.Disabled {
				if err := m.com.Workspace.MCPStartServer(context.TODO(), name); err != nil {
					return util.NewErrorMsg(err)
				}
			}
		}
		if err := m.com.Workspace.MCPSetServerDisabled(context.TODO(), name, disable); err != nil {
			return util.NewErrorMsg(err)
		}
		status := "enabled"
		if disable {
			status = "disabled"
		}
		return util.NewInfoMsg(fmt.Sprintf("MCP %q %s for this repository", name, status))
	}
}

// mcpStatusText renders a connection state as plain dialog text.
func mcpStatusText(info mcp.ClientInfo) string {
	switch info.State {
	case mcp.StateStarting:
		return "starting..."
	case mcp.StateConnected:
		return "connected"
	case mcp.StateError:
		if info.Error != nil {
			return fmt.Sprintf("error: %s", info.Error.Error())
		}
		return "error"
	case mcp.StateNeedsAuth:
		return "needs authentication"
	case mcp.StateDisabled:
		return "disabled"
	default:
		return "offline"
	}
}
