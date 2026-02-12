package model

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// LSPInfo wraps LSP client information with diagnostic counts by severity.
type LSPInfo struct {
	app.LSPClientInfo
	Diagnostics map[protocol.DiagnosticSeverity]int
}

// lspInfo renders the LSP status section showing active LSP clients and their
// diagnostic counts.
func (m *UI) lspInfo(width, maxItems int, isSection bool) string {
	t := m.com.Styles

	states := slices.SortedFunc(maps.Values(m.lspStates), func(a, b app.LSPClientInfo) int {
		return strings.Compare(a.Name, b.Name)
	})

	var lsps []LSPInfo
	for _, state := range states {
		lspErrs := map[protocol.DiagnosticSeverity]int{}
		if client, ok := m.com.App.LSPManager.Clients().Get(state.Name); ok {
			counts := client.GetDiagnosticCounts()
			lspErrs[protocol.SeverityError] = counts.Error
			lspErrs[protocol.SeverityWarning] = counts.Warning
			lspErrs[protocol.SeverityHint] = counts.Hint
			lspErrs[protocol.SeverityInformation] = counts.Information
		}

		lsps = append(lsps, LSPInfo{LSPClientInfo: state, Diagnostics: lspErrs})
	}

	title := t.ResourceGroupTitle.Render("LSPs")
	if isSection {
		title = common.Section(t, title, width)
	}
	list := t.ResourceAdditionalText.Render("None")
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
		title := t.ResourceName.Render(l.Name)
		var description string
		var diagnostics string
		switch l.State {
		case lsp.StateUnstarted:
			icon = t.ResourceOfflineIcon.String()
			description = t.ResourceStatus.Render("unstarted")
		case lsp.StateStopped:
			icon = t.ResourceOfflineIcon.String()
			description = t.ResourceStatus.Render("stopped")
		case lsp.StateStarting:
			icon = t.ResourceBusyIcon.String()
			description = t.ResourceStatus.Render("starting...")
		case lsp.StateReady:
			icon = t.ResourceOnlineIcon.String()
			diagnostics = lspDiagnostics(t, l.Diagnostics)
		case lsp.StateError:
			icon = t.ResourceErrorIcon.String()
			description = t.ResourceStatus.Render("error")
			if l.Error != nil {
				description = t.ResourceStatus.Render(fmt.Sprintf("error: %s", l.Error.Error()))
			}
		case lsp.StateDisabled:
			icon = t.ResourceOfflineIcon.Foreground(t.Muted.GetBackground()).String()
			description = t.ResourceStatus.Render("disabled")
		default:
			icon = t.ResourceOfflineIcon.String()
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
		visibleItems = append(visibleItems, t.ResourceAdditionalText.Render(fmt.Sprintf("…and %d more", remaining)))
		return lipgloss.JoinVertical(lipgloss.Left, visibleItems...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, renderedLsps...)
}
