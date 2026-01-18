package lsp

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/lsp"
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
func RenderLSPList(lspClients *csync.Map[string, *lsp.Client], opts RenderOptions) []string {
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

	// Get LSP states
	lsps := slices.SortedFunc(maps.Values(app.GetLSPStates()), func(a, b app.LSPClientInfo) int {
		return strings.Compare(a.Name, b.Name)
	})
	if len(lsps) == 0 {
		lspList = append(lspList, t.S().Base.Foreground(t.Border).Render("None"))
		return lspList
	}

	// Determine how many items to show
	maxItems := len(lsps)
	if opts.MaxItems > 0 {
		maxItems = min(opts.MaxItems, len(lsps))
	}

	for i, info := range lsps {
		if i >= maxItems {
			break
		}

		icon, description := iconAndDescription(t, info)

		// Calculate diagnostic counts if we have LSP clients
		var extraContent string
		if lspClients != nil {
			if client, ok := lspClients.Get(info.Name); ok {
				counts := client.GetDiagnosticCounts()
				errs := []string{}
				if counts.Error > 0 {
					errs = append(errs, t.S().Base.Foreground(t.Error).Render(fmt.Sprintf("%s %d", styles.ErrorIcon, counts.Error)))
				}
				if counts.Warning > 0 {
					errs = append(errs, t.S().Base.Foreground(t.Warning).Render(fmt.Sprintf("%s %d", styles.WarningIcon, counts.Warning)))
				}
				if counts.Hint > 0 {
					errs = append(errs, t.S().Base.Foreground(t.FgHalfMuted).Render(fmt.Sprintf("%s %d", styles.HintIcon, counts.Hint)))
				}
				if counts.Information > 0 {
					errs = append(errs, t.S().Base.Foreground(t.FgHalfMuted).Render(fmt.Sprintf("%s %d", styles.InfoIcon, counts.Information)))
				}
				extraContent = strings.Join(errs, " ")
			}
		}

		lspList = append(lspList,
			core.Status(
				core.StatusOpts{
					Icon:         icon.String(),
					Title:        info.Name,
					Description:  description,
					ExtraContent: extraContent,
				},
				opts.MaxWidth,
			),
		)
	}

	return lspList
}

func iconAndDescription(t *styles.Theme, info app.LSPClientInfo) (lipgloss.Style, string) {
	switch info.State {
	case lsp.StateStarting:
		return t.ItemBusyIcon, t.S().Subtle.Render("starting...")
	case lsp.StateReady:
		return t.ItemOnlineIcon, ""
	case lsp.StateError:
		description := t.S().Subtle.Render("error")
		if info.Error != nil {
			description = t.S().Subtle.Render(fmt.Sprintf("error: %s", info.Error.Error()))
		}
		return t.ItemErrorIcon, description
	case lsp.StateDisabled:
		return t.ItemOfflineIcon.Foreground(t.FgMuted), t.S().Subtle.Render("inactive")
	default:
		return t.ItemOfflineIcon, ""
	}
}

// RenderLSPBlock renders a complete LSP block with optional truncation indicator.
func RenderLSPBlock(lspClients *csync.Map[string, *lsp.Client], opts RenderOptions, showTruncationIndicator bool) string {
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
