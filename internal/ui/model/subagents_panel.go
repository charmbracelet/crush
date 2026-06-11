package model

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

type subagentStatusItem struct {
	icon        string
	name        string
	title       string
	description string
}

// subagentsInfo renders the running subagents status section.
func (m *UI) subagentsInfo(width, maxItems int, isSection bool) string {
	t := m.com.Styles

	title := t.Resource.Heading.Render("Subagents")
	if isSection {
		title = common.Section(t, title, width)
	}

	if len(m.runningSubagents) == 0 {
		list := t.Resource.AdditionalText.Render("None")
		return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
	}

	items := make([]subagentStatusItem, 0, len(m.runningSubagents))
	for _, e := range m.runningSubagents {
		tokens := e.PromptTokens + e.CompletionTokens
		desc := e.Model
		if tokens > 0 {
			desc = fmt.Sprintf("%s %s", e.Model, t.Resource.AdditionalText.Render(fmt.Sprintf("%d tok", tokens)))
		}
		items = append(items, subagentStatusItem{
			icon:        styles.SubagentDot(e.Color),
			name:        e.Name,
			title:       t.Resource.Name.Render(e.Name),
			description: desc,
		})
	}

	list := subagentsList(t, items, width, maxItems)
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

func subagentsList(t *styles.Styles, items []subagentStatusItem, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}

	if len(items) > maxItems {
		visibleItems := items[:maxItems-1]
		remaining := len(items) - (maxItems - 1)
		items = append(visibleItems, subagentStatusItem{
			name:  "more",
			title: t.Resource.AdditionalText.Render(fmt.Sprintf("…and %d more", remaining)),
		})
	}

	renderedItems := make([]string, 0, len(items))
	for _, item := range items {
		renderedItems = append(renderedItems, common.Status(t, common.StatusOpts{
			Icon:        item.icon,
			Title:       item.title,
			Description: item.description,
		}, width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, renderedItems...)
}
