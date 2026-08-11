package model

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// lspStatesTTL bounds how long the memoized LSP state may go without a
// re-probe being scheduled; LSP events normally refresh it much sooner. The
// backstop covers events missed across SSE reconnects in client/server
// mode, so it can be an order of magnitude looser than the busy/permission
// TTL (which drives interactive affordances like the spinner and queue
// pill): a few seconds of stale LSP counts is invisible, a few seconds of
// stale busy state is not. Package var so tests can pin it.
var lspStatesTTL = 5 * time.Second

// lspStatesMsg delivers LSP states and per-server diagnostic counts fetched
// off-thread.
type lspStatesMsg struct {
	states      map[string]workspace.LSPClientInfo
	diagnostics map[string]lsp.DiagnosticCounts
}

// LSPInfo wraps LSP client information with diagnostic counts by severity.
type LSPInfo struct {
	workspace.LSPClientInfo
	Diagnostics map[protocol.DiagnosticSeverity]int
}

// requestLSPRefresh schedules an off-thread refresh of the memoized LSP
// state. While a fetch is already in flight it only marks the state dirty;
// applyLSPStates re-dispatches so the freshest data still lands.
func (m *UI) requestLSPRefresh() tea.Cmd {
	if m.lspFetchInFlight {
		m.lspRefreshQueued = true
		return nil
	}
	return m.dispatchLSPRefresh()
}

// dispatchLSPRefresh returns a command that fetches the LSP states and
// per-server diagnostic counts off the Update goroutine (each a synchronous
// HTTP round-trip in client/server mode), delivering an lspStatesMsg. It
// returns nil while a fetch is already in flight. The closure captures only
// locals (never m) so it is safe off-thread.
func (m *UI) dispatchLSPRefresh() tea.Cmd {
	if m.lspFetchInFlight || m.com == nil || m.com.Workspace == nil {
		return nil
	}
	m.lspFetchInFlight = true
	// Stamp the check time at dispatch too so the TTL backstop doesn't
	// keep re-requesting while this fetch is in flight.
	m.lspCheckedAt = time.Now()
	ws := m.com.Workspace
	return func() tea.Msg {
		states := ws.LSPGetStates()
		diagnostics := make(map[string]lsp.DiagnosticCounts, len(states))
		for name := range states {
			diagnostics[name] = ws.LSPGetDiagnosticCounts(name)
		}
		return lspStatesMsg{states: states, diagnostics: diagnostics}
	}
}

// applyLSPStates stores an off-thread LSP fetch result and re-dispatches
// when events arrived while it was in flight. Runs on the Update goroutine.
func (m *UI) applyLSPStates(msg lspStatesMsg) tea.Cmd {
	m.lspFetchInFlight = false
	m.lspCheckedAt = time.Now()
	m.lspStates = msg.states
	m.lspDiagnostics = msg.diagnostics
	if m.lspRefreshQueued {
		m.lspRefreshQueued = false
		return m.dispatchLSPRefresh()
	}
	return nil
}

// lspErrorCount returns the total diagnostic count across the memoized LSP
// states, shown in the compact header.
func (m *UI) lspErrorCount() int {
	count := 0
	for _, info := range m.lspStates {
		count += info.DiagnosticCount
	}
	return count
}

// lspInfo renders the LSP status section showing active LSP clients and their
// diagnostic counts. It renders from the memoized state only: this runs on
// every frame, and the workspace probes behind it are synchronous HTTP
// round-trips in client/server mode. LSP events (plus the TTL backstop)
// keep the memoized state fresh off-thread; see requestLSPRefresh.
func (m *UI) lspInfo(width, maxItems int, isSection bool) string {
	t := m.com.Styles

	states := slices.SortedFunc(maps.Values(m.lspStates), func(a, b workspace.LSPClientInfo) int {
		return strings.Compare(a.Name, b.Name)
	})

	var lsps []LSPInfo
	for _, state := range states {
		counts := m.lspDiagnostics[state.Name]
		lsps = append(lsps, LSPInfo{LSPClientInfo: state, Diagnostics: map[protocol.DiagnosticSeverity]int{
			protocol.SeverityError:       counts.Error,
			protocol.SeverityWarning:     counts.Warning,
			protocol.SeverityHint:        counts.Hint,
			protocol.SeverityInformation: counts.Information,
		}})
	}

	title := t.Resource.Heading.Render("LSPs")
	if isSection {
		title = common.Section(t, title, width)
	}
	list := t.Resource.AdditionalText.Render("None")
	if len(lsps) > 0 {
		list = lspList(t, lsps, width, maxItems)
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

// lspDiagnostics formats diagnostic counts with appropriate icons and colors.
func lspDiagnostics(t *styles.Styles, diagnostics map[protocol.DiagnosticSeverity]int) string {
	var errs []string
	if diagnostics[protocol.SeverityError] > 0 {
		errs = append(errs, t.LSP.ErrorDiagnostic.Render(fmt.Sprintf("%s%d", styles.LSPErrorIcon, diagnostics[protocol.SeverityError])))
	}
	if diagnostics[protocol.SeverityWarning] > 0 {
		errs = append(errs, t.LSP.WarningDiagnostic.Render(fmt.Sprintf("%s%d", styles.LSPWarningIcon, diagnostics[protocol.SeverityWarning])))
	}
	if diagnostics[protocol.SeverityHint] > 0 {
		errs = append(errs, t.LSP.HintDiagnostic.Render(fmt.Sprintf("%s%d", styles.LSPHintIcon, diagnostics[protocol.SeverityHint])))
	}
	if diagnostics[protocol.SeverityInformation] > 0 {
		errs = append(errs, t.LSP.InfoDiagnostic.Render(fmt.Sprintf("%s%d", styles.LSPInfoIcon, diagnostics[protocol.SeverityInformation])))
	}
	return strings.Join(errs, " ")
}

// lspList renders a list of LSP clients with their status and diagnostics,
// truncating to maxItems if needed.
func lspList(t *styles.Styles, lsps []LSPInfo, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}
	var renderedLsps []string
	for _, l := range lsps {
		var icon string
		title := t.Resource.Name.Render(l.Name)
		var description string
		var diagnostics string
		switch l.State {
		case lsp.StateUnstarted:
			icon = t.Resource.OfflineIcon.String()
			description = t.Resource.StatusText.Render("unstarted")
		case lsp.StateStopped:
			icon = t.Resource.OfflineIcon.String()
			description = t.Resource.StatusText.Render("stopped")
		case lsp.StateStarting:
			icon = t.Resource.BusyIcon.String()
			description = t.Resource.StatusText.Render("starting...")
		case lsp.StateReady:
			icon = t.Resource.OnlineIcon.String()
			diagnostics = lspDiagnostics(t, l.Diagnostics)
		case lsp.StateError:
			icon = t.Resource.ErrorIcon.String()
			description = t.Resource.StatusText.Render("error")
			if l.Error != nil {
				description = t.Resource.StatusText.Render(fmt.Sprintf("error: %s", l.Error.Error()))
			}
		case lsp.StateDisabled:
			icon = t.Resource.DisabledIcon.String()
			description = t.Resource.StatusText.Render("disabled")
		default:
			continue
		}
		renderedLsps = append(renderedLsps, common.Status(t, common.StatusOpts{
			Icon:         icon,
			Title:        title,
			Description:  description,
			ExtraContent: diagnostics,
		}, width))
	}

	if len(renderedLsps) > maxItems {
		visibleItems := renderedLsps[:maxItems-1]
		remaining := len(renderedLsps) - maxItems
		visibleItems = append(visibleItems, t.Resource.AdditionalText.Render(fmt.Sprintf("…and %d more", remaining)))
		return lipgloss.JoinVertical(lipgloss.Left, visibleItems...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, renderedLsps...)
}
