package model

import (
	"image"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/charmbracelet/ultraviolet/layout"
)

// selectedLargeModel returns the currently selected large language model as
// memoized by the off-thread busy/agent probe (see workspace_cache.go), or
// nil when the agent isn't ready. It must never probe the workspace: it is
// called on every frame and AgentIsReady/AgentModel are synchronous HTTP
// round-trips in client/server mode.
func (m *UI) selectedLargeModel() *workspace.AgentModel {
	if m.agentReady {
		model := m.agentModel
		return &model
	}
	return nil
}

// landingView renders the landing page view showing the current working
// directory, model information, and LSP/MCP status in a two-column layout.
func (m *UI) landingView() string {
	t := m.com.Styles
	width := m.layout.main.Dx()
	cwd := common.PrettyPath(t, m.com.Workspace.WorkingDir(), width)

	parts := []string{
		cwd,
	}

	parts = append(parts, "", m.modelInfo(width))
	infoSection := lipgloss.JoinVertical(lipgloss.Left, parts...)

	var remainingHeightArea image.Rectangle
	layout.Vertical(
		layout.Len(lipgloss.Height(infoSection)+1),
		layout.Fill(1),
	).Split(m.layout.main).Assign(new(image.Rectangle), &remainingHeightArea)

	// Channels are experimental and opt-in, so their column only appears
	// once a server is opted in; otherwise the other columns keep their
	// full width.
	channels := m.channelStatusItems()
	columns := 3
	if len(channels) > 0 {
		columns = 4
	}
	mcpLspSectionWidth := min(30, (width-(columns-1))/columns)
	sectionHeight := max(1, remainingHeightArea.Dy())

	lspSection := m.lspInfo(mcpLspSectionWidth, sectionHeight, false)
	mcpSection := m.mcpInfo(mcpLspSectionWidth, sectionHeight, false)
	skillsSection := m.skillsInfo(mcpLspSectionWidth, sectionHeight, false)
	sections := []string{lspSection, " ", mcpSection, " ", skillsSection}
	if len(channels) > 0 {
		sections = append(sections, " ", m.channelsInfo(channels, mcpLspSectionWidth, sectionHeight, false))
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, sections...)

	return lipgloss.NewStyle().
		Width(width).
		Height(m.layout.main.Dy() - 1).
		PaddingTop(1).
		Render(
			lipgloss.JoinVertical(lipgloss.Left, infoSection, "", content),
		)
}
