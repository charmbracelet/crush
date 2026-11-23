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

// showsWorkspaceStatus reports whether the landing page lists LSP, MCP and
// skills. It does not while the user is going through first run: with no
// provider yet, or with a project that still needs its context file. Those
// moments are about setting up, and the extra columns would only compete with
// the prompt.
func (m *UI) showsWorkspaceStatus() bool {
	return !m.isOnboarding() && !m.firstRun.needsInit
}

// landingView renders the landing page view showing the current working
// directory, model information, and LSP/MCP status in a two-column layout.
func (m *UI) landingView() string {
	t := m.com.Styles
	width := m.layout.main.Dx()
	cwd := common.PrettyPathInline(t, m.com.Workspace.WorkingDir(), m.gitBranchLabel(), width)

	parts := []string{cwd}

	if m.isOnboarding() {
		// Nothing is set up yet, so LSP, MCP and skills status would only
		// be noise next to the first-run prompts. The instruction (or the
		// held message) sits at the bottom, right above the editor.
		if welcome := m.welcomeView(width); welcome != "" {
			parts = append(parts, "", welcome)
		}
		top := lipgloss.JoinVertical(lipgloss.Left, parts...)
		return anchorBottom(width, m.layout.main.Dy()-1, top, m.firstRunPromptView(width))
	}

	parts = append(parts, "", m.modelInfo(width), "", m.welcomeView(width))
	infoSection := lipgloss.JoinVertical(lipgloss.Left, parts...)

	if !m.showsWorkspaceStatus() {
		// The project is still being set up (the /init hint is showing), so
		// the page stays with the welcome and leaves LSP, MCP and skills
		// out of it.
		return lipgloss.NewStyle().
			Width(width).
			Height(m.layout.main.Dy() - 1).
			PaddingTop(1).
			Render(infoSection)
	}

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
