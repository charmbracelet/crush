package layout

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

type Container interface {
	util.Model
	Sizeable
	Help
	Positional
	Focusable
}
type container struct {
	width     int
	height    int
	isFocused bool

	x, y int

	content util.Model

	// Style options
	paddingTop    int
	paddingRight  int
	paddingBottom int
	paddingLeft   int

	borderTop    bool
	borderRight  bool
	borderBottom bool
	borderLeft   bool
	borderStyle  lipgloss.Border
}

type ContainerOption func(*container)

func NewContainer(content util.Model, options ...ContainerOption) Container {
	c := &container{
		content:     content,
		borderStyle: lipgloss.NormalBorder(),
	}

	for _, option := range options {
		option(c)
	}

	return c
}

func (c *container) Init() tea.Cmd {
	return c.content.Init()
}

func (c *container) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if c.IsFocused() {
			u, cmd := c.content.Update(msg)
			c.content = u.(util.Model)
			return c, cmd
		}
		return c, nil
	default:
		u, cmd := c.content.Update(msg)
		c.content = u.(util.Model)
		return c, cmd
	}
}

func (c *container) View() tea.View {
	t := styles.CurrentTheme()
	width := c.width
	height := c.height

	style := t.S().Base

	// Apply border if any side is enabled
	if c.borderTop || c.borderRight || c.borderBottom || c.borderLeft {
		// Adjust width and height for borders
		if c.borderTop {
			height--
		}
		if c.borderBottom {
			height--
		}
		if c.borderLeft {
			width--
		}
		if c.borderRight {
			width--
		}
		style = style.Border(c.borderStyle, c.borderTop, c.borderRight, c.borderBottom, c.borderLeft)
		style = style.BorderBackground(t.BgBase).BorderForeground(t.Border)
	}
	style = style.
		Width(width).
		Height(height).
		PaddingTop(c.paddingTop).
		PaddingRight(c.paddingRight).
		PaddingBottom(c.paddingBottom).
		PaddingLeft(c.paddingLeft)

	contentView := c.content.View()
	view := tea.NewView(style.Render(contentView.String()))
	cursor := contentView.Cursor()
	view.SetCursor(cursor)
	return view
}

func (c *container) SetSize(width, height int) tea.Cmd {
	c.width = width
	c.height = height

	// If the content implements Sizeable, adjust its size to account for padding and borders
	if sizeable, ok := c.content.(Sizeable); ok {
		// Calculate horizontal space taken by padding and borders
		horizontalSpace := c.paddingLeft + c.paddingRight
		if c.borderLeft {
			horizontalSpace++
		}
		if c.borderRight {
			horizontalSpace++
		}

		// Calculate vertical space taken by padding and borders
		verticalSpace := c.paddingTop + c.paddingBottom
		if c.borderTop {
			verticalSpace++
		}
		if c.borderBottom {
			verticalSpace++
		}

		// Set content size with adjusted dimensions
		contentWidth := max(0, width-horizontalSpace)
		contentHeight := max(0, height-verticalSpace)
		return sizeable.SetSize(contentWidth, contentHeight)
	}
	return nil
}

func (c *container) GetSize() (int, int) {
	return c.width, c.height
}

func (c *container) SetPosition(x, y int) tea.Cmd {
	c.x = x
	c.y = y
	if positionable, ok := c.content.(Positional); ok {
		return positionable.SetPosition(x, y)
	}
	return nil
}

func (c *container) Bindings() []key.Binding {
	if b, ok := c.content.(Help); ok {
		return b.Bindings()
	}
	return nil
}

// Blur implements Container.
func (c *container) Blur() tea.Cmd {
	c.isFocused = false
	if focusable, ok := c.content.(Focusable); ok {
		return focusable.Blur()
	}
	return nil
}

// Focus implements Container.
func (c *container) Focus() tea.Cmd {
	c.isFocused = true
	if focusable, ok := c.content.(Focusable); ok {
		return focusable.Focus()
	}
	return nil
}

// IsFocused implements Container.
func (c *container) IsFocused() bool {
	isFocused := c.isFocused
	if focusable, ok := c.content.(Focusable); ok {
		isFocused = isFocused || focusable.IsFocused()
	}
	return isFocused
}

// Padding options
func WithPadding(top, right, bottom, left int) ContainerOption {
	return func(c *container) {
		c.paddingTop = top
		c.paddingRight = right
		c.paddingBottom = bottom
		c.paddingLeft = left
	}
}

func WithPaddingAll(padding int) ContainerOption {
	return WithPadding(padding, padding, padding, padding)
}

func WithPaddingHorizontal(padding int) ContainerOption {
	return func(c *container) {
		c.paddingLeft = padding
		c.paddingRight = padding
	}
}

func WithPaddingVertical(padding int) ContainerOption {
	return func(c *container) {
		c.paddingTop = padding
		c.paddingBottom = padding
	}
}

func WithBorder(top, right, bottom, left bool) ContainerOption {
	return func(c *container) {
		c.borderTop = top
		c.borderRight = right
		c.borderBottom = bottom
		c.borderLeft = left
	}
}

func WithBorderAll() ContainerOption {
	return WithBorder(true, true, true, true)
}

func WithBorderHorizontal() ContainerOption {
	return WithBorder(true, false, true, false)
}

func WithBorderVertical() ContainerOption {
	return WithBorder(false, true, false, true)
}

func WithBorderStyle(style lipgloss.Border) ContainerOption {
	return func(c *container) {
		c.borderStyle = style
	}
}

func WithRoundedBorder() ContainerOption {
	return WithBorderStyle(lipgloss.RoundedBorder())
}

func WithThickBorder() ContainerOption {
	return WithBorderStyle(lipgloss.ThickBorder())
}

func WithDoubleBorder() ContainerOption {
	return WithBorderStyle(lipgloss.DoubleBorder())
}
