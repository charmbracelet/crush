package status

import (
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/x/ansi"
)

type StatusCmp interface {
	util.Model
	ToggleFullHelp()
	SetKeyMap(keyMap help.KeyMap)
}

type statusCmp struct {
	info       util.InfoMsg
	width      int
	messageTTL time.Duration
	help       help.Model
	keyMap     help.KeyMap
}

// clearMessageCmd is a command that clears status messages after a timeout
func (m *statusCmp) clearMessageCmd(ttl time.Duration) tea.Cmd {
	return tea.Tick(ttl, func(time.Time) tea.Msg {
		return util.ClearStatusMsg{}
	})
}

func (m *statusCmp) Init() tea.Cmd {
	return nil
}

func (m *statusCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.help.SetWidth(msg.Width - 2)
		return m, nil

	// Handle status info
	case util.InfoMsg:
		m.info = msg
		ttl := msg.TTL
		if ttl == 0 {
			ttl = m.messageTTL
		}
		return m, m.clearMessageCmd(ttl)
	case util.ClearStatusMsg:
		m.info = util.InfoMsg{}
	}
	return m, nil
}

func (m *statusCmp) View() string {
	t := styles.CurrentTheme()
	status := t.S().Base.Padding(0, 1, 1, 1).Render(m.help.View(m.keyMap))
	if m.info.Msg != "" {
		status = m.infoMsg()
	}
	return status
}

func (m *statusCmp) infoMsg() string {
	t := styles.CurrentTheme()
	message := ""
	infoType := ""
	switch m.info.Type {
	case util.InfoTypeError:
		infoType = t.S().Base.Background(t.Red).Padding(0, 1).Render("ERROR")
		widthLeft := m.width - (lipgloss.Width(infoType) + 2)
		info := ansi.Truncate(m.info.Msg, widthLeft, "…")
		message = t.S().Base.Background(t.Error).Width(widthLeft+2).Foreground(t.White).Padding(0, 1).Render(info)
	case util.InfoTypeWarn:
		infoType = t.S().Base.Foreground(t.BgOverlay).Background(t.Yellow).Padding(0, 1).Render("WARNING")
		widthLeft := m.width - (lipgloss.Width(infoType) + 2)
		info := ansi.Truncate(m.info.Msg, widthLeft, "…")
		message = t.S().Base.Foreground(t.BgOverlay).Width(widthLeft+2).Background(t.Warning).Padding(0, 1).Render(info)
	default:
		infoType = t.S().Base.Foreground(t.BgSubtle).Background(t.Green).Padding(0, 1).Bold(true).Render("HEY!")
		widthLeft := m.width - (lipgloss.Width(infoType) + 2)
		info := ansi.Truncate(m.info.Msg, widthLeft, "…")
		message = t.S().Base.Background(t.GreenDark).Width(widthLeft+2).Foreground(t.BgSubtle).Padding(0, 1).Render(info)
	}
	return ansi.Truncate(infoType+message, m.width, "…")
}

func (m *statusCmp) ToggleFullHelp() {
	m.help.ShowAll = !m.help.ShowAll
}

func (m *statusCmp) SetKeyMap(keyMap help.KeyMap) {
	m.keyMap = keyMap
}

func NewStatusCmp() StatusCmp {
	t := styles.CurrentTheme()
	help := help.New()
	help.Styles = t.S().Help
	return &statusCmp{
		messageTTL: 5 * time.Second,
		help:       help,
	}
}
