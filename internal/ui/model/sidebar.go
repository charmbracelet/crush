package model

import (
	"cmp"
	"context"
	"fmt"
	"image"

	"charm.land/lipgloss/v2"
	"github.com/taigrr/crush/internal/ui/common"
	"github.com/taigrr/crush/internal/ui/logo"
	"github.com/taigrr/crush/internal/worktree"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/layout"
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

			// Show context mode if the model supports 1M context and mode is non-standard.
			if model.CatwalkCfg.Supports1MContext && model.ModelCfg.ContextMode != "" && model.ModelCfg.ContextMode != "standard" {
				var modeStr string
				if model.ModelCfg.ContextMode == "dynamic" && m.session != nil &&
					m.com.Workspace.AgentIsExtendedContext(m.session.ID) {
					modeStr = "1M Active"
				} else {
					modeStr = common.FormatContextMode(string(model.ModelCfg.ContextMode))
				}
				if reasoningInfo != "" {
					reasoningInfo += " · " + modeStr
				} else {
					reasoningInfo = modeStr
				}
			}
		}
	}

	var modelContext *common.ModelContextInfo
	if model != nil && m.session != nil {
		// Use extended (1M) window when extended/dynamic mode is enabled.
		ctxWindow := model.CatwalkCfg.ContextWindow
		if model.CatwalkCfg.Supports1MContext {
			switch model.ModelCfg.ContextMode {
			case "extended", "dynamic":
				ctxWindow = 1_000_000
			}
		}
		modelContext = &common.ModelContextInfo{
			ContextUsed:  m.session.CompletionTokens + m.session.PromptTokens,
			Cost:         m.session.Cost,
			ModelContext: ctxWindow,
		}
	}
	var modelName string
	var rainbow bool
	if model != nil {
		modelName = model.CatwalkCfg.Name
		rainbow = model.CatwalkCfg.Supports1MContext && model.ModelCfg.ContextMode == "dynamic"
	}
	return common.ModelInfo(m.com.Styles, modelName, providerName, reasoningInfo, modelContext, width, m.hyperCredits, rainbow)
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

// sidebar renders the chat sidebar containing session title, working
// directory, model info, file list, LSP status, and MCP status.
func (m *UI) drawSidebar(scr uv.Screen, area uv.Rectangle) {
	if m.session == nil {
		return
	}

	const logoHeightBreakpoint = 30

	t := m.com.Styles
	width := area.Dx()
	height := area.Dy()

	title := t.Sidebar.SessionTitle.Width(width).MaxHeight(2).Render(m.session.Title)
	// Use BaseDir to show project root, not worktree path.
	cwd := common.PrettyPath(t, m.com.Workspace.BaseDir(), width)
	sidebarLogo := m.sidebarLogo
	if height < logoHeightBreakpoint {
		sidebarLogo = logo.SmallRender(m.com.Styles, width, logo.Opts{
			Hyper: m.com.IsHyper(),
		})
	}

	// Build git/worktree info line.
	// If in a worktree, show worktree name; otherwise show git branch.
	var gitInfo string
	var activeWorktree *worktree.Worktree
	if m.com.Workspace.WorktreesEnabled() {
		activeWorktree, _ = m.com.Workspace.GetActiveWorktree(context.Background(), m.session.ID)
	}
	if activeWorktree != nil {
		gitInfo = t.Sidebar.WorkingDir.Render("⑂ " + activeWorktree.Name)
	} else if branch := m.com.Workspace.GitBranch(); branch != "" {
		gitInfo = t.Sidebar.WorkingDir.Render(" " + branch)
	}

	blocks := []string{
		sidebarLogo,
		title,
		"",
		cwd,
	}
	if gitInfo != "" {
		blocks = append(blocks, gitInfo)
	}
	blocks = append(
		blocks,
		"",
		m.modelInfo(width),
		"",
	)

	sidebarHeader := lipgloss.JoinVertical(
		lipgloss.Left,
		blocks...,
	)

	var remainingHeightArea image.Rectangle
	layout.Vertical(
		layout.Len(lipgloss.Height(sidebarHeader)),
		layout.Fill(1),
	).Split(m.layout.sidebar).Assign(new(image.Rectangle), &remainingHeightArea)
	remainingHeight := remainingHeightArea.Dy() - 6
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

	maxFiles, maxLSPs, maxMCPs, maxSkills := getDynamicHeightLimits(remainingHeight, filesCount, lspsCount, mcpsCount, skillsCount)

	lspSection := m.lspInfo(width, maxLSPs, true)
	mcpSection := m.mcpInfo(width, maxMCPs, true)
	skillsSection := m.skillsInfo(width, maxSkills, true)
	filesSection := m.filesInfo(m.com.Workspace.WorkingDir(), width, maxFiles, true)

	uv.NewStyledString(
		lipgloss.NewStyle().
			MaxWidth(width).
			MaxHeight(height).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					sidebarHeader,
					filesSection,
					"",
					lspSection,
					"",
					mcpSection,
					"",
					skillsSection,
				),
			),
	).Draw(scr, area)
}
