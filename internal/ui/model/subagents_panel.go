package model

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
)

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

	items := make([]skillStatusItem, 0, len(m.runningSubagents))
	for _, e := range m.runningSubagents {
		tokens := e.PromptTokens + e.CompletionTokens
		desc := e.Model
		if tokens > 0 {
			desc = fmt.Sprintf("%s %s", e.Model, t.Resource.AdditionalText.Render(fmt.Sprintf("%d tok", tokens)))
		}
		items = append(items, skillStatusItem{
			icon:        t.SubagentDot(e.Color),
			name:        e.Name,
			title:       t.Resource.Name.Render(e.Name),
			description: desc,
		})
	}

	list := skillsList(t, items, width, maxItems)
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}
