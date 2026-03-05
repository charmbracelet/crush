package model

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

type skillStatusItem struct {
	icon  string
	title string
	// description is reserved for future use (e.g. showing error details).
	description string
}

// skillsInfo renders the skill discovery status section showing loaded and
// invalid skills.
func (m *UI) skillsInfo(width, maxItems int, isSection bool) string {
	t := m.com.Styles

	title := t.ResourceGroupTitle.Render("Skills")
	if isSection {
		title = common.Section(t, title, width)
	}

	items := m.skillStatusItems()
	if len(items) == 0 {
		list := t.ResourceAdditionalText.Render("None")
		return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
	}

	list := skillsList(t, items, width, maxItems)
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

func (m *UI) skillStatusItems() []skillStatusItem {
	t := m.com.Styles
	var items []skillStatusItem

	states := slices.Clone(m.skillStates)
	slices.SortFunc(states, func(a, b *skills.SkillState) int {
		return strings.Compare(a.Path, b.Path)
	})
	for _, state := range states {
		title := state.Name
		if title == "" {
			title = filepath.Base(state.Path)
		}
		if title == skills.SkillFileName {
			title = filepath.Base(filepath.Dir(state.Path))
		}
		icon := t.ResourceOnlineIcon.String()
		if state.State == skills.StateError {
			icon = t.ResourceErrorIcon.String()
		}
		items = append(items, skillStatusItem{
			icon:  icon,
			title: t.ResourceName.Render(title),
		})
	}

	return items
}

func skillsList(t *styles.Styles, items []skillStatusItem, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}

	if len(items) > maxItems {
		visibleItems := items[:maxItems-1]
		remaining := len(items) - maxItems
		items = append(visibleItems, skillStatusItem{
			title: t.ResourceAdditionalText.Render(fmt.Sprintf("…and %d more", remaining)),
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
