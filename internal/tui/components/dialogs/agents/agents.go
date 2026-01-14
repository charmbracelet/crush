package agents

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/subagent"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	AgentsDialogID dialogs.DialogID = "agents"
	defaultWidth   int              = 70
)

type listModel = list.FilterableList[list.CompletionItem[*subagent.Subagent]]

type AgentsDialog interface {
	dialogs.DialogModel
}

type agentsDialogCmp struct {
	width   int
	wWidth  int
	wHeight int

	agentList listModel
	keyMap    AgentsDialogKeyMap
	help      help.Model
}

func NewAgentsDialog() AgentsDialog {
	keyMap := DefaultAgentsDialogKeyMap()
	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	t := styles.CurrentTheme()
	inputStyle := t.S().Base.PaddingLeft(1).PaddingBottom(1)
	agentList := list.NewFilterableList(
		[]list.CompletionItem[*subagent.Subagent]{},
		list.WithFilterInputStyle(inputStyle),
		list.WithFilterListOptions(
			list.WithKeyMap(listKeyMap),
			list.WithWrapNavigation(),
			list.WithResizeByList(),
		),
	)
	h := help.New()
	h.Styles = t.S().Help
	return &agentsDialogCmp{
		agentList: agentList,
		width:     defaultWidth,
		keyMap:    keyMap,
		help:      h,
	}
}

func (c *agentsDialogCmp) Init() tea.Cmd {
	return c.loadAgents()
}

func (c *agentsDialogCmp) loadAgents() tea.Cmd {
	cfg := config.Get()
	var agents []*subagent.Subagent
	for _, sub := range cfg.Subagents {
		agents = append(agents, sub)
	}

	slices.SortFunc(agents, func(a, b *subagent.Subagent) int {
		return strings.Compare(a.Name, b.Name)
	})

	var items []list.CompletionItem[*subagent.Subagent]
	for _, agent := range agents {
		items = append(items, list.NewCompletionItem(agent.Name, agent))
	}
	return c.agentList.SetItems(items)
}

func (c *agentsDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.wWidth = msg.Width
		c.wHeight = msg.Height
		return c, c.agentList.SetSize(c.listWidth(), c.listHeight())
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, c.keyMap.Select):
			selectedItem := c.agentList.SelectedItem()
			if selectedItem == nil {
				return c, nil
			}
			agent := (*selectedItem).Value()
			return c, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				// For now, just close. Later we can add actions.
				util.ReportInfo(fmt.Sprintf("Agent selected: %s", agent.Name)),
			)
		case key.Matches(msg, c.keyMap.Close):
			return c, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			u, cmd := c.agentList.Update(msg)
			c.agentList = u.(listModel)
			return c, cmd
		}
	}
	return c, nil
}

func (c *agentsDialogCmp) View() string {
	t := styles.CurrentTheme()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Subagents", c.width-4)),
		c.agentList.View(),
		"",
		t.S().Base.Width(c.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(c.help.View(c.keyMap)),
	)
	return c.style().Render(content)
}

func (c *agentsDialogCmp) listWidth() int {
	return c.width - 2
}

func (c *agentsDialogCmp) listHeight() int {
	return min(len(c.agentList.Items())+6, c.wHeight/2)
}

func (c *agentsDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(c.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (c *agentsDialogCmp) Position() (int, int) {
	row := c.wHeight/4 - 2
	col := c.wWidth/2 - c.width/2
	return row, col
}

func (c *agentsDialogCmp) ID() dialogs.DialogID {
	return AgentsDialogID
}

func (c *agentsDialogCmp) Cursor() *tea.Cursor {
	if cursor, ok := c.agentList.(util.Cursor); ok {
		cur := cursor.Cursor()
		if cur != nil {
			row, col := c.Position()
			c := *cur
			c.Y += row + 3
			c.X += col + 2
			return &c
		}
	}
	return nil
}
