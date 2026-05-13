package dialog

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/taigrr/crush/internal/ui/common"
	"github.com/taigrr/crush/internal/ui/list"
	"github.com/taigrr/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

// MergeWorktreeID is the identifier for the merge worktree dialog.
const MergeWorktreeID = "merge_worktree"

// MergeWorktree is a dialog for merging a worktree onto a target branch.
type MergeWorktree struct {
	com          *common.Common
	help         help.Model
	list         *list.FilterableList
	worktreeID   string
	worktreeName string
	branches     []string
	rebase       bool

	keyMap struct {
		Select       key.Binding
		Next         key.Binding
		Previous     key.Binding
		UpDown       key.Binding
		ToggleRebase key.Binding
		Close        key.Binding
	}
}

var _ Dialog = (*MergeWorktree)(nil)

// NewMergeWorktree creates a new MergeWorktree dialog.
func NewMergeWorktree(com *common.Common, worktreeID, worktreeName string) (*MergeWorktree, error) {
	m := &MergeWorktree{
		com:          com,
		worktreeID:   worktreeID,
		worktreeName: worktreeName,
		rebase:       false,
	}

	// List all git branches.
	branches, err := com.Workspace.ListGitBranches(context.TODO())
	if err != nil {
		return nil, err
	}
	m.branches = branches

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	m.help = help

	m.list = list.NewFilterableList(branchItems(com.Styles, branches)...)
	m.list.Focus()

	// Default to the current git branch.
	currentBranch := com.Workspace.GitBranch()
	defaultBranch := selectDefaultBranch(branches, currentBranch)
	if defaultBranch >= 0 {
		m.list.SetSelected(defaultBranch)
	} else {
		m.list.SetSelected(0)
	}

	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "merge"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "navigate"),
	)
	m.keyMap.ToggleRebase = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "toggle rebase"),
	)

	closeKey := CloseKey
	closeKey.SetHelp("esc", "close")
	m.keyMap.Close = closeKey

	return m, nil
}

// selectDefaultBranch returns the index of the preferred default branch.
// Prefers the current branch, then main, then master.
func selectDefaultBranch(branches []string, currentBranch string) int {
	// First try current branch.
	if currentBranch != "" {
		for i, b := range branches {
			if b == currentBranch {
				return i
			}
		}
	}
	// Then prefer main.
	for i, b := range branches {
		if b == "main" {
			return i
		}
	}
	// Then master.
	for i, b := range branches {
		if b == "master" {
			return i
		}
	}
	return -1
}

// ID implements Dialog.
func (m *MergeWorktree) ID() string {
	return MergeWorktreeID
}

// HandleMsg implements Dialog.
func (m *MergeWorktree) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.ToggleRebase):
			m.rebase = !m.rebase
			return nil
		case key.Matches(msg, m.keyMap.Previous):
			m.list.Focus()
			if m.list.IsSelectedFirst() {
				m.list.SelectLast()
			} else {
				m.list.SelectPrev()
			}
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Next):
			m.list.Focus()
			if m.list.IsSelectedLast() {
				m.list.SelectFirst()
			} else {
				m.list.SelectNext()
			}
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Select):
			if item := m.list.SelectedItem(); item != nil {
				if brItem, ok := item.(*branchItem); ok {
					return ActionMergeWorktree{
						WorktreeID:   m.worktreeID,
						TargetBranch: brItem.branch,
						Rebase:       m.rebase,
					}
				}
			}
		}
	}
	return nil
}

// InitialCmd implements Dialog.
func (m *MergeWorktree) InitialCmd() tea.Cmd {
	return nil
}

// Cursor implements Dialog.
func (m *MergeWorktree) Cursor() *tea.Cursor {
	return nil
}

// Draw implements Dialog.
func (m *MergeWorktree) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))

	innerWidth := width - m.com.Styles.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize() + 2 // Extra for rebase toggle

	m.list.SetSize(innerWidth, height-heightOffset)
	m.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Merge Worktree"
	rc.TitleInfo = m.worktreeName

	// Rebase toggle.
	mode := "Merge"
	if m.rebase {
		mode = "Rebase"
	}
	modeLabel := t.Dialog.SecondaryText.Render("Mode:")
	modeValue := t.Dialog.PrimaryText.Render(mode + "  (tab to toggle)")
	rc.AddPart(lipgloss.JoinVertical(lipgloss.Left, modeLabel, modeValue))

	// Branch list.
	if len(m.branches) == 0 {
		rc.AddPart(t.Dialog.SecondaryText.Render("No branches found"))
	} else {
		branchLabel := t.Dialog.SecondaryText.Render(fmt.Sprintf("Target branch (%d):", len(m.branches)))
		listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
		rc.AddPart(lipgloss.JoinVertical(lipgloss.Left, branchLabel, listView))
	}

	rc.Help = m.help.View(m)

	view := rc.Render()
	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements help.KeyMap.
func (m *MergeWorktree) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.UpDown,
		m.keyMap.ToggleRebase,
		m.keyMap.Select,
		m.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (m *MergeWorktree) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{m.keyMap.Select, m.keyMap.ToggleRebase, m.keyMap.Next, m.keyMap.Previous},
		{m.keyMap.Close},
	}
}

// branchItem represents a branch in the list.
type branchItem struct {
	t       *styles.Styles
	branch  string
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var _ ListItem = &branchItem{}

// Filter returns the filterable value of the branch.
func (i *branchItem) Filter() string {
	return i.branch
}

// ID returns the unique identifier of the branch.
func (i *branchItem) ID() string {
	return i.branch
}

// SetMatch sets the fuzzy match for the branch item.
func (i *branchItem) SetMatch(m fuzzy.Match) {
	i.cache = nil
	i.m = m
}

// SetFocused sets the focus state of the branch item.
func (i *branchItem) SetFocused(focused bool) {
	if i.focused != focused {
		i.cache = nil
	}
	i.focused = focused
}

// Render returns the string representation of the branch item.
func (i *branchItem) Render(width int) string {
	styles := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: i.t.Dialog.ListItem.InfoFocused,
	}

	return renderItem(styles, i.branch, "", i.focused, width, i.cache, &i.m)
}

func branchItems(styles *styles.Styles, branches []string) []list.FilterableItem {
	items := make([]list.FilterableItem, len(branches))
	for i, branch := range branches {
		items[i] = &branchItem{
			t:      styles,
			branch: branch,
		}
	}
	return items
}
