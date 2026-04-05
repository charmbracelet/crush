package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent"
	sessionpkg "github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/logo"
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

			// Only check reasoning if model can reason.
			if model.CatwalkCfg.CanReason {
				effectiveEffort := model.CatwalkCfg.DefaultReasoningEffort
				if len(model.CatwalkCfg.ReasoningLevels) == 0 {
					// Anthropic-style thinking models. Think==nil or true means on
					// (default), Think==&false means explicitly disabled.
					thinkingDisabled := model.ModelCfg.Think != nil && !*model.ModelCfg.Think
					if thinkingDisabled {
						reasoningInfo = "Thinking Off"
					} else {
						displayEffort := effectiveEffort
						if displayEffort == "" {
							displayEffort = "high"
						}
						reasoningInfo = fmt.Sprintf("Thinking On (%s)", common.FormatReasoningEffort(displayEffort))
					}
				} else {
					// Models with explicit reasoning levels (e.g. OpenAI).
					thinkingDisabled := model.ModelCfg.Think != nil && !*model.ModelCfg.Think
					if thinkingDisabled {
						reasoningInfo = "Reasoning Off"
					} else {
						displayEffort := effectiveEffort
						if displayEffort == "" {
							displayEffort = "high"
						}
						reasoningInfo = fmt.Sprintf("Reasoning %s", common.FormatReasoningEffort(displayEffort))
					}
				}
			}
		}
	}

	var modelContext *common.ModelContextInfo
	if model != nil && m.session != nil {
		modelContext = &common.ModelContextInfo{
			InputTokens:  m.session.LastInputTokens(),
			OutputTokens: m.session.LastOutputTokens(),
			Cost:         m.session.Cost,
			ModelContext: effectiveContextWindow(*model),
		}
	}
	info := common.ModelInfo(m.com.Styles, model.CatwalkCfg.Name, providerName, reasoningInfo, modelContext, width)
	modeLine := m.modeInfo(width)
	if modeLine == "" {
		return info
	}
	return lipgloss.JoinVertical(lipgloss.Left, info, modeLine)
}

func effectiveContextWindow(model agent.Model) int64 {
	window := model.CatwalkCfg.ContextWindow
	options := model.CatwalkCfg.Options.ProviderOptions
	if options == nil {
		return window
	}
	value, ok := options["max_prompt_tokens"]
	if !ok {
		return window
	}
	maxPromptTokens, ok := contextWindowInt64(value)
	if !ok || maxPromptTokens <= 0 {
		return window
	}
	if window <= 0 {
		return maxPromptTokens
	}
	return min(window, maxPromptTokens)
}

func contextWindowInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case json.Number:
		parsed, err := v.Int64()
		if err == nil {
			return parsed, true
		}
		f, ferr := v.Float64()
		if ferr != nil {
			return 0, false
		}
		return int64(f), true
	default:
		return 0, false
	}
}

func (m *UI) modeInfo(width int) string {
	if m.session == nil || m.com.App == nil {
		return ""
	}

	modes := make([]string, 0, 2)
	modes = append(modes, "SESSION "+strings.ToUpper(m.sessionRoleLabel(m.session)))
	if m.session.CollaborationMode == sessionpkg.CollaborationModePlan {
		modes = append(modes, "PLAN")
	} else {
		switch m.currentExecutionMode() {
		case executionModeAuto:
			modes = append(modes, "AUTO")
		case executionModeYolo:
			modes = append(modes, "YOLO")
		default:
			modes = append(modes, "ASK")
		}
	}
	if len(modes) == 0 {
		return ""
	}

	text := fmt.Sprintf("Modes: %s", strings.Join(modes, " | "))
	return m.com.Styles.Subtle.PaddingLeft(2).Width(width).Render(text)
}

// getDynamicHeightLimits will give us the num of items to show in each section based on the hight
// some items are more important than others.
func getDynamicHeightLimits(availableHeight int) (maxFiles, maxLSPs, maxMCPs, maxTimeline int) {
	const (
		minItemsPerSection      = 2
		defaultMaxFilesShown    = 10
		defaultMaxLSPsShown     = 8
		defaultMaxMCPsShown     = 8
		defaultMaxTimelineShown = 6
		minAvailableHeightLimit = 12
	)

	// If we have very little space, use minimum values.
	if availableHeight < minAvailableHeightLimit {
		return minItemsPerSection, minItemsPerSection, minItemsPerSection, minItemsPerSection
	}

	// Distribute available height among the four sections.
	// Give priority to files, then LSPs, MCPs, and timeline.
	totalSections := 4
	heightPerSection := availableHeight / totalSections

	// Calculate limits for each section, ensuring minimums.
	maxFiles = max(minItemsPerSection, min(defaultMaxFilesShown, heightPerSection))
	maxLSPs = max(minItemsPerSection, min(defaultMaxLSPsShown, heightPerSection))
	maxMCPs = max(minItemsPerSection, min(defaultMaxMCPsShown, heightPerSection))
	maxTimeline = max(minItemsPerSection, min(defaultMaxTimelineShown, heightPerSection))

	// If we have extra space, give it to files first.
	remainingHeight := availableHeight - (maxFiles + maxLSPs + maxMCPs + maxTimeline)
	if remainingHeight > 0 {
		extraForFiles := min(remainingHeight, defaultMaxFilesShown-maxFiles)
		maxFiles += extraForFiles
		remainingHeight -= extraForFiles

		if remainingHeight > 0 {
			extraForLSPs := min(remainingHeight, defaultMaxLSPsShown-maxLSPs)
			maxLSPs += extraForLSPs
			remainingHeight -= extraForLSPs

			if remainingHeight > 0 {
				extraForMCPs := min(remainingHeight, defaultMaxMCPsShown-maxMCPs)
				maxMCPs += extraForMCPs
				remainingHeight -= extraForMCPs

				if remainingHeight > 0 {
					maxTimeline += min(remainingHeight, defaultMaxTimelineShown-maxTimeline)
				}
			}
		}
	}

	return maxFiles, maxLSPs, maxMCPs, maxTimeline
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

	title := t.Muted.Width(width).MaxHeight(2).Render(m.session.Title)
	cwd := common.PrettyPath(t, m.com.Store().WorkingDir(), width)
	sidebarLogo := m.sidebarLogo
	if height < logoHeightBreakpoint {
		sidebarLogo = logo.SmallRender(m.com.Styles, width)
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

	_, remainingHeightArea := layout.SplitVertical(m.layout.sidebar, layout.Fixed(lipgloss.Height(sidebarHeader)))
	remainingHeight := remainingHeightArea.Dy() - 12
	maxFiles, maxLSPs, maxMCPs, maxTimeline := getDynamicHeightLimits(remainingHeight)

	lspSection := m.lspInfo(width, maxLSPs, true)
	mcpSection := m.mcpInfo(width, maxMCPs, true)
	filesSection := m.filesInfo(m.com.Store().WorkingDir(), width, maxFiles, true)
	timelineSection := m.timelineInfo(width, maxTimeline, true)

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
					timelineSection,
				),
			),
	).Draw(scr, area)
}
