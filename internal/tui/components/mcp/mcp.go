package mcp

import (
	"fmt"

	"github.com/charmbracelet/lipgloss/v2"

	"github.com/nom-nom-hub/blush/internal/config"
	"github.com/nom-nom-hub/blush/internal/llm/agent"
	"github.com/nom-nom-hub/blush/internal/tui/styles"
)

// RenderOptions contains options for rendering MCP lists.
type RenderOptions struct {
	MaxWidth    int
	MaxItems    int
	ShowSection bool
	SectionName string
}

// RenderMCPList renders a list of MCP status items with the given options.
func RenderMCPList(opts RenderOptions) []string {
	t := styles.CurrentTheme()
	mcpList := []string{}

	if opts.ShowSection {
		sectionName := opts.SectionName
		if sectionName == "" {
			sectionName = "MCPs"
		}
		// Create a beautiful section header with decorative elements
		sectionStyle := t.S().Subtle.Bold(true).Foreground(t.Secondary)
		section := sectionStyle.Render(sectionName)
		mcpList = append(mcpList, section, "")
	}

	mcps := config.Get().MCP.Sorted()
	if len(mcps) == 0 {
		// Beautiful empty state with decorative elements
		emptyStyle := t.S().Base.Foreground(t.FgSubtle).Italic(true)
		emptyIcon := t.S().Base.Foreground(t.Border).Render("🔌")
		emptyMessage := emptyStyle.Render("No MCP servers configured")
		mcpList = append(mcpList, fmt.Sprintf("%s %s", emptyIcon, emptyMessage))
		return mcpList
	}

	// Get MCP states
	mcpStates := agent.GetMCPStates()

	// Determine how many items to show
	maxItems := len(mcps)
	if opts.MaxItems > 0 {
		maxItems = min(opts.MaxItems, len(mcps))
	}

	for i, l := range mcps {
		if i >= maxItems {
			break
		}

		// Beautiful status indicators with enhanced styling
		icon := "●" // Default offline icon
		iconStyle := t.ItemOfflineIcon
		titleStyle := t.FgMuted
		descStyle := t.S().Base.Foreground(t.FgSubtle).Italic(true)
		description := l.MCP.Command
		extraContent := ""

		if state, exists := mcpStates[l.Name]; exists {
			switch state.State {
			case agent.MCPStateDisabled:
				description = descStyle.Render("disabled")
			case agent.MCPStateStarting:
				icon = "⏳"
				iconStyle = t.ItemBusyIcon.Foreground(t.Warning)
				description = descStyle.Render("starting...")
				titleStyle = t.FgHalfMuted
			case agent.MCPStateConnected:
				icon = "✓"
				iconStyle = t.ItemOnlineIcon.Foreground(t.Success)
				if state.ToolCount > 0 {
					extraContent = t.S().Subtle.Render(fmt.Sprintf("%d tools", state.ToolCount))
				}
				titleStyle = t.FgBase
			case agent.MCPStateError:
				icon = "✗"
				iconStyle = t.ItemErrorIcon.Foreground(t.Error)
				if state.Error != nil {
					description = descStyle.Render(fmt.Sprintf("error: %s", state.Error.Error()))
				} else {
					description = descStyle.Render("error")
				}
				titleStyle = t.Error
			}
		} else if l.MCP.Disabled {
			description = descStyle.Render("disabled")
		}

		// Create a beautiful MCP entry with enhanced visual styling
		iconPart := iconStyle.SetString(icon).String()
		titlePart := t.S().Base.Foreground(titleStyle).Render(l.Name)
		
		// Combine all parts in a beautiful layout
		mcpEntry := fmt.Sprintf("%s %s", iconPart, titlePart)
		if description != "" {
			mcpEntry = fmt.Sprintf("%s\n  %s", mcpEntry, description)
		}
		if extraContent != "" {
			extraStyle := t.S().Base.Foreground(t.FgSubtle)
			mcpEntry = fmt.Sprintf("%s\n  %s", mcpEntry, extraStyle.Render(extraContent))
		}
		
		mcpList = append(mcpList, mcpEntry)
	}

	return mcpList
}

// RenderMCPBlock renders a complete MCP block with optional truncation indicator.
func RenderMCPBlock(opts RenderOptions, showTruncationIndicator bool) string {
	t := styles.CurrentTheme()
	mcpList := RenderMCPList(opts)

	// Add truncation indicator if needed
	if showTruncationIndicator && opts.MaxItems > 0 {
		mcps := config.Get().MCP.Sorted()
		if len(mcps) > opts.MaxItems {
			remaining := len(mcps) - opts.MaxItems
			if remaining == 1 {
				mcpList = append(mcpList, t.S().Base.Foreground(t.FgMuted).Render("…"))
			} else {
				mcpList = append(mcpList,
					t.S().Base.Foreground(t.FgSubtle).Render(fmt.Sprintf("…and %d more", remaining)),
				)
			}
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, mcpList...)
	if opts.MaxWidth > 0 {
		return lipgloss.NewStyle().Width(opts.MaxWidth).Render(content)
	}
	return content
}
