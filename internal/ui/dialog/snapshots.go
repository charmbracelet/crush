package dialog

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/checkpoint"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/dustin/go-humanize"
	"github.com/sahilm/fuzzy"
)

// SnapshotsID is the identifier for the snapshots dialog.
const SnapshotsID = "snapshots"

// Snapshots is a snapshot selector dialog.
type Snapshots struct {
	com       *common.Common
	help      help.Model
	list      *list.FilterableList
	sessionID string
	snapshots []*checkpoint.Snapshot

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*Snapshots)(nil)

// NewSnapshots creates a new Snapshots dialog.
func NewSnapshots(com *common.Common, sessionID string) (*Snapshots, error) {
	s := &Snapshots{
		com:       com,
		sessionID: sessionID,
	}

	snapshots, err := com.Workspace.ListSnapshots(context.TODO(), sessionID)
	if err != nil {
		return nil, err
	}
	s.snapshots = snapshots

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	s.help = help

	s.list = list.NewFilterableList(snapshotItems(com.Styles, snapshots)...)
	s.list.Focus()
	s.list.SetSelected(0)

	s.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "restore"),
	)
	s.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next"),
	)
	s.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous"),
	)
	s.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "navigate"),
	)

	closeKey := CloseKey
	closeKey.SetHelp("esc", "close")
	s.keyMap.Close = closeKey

	return s, nil
}

// ID implements Dialog.
func (s *Snapshots) ID() string {
	return SnapshotsID
}

// HandleMsg implements Dialog.
func (s *Snapshots) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, s.keyMap.Previous):
			s.list.Focus()
			if s.list.IsSelectedFirst() {
				s.list.SelectLast()
			} else {
				s.list.SelectPrev()
			}
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Next):
			s.list.Focus()
			if s.list.IsSelectedLast() {
				s.list.SelectFirst()
			} else {
				s.list.SelectNext()
			}
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Select):
			if item := s.list.SelectedItem(); item != nil {
				if snapItem, ok := item.(*snapshotItem); ok {
					return ActionRestoreSnapshot{SnapshotID: snapItem.snapshot.ID}
				}
			}
		}
	}
	return nil
}

// InitialCmd implements Dialog.
func (s *Snapshots) InitialCmd() tea.Cmd {
	return nil
}

// Cursor implements Dialog.
func (s *Snapshots) Cursor() *tea.Cursor {
	return nil
}

// Draw implements Dialog.
func (s *Snapshots) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))

	innerWidth := width - s.com.Styles.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	s.list.SetSize(innerWidth, height-heightOffset)
	s.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Snapshots"
	if len(s.snapshots) == 0 {
		rc.TitleInfo = "No snapshots"
	} else {
		rc.TitleInfo = fmt.Sprintf("%d snapshots", len(s.snapshots))
	}
	listView := t.Dialog.List.Height(s.list.Height()).Render(s.list.Render())
	rc.AddPart(listView)
	rc.Help = s.help.View(s)

	view := rc.Render()
	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements help.KeyMap.
func (s *Snapshots) ShortHelp() []key.Binding {
	return []key.Binding{
		s.keyMap.UpDown,
		s.keyMap.Select,
		s.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (s *Snapshots) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{s.keyMap.Select, s.keyMap.Next, s.keyMap.Previous},
		{s.keyMap.Close},
	}
}

// snapshotItem represents a snapshot in the list.
type snapshotItem struct {
	t        *styles.Styles
	snapshot *checkpoint.Snapshot
	m        fuzzy.Match
	cache    map[int]string
	focused  bool
}

var _ ListItem = &snapshotItem{}

// Filter returns the filterable value of the snapshot.
func (i *snapshotItem) Filter() string {
	if i.snapshot.Description != "" {
		return i.snapshot.Description
	}
	return i.snapshot.ID
}

// ID returns the unique identifier of the snapshot.
func (i *snapshotItem) ID() string {
	return i.snapshot.ID
}

// SetMatch sets the fuzzy match for the snapshot item.
func (i *snapshotItem) SetMatch(m fuzzy.Match) {
	i.cache = nil
	i.m = m
}

// SetFocused sets the focus state of the snapshot item.
func (i *snapshotItem) SetFocused(focused bool) {
	if i.focused != focused {
		i.cache = nil
	}
	i.focused = focused
}

// Render returns the string representation of the snapshot item.
func (i *snapshotItem) Render(width int) string {
	desc := i.snapshot.Description
	if desc == "" {
		desc = "Snapshot"
	}

	info := humanize.Time(i.snapshot.CreatedAt)
	styles := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: i.t.Dialog.ListItem.InfoFocused,
	}

	return renderItem(styles, desc, info, i.focused, width, i.cache, &i.m)
}

func snapshotItems(styles *styles.Styles, snapshots []*checkpoint.Snapshot) []list.FilterableItem {
	items := make([]list.FilterableItem, len(snapshots))
	for i, snap := range snapshots {
		items[i] = &snapshotItem{
			t:        styles,
			snapshot: snap,
		}
	}
	return items
}
