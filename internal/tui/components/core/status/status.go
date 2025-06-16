package status

import (
	"time"

	"github.com/charmbracelet/bubbles/v2/help"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/logging"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
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
		}
	}
	return m, nil
}

func (m *statusCmp) View() tea.View {
	t := styles.CurrentTheme()
	status := t.S().Base.Padding(0, 1, 1, 1).Render(m.help.View(m.keyMap))
	if m.info.Msg != "" {
		switch m.info.Type {
		case util.InfoTypeError:
			status = t.S().Base.Background(t.Error).Padding(0, 1).Width(m.width).Render(m.info.Msg)
		case util.InfoTypeWarn:
			status = t.S().Base.Background(t.Warning).Padding(0, 1).Width(m.width).Render(m.info.Msg)
		default:
			status = t.S().Base.Background(t.Info).Padding(0, 1).Width(m.width).Render(m.info.Msg)
		}
	}
	return tea.NewView(status)
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
		messageTTL: 10 * time.Second,
		help:       help,
		keyMap:     keyMap,
	}
}
