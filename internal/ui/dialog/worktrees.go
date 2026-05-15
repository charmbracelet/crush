package dialog

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/taigrr/crush/internal/ui/common"
	"github.com/taigrr/crush/internal/ui/list"
	"github.com/taigrr/crush/internal/ui/styles"
	"github.com/taigrr/crush/internal/worktree"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/dustin/go-humanize"
	"github.com/sahilm/fuzzy"
)

// WorktreesID is the identifier for the worktrees dialog.
const WorktreesID = "worktrees"

// Worktrees is a worktree selector dialog.
type Worktrees struct {
	com       *common.Common
	help      help.Model
	list      *list.FilterableList
	sessionID string
	worktrees []*worktree.Worktree

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Delete   key.Binding
		Merge    key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*Worktrees)(nil)

// NewWorktrees creates a new Worktrees dialog.
func NewWorktrees(com *common.Common, sessionID string) (*Worktrees, error) {
	w := &Worktrees{
		com:       com,
		sessionID: sessionID,
	}

	// List all worktrees for this workspace.
	worktrees, err := com.Workspace.ListAllWorktrees(context.TODO())
	if err != nil {
		return nil, err
	}
	w.worktrees = worktrees

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	w.help = help

	w.list = list.NewFilterableList(worktreeItems(com.Styles, worktrees)...)
	w.list.Focus()
	w.list.SetSelected(0)

	w.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "switch to"),
	)
	w.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next"),
	)
	w.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous"),
	)
	w.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "navigate"),
	)
	w.keyMap.Delete = key.NewBinding(
		key.WithKeys("d", "backspace"),
		key.WithHelp("d", "delete"),
	)
	w.keyMap.Merge = key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "merge"),
	)

	closeKey := CloseKey
	closeKey.SetHelp("esc", "close")
	w.keyMap.Close = closeKey

	return w, nil
}

// ID implements Dialog.
func (w *Worktrees) ID() string {
	return WorktreesID
}

// HandleMsg implements Dialog.
func (w *Worktrees) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, w.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, w.keyMap.Previous):
			w.list.Focus()
			if w.list.IsSelectedFirst() {
				w.list.SelectLast()
			} else {
				w.list.SelectPrev()
			}
			w.list.ScrollToSelected()
		case key.Matches(msg, w.keyMap.Next):
			w.list.Focus()
			if w.list.IsSelectedLast() {
				w.list.SelectFirst()
			} else {
				w.list.SelectNext()
			}
			w.list.ScrollToSelected()
		case key.Matches(msg, w.keyMap.Select):
			if item := w.list.SelectedItem(); item != nil {
				if wtItem, ok := item.(*worktreeItem); ok {
					// Use the worktree's own session ID, not the passed-in one.
					// This allows switching to worktrees from any context, including
					// when no session is active.
					return ActionSwitchWorktree{
						SessionID:  wtItem.worktree.SessionID,
						WorktreeID: wtItem.worktree.ID,
					}
				}
			}
		case key.Matches(msg, w.keyMap.Merge):
			if item := w.list.SelectedItem(); item != nil {
				if wtItem, ok := item.(*worktreeItem); ok {
					return ActionOpenMergeWorktreeDialog{
						WorktreeID:   wtItem.worktree.ID,
						WorktreeName: wtItem.worktree.Name,
					}
				}
			}
		}
	}
	return nil
}

// InitialCmd implements Dialog.
func (w *Worktrees) InitialCmd() tea.Cmd {
	return nil
}

// Cursor implements Dialog.
func (w *Worktrees) Cursor() *tea.Cursor {
	return nil
}

// Draw implements Dialog.
func (w *Worktrees) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := w.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))

	innerWidth := width - w.com.Styles.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	w.list.SetSize(innerWidth, height-heightOffset)
	w.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Worktrees"
	if len(w.worktrees) == 0 {
		rc.TitleInfo = "No worktrees"
	} else {
		rc.TitleInfo = fmt.Sprintf("%d worktrees", len(w.worktrees))
	}
	listView := t.Dialog.List.Height(w.list.Height()).Render(w.list.Render())
	rc.AddPart(listView)
	rc.Help = w.help.View(w)

	view := rc.Render()
	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements help.KeyMap.
func (w *Worktrees) ShortHelp() []key.Binding {
	return []key.Binding{
		w.keyMap.UpDown,
		w.keyMap.Select,
		w.keyMap.Merge,
		w.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (w *Worktrees) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{w.keyMap.Select, w.keyMap.Merge, w.keyMap.Next, w.keyMap.Previous},
		{w.keyMap.Close},
	}
}

// worktreeItem represents a worktree in the list.
type worktreeItem struct {
	*list.Versioned
	t        *styles.Styles
	worktree *worktree.Worktree
	m        fuzzy.Match
	cache    map[int]string
	focused  bool
}

var _ ListItem = &worktreeItem{Versioned: list.NewVersioned()}

// Filter returns the filterable value of the worktree.
func (i *worktreeItem) Filter() string {
	return i.worktree.Name
}

// ID returns the unique identifier of the worktree.
func (i *worktreeItem) ID() string {
	return i.worktree.ID
}

// SetMatch sets the fuzzy match for the worktree item.
func (i *worktreeItem) SetMatch(m fuzzy.Match) {
	i.cache = nil
	i.m = m
}

// SetFocused sets the focus state of the worktree item.
func (i *worktreeItem) SetFocused(focused bool) {
	if i.focused != focused {
		i.cache = nil
	}
	i.focused = focused
}

// Render returns the string representation of the worktree item.
func (i *worktreeItem) Render(width int) string {
	name := i.worktree.Name
	if i.worktree.IsActive {
		name = "● " + name
	}

	info := humanize.Time(i.worktree.CreatedAt)
	styles := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: i.t.Dialog.ListItem.InfoFocused,
	}

	return renderItem(styles, name, info, i.focused, width, i.cache, &i.m)
}

func worktreeItems(styles *styles.Styles, worktrees []*worktree.Worktree) []list.FilterableItem {
	items := make([]list.FilterableItem, len(worktrees))
	for i, wt := range worktrees {
		items[i] = &worktreeItem{
			Versioned: list.NewVersioned(),
			t:         styles,
			worktree:  wt,
		}
	}
	return items
}

// Finished implements list.Item.
func (w *worktreeItem) Finished() bool { return true }
