package model

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
)

func (m *UI) sourcesInfo(width int) string {
	t := m.com.Styles
	title := common.Section(t, "Sources", width)
	if len(m.session.Sources) == 0 {
		return lipgloss.NewStyle().Width(width).Render(
			fmt.Sprintf("%s\n\n%s", title, t.Resource.AdditionalText.Render("None")),
		)
	}

	items := make([]string, 0, len(m.session.Sources))
	for _, source := range m.session.Sources {
		items = append(items, common.Status(t, common.StatusOpts{
			Icon:        t.Resource.OnlineIcon.String(),
			Title:       t.Resource.Name.Render(source.Label),
			Description: t.Resource.StatusText.Render(string(source.Kind)),
		}, width))
	}
	return lipgloss.NewStyle().Width(width).Render(
		fmt.Sprintf("%s\n\n%s", title, lipgloss.JoinVertical(lipgloss.Left, items...)),
	)
}
