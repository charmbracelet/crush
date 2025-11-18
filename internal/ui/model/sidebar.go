package model

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// SidebarModel is the model for the sidebar UI component.
type SidebarModel struct {
	com *common.Common

	// width of the sidebar.
	width int

	// Cached rendered logo string.
	logo string
	// Cached cwd string.
	cwd string

	// TODO: lsp, files, session

	// Whether to render the sidebar in compact mode.
	compact bool
}

// NewSidebarModel creates a new SidebarModel instance.
func NewSidebarModel(com *common.Common) *SidebarModel {
	return &SidebarModel{
		com:     com,
		compact: true,
		cwd:     com.Config().WorkingDir(),
	}
}

// Init initializes the sidebar model.
func (m *SidebarModel) Init() tea.Cmd {
	return nil
}

// Update updates the sidebar model based on incoming messages.
func (m *SidebarModel) Update(msg tea.Msg) (*SidebarModel, tea.Cmd) {
	return m, nil
}

// View renders the sidebar model as a string.
func (m *SidebarModel) View() string {
	s := m.com.Styles.SidebarFull
	if m.compact {
		s = m.com.Styles.SidebarCompact
	}

	blocks := []string{
		m.logo,
	}

	return s.Render(lipgloss.JoinVertical(
		lipgloss.Top,
		blocks...,
	))
}

// SetWidth sets the width of the sidebar and updates the logo accordingly.
func (m *SidebarModel) SetWidth(width int) {
	m.logo = renderLogo(m.com.Styles, true, width)
	m.width = width
}
