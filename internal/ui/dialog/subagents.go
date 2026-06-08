package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
)

// SubagentsID is the identifier for the subagents dialog.
const SubagentsID = "subagents"

// SubagentsTab identifies which tab of the subagents dialog is active.
type SubagentsTab int

// Possible tabs in the subagents dialog.
const (
	SubagentsTabRunning SubagentsTab = iota
	SubagentsTabLibrary
)

// Subagents is a dialog that shows running and library subagents.
type Subagents struct {
	com             *common.Common
	tab             SubagentsTab
	parentSessionID string
	runningList     *list.FilterableList
	libraryList     *list.FilterableList
	runningItems    []*RunningSubagentItem
	libraryItems    []*LibrarySubagentItem
	confirmDelete   bool

	keyMap struct {
		Tab           key.Binding
		Next          key.Binding
		Previous      key.Binding
		Enter         key.Binding
		Cancel        key.Binding
		Delete        key.Binding
		Toggle        key.Binding
		ConfirmDelete key.Binding
		CancelDelete  key.Binding
		Close         key.Binding
	}
	help help.Model
}

var _ Dialog = (*Subagents)(nil)

// NewSubagents creates a new [Subagents] dialog. It populates the running tab
// from com.Workspace.RunningSubagents(parentSessionID) and the library tab
// from com.Workspace.AllSubagents().
func NewSubagents(com *common.Common, parentSessionID string) *Subagents {
	s := &Subagents{
		com:             com,
		tab:             SubagentsTabRunning,
		parentSessionID: parentSessionID,
	}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	s.help = h

	// Build running items.
	running := com.Workspace.RunningSubagents(parentSessionID)
	runningFilterable := make([]list.FilterableItem, len(running))
	s.runningItems = make([]*RunningSubagentItem, len(running))
	for i, r := range running {
		item := NewRunningSubagentItem(com.Styles, RunningSubagentItemData{
			ChildSessionID:   r.ChildSessionID,
			Name:             r.Name,
			Color:            r.Color,
			Model:            r.Model,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
		})
		s.runningItems[i] = item
		runningFilterable[i] = item
	}
	s.runningList = list.NewFilterableList(runningFilterable...)
	s.runningList.Focus()
	s.runningList.SetSelected(0)

	// Build library items.
	defs := com.Workspace.AllSubagents()
	libraryFilterable := make([]list.FilterableItem, len(defs))
	s.libraryItems = make([]*LibrarySubagentItem, len(defs))
	for i, d := range defs {
		item := NewLibrarySubagentItem(com.Styles, LibrarySubagentItemData{
			Name:        d.Name,
			Description: d.Description,
			Color:       d.Color,
			FilePath:    d.FilePath,
			Scope:       d.Scope,
			Disabled:    d.Disabled,
		})
		s.libraryItems[i] = item
		libraryFilterable[i] = item
	}
	s.libraryList = list.NewFilterableList(libraryFilterable...)
	s.libraryList.SetSelected(0)

	s.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab", "shift+tab"),
		key.WithHelp("tab", "switch tab"),
	)
	s.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	s.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	s.keyMap.Enter = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	)
	s.keyMap.Cancel = key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "cancel subagent"),
	)
	s.keyMap.Delete = key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	)
	s.keyMap.Toggle = key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "enable/disable"),
	)
	s.keyMap.ConfirmDelete = key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "confirm delete"),
	)
	s.keyMap.CancelDelete = key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n", "cancel delete"),
	)
	s.keyMap.Close = key.NewBinding(
		key.WithKeys("esc", "alt+esc"),
		key.WithHelp("esc", "close"),
	)

	return s
}

// ID implements [Dialog].
func (s *Subagents) ID() string {
	return SubagentsID
}

// ActiveTab returns the currently active tab.
func (s *Subagents) ActiveTab() SubagentsTab {
	return s.tab
}

// IsConfirmingDelete reports whether the dialog is in confirm-delete mode.
func (s *Subagents) IsConfirmingDelete() bool {
	return s.confirmDelete
}

// activeList returns the list for the currently active tab.
func (s *Subagents) activeList() *list.FilterableList {
	if s.tab == SubagentsTabLibrary {
		return s.libraryList
	}
	return s.runningList
}

// HandleMsg implements [Dialog].
func (s *Subagents) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	// In confirm-delete mode, only accept y/n/esc.
	if s.confirmDelete {
		switch {
		case key.Matches(keyMsg, s.keyMap.ConfirmDelete):
			return s.confirmDeleteSelected()
		case key.Matches(keyMsg, s.keyMap.CancelDelete):
			s.confirmDelete = false
		}
		return nil
	}

	switch {
	case key.Matches(keyMsg, s.keyMap.Close):
		return ActionClose{}

	case key.Matches(keyMsg, s.keyMap.Tab):
		s.toggleTab()

	case key.Matches(keyMsg, s.keyMap.Previous):
		l := s.activeList()
		if l.IsSelectedFirst() {
			l.SelectLast()
		} else {
			l.SelectPrev()
		}
		l.ScrollToSelected()

	case key.Matches(keyMsg, s.keyMap.Next):
		l := s.activeList()
		if l.IsSelectedLast() {
			l.SelectFirst()
		} else {
			l.SelectNext()
		}
		l.ScrollToSelected()

	case s.tab == SubagentsTabRunning && key.Matches(keyMsg, s.keyMap.Enter):
		return s.loadSelectedRunning()

	case s.tab == SubagentsTabRunning && key.Matches(keyMsg, s.keyMap.Cancel):
		s.cancelSelectedRunning()

	case s.tab == SubagentsTabLibrary && key.Matches(keyMsg, s.keyMap.Toggle):
		return s.toggleSelectedLibrary()

	case s.tab == SubagentsTabLibrary && key.Matches(keyMsg, s.keyMap.Delete):
		s.enterConfirmDelete()
	}

	return nil
}

// toggleSelectedLibrary flips the enabled/disabled state of the selected
// library item, optimistically dimming/undimming it, and issues a cmd that
// persists the change via the workspace.
func (s *Subagents) toggleSelectedLibrary() Action {
	item := s.libraryList.SelectedItem()
	if item == nil {
		return nil
	}
	li, ok := item.(*LibrarySubagentItem)
	if !ok {
		return nil
	}
	li.data.Disabled = !li.data.Disabled
	li.Bump()
	return ActionCmd{s.setDisabledCmd(li.ID(), li.data.Disabled)}
}

// setDisabledCmd returns a cmd that persists the disabled state for name and
// reports any error back to the program.
func (s *Subagents) setDisabledCmd(name string, disabled bool) tea.Cmd {
	return func() tea.Msg {
		if err := s.com.Workspace.SetSubagentDisabled(name, disabled); err != nil {
			return util.ReportError(err)()
		}
		return nil
	}
}

// toggleTab switches between the Running and Library tabs.
func (s *Subagents) toggleTab() {
	if s.tab == SubagentsTabRunning {
		s.tab = SubagentsTabLibrary
		s.libraryList.Focus()
	} else {
		s.tab = SubagentsTabRunning
		s.runningList.Focus()
	}
}

// loadSelectedRunning returns an [ActionLoadSubagentSession] for the currently
// selected running subagent, or nil if nothing is selected.
func (s *Subagents) loadSelectedRunning() Action {
	item := s.runningList.SelectedItem()
	if item == nil {
		return nil
	}
	ri, ok := item.(*RunningSubagentItem)
	if !ok {
		return nil
	}
	return ActionLoadSubagentSession{SessionID: ri.ID()}
}

// cancelSelectedRunning cancels the currently selected running subagent via
// the workspace and removes it from the list.
func (s *Subagents) cancelSelectedRunning() {
	item := s.runningList.SelectedItem()
	if item == nil {
		return
	}
	ri, ok := item.(*RunningSubagentItem)
	if !ok {
		return
	}
	childID := ri.ID()
	s.com.Workspace.CancelSubagent(childID)
	s.removeRunningItem(childID)
}

// removeRunningItem removes the running item with the given child session ID
// from the list.
func (s *Subagents) removeRunningItem(childID string) {
	var newItems []*RunningSubagentItem
	for _, item := range s.runningItems {
		if item.ID() == childID {
			continue
		}
		newItems = append(newItems, item)
	}
	s.runningItems = newItems
	filterable := make([]list.FilterableItem, len(s.runningItems))
	for i, item := range s.runningItems {
		filterable[i] = item
	}
	s.runningList.SetItems(filterable...)
	s.runningList.SelectFirst()
}

// enterConfirmDelete sets confirm-delete mode for the currently selected
// library item, if it has user scope.
func (s *Subagents) enterConfirmDelete() {
	item := s.libraryList.SelectedItem()
	if item == nil {
		return
	}
	li, ok := item.(*LibrarySubagentItem)
	if !ok {
		return
	}
	if li.data.Scope != "user" {
		return
	}
	s.confirmDelete = true
}

// confirmDeleteSelected issues a delete cmd for the selected library item and
// removes it from the list optimistically.
func (s *Subagents) confirmDeleteSelected() Action {
	s.confirmDelete = false
	item := s.libraryList.SelectedItem()
	if item == nil {
		return nil
	}
	li, ok := item.(*LibrarySubagentItem)
	if !ok {
		return nil
	}
	name := li.ID()
	s.removeLibraryItem(name)
	return ActionCmd{s.deleteSubagentCmd(name)}
}

// deleteSubagentCmd returns a cmd that calls DeleteUserSubagent and reports any
// error back to the program.
func (s *Subagents) deleteSubagentCmd(name string) tea.Cmd {
	return func() tea.Msg {
		if err := s.com.Workspace.DeleteUserSubagent(name); err != nil {
			return util.ReportError(err)()
		}
		return nil
	}
}

// removeLibraryItem removes the library item with the given name from the list.
func (s *Subagents) removeLibraryItem(name string) {
	var newItems []*LibrarySubagentItem
	for _, item := range s.libraryItems {
		if item.ID() == name {
			continue
		}
		newItems = append(newItems, item)
	}
	s.libraryItems = newItems
	filterable := make([]list.FilterableItem, len(s.libraryItems))
	for i, item := range s.libraryItems {
		filterable[i] = item
	}
	s.libraryList.SetItems(filterable...)
	s.libraryList.SelectFirst()
}

// Draw implements [Dialog].
func (s *Subagents) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	listHeight := height - heightOffset
	listWidth := max(0, innerWidth-3)

	l := s.activeList()
	l.SetSize(listWidth, listHeight)
	s.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Subagents"

	// Build tab indicator for title info.
	runningLabel := "Running"
	libraryLabel := "Library"
	var tabInfo string
	if s.tab == SubagentsTabRunning {
		tabInfo = t.Dialog.SelectedItem.Render(runningLabel) + " | " + libraryLabel
	} else {
		tabInfo = runningLabel + " | " + t.Dialog.SelectedItem.Render(libraryLabel)
	}
	rc.TitleInfo = " " + tabInfo

	listView := t.Dialog.List.Height(l.Height()).Render(l.Render())
	rc.AddPart(listView)
	rc.Help = s.help.View(s)

	view := rc.Render()
	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements [help.KeyMap].
func (s *Subagents) ShortHelp() []key.Binding {
	if s.confirmDelete {
		return []key.Binding{
			s.keyMap.ConfirmDelete,
			s.keyMap.CancelDelete,
		}
	}
	if s.tab == SubagentsTabRunning {
		return []key.Binding{
			s.keyMap.Next,
			s.keyMap.Enter,
			s.keyMap.Cancel,
			s.keyMap.Tab,
			s.keyMap.Close,
		}
	}
	return []key.Binding{
		s.keyMap.Next,
		s.keyMap.Toggle,
		s.keyMap.Delete,
		s.keyMap.Tab,
		s.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (s *Subagents) FullHelp() [][]key.Binding {
	bindings := s.ShortHelp()
	var out [][]key.Binding
	for i := 0; i < len(bindings); i += 4 {
		end := min(i+4, len(bindings))
		out = append(out, bindings[i:end])
	}
	return out
}
