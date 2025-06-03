package commands

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/tui/components/core"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type ItemSection interface {
	util.Model
	layout.Sizeable
	list.SectionHeader
}
type itemSectionModel struct {
	width     int
	title     string
	noPadding bool // No padding for the section header
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
	t := styles.CurrentTheme()
	title := ansi.Truncate(m.title, m.width-2, "…")
	style := t.S().Base.Padding(1, 1, 0, 1)
	title = t.S().Muted.Render(title)
	return tea.NewView(style.Render(core.Section(title, m.width-2)))
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
