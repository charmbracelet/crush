package lsp

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/styles"
)

// RenderOptions contains options for rendering LSP lists.
type RenderOptions struct {
	MaxWidth    int
	MaxItems    int
	ShowSection bool
	SectionName string
}

// RenderLSPList renders a list of LSP status items with the given options.
func RenderLSPList(lspClients map[string]*lsp.Client, opts RenderOptions) []string {
	t := styles.CurrentTheme()
	lspList := []string{}

	if opts.ShowSection {
		sectionName := opts.SectionName
		if sectionName == "" {
			sectionName = "LSPs"
		}
		section := t.S().Subtle.Render(sectionName)
		lspList = append(lspList, section, "")
	}

	lspConfigs := config.Get().LSP.Sorted()
	if len(lspConfigs) == 0 {
		lspList = append(lspList, t.S().Base.Foreground(t.Border).Render("None"))
		return lspList
	}

	// Get LSP states
	lspStates := app.GetLSPStates()

	// Determine how many items to show
	maxItems := len(lspConfigs)
	if opts.MaxItems > 0 {
		maxItems = min(opts.MaxItems, len(lspConfigs))
	}

	for i, l := range lspConfigs {
		if i >= maxItems {
			break
		}

		// Determine icon color and description based on state
		icon := t.ItemOfflineIcon
		description := l.LSP.ResolvedCommand()

		if l.LSP.Disabled {
			description = t.S().Subtle.Render("disabled")
		} else if state, exists := lspStates[l.Name]; exists {
			switch state.State {
			case lsp.StateStarting:
				icon = t.ItemBusyIcon
				description = t.S().Subtle.Render("starting...")
			case lsp.StateReady:
				icon = t.ItemOnlineIcon
				description = l.LSP.ResolvedCommand()
			case lsp.StateError:
				icon = t.ItemErrorIcon
				if state.Error != nil {
					description = t.S().Subtle.Render(fmt.Sprintf("error: %s", state.Error.Error()))
				} else {
					description = t.S().Subtle.Render("error")
				}
			}
		}

		// Calculate diagnostic counts if we have LSP clients
		var extraContent string
		if lspClients != nil {
			lspErrs := map[protocol.DiagnosticSeverity]int{
				protocol.SeverityError:       0,
				protocol.SeverityWarning:     0,
				protocol.SeverityHint:        0,
				protocol.SeverityInformation: 0,
			}
			if client, ok := lspClients[l.Name]; ok {
				for _, diagnostics := range client.GetDiagnostics() {
					for _, diagnostic := range diagnostics {
						if severity, ok := lspErrs[diagnostic.Severity]; ok {
							lspErrs[diagnostic.Severity] = severity + 1
						}
					}
				}
			}

			errs := []string{}
			if lspErrs[protocol.SeverityError] > 0 {
				errs = append(errs, t.S().Base.Foreground(t.Error).Render(fmt.Sprintf("%s %d", styles.ErrorIcon, lspErrs[protocol.SeverityError])))
			}
			if lspErrs[protocol.SeverityWarning] > 0 {
				errs = append(errs, t.S().Base.Foreground(t.Warning).Render(fmt.Sprintf("%s %d", styles.WarningIcon, lspErrs[protocol.SeverityWarning])))
			}
			if lspErrs[protocol.SeverityHint] > 0 {
				errs = append(errs, t.S().Base.Foreground(t.FgHalfMuted).Render(fmt.Sprintf("%s %d", styles.HintIcon, lspErrs[protocol.SeverityHint])))
			}
			if lspErrs[protocol.SeverityInformation] > 0 {
				errs = append(errs, t.S().Base.Foreground(t.FgHalfMuted).Render(fmt.Sprintf("%s %d", styles.InfoIcon, lspErrs[protocol.SeverityInformation])))
			}
			extraContent = strings.Join(errs, " ")
		}

		lspList = append(lspList,
			core.Status(
				core.StatusOpts{
					Icon:         icon.String(),
					Title:        l.Name,
					Description:  description,
					ExtraContent: extraContent,
				},
				opts.MaxWidth,
			),
		)
	}

	return lspList
}

// RenderLSPBlock renders a complete LSP block with optional truncation indicator.
func RenderLSPBlock(lspClients map[string]*lsp.Client, opts RenderOptions, showTruncationIndicator bool) string {
	t := styles.CurrentTheme()
	lspList := RenderLSPList(lspClients, opts)

	// Add truncation indicator if needed
	if showTruncationIndicator && opts.MaxItems > 0 {
		lspConfigs := config.Get().LSP.Sorted()
		if len(lspConfigs) > opts.MaxItems {
			remaining := len(lspConfigs) - opts.MaxItems
			if remaining == 1 {
				lspList = append(lspList, t.S().Base.Foreground(t.FgMuted).Render("…"))
			} else {
				lspList = append(lspList,
					t.S().Base.Foreground(t.FgSubtle).Render(fmt.Sprintf("…and %d more", remaining)),
				)
			}
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lspList...)
	if opts.MaxWidth > 0 {
		return lipgloss.NewStyle().Width(opts.MaxWidth).Render(content)
	}
	return content
}
