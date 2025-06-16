package layout

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

type LayoutPanel string

const (
	LeftPanel   LayoutPanel = "left"
	RightPanel  LayoutPanel = "right"
	BottomPanel LayoutPanel = "bottom"
)

type SplitPaneLayout interface {
	util.Model
	Sizeable
	Help
	SetLeftPanel(panel Container) tea.Cmd
	SetRightPanel(panel Container) tea.Cmd
	SetBottomPanel(panel Container) tea.Cmd

	ClearLeftPanel() tea.Cmd
	ClearRightPanel() tea.Cmd
	ClearBottomPanel() tea.Cmd

	FocusPanel(panel LayoutPanel) tea.Cmd
}

type splitPaneLayout struct {
	width  int
	height int

	ratio         float64
	verticalRatio float64

	rightPanel  Container
	leftPanel   Container
	bottomPanel Container

	fixedBottomHeight int // Fixed height for the bottom panel, if any
	fixedRightWidth   int // Fixed width for the right panel, if any
}

type SplitPaneOption func(*splitPaneLayout)

func (s *splitPaneLayout) Init() tea.Cmd {
	var cmds []tea.Cmd

	if s.leftPanel != nil {
		cmds = append(cmds, s.leftPanel.Init())
	}

	if s.rightPanel != nil {
		cmds = append(cmds, s.rightPanel.Init())
	}

	if s.bottomPanel != nil {
		cmds = append(cmds, s.bottomPanel.Init())
	}

	return tea.Batch(cmds...)
}

func (s *splitPaneLayout) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s, s.SetSize(msg.Width, msg.Height)
	}

	if s.rightPanel != nil {
		u, cmd := s.rightPanel.Update(msg)
		s.rightPanel = u.(Container)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if s.leftPanel != nil {
		u, cmd := s.leftPanel.Update(msg)
		s.leftPanel = u.(Container)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if s.bottomPanel != nil {
		u, cmd := s.bottomPanel.Update(msg)
		s.bottomPanel = u.(Container)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return s, tea.Batch(cmds...)
}

func (s *splitPaneLayout) View() tea.View {
	var topSection string

	if s.leftPanel != nil && s.rightPanel != nil {
		leftView := s.leftPanel.View()
		rightView := s.rightPanel.View()
		topSection = lipgloss.JoinHorizontal(lipgloss.Top, leftView.String(), rightView.String())
	} else if s.leftPanel != nil {
		topSection = s.leftPanel.View().String()
	} else if s.rightPanel != nil {
		topSection = s.rightPanel.View().String()
	} else {
		topSection = ""
	}

	var finalView string

	if s.bottomPanel != nil && topSection != "" {
		bottomView := s.bottomPanel.View()
		finalView = lipgloss.JoinVertical(lipgloss.Left, topSection, bottomView.String())
	} else if s.bottomPanel != nil {
		finalView = s.bottomPanel.View().String()
	} else {
		finalView = topSection
	}

	// TODO: think of a better way to handle multiple cursors
	var cursor *tea.Cursor
	if s.bottomPanel != nil {
		cursor = s.bottomPanel.View().Cursor()
	} else if s.rightPanel != nil {
		cursor = s.rightPanel.View().Cursor()
	} else if s.leftPanel != nil {
		cursor = s.leftPanel.View().Cursor()
	}

	t := styles.CurrentTheme()

	style := t.S().Base.
		Width(s.width).
		Height(s.height)

	view := tea.NewView(style.Render(finalView))
	view.SetCursor(cursor)
	return view
}

func (s *splitPaneLayout) SetSize(width, height int) tea.Cmd {
	s.width = width
	s.height = height

	var topHeight, bottomHeight int
	var cmds []tea.Cmd
	if s.bottomPanel != nil {
		if s.fixedBottomHeight > 0 {
			bottomHeight = s.fixedBottomHeight
			topHeight = height - bottomHeight
		} else {
			topHeight = int(float64(height) * s.verticalRatio)
			bottomHeight = height - topHeight
			if bottomHeight <= 0 {
				bottomHeight = 2
				topHeight = height - bottomHeight
			}
		}
	} else {
		topHeight = height
		bottomHeight = 0
	}

	var leftWidth, rightWidth int
	if s.leftPanel != nil && s.rightPanel != nil {
		if s.fixedRightWidth > 0 {
			rightWidth = s.fixedRightWidth
			leftWidth = width - rightWidth
		} else {
			leftWidth = int(float64(width) * s.ratio)
			rightWidth = width - leftWidth
			if rightWidth <= 0 {
				rightWidth = 2
				leftWidth = width - rightWidth
			}
		}
	} else if s.leftPanel != nil {
		leftWidth = width
		rightWidth = 0
	} else if s.rightPanel != nil {
		leftWidth = 0
		rightWidth = width
	}

	if s.leftPanel != nil {
		cmd := s.leftPanel.SetSize(leftWidth, topHeight)
		cmds = append(cmds, cmd)
		if positionable, ok := s.leftPanel.(Positionable); ok {
			cmds = append(cmds, positionable.SetPosition(0, 0))
		}
	}

	if s.rightPanel != nil {
		cmd := s.rightPanel.SetSize(rightWidth, topHeight)
		cmds = append(cmds, cmd)
		if positionable, ok := s.rightPanel.(Positionable); ok {
			cmds = append(cmds, positionable.SetPosition(leftWidth, 0))
		}
	}

	if s.bottomPanel != nil {
		cmd := s.bottomPanel.SetSize(width, bottomHeight)
		cmds = append(cmds, cmd)
		if positionable, ok := s.bottomPanel.(Positionable); ok {
			cmds = append(cmds, positionable.SetPosition(0, topHeight))
		}
	}
	return tea.Batch(cmds...)
}

func (s *splitPaneLayout) GetSize() (int, int) {
	return s.width, s.height
}

func (s *splitPaneLayout) SetLeftPanel(panel Container) tea.Cmd {
	s.leftPanel = panel
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) SetRightPanel(panel Container) tea.Cmd {
	s.rightPanel = panel
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) SetBottomPanel(panel Container) tea.Cmd {
	s.bottomPanel = panel
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) ClearLeftPanel() tea.Cmd {
	s.leftPanel = nil
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) ClearRightPanel() tea.Cmd {
	s.rightPanel = nil
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) ClearBottomPanel() tea.Cmd {
	s.bottomPanel = nil
	if s.width > 0 && s.height > 0 {
		return s.SetSize(s.width, s.height)
	}
	return nil
}

func (s *splitPaneLayout) Bindings() []key.Binding {
	if s.leftPanel != nil {
		if b, ok := s.leftPanel.(Help); ok && s.leftPanel.IsFocused() {
			return b.Bindings()
		}
	}
	if s.rightPanel != nil {
		if b, ok := s.rightPanel.(Help); ok && s.rightPanel.IsFocused() {
			return b.Bindings()
		}
	}
	if s.bottomPanel != nil {
		if b, ok := s.bottomPanel.(Help); ok && s.bottomPanel.IsFocused() {
			return b.Bindings()
		}
	}
	return nil
}

func (s *splitPaneLayout) FocusPanel(panel LayoutPanel) tea.Cmd {
	panels := map[LayoutPanel]Container{
		LeftPanel:   s.leftPanel,
		RightPanel:  s.rightPanel,
		BottomPanel: s.bottomPanel,
	}
	var cmds []tea.Cmd
	for p, container := range panels {
		if container == nil {
			continue
		}
		if p == panel {
			cmds = append(cmds, container.Focus())
		} else {
			cmds = append(cmds, container.Blur())
		}
	}
	return tea.Batch(cmds...)
}

func NewSplitPane(options ...SplitPaneOption) SplitPaneLayout {
	layout := &splitPaneLayout{
		ratio:         0.8,
		verticalRatio: 0.92, // Default 90% for top section, 10% for bottom
	}
	for _, option := range options {
		option(layout)
	}
	return layout
}

func WithLeftPanel(panel Container) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.leftPanel = panel
	}
}

func WithRightPanel(panel Container) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.rightPanel = panel
	}
}

func WithRatio(ratio float64) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.ratio = ratio
	}
}

func WithBottomPanel(panel Container) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.bottomPanel = panel
	}
}

func WithVerticalRatio(ratio float64) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.verticalRatio = ratio
	}
}

func WithFixedBottomHeight(height int) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.fixedBottomHeight = height
	}
}

func WithFixedRightWidth(width int) SplitPaneOption {
	return func(s *splitPaneLayout) {
		s.fixedRightWidth = width
	}
}
