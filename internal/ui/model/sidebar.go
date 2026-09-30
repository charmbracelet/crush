package model

import (
	"cmp"
	"fmt"
	"image"
	"strings"

	"charm.land/lipgloss/v2"
	mcp "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
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
					if m.status.hasRouterDecision && m.status.routerDecision.ReasoningEffort != "" {
						// The router overrides this per-message; label the
						// static config value as the default so it isn't
						// read as the effort actually used last message —
						// that's the Router line below.
						reasoningInfo += " (default)"
					}
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

// routerInfo renders the model router's live status as a single line
// styled like the model block's reasoning line, so it sits directly under
// the model/provider/reasoning info rather than as its own sidebar
// section: a "consulting" indicator naming the router's configured model
// while a call is in flight, or the most recently applied decision
// (chosen model and/or reasoning effort, with confidence) once one
// exists. Returns "" when the router has never been consulted for this
// session, so the model block gets no extra line at all.
func (m *UI) routerInfo(width int) string {
	t := m.com.Styles

	var body string
	switch {
	case m.routerQuerying:
		if m.routerQueryingModel != "" {
			body = fmt.Sprintf("Router: consulting %s…", m.routerQueryingModel)
		} else {
			body = "Router: consulting…"
		}
	case m.status.hasRouterDecision:
		d := m.status.routerDecision
		var parts []string
		if d.ModelID != "" {
			parts = append(parts, shortModelName(d.ModelID))
		}
		if d.ReasoningEffort != "" {
			parts = append(parts, d.ReasoningEffort)
		}
		if len(parts) == 0 {
			return ""
		}
		body = "Router: " + strings.Join(parts, " · ")
		if d.Confidence > 0 {
			body += fmt.Sprintf(" (%.0f%%)", d.Confidence*100)
		}
		if m.routerModel != "" {
			body += fmt.Sprintf(" via %s", shortModelName(m.routerModel))
		}
	case m.routerError != "":
		body = fmt.Sprintf("Router: error — %s", m.routerError)
		return lipgloss.NewStyle().Width(width).Render(t.LSP.ErrorDiagnostic.Render(body))
	default:
		return ""
	}

	return lipgloss.NewStyle().Width(width).Render(t.ModelInfo.Reasoning.Render(body))
}

// routerSavingsInfo renders the router's cumulative estimated dollar
// figure for the whole session as its own line, separate from
// routerInfo's per-message decision line — a session-level running
// total isn't "part of" any one decision, and mixing the two made the
// total hard to spot at the end of a longer decision line. Shown
// whenever the total is non-zero, regardless of whether the router is
// currently mid-call, just applied a decision, or just failed — the
// accumulated total from earlier in the session stays relevant either
// way. Returns "" when nothing has been saved or cost yet (e.g. the
// router has never been consulted, or model_pool isn't configured so
// there's nothing but reasoning-effort routing to price).
func (m *UI) routerSavingsInfo(width int) string {
	if m.routerSavings == 0 {
		return ""
	}
	t := m.com.Styles

	var body string
	if m.routerSavings > 0 {
		body = fmt.Sprintf("Router savings (session): $%.4f", m.routerSavings)
	} else {
		// A negative cumulative value means the router's choices cost
		// more than the session's default model would have, e.g.
		// escalating to a pricier model for "high" effort more often
		// than it saved by dropping to "low".
		body = fmt.Sprintf("Router savings (session): -$%.4f", -m.routerSavings)
	}
	return lipgloss.NewStyle().Width(width).Render(t.ModelInfo.Reasoning.Render(body))
}

// updateSidebarScrollState renders the sidebar content and computes scroll
// state (scrollability, max offset, clamp) before drawing. This keeps all
// state mutation in the update path rather than in the draw function.
func (m *UI) updateSidebarScrollState() {
	if m.session == nil || m.isCompact {
		return
	}

	const logoHeightBreakpoint = 30

	t := m.com.Styles
	width := m.layout.sidebar.Dx()
	height := m.layout.sidebar.Dy()

	contentWidth := max(width-2, 1)

	title := t.Sidebar.SessionTitle.Width(contentWidth).MaxHeight(2).Render(m.session.Title)
	cwd := common.PrettyPathWithBranch(t, m.com.Workspace.WorkingDir(), m.gitBranch, contentWidth)
	sidebarLogo := m.sidebarLogo
	if height < logoHeightBreakpoint {
		sidebarLogo = lipgloss.JoinVertical(lipgloss.Left, logo.SmallRender(m.com.Styles, contentWidth, logo.Opts{
			Hyper: m.com.IsHyper(),
		}), "")
	}

	var logoRect, contentRect image.Rectangle
	layout.Vertical(
		layout.Len(lipgloss.Height(sidebarLogo)),
		layout.Fill(1),
	).Split(m.layout.sidebar).Assign(&logoRect, &contentRect)

	contentHeight := contentRect.Dy()

	// Render all items without truncation; virtual scrolling handles overflow.
	lspSection := m.lspInfo(contentWidth, len(m.lspStates), true)
	mcpSection := m.mcpInfo(contentWidth, mcpCount(m.com.Config().MCP.Sorted(), m.mcpStates), true)
	skillsSection := m.skillsInfo(contentWidth, len(m.skillStatusItems()), true)
	filesSection := m.filesInfo(m.com.Workspace.WorkingDir(), contentWidth, fileChangeCount(m.sessionFiles), true)

	// Build the scrollable content.
	parts := []string{
		title,
		"",
		cwd,
		"",
		m.modelInfo(contentWidth),
	}
	if routerLine := m.routerInfo(contentWidth); routerLine != "" {
		parts = append(parts, routerLine)
	}
	if savingsLine := m.routerSavingsInfo(contentWidth); savingsLine != "" {
		parts = append(parts, savingsLine)
	}
	parts = append(parts,
		"",
		filesSection,
		"",
		lspSection,
		"",
		mcpSection,
		"",
		skillsSection,
	)
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	totalLines := strings.Count(content, "\n") + 1
	m.sidebarContent = content
	m.sidebarTotalLines = totalLines
	m.sidebarContentWidth = contentWidth
	m.sidebarContentHeight = contentHeight
	m.sidebarDrawLogo = sidebarLogo
	m.sidebarScrollable = totalLines > contentHeight
	m.sidebarMaxOffsetVal = max(0, totalLines-contentHeight)

	// If the sidebar is focused but no longer scrollable (e.g. after a
	// resize), return focus to the chat.
	if m.focus == uiFocusSidebar && !m.sidebarScrollable {
		m.focus = uiFocusMain
		m.chat.Focus()
	}

	// Clamp sidebarOffset.
	if m.sidebarOffset > m.sidebarMaxOffsetVal {
		m.sidebarOffset = m.sidebarMaxOffsetVal
	}
}

// drawSidebar renders the chat sidebar with a fixed logo and a
// virtual-scrolling content area with an auto-hiding scrollbar. While the
// sidebar is focused, the scrollbar stays visible.
func (m *UI) drawSidebar(scr uv.Screen, area uv.Rectangle) {
	if m.session == nil {
		return
	}

	sidebarLogo := m.sidebarDrawLogo
	contentWidth := m.sidebarContentWidth
	contentHeight := m.sidebarContentHeight
	totalLines := m.sidebarTotalLines

	var logoRect, contentRect image.Rectangle
	layout.Vertical(
		layout.Len(lipgloss.Height(sidebarLogo)),
		layout.Fill(1),
	).Split(area).Assign(&logoRect, &contentRect)

	// Slice visible lines.
	end := min(m.sidebarOffset+contentHeight, totalLines)
	lines := strings.Split(m.sidebarContent, "\n")
	visibleLines := lines[m.sidebarOffset:end]
	visibleStr := strings.Join(visibleLines, "\n")

	// Determine scrollbar visibility: always visible when focused, otherwise
	// auto-hide.
	scrollbarVisible := totalLines > contentHeight && (m.sidebarScrollbarVisible || m.focus == uiFocusSidebar)

	// Draw the fixed logo.
	uv.NewStyledString(
		lipgloss.NewStyle().
			MaxWidth(contentWidth).
			MaxHeight(lipgloss.Height(sidebarLogo)).
			Render(sidebarLogo),
	).Draw(scr, logoRect)

	// Draw the visible content in the scrollable area.
	uv.NewStyledString(
		lipgloss.NewStyle().
			MaxWidth(contentWidth).
			MaxHeight(contentHeight).
			Render(visibleStr),
	).Draw(scr, contentRect)

	// Draw scrollbar in the reserved column.
	if scrollbarVisible {
		scrollbar := common.Scrollbar(m.com.Styles, contentHeight, totalLines, contentHeight, m.sidebarOffset)
		if scrollbar != "" {
			scrollbarArea := image.Rectangle{
				Min: image.Point{X: area.Max.X - 1, Y: contentRect.Min.Y},
				Max: image.Point{X: area.Max.X, Y: area.Max.Y},
			}
			uv.NewStyledString(scrollbar).Draw(scr, scrollbarArea)
		}
	}
}

// fileChangeCount returns the number of session files with non-zero additions
// or deletions.
func fileChangeCount(files []SessionFile) int {
	count := 0
	for _, f := range files {
		if f.Additions == 0 && f.Deletions == 0 {
			continue
		}
		count++
	}
	return count
}

// mcpCount returns the number of MCP servers that have a state entry.
func mcpCount(mcpCfgs []config.MCP, states map[string]mcp.ClientInfo) int {
	count := 0
	for _, cfg := range mcpCfgs {
		if _, ok := states[cfg.Name]; ok {
			count++
		}
	}
	return count
}
