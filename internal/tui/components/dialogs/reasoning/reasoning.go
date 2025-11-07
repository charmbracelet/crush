package reasoning

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	ReasoningDialogID dialogs.DialogID = "reasoning"

	defaultWidth int = 50
)

type listModel = list.FilterableList[list.CompletionItem[EffortOption]]

type EffortOption struct {
	Title  string
	Effort string
}

type ReasoningDialog interface {
	dialogs.DialogModel
}

type reasoningDialogCmp struct {
	width   int
	wWidth  int // Width of the terminal window
	wHeight int // Height of the terminal window

	effortList listModel
	keyMap     ReasoningDialogKeyMap
	help       help.Model
}

type ReasoningEffortSelectedMsg struct {
	Effort string
}

type ReasoningDialogKeyMap struct {
	Next     key.Binding
	Previous key.Binding
	Select   key.Binding
	Close    key.Binding
}

func DefaultReasoningDialogKeyMap() ReasoningDialogKeyMap {
	return ReasoningDialogKeyMap{
		Next: key.NewBinding(
			key.WithKeys("down", "j", "ctrl+n"),
			key.WithHelp("↓/j/ctrl+n", "next"),
		),
		Previous: key.NewBinding(
			key.WithKeys("up", "k", "ctrl+p"),
			key.WithHelp("↑/k/ctrl+p", "previous"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc/ctrl+c", "close"),
		),
	}
}

func (k ReasoningDialogKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Close}
}

func (k ReasoningDialogKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Next, k.Previous},
		{k.Select, k.Close},
	}
}

func NewReasoningDialog() ReasoningDialog {
	keyMap := DefaultReasoningDialogKeyMap()
	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	t := styles.CurrentTheme()
	inputStyle := t.S().Base.PaddingLeft(1).PaddingBottom(1)
	effortList := list.NewFilterableList(
		[]list.CompletionItem[EffortOption]{},
		list.WithFilterInputStyle(inputStyle),
		list.WithFilterListOptions(
			list.WithKeyMap(listKeyMap),
			list.WithWrapNavigation(),
			list.WithResizeByList(),
		),
	)
	help := help.New()
	help.Styles = t.S().Help

	return &reasoningDialogCmp{
		effortList: effortList,
		width:      defaultWidth,
		keyMap:     keyMap,
		help:       help,
	}
}

func (r *reasoningDialogCmp) Init() tea.Cmd {
	return r.populateEffortOptions()
}

func (r *reasoningDialogCmp) populateEffortOptions() tea.Cmd {
	cfg := config.Get()
	if agentCfg, ok := cfg.Agents[config.AgentCoder]; ok {
		selectedModel := cfg.Models[agentCfg.Model]
		model := cfg.GetModelByType(agentCfg.Model)

		// Get current reasoning effort
		currentEffort := selectedModel.ReasoningEffort
		if currentEffort == "" && model != nil {
			currentEffort = model.DefaultReasoningEffort
		}

		efforts := []EffortOption{}
		caser := cases.Title(language.Und)
		for _, level := range model.ReasoningLevels {
			efforts = append(efforts, EffortOption{
				Title:  caser.String(level),
				Effort: level,
			})
		}

		effortItems := []list.CompletionItem[EffortOption]{}
		selectedID := ""
		for _, effort := range efforts {
			opts := []list.CompletionItemOption{
				list.WithCompletionID(effort.Effort),
			}
			if effort.Effort == currentEffort {
				opts = append(opts, list.WithCompletionShortcut("current"))
				selectedID = effort.Effort
			}
			effortItems = append(effortItems, list.NewCompletionItem(
				effort.Title,
				effort,
				opts...,
			))
		}

		cmd := r.effortList.SetItems(effortItems)
		// Set the current effort as the selected item
		if currentEffort != "" && selectedID != "" {
			return tea.Sequence(cmd, r.effortList.SetSelected(selectedID))
		}
		return cmd
	}
	return nil
}

func (r *reasoningDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.wWidth = msg.Width
		r.wHeight = msg.Height
		return r, r.effortList.SetSize(r.listWidth(), r.listHeight())
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, r.keyMap.Select):
			selectedItem := r.effortList.SelectedItem()
			if selectedItem == nil {
				return r, nil // No item selected, do nothing
			}
			effort := (*selectedItem).Value()
			return r, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				func() tea.Msg {
					return ReasoningEffortSelectedMsg{
						Effort: effort.Effort,
					}
				},
			)
		case key.Matches(msg, r.keyMap.Close):
			return r, util.CmdHandler(dialogs.CloseDialogMsg{})
		default:
			u, cmd := r.effortList.Update(msg)
			r.effortList = u.(listModel)
			return r, cmd
		}
	}
	return r, nil
}

func (r *reasoningDialogCmp) View() string {
	t := styles.CurrentTheme()
	listView := r.effortList

	header := t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Select Reasoning Effort", r.width-4))
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		listView.View(),
		"",
		t.S().Base.Width(r.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(r.help.View(r.keyMap)),
	)
	return r.style().Render(content)
}

func (r *reasoningDialogCmp) Cursor() *tea.Cursor {
	if cursor, ok := r.effortList.(util.Cursor); ok {
		cursor := cursor.Cursor()
		if cursor != nil {
			cursor = r.moveCursor(cursor)
		}
		return cursor
	}
	return nil
}

func (r *reasoningDialogCmp) listWidth() int {
	return r.width - 2 // 4 for padding
}

func (r *reasoningDialogCmp) listHeight() int {
	listHeight := len(r.effortList.Items()) + 2 + 4 // height based on items + 2 for the input + 4 for the sections
	return min(listHeight, r.wHeight/2)
}

func (r *reasoningDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := r.Position()
	offset := row + 3
	cursor.Y += offset
	cursor.X = cursor.X + col + 2
	return cursor
}

func (r *reasoningDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(r.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (r *reasoningDialogCmp) Position() (int, int) {
	row := r.wHeight/4 - 2 // just a bit above the center
	col := r.wWidth / 2
	col -= r.width / 2
	return row, col
}

func (r *reasoningDialogCmp) ID() dialogs.DialogID {
	return ReasoningDialogID
}
