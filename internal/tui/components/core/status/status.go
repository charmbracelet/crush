package status

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/help"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/logging"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
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
	session    session.Session
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

func (m *statusCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.help.Width = msg.Width - 2
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

	// Handle persistent logs
	case pubsub.Event[logging.LogMessage]:
		if msg.Payload.Persist {
			switch msg.Payload.Level {
			case "error":
				m.info = util.InfoMsg{
					Type: util.InfoTypeError,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				}
			case "info":
				m.info = util.InfoMsg{
					Type: util.InfoTypeInfo,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				}
			case "warn":
				m.info = util.InfoMsg{
					Type: util.InfoTypeWarn,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				}
			default:
				m.info = util.InfoMsg{
					Type: util.InfoTypeInfo,
					Msg:  msg.Payload.Message,
					TTL:  msg.Payload.PersistTime,
				}
			}
			return m, m.clearMessageCmd(m.info.TTL)
		}
	}
	return m, nil
}

func (m *statusCmp) View() tea.View {
	t := styles.CurrentTheme()
	status := t.S().Base.Padding(0, 1, 1, 1).Render(m.help.View(m.keyMap))
	if m.info.Msg != "" {
		status = m.infoMsg()
	}
	return tea.NewView(status)
}

func (m *statusCmp) infoMsg() string {
	t := styles.CurrentTheme()
	message := ""
	infoType := ""
	switch m.info.Type {
	case util.InfoTypeError:
		// NOTE(@andreynering): We're not reaching this code!
		panic("error!")

		infoType = t.S().Base.Background(t.Red).Padding(0, 1).Render("ERROR")
		width := m.width - lipgloss.Width(infoType)
		message = t.S().Base.Background(t.Error).Foreground(t.White).Padding(0, 1).Width(width).Render(ansi.Truncate(m.info.Msg, width, "…"))
	case util.InfoTypeWarn:
		infoType = t.S().Base.Foreground(t.BgOverlay).Background(t.Yellow).Padding(0, 1).Render("WARNING")
		width := m.width - lipgloss.Width(infoType)
		message = t.S().Base.Foreground(t.BgOverlay).Background(t.Warning).Padding(0, 1).Width(width).Render(ansi.Truncate(m.info.Msg, width, "…"))
	default:
		infoType = t.S().Base.Foreground(t.BgOverlay).Background(t.Green).Padding(0, 1).Render("OKAY!")
		width := m.width - lipgloss.Width(infoType)
		message = t.S().Base.Background(t.Success).Foreground(t.White).Padding(0, 1).Width(width).Render(ansi.Truncate(m.info.Msg, width, "…"))
	}
	return strings.Join([]string{infoType, message}, "")
}

func (m *statusCmp) ToggleFullHelp() {
	m.help.ShowAll = !m.help.ShowAll
}

func (m *statusCmp) SetKeyMap(keyMap help.KeyMap) {
	m.keyMap = keyMap
}

func NewStatusCmp(keyMap help.KeyMap) StatusCmp {
	t := styles.CurrentTheme()
	help := help.New()
	help.Styles = t.S().Help
	return &statusCmp{
		messageTTL: 5 * time.Second,
		help:       help,
		keyMap:     keyMap,
	}
}
