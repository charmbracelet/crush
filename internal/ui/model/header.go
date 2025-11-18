package model

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/logo"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/version"
)

const (
	// The default hight for the header
	headerHeight = 4
	// The compact header height
	headerCompactHeight = 1
)

// HeaderModel is the model for the sidebar UI component.
type HeaderModel struct {
	com *common.Common

	// Cached rendered logo string.
	logo string

	// width of the sidebar.
	width int

	// Whether to render the header in compact mode.
	compact bool
}

// NewHeaderModel creates a new HeaderModel instance.
func NewHeaderModel(com *common.Common) *HeaderModel {
	return &HeaderModel{
		com:     com,
		compact: false,
	}
}

// Init initializes the sidebar model.
func (m *HeaderModel) Init() tea.Cmd {
	return nil
}

// Update updates the header model based on incoming messages.
func (m *HeaderModel) Update(msg tea.Msg) (*HeaderModel, tea.Cmd) {
	return m, nil
}

// View renders the sidebar model as a string.
func (m *HeaderModel) View() string {
	if m.compact {
		// TODO: we need to show the compact version of the header with important information
		return ""
	}
	return m.logo
}

// SetWidth sets the width of the header and updates the logo accordingly.
func (m *HeaderModel) SetWidth(width int) {
	m.logo = renderLogo(m.com.Styles, false, width)
	m.width = width
}

func renderLogo(t *styles.Styles, compact bool, width int) string {
	return logo.Render(version.Version, compact, logo.Opts{
		FieldColor:   t.LogoFieldColor,
		TitleColorA:  t.LogoTitleColorA,
		TitleColorB:  t.LogoTitleColorB,
		CharmColor:   t.LogoCharmColor,
		VersionColor: t.LogoVersionColor,
		Width:        max(0, width-2),
	})
}
