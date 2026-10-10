package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
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

// mcpStartingRetryDelay is the initial cadence of the retry loop that keeps
// re-probing MCP states while any server is still connecting, so a
// "starting..." entry converges to the server's settled state even when
// the state_changed event never arrives. The cadence backs off
// exponentially to mcpStartingRetryMaxDelay so a server that never settles
// cannot drive a round-trip per second for the whole session. Package vars
// so tests can pin them.
var mcpStartingRetryDelay = 1 * time.Second

// mcpStartingRetryMaxDelay caps the starting-retry backoff. Once the loop
// has backed off this far, the TTL backstop (mcpStatesTTL) dominates the
// probe cadence anyway.
var mcpStartingRetryMaxDelay = 30 * time.Second

// mcpStartingRetryBackoff returns the retry delay for the given
// zero-based attempt, doubling from mcpStartingRetryDelay and clamping at
// mcpStartingRetryMaxDelay.
func mcpStartingRetryBackoff(attempt int) time.Duration {
	delay := mcpStartingRetryDelay
	for i := 0; i < attempt && delay < mcpStartingRetryMaxDelay; i++ {
		delay *= 2
	}
	return min(delay, mcpStartingRetryMaxDelay)
}

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
	// re-probe lands within the backoff delay even if the state_changed
	// event is never delivered (late SSE attach, a reconnect gap, or a
	// lossy broker in client/server mode). The delay doubles per
	// consecutive starting observation up to mcpStartingRetryMaxDelay so
	// a server that never settles stops driving per-second round-trips.
	if anyMCPStarting(msg.states) {
		cmds = append(cmds, tea.Tick(mcpStartingRetryBackoff(m.mcpStartingRetries), func(time.Time) tea.Msg {
			return mcpStartingRetryMsg{}
		}))
		m.mcpStartingRetries++
	} else {
		m.mcpStartingRetries = 0
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
	cfg := m.com.Config()
	opts := cfg.Options

	// Which servers hide their tools until the agent searches for them.
	lazyServers := make(map[string]bool, len(m.mcpStates))
	for name, mcpCfg := range cfg.MCP {
		lazyServers[name] = mcpCfg.IsLazy(opts)
	}

	for _, mcp := range cfg.MCP.Sorted() {
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
		list = mcpList(t, mcps, lazyServers, width, maxItems)
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
// truncating to maxItems if needed. Servers marked lazy show their tools as
// available on demand instead of counting them as loaded context.
func mcpList(t *styles.Styles, mcps []mcp.ClientInfo, lazyServers map[string]bool, width, maxItems int) string {
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
			if m.Channel {
				badge := t.Resource.StatusText.Render("channel")
				if extraContent != "" {
					extraContent += " " + badge
				} else {
					extraContent = badge
				}
			}
			if lazyServers[m.Name] {
				extraContent = strings.TrimSpace(extraContent + " " +
					t.Resource.AdditionalText.Render("· lazy"))
			}
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
			Name:            configured.Name,
			ConfigDisabled:  configured.MCP.Disabled,
			EnabledOverride: enabledOverride,
			Status:          mcpStatusText(m.mcpStates[configured.Name]),
			Lazy:            configured.MCP.IsLazy(m.com.Config().Options),
		}
		if _, off := disabled[configured.Name]; off {
			item.Disabled = true
		}
		items = append(items, item)
	}
	return items, nil
}

// applyMCPServerSetting persists a tri-state choice for one MCP server.
// The connection half follows the dialog's scope: a local change writes a
// repository-scoped override (and force-starts a config-disabled server so
// it stays connected across restarts), while a global change writes the
// config's disabled flag and applies it to the running client.
//
// The two connected settings then also carry the lazy half, which always
// lives in the global config because it only decides whether the server's
// schemas go into the model's next request, never which servers connect.
// Disabled leaves the lazy flag untouched so cycling back restores the
// server's previous placement. Failures surface as an error toast; the
// dialog keeps its own optimistic state meanwhile.
func (m *UI) applyMCPServerSetting(msg dialog.ActionSetMCPServerSetting) tea.Cmd {
	name := msg.Name
	return func() tea.Msg {
		switch msg.Setting {
		case dialog.MCPServerSettingDisabled:
			// Take the server offline for this scope; the lazy placement is
			// preserved for when it is cycled back to a connected setting.
			if err := m.setMCPServerConnected(name, false, msg.Global); err != nil {
				return util.NewErrorMsg(err)
			}
			if msg.Global {
				return util.NewInfoMsg(fmt.Sprintf("MCP %q disabled globally", name))
			}
			return util.NewInfoMsg(fmt.Sprintf("MCP %q disabled for this repository", name))
		default:
			if err := m.setMCPServerConnected(name, true, msg.Global); err != nil {
				return util.NewErrorMsg(err)
			}
			lazy := msg.Setting == dialog.MCPServerSettingLazy
			if err := m.com.Workspace.MCPSetLazy(context.TODO(), name, lazy); err != nil {
				return util.NewErrorMsg(err)
			}
			if msg.Setting == dialog.MCPServerSettingLazy {
				return util.NewInfoMsg(fmt.Sprintf("MCP %q lazy loading: tools served on demand", name))
			}
			return util.NewInfoMsg(fmt.Sprintf("MCP %q enabled: tools always shown", name))
		}
	}
}

// setMCPServerConnected turns a server's connection on or off for the given
// scope. Enabling a config-disabled server locally also force-starts it so
// the running client reflects the change immediately, mirroring the startup
// path that reads the repository overrides.
func (m *UI) setMCPServerConnected(name string, enabled, global bool) error {
	disabled := !enabled
	if global {
		return m.com.Workspace.MCPSetServerConfigDisabled(context.TODO(), name, disabled)
	}
	if enabled {
		if configured, ok := m.com.Config().MCP[name]; ok && configured.Disabled {
			if err := m.com.Workspace.MCPStartServer(context.TODO(), name); err != nil {
				return err
			}
		}
	}
	return m.com.Workspace.MCPSetServerDisabled(context.TODO(), name, disabled)
}

// mcpStatusText renders a connection state as plain dialog text.
func mcpStatusText(info mcp.ClientInfo) string {
	switch info.State {
	case mcp.StateStarting:
		// No ellipsis: the text stays put when the state transitions,
		// instead of jumping as the trailing dots appear and vanish.
		return "starting"
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
