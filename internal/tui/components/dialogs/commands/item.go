package commands

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type ItemSection interface {
	util.Model
	layout.Sizeable
	list.SectionHeader
}
type itemSectionModel struct {
	width int
	title string
}

func NewItemSection(title string) ItemSection {
	return &itemSectionModel{
		title: title,
	}
}

func (m *itemSectionModel) Init() tea.Cmd {
	return nil
}

func (m *itemSectionModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m *itemSectionModel) View() tea.View {
	t := theme.CurrentTheme()
	title := ansi.Truncate(m.title, m.width-1, "…")
	style := styles.BaseStyle().Padding(1, 0, 0, 0).Width(m.width).Foreground(t.TextMuted()).Bold(true)
	if len(title) < m.width {
		remainingWidth := m.width - lipgloss.Width(title)
		if remainingWidth > 0 {
			title += " " + strings.Repeat("─", remainingWidth-1)
		}
	}
	return tea.NewView(style.Render(title))
}

func (m *itemSectionModel) GetSize() (int, int) {
	return m.width, 1
}

func (m *itemSectionModel) SetSize(width int, height int) tea.Cmd {
	m.width = width
	return nil
}

func (m *itemSectionModel) IsSectionHeader() bool {
	return true
}
