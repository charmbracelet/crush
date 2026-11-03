package model

import (
	"image"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/charmbracelet/ultraviolet/layout"
)

// Bounds below which the first-run cheat sheet is dropped rather than
// crowding the working directory, model, and status sections.
const (
	landingTipsMinWidth  = 60
	landingTipsMinHeight = 20
	landingTipsKeyWidth  = 12
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
	cwd := common.PrettyPathInline(t, m.com.Workspace.WorkingDir(), m.gitBranch, width)

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

	mcpLspSectionWidth := min(30, (width-2)/3)

	lspSection := m.lspInfo(mcpLspSectionWidth, max(1, remainingHeightArea.Dy()), false)
	mcpSection := m.mcpInfo(mcpLspSectionWidth, max(1, remainingHeightArea.Dy()), false)
	skillsSection := m.skillsInfo(mcpLspSectionWidth, max(1, remainingHeightArea.Dy()), false)

	content := lipgloss.JoinHorizontal(lipgloss.Left, lspSection, " ", mcpSection, " ", skillsSection)

	sections := []string{infoSection, ""}
	if m.showLandingTips() {
		sections = append(sections, m.landingTipsView(), "")
	}
	sections = append(sections, content)

	return lipgloss.NewStyle().
		Width(width).
		Height(m.layout.main.Dy() - 1).
		PaddingTop(1).
		Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// showLandingTips reports whether the first-run cheat sheet belongs on the
// landing view: only until the first prompt, and only when there is room for
// it.
func (m *UI) showLandingTips() bool {
	if m.state != uiLanding || m.tipsDismissed {
		return false
	}
	cfg := m.com.Config()
	if cfg == nil || cfg.Options == nil || !cfg.Options.TUI.ShowTips() {
		return false
	}
	if m.layout.main.Dx() < landingTipsMinWidth || m.layout.main.Dy() < landingTipsMinHeight {
		return false
	}
	return !m.hasSession() || m.chat.Len() == 0
}

// landingTipsView renders the short key cheat sheet shown on first run.
func (m *UI) landingTipsView() string {
	t := m.com.Styles
	hints := []dialog.KeyHint{
		keyHint(m.keyMap.Editor.SendMessage, "send"),
		keyHint(m.keyMap.Editor.MentionFile, "add a file to the conversation"),
		keyHint(m.keyMap.Editor.Commands, "slash commands"),
		keyHint(m.keyMap.Commands, "palette: models, sessions, init"),
	}

	heading := t.Dialog.Onboarding.Step.PaddingLeft(1).Render("Getting started")
	hintBlock := dialog.RenderKeyHints(t, hints, landingTipsKeyWidth)
	more := t.Dialog.Onboarding.Tip.PaddingLeft(1).Render("More keys in ctrl+g.")

	return lipgloss.JoinVertical(lipgloss.Left, heading, "", hintBlock, "", more)
}
