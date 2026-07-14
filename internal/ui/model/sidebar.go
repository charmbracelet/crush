package model

import (
	"cmp"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// modelInfo renders the current model information including reasoning
// settings and context usage/cost for the sidebar.
func (m *UI) modelInfo(width int) string {
	model := m.selectedLargeModel()
	reasoningInfo := ""
	providerName := ""

	if model != nil {
		// Get provider name first
		providerConfig, ok := m.com.Config().Providers.Get(model.ModelCfg.Provider)
		if ok {
			providerName = providerConfig.Name

			// Only check reasoning if model can reason
			if model.CatwalkCfg.CanReason {
				if len(model.CatwalkCfg.ReasoningLevels) == 0 {
					if model.ModelCfg.Think {
						reasoningInfo = "Thinking On"
					} else {
						reasoningInfo = "Thinking Off"
					}
				} else {
					reasoningEffort := cmp.Or(model.ModelCfg.ReasoningEffort, model.CatwalkCfg.DefaultReasoningEffort)
					reasoningInfo = fmt.Sprintf("Reasoning %s", common.FormatReasoningEffort(reasoningEffort))
				}
			}
		}
	}

	var modelContext *common.ModelContextInfo
	if model != nil && m.session != nil {
		modelContext = &common.ModelContextInfo{
			ContextUsed:    m.session.CompletionTokens + m.session.PromptTokens,
			Cost:           m.session.Cost,
			ModelContext:   model.CatwalkCfg.ContextWindow,
			EstimatedUsage: m.session.EstimatedUsage,
		}
	}
	var modelName string
	if model != nil {
		modelName = model.CatwalkCfg.Name
	}
	return common.ModelInfo(m.com.Styles, modelName, providerName, reasoningInfo, modelContext, width, m.hyperCredits)
}

// sidebar renders the chat sidebar containing session title, working
// directory, model info, file list, LSP status, and MCP status.
func (m *UI) drawSidebar(scr uv.Screen, area uv.Rectangle) {
	if m.session == nil {
		return
	}

	width := area.Dx()
	header := m.sidebarHeader(width)
	headerHeight := min(lipgloss.Height(header), area.Dy())
	headerArea := area
	headerArea.Max.Y = headerArea.Min.Y + headerHeight
	uv.NewStyledString(header).Draw(scr, headerArea)

	bodyArea := area
	bodyArea.Min.Y = headerArea.Max.Y
	if bodyArea.Dx() <= 1 || bodyArea.Dy() <= 0 {
		return
	}

	contentWidth := bodyArea.Dx() - 1
	content := m.sidebarContent(contentWidth)
	lines := strings.Split(content, "\n")
	viewportHeight := bodyArea.Dy()
	maxOffset := max(0, len(lines)-viewportHeight)
	offset := min(max(m.sidebarScrollOffset, 0), maxOffset)
	end := min(len(lines), offset+viewportHeight)
	visible := strings.Join(lines[offset:end], "\n")

	contentArea := bodyArea
	contentArea.Max.X--
	uv.NewStyledString(visible).Draw(scr, contentArea)

	scrollbar := common.Scrollbar(m.com.Styles, viewportHeight, len(lines), viewportHeight, offset)
	if scrollbar != "" {
		scrollbarArea := bodyArea
		scrollbarArea.Min.X = scrollbarArea.Max.X - 1
		uv.NewStyledString(scrollbar).Draw(scr, scrollbarArea)
	}
}

func (m *UI) sidebarHeader(width int) string {
	t := m.com.Styles
	cwd := common.PrettyPath(t, m.com.Workspace.WorkingDir(), width)
	sidebarLogo := m.sidebarLogo
	if sidebarLogo == "" {
		sidebarLogo = renderSidebarLogo(m.com.Styles, true, m.com.IsHyper(), width)
	}
	blocks := []string{
		sidebarLogo,
		cwd,
		"",
		m.modelInfo(width),
		"",
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		blocks...,
	)
}

func (m *UI) sidebarContent(width int) string {
	filesCount := 0
	for _, f := range m.sessionFiles {
		if f.Additions == 0 && f.Deletions == 0 {
			continue
		}
		filesCount++
	}

	mcpsCount := 0
	for _, mcpCfg := range m.com.Config().MCP.Sorted() {
		if _, ok := m.mcpStates[mcpCfg.Name]; ok {
			mcpsCount++
		}
	}

	return lipgloss.NewStyle().MaxWidth(width).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.sourcesInfo(width),
			"",
			m.filesInfo(m.com.Workspace.WorkingDir(), width, max(filesCount, 1), true),
			"",
			m.lspInfo(width, max(len(m.lspStates), 1), true),
			"",
			m.mcpInfo(width, max(mcpsCount, 1), true),
			"",
			m.skillsInfo(width, max(len(m.skillStatusItems()), 1), true),
		),
	)
}

func (m *UI) scrollSidebar(lines int) {
	if m.session == nil || m.layout.sidebar.Dx() <= 1 {
		return
	}
	headerHeight := lipgloss.Height(m.sidebarHeader(m.layout.sidebar.Dx()))
	viewportHeight := max(0, m.layout.sidebar.Dy()-headerHeight)
	contentHeight := lipgloss.Height(m.sidebarContent(m.layout.sidebar.Dx() - 1))
	maxOffset := max(0, contentHeight-viewportHeight)
	m.sidebarScrollOffset = min(max(m.sidebarScrollOffset+lines, 0), maxOffset)
}
