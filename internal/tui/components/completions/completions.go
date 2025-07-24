package completions

import (
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
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
}

type FilterCompletionsMsg struct {
	Query  string // The query to filter completions
	Reopen bool
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
	width  int
	height int  // Height of the completions component`
	x      int  // X position for the completions popup
	y      int  // Y position for the completions popup
	open   bool // Indicates if the completions are open
	keyMap KeyMap

	list  listModel
	query string // The current filter query
}

const maxCompletionsWidth = 80 // Maximum width for the completions popup

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
		height: 0,
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
func (c *completionsCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width = min(msg.Width-c.x, maxCompletionsWidth)
		c.height = min(msg.Height-c.y, 15)
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
				Value:  selectedItem,
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
				Value:  selectedItem,
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
				Value: selectedItem,
			})
		case key.Matches(msg, c.keyMap.Cancel):
			return c, util.CmdHandler(CloseCompletionsMsg{})
		}
	case CloseCompletionsMsg:
		c.open = false
		return c, util.CmdHandler(CompletionsClosedMsg{})
	case OpenCompletionsMsg:
		c.open = true
		c.query = ""
		c.x = msg.X
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
		c.height = max(min(c.height, len(items)), 1) // Ensure at least 1 item height
		return c, tea.Batch(
			c.list.SetSize(c.width, c.height),
			c.list.SetItems(items),
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
		itemsLen := len(c.list.Items())
		c.height = max(min(maxCompletionsHeight, itemsLen), 1)
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

// View implements Completions.
func (c *completionsCmp) View() string {
	if !c.open || len(c.list.Items()) == 0 {
		return ""
	}

	return c.style().Render(c.list.View())
}

func (c *completionsCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(c.width).
		Height(c.height).
		Background(t.BgSubtle)
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
