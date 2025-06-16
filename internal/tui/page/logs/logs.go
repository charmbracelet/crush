package logs

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	logsComponents "github.com/charmbracelet/crush/internal/tui/components/logs"
	"github.com/charmbracelet/crush/internal/tui/page"
	"github.com/charmbracelet/crush/internal/tui/page/chat"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

var LogsPage page.PageID = "logs"

type LogPage interface {
	util.Model
	layout.Sizeable
}

type logsPage struct {
	width, height int
	table         logsComponents.TableComponent
	details       logsComponents.DetailComponent
	keyMap        KeyMap
}

func (p *logsPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		return p, p.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, p.keyMap.Back):
			return p, util.CmdHandler(page.PageChangeMsg{ID: chat.ChatPage})
		}
	}

	table, cmd := p.table.Update(msg)
	cmds = append(cmds, cmd)
	p.table = table.(logsComponents.TableComponent)
	details, cmd := p.details.Update(msg)
	cmds = append(cmds, cmd)
	p.details = details.(logsComponents.DetailComponent)

	return p, tea.Batch(cmds...)
}

func (p *logsPage) View() tea.View {
	baseStyle := styles.CurrentTheme().S().Base
	style := baseStyle.Width(p.width).Height(p.height).Padding(1)
	title := core.Title("Logs", p.width-2)

	return tea.NewView(
		style.Render(
			lipgloss.JoinVertical(lipgloss.Top,
				title,
				p.details.View().String(),
				p.table.View().String(),
			),
		),
	)
}

// GetSize implements LogPage.
func (p *logsPage) GetSize() (int, int) {
	return p.width, p.height
}

// SetSize implements LogPage.
func (p *logsPage) SetSize(width int, height int) tea.Cmd {
	p.width = width
	p.height = height
	availableHeight := height - 2 // Padding for top and bottom
	availableHeight -= 1          // title height
	return tea.Batch(
		p.table.SetSize(width-2, availableHeight/2),
		p.details.SetSize(width-2, availableHeight/2),
	)
}

func (p *logsPage) Init() tea.Cmd {
	return tea.Batch(
		p.table.Init(),
		p.details.Init(),
	)
}

func NewLogsPage() LogPage {
	return &logsPage{
		details: logsComponents.NewLogsDetails(),
		table:   logsComponents.NewLogsTable(),
		keyMap:  DefaultKeyMap(),
	}
}
