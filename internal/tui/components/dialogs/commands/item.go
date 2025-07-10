package commands

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/core/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/x/ansi"
)

type ItemSection interface {
	util.Model
	layout.Sizeable
	list.SectionHeader
	SetInfo(info string)
}
type itemSectionModel struct {
	width int
	title string
	info  string
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

func (m *itemSectionModel) View() string {
	t := styles.CurrentTheme()
	title := ansi.Truncate(m.title, m.width-2, "…")
	style := t.S().Base.Padding(1, 1, 0, 1)
	title = t.S().Muted.Render(title)
	section := ""
	if m.info != "" {
		section = core.SectionWithInfo(title, m.width-2, m.info)
	} else {
		section = core.Section(title, m.width-2)
	}

	return style.Render(section)
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

func (m *itemSectionModel) SetInfo(info string) {
	m.info = info
}
