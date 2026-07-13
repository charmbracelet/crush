package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

const (
	SourcesID              = "sources"
	sourcesDialogMaxWidth  = 96
	sourcesDialogMaxHeight = 28
)

// Sources manages references attached to the active session.
type Sources struct {
	com           *common.Common
	sources       []session.Source
	help          help.Model
	list          *list.FilterableList
	confirmRemove bool

	keyMap struct {
		Next     key.Binding
		Previous key.Binding
		Add      key.Binding
		View     key.Binding
		Remove   key.Binding
		Confirm  key.Binding
		Cancel   key.Binding
		Close    key.Binding
	}
}

// SourceItem is a selectable session source.
type SourceItem struct {
	*list.Versioned
	source  session.Source
	styles  *styles.Styles
	match   fuzzy.Match
	focused bool
}

var (
	_ Dialog   = (*Sources)(nil)
	_ ListItem = (*SourceItem)(nil)
)

// NewSources creates a per-session source management dialog.
func NewSources(com *common.Common, sources []session.Source) *Sources {
	s := &Sources{com: com, sources: sources}
	s.help = help.New()
	s.help.Styles = com.Styles.DialogHelpStyles()
	s.list = list.NewFilterableList()
	s.list.Focus()

	s.keyMap.Next = key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("down", "next"))
	s.keyMap.Previous = key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("up", "previous"))
	s.keyMap.Add = key.NewBinding(key.WithKeys("a", "n"), key.WithHelp("a", "add"))
	s.keyMap.View = key.NewBinding(key.WithKeys("enter", "v"), key.WithHelp("enter", "view"))
	s.keyMap.Remove = key.NewBinding(key.WithKeys("x", "delete"), key.WithHelp("x", "remove"))
	s.keyMap.Confirm = key.NewBinding(key.WithKeys("y", "enter"), key.WithHelp("y", "confirm"))
	s.keyMap.Cancel = key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n", "cancel"))
	s.keyMap.Close = CloseKey
	s.setItems()
	return s
}

func (s *Sources) ID() string {
	return SourcesID
}

func (s *Sources) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	if s.confirmRemove {
		switch {
		case key.Matches(keyMsg, s.keyMap.Confirm):
			s.confirmRemove = false
			if item := s.selectedItem(); item != nil {
				return ActionSourceRemove{ID: item.source.ID}
			}
		case key.Matches(keyMsg, s.keyMap.Cancel):
			s.confirmRemove = false
		}
		return nil
	}

	switch {
	case key.Matches(keyMsg, s.keyMap.Close):
		return ActionClose{}
	case key.Matches(keyMsg, s.keyMap.Previous):
		if s.list.IsSelectedFirst() {
			s.list.SelectLast()
		} else {
			s.list.SelectPrev()
		}
		s.list.ScrollToSelected()
	case key.Matches(keyMsg, s.keyMap.Next):
		if s.list.IsSelectedLast() {
			s.list.SelectFirst()
		} else {
			s.list.SelectNext()
		}
		s.list.ScrollToSelected()
	case key.Matches(keyMsg, s.keyMap.Add):
		return ActionOpenSourceAdd{}
	case key.Matches(keyMsg, s.keyMap.View):
		if item := s.selectedItem(); item != nil {
			return ActionSourceView{Source: item.source}
		}
	case key.Matches(keyMsg, s.keyMap.Remove):
		if s.selectedItem() != nil {
			s.confirmRemove = true
		}
	}
	return nil
}

func (s *Sources) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(sourcesDialogMaxWidth, area.Dx()-2))
	height := max(0, min(sourcesDialogMaxHeight, area.Dy()-2))
	innerWidth := max(0, width-t.Dialog.View.GetHorizontalFrameSize())

	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() + t.Dialog.View.GetVerticalFrameSize() + 8
	available := max(4, height-heightOffset)
	listHeight := min(10, max(4, available/2))
	s.list.SetSize(innerWidth, listHeight)
	s.list.ScrollToSelected()
	s.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Gap = 1
	rc.Title = "Sources"
	rc.TitleInfo = t.Dialog.ListItem.InfoBlurred.Render(fmt.Sprintf(" %d attached", len(s.sources)))
	rc.AddPart(t.Dialog.Arguments.Description.Render("References belong only to this session and load on demand."))
	if s.confirmRemove {
		rc.AddPart(t.Dialog.Sessions.DeletingMessage.Render("Detach this source? The original file or URL will not be changed."))
	}
	rc.AddPart(t.Dialog.List.Height(listHeight).Render(s.list.Render()))
	rc.AddPart(s.selectedDetail(innerWidth, max(3, available-listHeight)))
	rc.Help = s.help.View(s)
	DrawCenter(scr, area, rc.Render())
	return nil
}

func (s *Sources) ShortHelp() []key.Binding {
	if s.confirmRemove {
		return []key.Binding{s.keyMap.Confirm, s.keyMap.Cancel}
	}
	return []key.Binding{s.keyMap.Add, s.keyMap.View, s.keyMap.Remove, s.keyMap.Close}
}

func (s *Sources) FullHelp() [][]key.Binding {
	return [][]key.Binding{{s.keyMap.Previous, s.keyMap.Next}, s.ShortHelp()}
}

func (s *Sources) setItems() {
	items := make([]list.FilterableItem, 0, len(s.sources))
	for _, source := range s.sources {
		items = append(items, &SourceItem{
			Versioned: list.NewVersioned(),
			source:    source,
			styles:    s.com.Styles,
		})
	}
	s.list.SetItems(items...)
	s.list.SetSelected(0)
	s.list.ScrollToSelected()
}

func (s *Sources) selectedItem() *SourceItem {
	item, _ := s.list.SelectedItem().(*SourceItem)
	return item
}

func (s *Sources) selectedDetail(width, maxLines int) string {
	item := s.selectedItem()
	if item == nil {
		return s.com.Styles.Dialog.HelpView.Render("No sources attached. Press a to add one.")
	}
	source := item.source
	location := source.Location
	if source.Kind == session.SourceKindText {
		location = source.Content
	}
	wrapped := ansi.Wordwrap(strings.TrimSpace(location), max(1, width), "")
	lines := strings.Split(wrapped, "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "...")
	}
	header := fmt.Sprintf("%s | %s", source.Label, source.Kind)
	return s.com.Styles.Dialog.Arguments.Description.Render(header) + "\n" +
		s.com.Styles.Dialog.HelpView.Render(strings.Join(lines, "\n"))
}

func (s *SourceItem) Finished() bool {
	return true
}

func (s *SourceItem) Filter() string {
	return s.source.Label + " " + string(s.source.Kind) + " " + s.source.Location
}

func (s *SourceItem) ID() string {
	return s.source.ID
}

func (s *SourceItem) SetFocused(focused bool) {
	if s.focused == focused {
		return
	}
	s.focused = focused
	s.Bump()
}

func (s *SourceItem) SetMatch(match fuzzy.Match) {
	if sameFuzzyMatch(s.match, match) {
		return
	}
	s.match = match
	s.Bump()
}

func (s *SourceItem) Render(width int) string {
	itemStyles := ListItemStyles{
		ItemBlurred:     s.styles.Dialog.NormalItem,
		ItemFocused:     s.styles.Dialog.SelectedItem,
		InfoTextBlurred: s.styles.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: s.styles.Dialog.ListItem.InfoFocused,
	}
	return renderItem(itemStyles, s.source.Label, string(s.source.Kind), s.focused, width, nil, &s.match)
}
