package completions

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const maxCompletionsHeight = 10

type Completion struct {
	Title string // The title of the completion item
	Value any    // The value of the completion item
}

type OpenCompletionsMsg struct {
	Completions []Completion
	X           int // X position for the completions popup
	Y           int // Y position for the completions popup
	MaxResults  int // Maximum number of results to render, 0 for no limit
}

type FilterCompletionsMsg struct {
	Query  string // The query to filter completions
	Reopen bool
	X      int // X position for the completions popup
	Y      int // Y position for the completions popup
}

type RepositionCompletionsMsg struct {
	X, Y int
}

type CompletionsClosedMsg struct{}

type CompletionsOpenedMsg struct{}

type CloseCompletionsMsg struct{}

type SelectCompletionMsg struct {
	Value  any // The value of the selected completion item
	Insert bool
}

type Completions interface {
	util.Model
	Open() bool
	Query() string // Returns the current filter query
	KeyMap() KeyMap
	Position() (int, int) // Returns the X and Y position of the completions popup
	Width() int
	Height() int
}

type listModel = list.FilterableList[list.CompletionItem[any]]

type completionsCmp struct {
	wWidth    int // The window width
	wHeight   int // The window height
	width     int
	lastWidth int
	height    int  // Height of the completions component`
	x, xorig  int  // X position for the completions popup
	y         int  // Y position for the completions popup
	open      bool // Indicates if the completions are open
	keyMap    KeyMap

	list  listModel
	query string // The current filter query
}

func New() Completions {
	completionsKeyMap := DefaultKeyMap()
	keyMap := list.DefaultKeyMap()
	keyMap.Up.SetEnabled(false)
	keyMap.Down.SetEnabled(false)
	keyMap.HalfPageDown.SetEnabled(false)
	keyMap.HalfPageUp.SetEnabled(false)
	keyMap.Home.SetEnabled(false)
	keyMap.End.SetEnabled(false)
	keyMap.UpOneItem = completionsKeyMap.Up
	keyMap.DownOneItem = completionsKeyMap.Down

	l := list.NewFilterableList(
		[]list.CompletionItem[any]{},
		list.WithFilterInputHidden(),
		list.WithFilterListOptions(
			list.WithDirectionBackward(),
			list.WithKeyMap(keyMap),
		),
	)
	return &completionsCmp{
		width:  0,
		height: maxCompletionsHeight,
		list:   l,
		query:  "",
		keyMap: completionsKeyMap,
	}
}

// Init implements Completions.
func (c *completionsCmp) Init() tea.Cmd {
	return tea.Sequence(
		c.list.Init(),
		c.list.SetSize(c.width, c.height),
	)
}

// Update implements Completions.
func (c *completionsCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.wWidth, c.wHeight = msg.Width, msg.Height
		return c, nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.Up):
			u, cmd := c.list.Update(msg)
			c.list = u.(listModel)
			return c, cmd

		case key.Matches(msg, c.keyMap.Down):
			d, cmd := c.list.Update(msg)
			c.list = d.(listModel)
			return c, cmd
		case key.Matches(msg, c.keyMap.UpInsert):
			s := c.list.SelectedItem()
			if s == nil {
				return c, nil
			}
			selectedItem := *s
			c.list.SetSelected(selectedItem.ID())
			return c, util.CmdHandler(SelectCompletionMsg{
				Value:  selectedItem.Value(),
				Insert: true,
			})
		case key.Matches(msg, c.keyMap.DownInsert):
			s := c.list.SelectedItem()
			if s == nil {
				return c, nil
			}
			selectedItem := *s
			c.list.SetSelected(selectedItem.ID())
			return c, util.CmdHandler(SelectCompletionMsg{
				Value:  selectedItem.Value(),
				Insert: true,
			})
		case key.Matches(msg, c.keyMap.Select):
			s := c.list.SelectedItem()
			if s == nil {
				return c, nil
			}
			selectedItem := *s
			c.open = false // Close completions after selection
			return c, util.CmdHandler(SelectCompletionMsg{
				Value: selectedItem.Value(),
			})
		case key.Matches(msg, c.keyMap.Cancel):
			return c, util.CmdHandler(CloseCompletionsMsg{})
		}
	case RepositionCompletionsMsg:
		c.x, c.y = msg.X, msg.Y
		c.adjustPosition()
	case CloseCompletionsMsg:
		c.open = false
		return c, util.CmdHandler(CompletionsClosedMsg{})
	case OpenCompletionsMsg:
		c.open = true
		c.query = ""
		c.x, c.xorig = msg.X, msg.X
		c.y = msg.Y
		items := []list.CompletionItem[any]{}
		t := styles.CurrentTheme()
		for _, completion := range msg.Completions {
			item := list.NewCompletionItem(
				completion.Title,
				completion.Value,
				list.WithCompletionBackgroundColor(t.BgSubtle),
			)
			items = append(items, item)
		}
		width := listWidth(items)
		if len(items) == 0 {
			width = listWidth(c.list.Items())
		}
		if c.x+width >= c.wWidth {
			c.x = c.wWidth - width - 1
		}
		c.width = width
		c.height = max(min(maxCompletionsHeight, len(items)), 1) // Ensure at least 1 item height
		c.list.SetResultsSize(msg.MaxResults)
		return c, tea.Batch(
			c.list.SetItems(items),
			c.list.SetSize(c.width, c.height),
			util.CmdHandler(CompletionsOpenedMsg{}),
		)
	case FilterCompletionsMsg:
		if !c.open && !msg.Reopen {
			return c, nil
		}
		if msg.Query == c.query {
			// PERF: if same query, don't need to filter again
			return c, nil
		}
		if len(c.list.Items()) == 0 &&
			len(msg.Query) > len(c.query) &&
			strings.HasPrefix(msg.Query, c.query) {
			// PERF: if c.query didn't match anything,
			// AND msg.Query is longer than c.query,
			// AND msg.Query is prefixed with c.query - which means
			//		that the user typed more chars after a 0 match,
			// it won't match anything, so return earlier.
			return c, nil
		}
		c.query = msg.Query
		var cmds []tea.Cmd
		cmds = append(cmds, c.list.Filter(msg.Query))
		items := c.list.Items()
		itemsLen := len(items)
		c.xorig = msg.X
		c.x, c.y = msg.X, msg.Y
		c.adjustPosition()
		cmds = append(cmds, c.list.SetSize(c.width, c.height))
		if itemsLen == 0 {
			cmds = append(cmds, util.CmdHandler(CloseCompletionsMsg{}))
		} else if msg.Reopen {
			c.open = true
			cmds = append(cmds, util.CmdHandler(CompletionsOpenedMsg{}))
		}
		return c, tea.Batch(cmds...)
	}
	return c, nil
}

func (c *completionsCmp) adjustPosition() {
	items := c.list.Items()
	itemsLen := len(items)
	width := listWidth(items)
	c.lastWidth = c.width
	if c.x < 0 || width < c.lastWidth {
		c.x = c.xorig
	} else if c.x+width >= c.wWidth {
		c.x = c.wWidth - width - 1
	}
	c.width = width
	c.height = max(min(maxCompletionsHeight, itemsLen), 1)
}

// View implements Completions.
func (c *completionsCmp) View() string {
	if !c.open || len(c.list.Items()) == 0 {
		return ""
	}

	t := styles.CurrentTheme()
	style := t.S().Base.
		Width(c.width).
		Height(c.height).
		Background(t.BgSubtle)

	return style.Render(c.list.View())
}

// listWidth returns the width of the last 10 items in the list, which is used
// to determine the width of the completions popup.
// Note this only works for [completionItemCmp] items.
func listWidth(items []list.CompletionItem[any]) int {
	var width int
	if len(items) == 0 {
		return width
	}

	for i := len(items) - 1; i >= 0 && i >= len(items)-10; i-- {
		itemWidth := lipgloss.Width(items[i].Text()) + 2 // +2 for padding
		width = max(width, itemWidth)
	}

	return width
}

func (c *completionsCmp) Open() bool {
	return c.open
}

func (c *completionsCmp) Query() string {
	return c.query
}

func (c *completionsCmp) KeyMap() KeyMap {
	return c.keyMap
}

func (c *completionsCmp) Position() (int, int) {
	return c.x, c.y - c.height
}

func (c *completionsCmp) Width() int {
	return c.width
}

func (c *completionsCmp) Height() int {
	return c.height
}
