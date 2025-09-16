package lsp

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"

	"github.com/nom-nom-hub/blush/internal/config"
	"github.com/nom-nom-hub/blush/internal/lsp"
	"github.com/nom-nom-hub/blush/internal/lsp/protocol"
	"github.com/nom-nom-hub/blush/internal/tui/styles"
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
		// Create a beautiful section header with decorative elements
		sectionStyle := t.S().Subtle.Bold(true).Foreground(t.Secondary)
		section := sectionStyle.Render(sectionName)
		lspList = append(lspList, section, "")
	}

	lspConfigs := config.Get().LSP.Sorted()
	if len(lspConfigs) == 0 {
		// Beautiful empty state with decorative elements
		emptyStyle := t.S().Base.Foreground(t.FgSubtle).Italic(true)
		emptyIcon := t.S().Base.Foreground(t.Border).Render("🔌")
		emptyMessage := emptyStyle.Render("No LSP servers configured")
		lspList = append(lspList, fmt.Sprintf("%s %s", emptyIcon, emptyMessage))
		return lspList
	}

	// Get LSP states
	// Note: We can't import app package here, so we'll simplify this
	// In a real implementation, you'd need to pass the states as a parameter

	// Determine how many items to show
	maxItems := len(lspConfigs)
	if opts.MaxItems > 0 {
		maxItems = min(opts.MaxItems, len(lspConfigs))
	}

	for i, l := range lspConfigs {
		if i >= maxItems {
			break
		}

		// Beautiful status indicators with enhanced styling
		icon := "●" // Default offline icon
		iconStyle := t.ItemOfflineIcon
		titleStyle := t.FgMuted
		descStyle := t.S().Base.Foreground(t.FgSubtle).Italic(true)
		description := l.LSP.Command

		// Simplified state handling since we can't access app.GetLSPStates()
		if l.LSP.Disabled {
			description = descStyle.Render("disabled")
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

			// Beautiful diagnostic indicators
			errs := []string{}
			if lspErrs[protocol.SeverityError] > 0 {
				errStyle := t.S().Base.Foreground(t.Error).Bold(true)
				errs = append(errs, errStyle.Render(fmt.Sprintf("✗ %d", lspErrs[protocol.SeverityError])))
			}
			if lspErrs[protocol.SeverityWarning] > 0 {
				warnStyle := t.S().Base.Foreground(t.Warning).Bold(true)
				errs = append(errs, warnStyle.Render(fmt.Sprintf("⚠ %d", lspErrs[protocol.SeverityWarning])))
			}
			if lspErrs[protocol.SeverityHint] > 0 {
				hintStyle := t.S().Base.Foreground(t.FgHalfMuted)
				errs = append(errs, hintStyle.Render(fmt.Sprintf("ⓘ %d", lspErrs[protocol.SeverityHint])))
			}
			if lspErrs[protocol.SeverityInformation] > 0 {
				infoStyle := t.S().Base.Foreground(t.Info)
				errs = append(errs, infoStyle.Render(fmt.Sprintf("ℹ %d", lspErrs[protocol.SeverityInformation])))
			}
			extraContent = strings.Join(errs, " ")
		}

		// Create a beautiful LSP entry with enhanced visual styling
		iconPart := iconStyle.SetString(icon).String()
		titlePart := t.S().Base.Foreground(titleStyle).Render(l.Name)
		
		// Combine all parts in a beautiful layout
		lspEntry := fmt.Sprintf("%s %s", iconPart, titlePart)
		if description != "" {
			lspEntry = fmt.Sprintf("%s\n  %s", lspEntry, description)
		}
		if extraContent != "" {
			lspEntry = fmt.Sprintf("%s\n  %s", lspEntry, extraContent)
		}
		
		lspList = append(lspList, lspEntry)
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