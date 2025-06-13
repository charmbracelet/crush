package sidebar

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/logo"
	"github.com/charmbracelet/crush/internal/tui/layout"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	logoBreakpoint = 65
)

type Sidebar interface {
	util.Model
	layout.Sizeable
}

type sidebarCmp struct {
	width, height int
	session       session.Session
	logo          string
	cwd           string
}

func NewSidebarCmp() Sidebar {
	return &sidebarCmp{}
}

func (m *sidebarCmp) Init() tea.Cmd {
	m.logo = m.logoBlock(false)
	m.cwd = cwd()
	return nil
}

func (m *sidebarCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chat.SessionSelectedMsg:
		if msg.ID != m.session.ID {
			m.session = msg
		}
	case chat.SessionClearedMsg:
		m.session = session.Session{}
	case pubsub.Event[session.Session]:
		if msg.Type == pubsub.UpdatedEvent {
			if m.session.ID == msg.Payload.ID {
				m.session = msg.Payload
			}
		}
	}
	return m, nil
}

func (m *sidebarCmp) View() tea.View {
	t := styles.CurrentTheme()
	parts := []string{
		m.logo,
	}

	if m.session.ID != "" {
		parts = append(parts, t.S().Muted.Render(m.session.Title), "")
	}

	parts = append(parts,
		m.cwd,
		"",
		m.lspBlock(),
		"",
		m.mcpBlock(),
	)

	return tea.NewView(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)
}

func (m *sidebarCmp) SetSize(width, height int) tea.Cmd {
	if width < logoBreakpoint && m.width >= logoBreakpoint {
		m.logo = m.logoBlock(true)
	} else if width >= logoBreakpoint && m.width < logoBreakpoint {
		m.logo = m.logoBlock(false)
	}

	m.width = width
	m.height = height
	return nil
}

func (m *sidebarCmp) GetSize() (int, int) {
	return m.width, m.height
}

func (m *sidebarCmp) logoBlock(compact bool) string {
	t := styles.CurrentTheme()
	return logo.Render(version.Version, compact, logo.Opts{
		FieldColor:   t.Primary,
		TitleColorA:  t.Secondary,
		TitleColorB:  t.Primary,
		CharmColor:   t.Secondary,
		VersionColor: t.Primary,
	})
}

func (m *sidebarCmp) lspBlock() string {
	maxWidth := min(m.width, 58)
	t := styles.CurrentTheme()

	section := t.S().Muted.Render(
		core.Section("LSPs", maxWidth),
	)

	lspList := []string{section, ""}

	lsp := config.Get().LSP
	if len(lsp) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
	}

	for n, l := range lsp {
		iconColor := t.Success
		if l.Disabled {
			iconColor = t.FgMuted
		}
		lspList = append(lspList,
			core.Status(
				core.StatusOpts{
					IconColor:   iconColor,
					Title:       n,
					Description: l.Command,
				},
				m.width,
			),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		lspList...,
	)
}

func (m *sidebarCmp) mcpBlock() string {
	maxWidth := min(m.width, 58)
	t := styles.CurrentTheme()

	section := t.S().Muted.Render(
		core.Section("MCPs", maxWidth),
	)

	mcpList := []string{section, ""}

	mcp := config.Get().MCPServers
	if len(mcp) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			section,
			"",
			t.S().Base.Foreground(t.Border).Render("None"),
		)
	}

	for n, l := range mcp {
		iconColor := t.Success
		mcpList = append(mcpList,
			core.Status(
				core.StatusOpts{
					IconColor:   iconColor,
					Title:       n,
					Description: l.Command,
				},
				m.width,
			),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		mcpList...,
	)
}

func cwd() string {
	cwd := config.WorkingDirectory()
	t := styles.CurrentTheme()
	// replace home directory with ~
	homeDir, err := os.UserHomeDir()
	if err == nil {
		cwd = strings.ReplaceAll(cwd, homeDir, "~")
	}
	return t.S().Muted.Render(cwd)
}
