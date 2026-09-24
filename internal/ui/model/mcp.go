package model

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// mcpStatesTTL bounds how long the memoized MCP state may go without a
// re-probe being scheduled; MCP events normally refresh it much sooner. The
// backstop covers state_changed events missed in client/server mode: the
// SSE pipeline is events-only, so a transition that completes before the
// stream attaches, or while it is down, is otherwise invisible until the
// next state change. A few seconds of stale MCP status is invisible,
// matching the LSP backstop's cadence; see lsp.go. Package var so tests
// can pin it.
var mcpStatesTTL = 5 * time.Second

// mcpStartingRetryDelay is the cadence of the retry loop that keeps
// re-probing MCP states while any server is still connecting, so a
// "starting..." entry converges to the server's settled state even when
// the state_changed event never arrives. Package var so tests can pin it.
var mcpStartingRetryDelay = 1 * time.Second

// mcpStartingRetryMsg re-dispatches an MCP state refresh after the retry
// delay; applyMCPStates arms it while any server is in StateStarting.
type mcpStartingRetryMsg struct{}

// requestMCPRefresh schedules an off-thread refresh of the memoized MCP
// states. While a fetch is already in flight it only marks the state dirty;
// applyMCPStates re-dispatches so the freshest data still lands.
func (m *UI) requestMCPRefresh() tea.Cmd {
	if m.mcpFetchInFlight {
		m.mcpRefreshQueued = true
		return nil
	}
	return m.dispatchMCPRefresh()
}

// dispatchMCPRefresh returns a command that fetches the MCP states off the
// Update goroutine (a synchronous HTTP round-trip in client/server mode),
// delivering an mcpStateChangedMsg. It returns nil while a fetch is already
// in flight. The closure captures only locals (never m) so it is safe
// off-thread.
func (m *UI) dispatchMCPRefresh() tea.Cmd {
	if m.mcpFetchInFlight || m.com == nil || m.com.Workspace == nil {
		return nil
	}
	m.mcpFetchInFlight = true
	// Stamp the check time at dispatch too so the TTL backstop doesn't
	// keep re-requesting while this fetch is in flight.
	m.mcpCheckedAt = time.Now()
	ws := m.com.Workspace
	return func() tea.Msg {
		return mcpStateChangedMsg{
			states: ws.MCPGetStates(),
		}
	}
}

// applyMCPStates stores an off-thread MCP fetch result, re-dispatches when
// a refresh was requested while it was in flight, and keeps the retry loop
// running while any server is still connecting. A failed fetch keeps the
// last-known-good states instead of blanking the sidebar. Runs on the
// Update goroutine.
func (m *UI) applyMCPStates(msg mcpStateChangedMsg) []tea.Cmd {
	m.mcpFetchInFlight = false
	m.mcpCheckedAt = time.Now()
	if msg.states == nil {
		// A failed fetch (e.g. a transient HTTP error in client/server
		// mode) keeps the last-known-good states on screen.
		if m.mcpRefreshQueued {
			m.mcpRefreshQueued = false
			if cmd := m.dispatchMCPRefresh(); cmd != nil {
				return []tea.Cmd{cmd}
			}
		}
		return nil
	}

	// Auto-open the MCP auth dialog when a server newly needs
	// authentication. Comparing against the previous states keeps
	// backstop-driven refreshes from re-opening a dialog the user already
	// dismissed while a server stays in StateNeedsAuth.
	newlyNeedsAuth := false
	for name, info := range msg.states {
		if info.State != mcp.StateNeedsAuth {
			continue
		}
		if prev, ok := m.mcpStates[name]; !ok || prev.State != mcp.StateNeedsAuth {
			newlyNeedsAuth = true
			break
		}
	}
	m.mcpStates = msg.states

	var cmds []tea.Cmd
	if newlyNeedsAuth {
		if cmd := m.openMCPAuthDialog(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.mcpRefreshQueued {
		m.mcpRefreshQueued = false
		if cmd := m.dispatchMCPRefresh(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	// A server still connecting keeps the refresh loop armed: the next
	// re-probe lands within mcpStartingRetryDelay even if the
	// state_changed event is never delivered (late SSE attach, a
	// reconnect gap, or a lossy broker in client/server mode).
	if anyMCPStarting(msg.states) {
		cmds = append(cmds, tea.Tick(mcpStartingRetryDelay, func(time.Time) tea.Msg {
			return mcpStartingRetryMsg{}
		}))
	}
	return cmds
}

// anyMCPStarting reports whether any MCP client is still connecting.
func anyMCPStarting(states map[string]mcp.ClientInfo) bool {
	for _, info := range states {
		if info.State == mcp.StateStarting {
			return true
		}
	}
	return false
}

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
