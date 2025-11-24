package model

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type LSPInfo struct {
	app.LSPClientInfo
	Diagnostics map[protocol.DiagnosticSeverity]int
}

func (m *UI) lspInfo(t *styles.Styles, width, height int) string {
	var lsps []LSPInfo

	for _, state := range m.lspStates {
		client, ok := m.com.App.LSPClients.Get(state.Name)
		if !ok {
			continue
		}
		lspErrs := map[protocol.DiagnosticSeverity]int{
			protocol.SeverityError:       0,
			protocol.SeverityWarning:     0,
			protocol.SeverityHint:        0,
			protocol.SeverityInformation: 0,
		}

		for _, diagnostics := range client.GetDiagnostics() {
			for _, diagnostic := range diagnostics {
				if severity, ok := lspErrs[diagnostic.Severity]; ok {
					lspErrs[diagnostic.Severity] = severity + 1
				}
			}
		}

		lsps = append(lsps, LSPInfo{LSPClientInfo: state, Diagnostics: lspErrs})
	}
	title := t.Subtle.Render("LSPs")
	list := t.Subtle.Render("None")
	if len(lsps) > 0 {
		height = max(0, height-2) // remove title and space
		list = lspList(t, lsps, width, height)
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

func lspDiagnostics(t *styles.Styles, diagnostics map[protocol.DiagnosticSeverity]int) string {
	errs := []string{}
	if diagnostics[protocol.SeverityError] > 0 {
		errs = append(errs, t.LSP.ErrorDiagnostic.Render(fmt.Sprintf("%s %d", styles.ErrorIcon, diagnostics[protocol.SeverityError])))
	}
	if diagnostics[protocol.SeverityWarning] > 0 {
		errs = append(errs, t.LSP.WarningDiagnostic.Render(fmt.Sprintf("%s %d", styles.WarningIcon, diagnostics[protocol.SeverityWarning])))
	}
	if diagnostics[protocol.SeverityHint] > 0 {
		errs = append(errs, t.LSP.HintDiagnostic.Render(fmt.Sprintf("%s %d", styles.HintIcon, diagnostics[protocol.SeverityHint])))
	}
	if diagnostics[protocol.SeverityInformation] > 0 {
		errs = append(errs, t.LSP.InfoDiagnostic.Render(fmt.Sprintf("%s %d", styles.InfoIcon, diagnostics[protocol.SeverityInformation])))
	}
	return strings.Join(errs, " ")
}

func lspList(t *styles.Styles, lsps []LSPInfo, width, height int) string {
	var renderedLsps []string
	for _, l := range lsps {
		var icon string
		title := l.Name
		var description string
		var diagnostics string
		switch l.State {
		case lsp.StateStarting:
			icon = t.ItemBusyIcon.String()
			description = t.Subtle.Render("starting...")
		case lsp.StateReady:
			icon = t.ItemOnlineIcon.String()
			diagnostics = lspDiagnostics(t, l.Diagnostics)
		case lsp.StateError:
			icon = t.ItemErrorIcon.String()
			description = t.Subtle.Render("error")
			if l.Error != nil {
				description = t.Subtle.Render(fmt.Sprintf("error: %s", l.Error.Error()))
			}
		case lsp.StateDisabled:
			icon = t.ItemOfflineIcon.Foreground(t.Muted.GetBackground()).String()
			description = t.Subtle.Render("inactive")
		default:
			icon = t.ItemOfflineIcon.String()
		}
		renderedLsps = append(renderedLsps, common.Status(t, common.StatusOpts{
			Icon:         icon,
			Title:        title,
			Description:  description,
			ExtraContent: diagnostics,
		}, width))
	}

	if len(renderedLsps) > height {
		visibleItems := renderedLsps[:height-1]
		remaining := len(renderedLsps) - (height - 1)
		visibleItems = append(visibleItems, t.Subtle.Render(fmt.Sprintf("…and %d more", remaining)))
		return lipgloss.JoinVertical(lipgloss.Left, visibleItems...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, renderedLsps...)
}
