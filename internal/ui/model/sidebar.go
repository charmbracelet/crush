package model

import (
	"cmp"
	"fmt"
	"image"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/logo"
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

// getDynamicHeightLimits will give us the num of items to show in each section based on the height
// some items are more important than others.
func getDynamicHeightLimits(availableHeight, fileCount, lspCount, mcpCount, skillCount int) (maxFiles, maxLSPs, maxMCPs, maxSkills int) {
	const (
		minItemsPerSection = 2
		// Keep these high so dynamic layout uses available sidebar space
		// instead of hitting small hard limits.
		defaultMaxFilesShown    = 1000
		defaultMaxLSPsShown     = 1000
		defaultMaxMCPsShown     = 1000
		defaultMaxSkillsShown   = 1000
		minAvailableHeightLimit = 10
	)

	if availableHeight < minAvailableHeightLimit {
		return minItemsPerSection, minItemsPerSection, minItemsPerSection, minItemsPerSection
	}

	maxFiles = minItemsPerSection
	maxLSPs = minItemsPerSection
	maxMCPs = minItemsPerSection
	maxSkills = minItemsPerSection

	remainingHeight := max(0, availableHeight-(minItemsPerSection*4))

	sectionValues := []*int{&maxFiles, &maxLSPs, &maxMCPs, &maxSkills}
	sectionCaps := []int{defaultMaxFilesShown, defaultMaxLSPsShown, defaultMaxMCPsShown, defaultMaxSkillsShown}
	sectionNeeds := []int{max(0, fileCount-maxFiles), max(0, lspCount-maxLSPs), max(0, mcpCount-maxMCPs), max(0, skillCount-maxSkills)}

	for remainingHeight > 0 {
		allocated := false
		for i, section := range sectionValues {
			if remainingHeight == 0 {
				break
			}
			if sectionNeeds[i] == 0 || *section >= sectionCaps[i] {
				continue
			}
			*section = *section + 1
			sectionNeeds[i]--
			remainingHeight--
			allocated = true
		}
		if !allocated {
			break
		}
	}

	for remainingHeight > 0 {
		allocated := false
		for i, section := range sectionValues {
			if remainingHeight == 0 {
				break
			}
			if *section >= sectionCaps[i] {
				continue
			}
			*section = *section + 1
			remainingHeight--
			allocated = true
		}
		if !allocated {
			break
		}
	}

	return maxFiles, maxLSPs, maxMCPs, maxSkills
}

// sidebarMaxOffset returns the maximum sidebar scroll offset based on
// the last drawn content height. The value is computed during drawSidebar.
func (m *UI) sidebarMaxOffset() int {
	return m.sidebarMaxOffsetVal
}

// drawSidebar renders the chat sidebar with virtual scrolling and an
// auto-hiding scrollbar. While the sidebar is focused, the scrollbar stays
// visible.
func (m *UI) drawSidebar(scr uv.Screen, area uv.Rectangle) {
	if m.session == nil {
		return
	}

	const logoHeightBreakpoint = 30

	t := m.com.Styles
	width := area.Dx()
	height := area.Dy()

	title := t.Sidebar.SessionTitle.Width(width).MaxHeight(2).Render(m.session.Title)
	cwd := common.PrettyPath(t, m.com.Workspace.WorkingDir(), width)
	sidebarLogo := m.sidebarLogo
	if height < logoHeightBreakpoint {
		sidebarLogo = logo.SmallRender(m.com.Styles, width, logo.Opts{
			Hyper: m.com.IsHyper(),
		})
	}
	blocks := []string{
		sidebarLogo,
		title,
		"",
		cwd,
		"",
		m.modelInfo(width),
		"",
	}

	sidebarHeader := lipgloss.JoinVertical(
		lipgloss.Left,
		blocks...,
	)

	// Give a very large available height so all items are rendered.
	const maxAvailableHeight = 100000
	filesCount := 0
	for _, f := range m.sessionFiles {
		if f.Additions == 0 && f.Deletions == 0 {
			continue
		}
		filesCount++
	}

	lspsCount := len(m.lspStates)

	mcpsCount := 0
	for _, mcpCfg := range m.com.Config().MCP.Sorted() {
		if _, ok := m.mcpStates[mcpCfg.Name]; ok {
			mcpsCount++
		}
	}

	skillsCount := len(m.skillStatusItems())

	maxFiles, maxLSPs, maxMCPs, maxSkills := getDynamicHeightLimits(maxAvailableHeight, filesCount, lspsCount, mcpsCount, skillsCount)

	lspSection := m.lspInfo(width, maxLSPs, true)
	mcpSection := m.mcpInfo(width, maxMCPs, true)
	skillsSection := m.skillsInfo(width, maxSkills, true)
	filesSection := m.filesInfo(m.com.Workspace.WorkingDir(), width, maxFiles, true)

	// Build the full sidebar content.
	fullContent := lipgloss.JoinVertical(
		lipgloss.Left,
		sidebarHeader,
		filesSection,
		"",
		lspSection,
		"",
		mcpSection,
		"",
		skillsSection,
	)

	// Split into lines for virtual scrolling.
	lines := strings.Split(fullContent, "\n")
	// Update scrollable flag and store max offset.
	totalLines := len(lines)
	m.sidebarScrollable = totalLines > height
	m.sidebarMaxOffsetVal = max(0, totalLines-height)

	// Clamp sidebarOffset.
	maxOffset := m.sidebarMaxOffsetVal
	if m.sidebarOffset > maxOffset {
		m.sidebarOffset = maxOffset
	}

	// Slice visible lines.
	end := min(m.sidebarOffset+height, totalLines)
	visibleLines := lines[m.sidebarOffset:end]
	visibleStr := strings.Join(visibleLines, "\n")

	// Determine scrollbar visibility: always visible when focused, otherwise
	// auto-hide.
	scrollbarVisible := totalLines > height && (m.sidebarScrollbarVisible || m.focus == uiFocusSidebar)

	// Draw content.
	contentWidth := width
	if scrollbarVisible {
		contentWidth = width - 1
	}

	uv.NewStyledString(
		lipgloss.NewStyle().
			MaxWidth(contentWidth).
			MaxHeight(height).
			Render(visibleStr),
	).Draw(scr, area)

	// Draw scrollbar if visible.
	if scrollbarVisible {
		scrollbar := common.Scrollbar(m.com.Styles, height, totalLines, height, m.sidebarOffset)
		if scrollbar != "" {
			scrollbarArea := image.Rectangle{
				Min: image.Point{X: area.Max.X - 1, Y: area.Min.Y},
				Max: area.Max,
			}
			uv.NewStyledString(scrollbar).Draw(scr, scrollbarArea)
		}
	}
}
