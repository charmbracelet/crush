package dialog

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	agentmcp "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// MCPItem renders one MCP server and its current status/details.
type MCPItem struct {
	name                    string
	state                   agentmcp.ClientInfo
	oauthEnabled            bool
	supportsInteractiveAuth bool
	toolNames               []string
	promptNames             []string
	resourceNames           []string
	t                       *styles.Styles
	m                       fuzzy.Match
	cache                   map[int]string
	focused                 bool
}

var _ ListItem = (*MCPItem)(nil)

func (i *MCPItem) Filter() string {
	parts := []string{i.name, i.state.State.String()}
	if i.state.Error != nil {
		parts = append(parts, i.state.Error.Error())
	}
	parts = append(parts, i.toolNames...)
	parts = append(parts, i.promptNames...)
	parts = append(parts, i.resourceNames...)
	return strings.Join(parts, " ")
}

func (i *MCPItem) ID() string {
	return i.name
}

func (i *MCPItem) SetFocused(focused bool) {
	if i.focused != focused {
		i.cache = nil
	}
	i.focused = focused
}

func (i *MCPItem) SetMatch(m fuzzy.Match) {
	i.cache = nil
	i.m = m
}

func (i *MCPItem) DefaultAction() Action {
	// Enter opens detail view instead of directly reconnecting
	return ActionOpenMCPDetail{Name: i.name}
}

func (i *MCPItem) CanAuthenticate() bool {
	return i.supportsInteractiveAuth && i.state.State != agentmcp.StateDisabled
}

func (i *MCPItem) CanReconnect() bool {
	return i.state.State != agentmcp.StateDisabled
}

func (i *MCPItem) Render(width int) string {
	if i.cache == nil {
		i.cache = make(map[int]string)
	}
	if cached, ok := i.cache[width]; ok {
		return cached
	}

	primaryStyle := i.t.Dialog.NormalItem
	// Sub-line style: no padding so "  " prefix aligns consistently
	secondaryStyle := i.t.Subtle
	if i.focused {
		primaryStyle = i.t.Dialog.SelectedItem
		secondaryStyle = lipgloss.NewStyle().Background(i.t.Dialog.SelectedItem.GetBackground())
	}

	lines := []string{
		fmt.Sprintf("%s %s", i.icon(), i.name),
		i.statusLine(),
	}
	if inventory := i.inventoryLine(); inventory != "" {
		lines = append(lines, inventory)
	}
	if i.state.Error != nil {
		lines = append(lines, "Last error: "+i.state.Error.Error())
	}

	rendered := make([]string, 0, len(lines))
	for idx, line := range lines {
		line = ansi.Truncate(line, max(0, width), "…")
		if idx == 0 {
			if i.focused {
				// Strip icon ANSI codes: the embedded \x1b[m reset would clear
				// the background set by primaryStyle mid-line.
				line = ansi.Strip(line)
			}
			rendered = append(rendered, primaryStyle.Width(width).Render(line))
			continue
		}
		rendered = append(rendered, secondaryStyle.Width(width).Render("  "+line))
	}

	view := lipgloss.JoinVertical(lipgloss.Left, rendered...)
	i.cache[width] = view
	return view
}

func (i *MCPItem) icon() string {
	switch i.state.State {
	case agentmcp.StateStarting:
		return i.t.ResourceBusyIcon.String()
	case agentmcp.StateConnected:
		return i.t.ResourceOnlineIcon.String()
	case agentmcp.StateNeedsAuth:
		return i.t.ResourceErrorIcon.String()
	case agentmcp.StateError:
		return i.t.ResourceErrorIcon.String()
	case agentmcp.StateDisabled:
		return i.t.ResourceOfflineIcon.String()
	default:
		return i.t.ResourceOfflineIcon.String()
	}
}

func (i *MCPItem) statusLine() string {
	parts := []string{i.stateLabel()}
	if i.oauthEnabled {
		parts = append(parts, "OAuth")
	}
	if counts := mcpCountsSummary(i.state.Counts); counts != "" {
		parts = append(parts, counts)
	}
	return strings.Join(parts, " · ")
}

func (i *MCPItem) inventoryLine() string {
	parts := []string{}
	if len(i.toolNames) > 0 {
		parts = append(parts, "Tools: "+joinPreview(i.toolNames, 3))
	}
	if len(i.promptNames) > 0 {
		parts = append(parts, "Prompts: "+joinPreview(i.promptNames, 2))
	}
	if len(i.resourceNames) > 0 {
		parts = append(parts, "Resources: "+joinPreview(i.resourceNames, 2))
	}
	return strings.Join(parts, " | ")
}

func (i *MCPItem) stateLabel() string {
	switch i.state.State {
	case agentmcp.StateStarting:
		return "Starting"
	case agentmcp.StateConnected:
		return "Connected"
	case agentmcp.StateNeedsAuth:
		return "Authentication required"
	case agentmcp.StateError:
		return "Error"
	case agentmcp.StateDisabled:
		return "Disabled"
	default:
		return "Unknown"
	}
}

func mcpDialogItems(t *styles.Styles, cfg config.MCPs, states map[string]agentmcp.ClientInfo) []list.FilterableItem {
	toolNamesByMCP := make(map[string][]string)
	for name, tools := range agentmcp.Tools() {
		names := make([]string, 0, len(tools))
		for _, tool := range tools {
			names = append(names, tool.Name)
		}
		slices.Sort(names)
		toolNamesByMCP[name] = names
	}

	promptNamesByMCP := make(map[string][]string)
	for name, prompts := range agentmcp.Prompts() {
		names := make([]string, 0, len(prompts))
		for _, prompt := range prompts {
			names = append(names, prompt.Name)
		}
		slices.Sort(names)
		promptNamesByMCP[name] = names
	}

	resourceNamesByMCP := make(map[string][]string)
	for name, resources := range agentmcp.Resources() {
		names := make([]string, 0, len(resources))
		for _, resource := range resources {
			names = append(names, cmp.Or(resource.Title, resource.Name, resource.URI))
		}
		slices.Sort(names)
		resourceNamesByMCP[name] = names
	}

	items := make([]list.FilterableItem, 0, len(cfg))
	for _, configured := range cfg.Sorted() {
		state, ok := states[configured.Name]
		if !ok {
			state = agentmcp.ClientInfo{Name: configured.Name, State: agentmcp.StateStarting}
			if configured.MCP.Disabled {
				state.State = agentmcp.StateDisabled
			}
		}
		items = append(items, &MCPItem{
			name:                    configured.Name,
			state:                   state,
			oauthEnabled:            configured.MCP.OAuthEnabled(),
			supportsInteractiveAuth: configured.MCP.SupportsInteractiveAuth(),
			toolNames:               toolNamesByMCP[configured.Name],
			promptNames:             promptNamesByMCP[configured.Name],
			resourceNames:           resourceNamesByMCP[configured.Name],
			t:                       t,
		})
	}
	return items
}

func mcpCountsSummary(counts agentmcp.Counts) string {
	parts := []string{}
	if counts.Tools > 0 {
		parts = append(parts, fmt.Sprintf("%d tools", counts.Tools))
	}
	if counts.Prompts > 0 {
		parts = append(parts, fmt.Sprintf("%d prompts", counts.Prompts))
	}
	if counts.Resources > 0 {
		parts = append(parts, fmt.Sprintf("%d resources", counts.Resources))
	}
	return strings.Join(parts, ", ")
}

func joinPreview(values []string, limit int) string {
	if len(values) == 0 {
		return ""
	}
	if limit <= 0 || len(values) <= limit {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:limit], ", ") + fmt.Sprintf(" +%d", len(values)-limit)
}
